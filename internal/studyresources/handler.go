package studyresources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"studsphere/backend/internal/shared/httpx"
	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/sanitize"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxFileSize is the historical cap for the four document/image types. It is
// intentionally unchanged; video lectures use the configurable
// MaxVideoSizeBytes() cap instead (see types.go).
const maxFileSize = 20 * 1024 * 1024 // 20MB

// MaxDescriptionLength caps the stored resource description.
const MaxDescriptionLength = 20000

// sanitizeDescription runs an admin-authored description through the tightened
// rich-text policy (scripts, event handlers, unsafe URLs and the inline style
// attribute are stripped) and caps the stored length.
func sanitizeDescription(input string) string {
	return sanitize.TruncatePlainText(sanitize.RichText(input), MaxDescriptionLength)
}

type Handler struct {
	service *Service
	// gate is the coin entitlement check on downloads. It is a field with a
	// setter rather than a constructor argument so that the modules that do not
	// know anything about coins — and the tests for them — keep constructing this
	// handler the way they always have. See download_gate.go for the port and why
	// it is declared on this side.
	gate DownloadGate
	// playbackGate is the same check on video playback, as a separate port. It is
	// separate because a playback grant is always the video class and the class
	// must not be a caller-chosen argument; see playback_gate.go. It is also
	// separately switchable, which is the point: gates_enabled.video and
	// gates_enabled.study_resource are independent kill switches and a deployment
	// that wants to charge for a video must be able to do so without touching the
	// document route.
	playbackGate PlaybackGate
	// approval is the §5.3 state machine, wired separately and optional. Nil means
	// this build has no moderation, and every approval route answers 500 rather
	// than quietly publishing something.
	approval *ApprovalService
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// WithDownloadGate wires the coin gate. Called once from main.go; a handler
// without it serves downloads ungated, which is the pre-coin behaviour and not
// a failure mode.
func (h *Handler) WithDownloadGate(gate DownloadGate) *Handler {
	h.gate = gate
	return h
}

// WithPlaybackGate wires the coin gate on video playback. Called once from
// main.go, and independently of WithDownloadGate: the two answer to different
// switches, and wiring one must not imply the other.
func (h *Handler) WithPlaybackGate(gate PlaybackGate) *Handler {
	h.playbackGate = gate
	return h
}

// gateClassFor is the coin class a stored row is priced under. A video lecture
// is a video for pricing and allowance purposes; everything else this module
// stores is a document. It mirrors studyResourceClass in internal/coins, and
// there is one test per side that says the two agree — a drift here would
// charge the document price for a video, which no constraint in the schema would
// catch.
func gateClassFor(resource *StudyResource) string {
	if IsVideoType(resource.ResourceType) {
		return GateClassVideo
	}
	return GateClassStudyResource
}

func (h *Handler) ListResources(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := ResourceFilters{
		Search: c.Query("q"),
		Type:   c.Query("type"),
		Course: c.Query("course"),
		Year:   c.Query("year"),
		// The public list never exposes drafts.
		PublishedOnly: true,
	}

	resources, total, err := h.service.GetResources(filters, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch study resources")
		return
	}

	page, limit = normalizePageLimit(page, limit)

	data := gin.H{
		"page":            page,
		"limit":           limit,
		"total":           total,
		"study_resources": resources,
	}
	// Filter facets: distinct years/courses across all PUBLISHED resources
	// (not just the current page), so drafts never surface as a facet.
	// Best-effort — a facet query failure must not break listing.
	years, courses, err := h.service.DistinctFacets()
	if err != nil {
		years, courses = []string{}, []string{}
	}
	data["years"] = years
	data["courses"] = courses

	response.Success(c, http.StatusOK, "Study resources fetched successfully", data)
}

// WithApproval wires the §5.3 state machine. A handler without it serves the
// pre-moderation surface — the admin CRUD only — which is what every existing
// caller and every existing test constructs. See approval.go for why the port is
// allowed to be absent on this side and not on the approve path.
func (h *Handler) WithApproval(approval *ApprovalService) *Handler {
	h.approval = approval
	return h
}

// SubmitResource is POST /api/v1/study-resources — a STUDENT upload for review.
//
// It is the same multipart shape as the admin CreateResource and it reuses the
// same storage path, deliberately: the file policy (type whitelist, 20MB cap,
// object prefix) is one policy and having a second implementation of it would be
// a second set of rules for the same bytes.
//
// What differs is what happens next, and that is the whole feature: the row is
// pending_review, unpublished, and pays nothing. See approval.go.
func (h *Handler) SubmitResource(c *gin.Context) {
	if h.approval == nil {
		response.Error(c, http.StatusInternalServerError, "Submissions are not available")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	var req CreateResourceRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	resourceType, err := NormalizeType(req.ResourceType)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	upload, err := uploadFile(c.Request.Context(), fileHeader, resourceType)
	if err != nil {
		response.Error(c, statusForUploadError(err), err.Error())
		return
	}

	resource := newResourceFromRequest(req, fileHeader, upload, resourceType, userID)
	submitted, err := h.approval.Submit(userID, *resource)
	if err != nil {
		// The upload succeeded and the row did not, so the object is an orphan.
		_ = storage.DeleteObject(upload.ObjectPath)
		response.Error(c, http.StatusInternalServerError, "Failed to submit resource")
		return
	}

	// 202, not 201: nothing is created in the sense the student cares about. The
	// file exists, and it is not available to anyone yet. 201 would say "here is
	// your resource" for something they cannot open.
	response.Success(c, http.StatusAccepted, "Resource submitted for review", submitted)
}

// MyUploads is GET /api/v1/study-resources/mine — the student's own submissions
// with their approval state.
//
// Needed because "awaiting review" is otherwise unanswerable, and a student who
// cannot tell whether their upload landed will upload it again.
func (h *Handler) MyUploads(c *gin.Context) {
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// Filtered by UploaderID from the session, never from a query parameter: a
	// caller who could name the uploader would be reading somebody else's list.
	resources, total, err := h.service.GetResources(ResourceFilters{UploaderID: userID}, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch your uploads")
		return
	}
	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Your uploads", gin.H{
		"page": page, "limit": limit, "total": total, "study_resources": resources,
	})
}

// PendingReviewQueue is GET /api/v1/admin/study-resources/pending — the §5.3
// moderation queue.
//
// Registered BEFORE the `:id` routes on purpose: gin would otherwise match
// "pending" against the `:id` parameter and answer with a not-found for the queue.
func (h *Handler) PendingReviewQueue(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	resources, total, err := h.service.GetResources(ResourceFilters{ApprovalStatus: ApprovalPending}, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch the review queue")
		return
	}
	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Pending review queue", gin.H{
		"page": page, "limit": limit, "total": total, "study_resources": resources,
	})
}

// ApproveResource is POST /api/v1/admin/study-resources/:id/approve.
//
// Publishes and pays in one step. The response reports the amount credited, read
// from the ledger, because an admin who has just moved a student's balance wants
// to see the figure rather than trust that something happened.
func (h *Handler) ApproveResource(c *gin.Context) {
	if h.approval == nil {
		response.Error(c, http.StatusInternalServerError, "Approval is not available")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}
	reviewerID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	resource, err := h.approval.Approve(uint(id), reviewerID)
	if err != nil {
		writeApprovalError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Resource approved and published", resource)
}

// RejectResource is POST /api/v1/admin/study-resources/:id/reject.
//
// A reason is required (approval.go). The 400 for a missing reason is deliberate
// rather than a validation nicety: without one the student is told nothing they can
// act on and will simply resubmit.
func (h *Handler) RejectResource(c *gin.Context) {
	if h.approval == nil {
		response.Error(c, http.StatusInternalServerError, "Approval is not available")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}
	reviewerID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req RejectResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	resource, err := h.approval.Reject(uint(id), reviewerID, req.RejectReason)
	if err != nil {
		writeApprovalError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Resource rejected", resource)
}

// writeApprovalError maps the state machine's refusals onto statuses.
//
// 404 for a not-pending row and for a missing one: an admin probing ids learns
// nothing about which ids exist, and this table is the queue they already have
// open. 409 for the unwired-grant case is a bug report rather than a user error,
// which is why it is a distinct status from a 500 — nothing the admin did is wrong,
// but it must not read as a generic outage.
func writeApprovalError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrApprovalNotPending):
		response.Error(c, http.StatusNotFound, "Resource not found")
	case errors.Is(err, ErrApprovalUnconfigured):
		response.Error(c, http.StatusConflict, "Approval is unavailable: the StudsToken award is not connected")
	case errors.Is(err, ErrRejectReasonRequired):
		response.Error(c, http.StatusBadRequest, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "Failed to record the decision")
	}
}

func (h *Handler) AdminListResources(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := ResourceFilters{
		Search: c.Query("q"),
		Type:   c.Query("type"),
		Course: c.Query("course"),
		Year:   c.Query("year"),
	}

	resources, total, err := h.service.GetResources(filters, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch study resources")
		return
	}

	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Study resources fetched successfully", gin.H{
		"page":            page,
		"limit":           limit,
		"total":           total,
		"study_resources": resources,
	})
}

// GetResource serves the PUBLIC detail route. Only published rows are
// reachable, so a draft cannot be read by id; the admin list and the admin
// write verbs keep using the unfiltered service lookup.
func (h *Handler) GetResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}
	resource, err := h.service.GetPublishedResource(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Resource not found")
		return
	}
	response.Success(c, http.StatusOK, "Resource fetched successfully", resource)
}

// DownloadResource serves the PUBLIC download route for the four document
// types (and video downloads). Drafts are 404 here for the same reason as on
// the public detail route.
func (h *Handler) DownloadResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}

	resource, err := h.service.GetPublishedResource(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Resource not found")
		return
	}

	// ── the coin gate ──────────────────────────────────────────────────────
	//
	// It runs HERE, after the publication check and before the download counter,
	// and the order of those three things is the whole contract:
	//
	//   - AFTER the publication check, because a draft is a 404 whether or not
	//     anyone can pay for it. Gating first would answer 402 to a request for
	//     something the public is not allowed to know exists, which both leaks
	//     the draft and leaves a student unable to tell "this does not exist"
	//     from "this costs coins".
	//   - BEFORE the download counter, because a refused download is not a
	//     download. Counting it would report engagement for bytes the student
	//     never received, and would make the counter disagree with the wallet.
	//   - BEFORE object storage, which is the point of the gate: the question is
	//     answered without ever opening the object, so a student who cannot pay
	//     does not get a byte and the bucket is not touched.
	//
	// A nil gate means no gate: the check is skipped entirely rather than
	// defaulting to "refuse", so a deployment that has not wired the economy yet
	// serves files exactly as it always has. That is the same reason the config
	// ships with every gate off — the kill switch is off, and an absent switch is
	// the off position.
	if h.gate != nil {
		userID, _ := httpx.CurrentUserID(c)
		decision := h.gate.AuthorizeDownload(c.Request.Context(), userID,
			gateClassFor(resource), uint64(resource.ID), resource.Title)
		if !decision.Allowed {
			refusal := decision.Refusal
			if refusal == nil {
				// A gate that says no without saying why is a bug in the gate, and
				// answering 500 is the only honest response: inventing a 402 here
				// would tell a student with a full wallet that they cannot afford
				// something.
				refusal = NewGateErrorRefusal(ErrGateFailed)
			}
			c.JSON(refusal.Status, refusal.RefusalBody())
			return
		}
	}

	// Download counting is best-effort and happens before streaming.
	h.service.IncrementDownloads(uint(id))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Legacy rows may store a "/uploads/..." URL-style path; normalize it the
	// same way the stream and delete paths do.
	objectKey := normalizeObjectKey(resource.FilePath)
	reader, info, err := storage.GetWithContext(ctx, objectKey)
	if err != nil {
		response.Error(c, http.StatusNotFound, "File not found")
		return
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	contentType := info.ContentType
	if contentType == "" {
		contentType = resource.MimeType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	filename := resource.FileName
	if filename == "" {
		filename = filepath.Base(objectKey)
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if resource.FileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(resource.FileSize, 10))
	}
	c.DataFromReader(http.StatusOK, resource.FileSize, contentType, reader, nil)
}

func sanitizeFileName(name, ext string) string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	sanitized := b.String()
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return "file" + ext
	}
	return sanitized + ext
}

// storedUpload is the result of a successful upload: what to persist about the
// object, and the metadata that must describe the STORED artifact rather than
// whatever the client sent.
type storedUpload struct {
	ObjectPath  string
	ContentType string
	Size        int64
	// FileName is the display/download name of the stored artifact.
	FileName string
}

// uploadFile validates the upload against the policy of resourceType, stores it
// in object storage and returns the metadata to persist.
//
// The four document types are unchanged: the same whitelist, the same 20MB cap,
// the same object prefix, the client content type and the original file name.
//
// Video lectures are normalized first: the upload is transcoded to a broadly
// playable H.264/AAC MP4, and it is the NORMALIZED artifact that is stored —
// under a private object prefix, with the .mp4 name, video/mp4 content type and
// normalized size. Raw source bytes are never stored. The multipart file header
// must remain unopened by the caller so the multipart form is still readable.
func uploadFile(ctx context.Context, fileHeader *multipart.FileHeader, resourceType string) (storedUpload, error) {
	ext, derivedContentType, needsNormalization, err := validateUploadForType(fileHeader, resourceType)
	if err != nil {
		return storedUpload{}, err
	}

	if needsNormalization {
		return uploadNormalizedVideo(ctx, fileHeader)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return storedUpload{}, errors.New("failed to open uploaded file")
	}
	defer src.Close()

	objectPath := storage.StudyResourcePrefix + uuid.NewString() + "-" + sanitizeFileName(fileHeader.Filename, ext)

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_ = derivedContentType

	if err := storage.Upload(objectPath, src, fileHeader.Size, contentType); err != nil {
		return storedUpload{}, errUploadFailed
	}
	return storedUpload{
		ObjectPath:  objectPath,
		ContentType: contentType,
		Size:        fileHeader.Size,
		FileName:    fileHeader.Filename,
	}, nil
}

// uploadNormalizedVideo transcodes a video upload to a playable MP4 and stores
// that artifact. A missing ffmpeg is surfaced as ErrFFmpegUnavailable so the
// handler can answer 503 instead of persisting unplayable bytes.
func uploadNormalizedVideo(ctx context.Context, fileHeader *multipart.FileHeader) (storedUpload, error) {
	if !utils.FFMPEGAvailable() {
		return storedUpload{}, utils.ErrFFmpegUnavailable
	}

	src, err := fileHeader.Open()
	if err != nil {
		return storedUpload{}, errors.New("failed to open uploaded file")
	}
	defer src.Close()

	normalized, err := utils.NormalizeVideoToMP4(ctx, src)
	if err != nil {
		return storedUpload{}, err
	}
	defer normalized.Cleanup()

	artifact, err := os.Open(normalized.Path)
	if err != nil {
		return storedUpload{}, errUploadFailed
	}
	defer artifact.Close()

	baseName := strings.TrimSuffix(filepath.Base(fileHeader.Filename), filepath.Ext(fileHeader.Filename))
	storedName := sanitizeFileName(baseName, NormalizedVideoExtension)
	objectPath := storage.PrivateVideoPrefix + uuid.NewString() + "-" + storedName

	if err := storage.Upload(objectPath, artifact, normalized.Size, NormalizedVideoContentType); err != nil {
		return storedUpload{}, errUploadFailed
	}

	return storedUpload{
		ObjectPath:  objectPath,
		ContentType: NormalizedVideoContentType,
		Size:        normalized.Size,
		FileName:    storedName,
	}, nil
}

var errUploadFailed = errors.New("failed to upload file")

func (h *Handler) CreateResource(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	var req CreateResourceRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	resourceType, err := NormalizeType(req.ResourceType)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Resolve the uploader before touching object storage, so an unresolvable
	// id costs a 401 instead of a full upload plus ffmpeg transcode and a
	// leftover orphan. This route sits behind Auth (routes.go), so the id is
	// always set in the normal path; the check is a backstop for a route wired
	// without the middleware. Failing is the point: proceeding would record
	// uploaded_by = 0, a wrong owner on a provenance column, instead of
	// surfacing the misconfiguration.
	userID, ok := httpx.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	upload, err := uploadFile(c.Request.Context(), fileHeader, resourceType)
	if err != nil {
		response.Error(c, statusForUploadError(err), err.Error())
		return
	}

	resource := newResourceFromRequest(req, fileHeader, upload, resourceType, userID)
	if err := h.service.CreateResource(resource); err != nil {
		// Clean up the uploaded object if the DB save fails.
		_ = storage.DeleteObject(upload.ObjectPath)
		response.Error(c, http.StatusInternalServerError, "Failed to create resource")
		return
	}

	response.Success(c, http.StatusCreated, "Resource created", resource)
}

// statusForUploadError maps an upload failure onto its HTTP status. A missing
// ffmpeg is a server capability problem (503), not a client mistake: the
// alternative would be storing bytes that cannot be guaranteed playable.
func statusForUploadError(err error) int {
	switch {
	case errors.Is(err, errUploadFailed):
		return http.StatusInternalServerError
	case errors.Is(err, utils.ErrFFmpegUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, utils.ErrVideoNormalizeTimeout), errors.Is(err, utils.ErrVideoNormalizeFailed):
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}

// newResourceFromRequest maps a validated upload plus its request onto the
// model. Descriptions go through the tightened rich-text sanitizer and capped
// length; a missing is_published flag means "published" so a fresh upload
// behaves like every legacy row. FileName/FileSize/MimeType describe the STORED
// artifact, which for video is the normalized MP4 rather than the source
// container the admin uploaded.
func newResourceFromRequest(req CreateResourceRequest, fileHeader *multipart.FileHeader, upload storedUpload, resourceType string, uploadedBy uint) *StudyResource {
	resource := &StudyResource{
		Title:        req.Title,
		Description:  sanitizeDescription(req.Description),
		ResourceType: resourceType,
		Course:       req.Course,
		Year:         req.Year,
		FileName:     upload.FileName,
		FilePath:     upload.ObjectPath,
		FileURL:      "/uploads/" + upload.ObjectPath,
		FileSize:     upload.Size,
		MimeType:     upload.ContentType,
		UploadedBy:   uploadedBy,
		IsPublished:  req.IsPublished == nil || *req.IsPublished,
	}
	// An admin-supplied duration is a convenience hint for the player UI only.
	// It is never trusted for playback: normalization is what guarantees the
	// codecs, so a wrong or missing value cannot break the video.
	if req.DurationSeconds != nil && *req.DurationSeconds > 0 {
		resource.DurationSeconds = *req.DurationSeconds
	}
	return resource
}

// ReplaceResourceFile handles POST /admin/study-resources/:id/file.
// It uploads a NEW object, saves the updated metadata, and then best-effort
// deletes the OLD object so a failed save never loses the original file.
func (h *Handler) ReplaceResourceFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}

	resource, err := h.service.GetResource(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Resource not found")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	// Capture the old object key BEFORE it is overwritten. Legacy rows may
	// hold a "/uploads/..." URL-style path, so normalize it up front for the
	// cleanup delete below.
	oldObjectPath := normalizeObjectKey(resource.FilePath)

	// The stored resource type decides the upload policy: a video lecture is
	// normalized to a playable MP4 under the private prefix, a document keeps
	// the historical extension whitelist and 20MB cap.
	upload, err := uploadFile(c.Request.Context(), fileHeader, resource.ResourceType)
	if err != nil {
		response.Error(c, statusForUploadError(err), err.Error())
		return
	}

	// FileName/FileSize/MimeType describe the STORED artifact (for video: the
	// normalized MP4, not the uploaded source container).
	resource.FileName = upload.FileName
	resource.FilePath = upload.ObjectPath
	resource.FileURL = "/uploads/" + upload.ObjectPath
	resource.FileSize = upload.Size
	resource.MimeType = upload.ContentType

	if err := h.service.UpdateResourceModel(resource); err != nil {
		// Clean up the new object if the DB save fails; the old file remains intact.
		_ = storage.DeleteObject(upload.ObjectPath)
		response.Error(c, http.StatusInternalServerError, "Failed to replace resource file")
		return
	}

	// Best-effort cleanup of the OLD object, only after a successful save.
	// Guard against deleting the new key if the upload collided (it cannot in
	// practice because of the UUID prefix, but stay defensive).
	if oldObjectPath != "" && oldObjectPath != upload.ObjectPath {
		_ = storage.DeleteObject(oldObjectPath)
	}

	response.Success(c, http.StatusOK, "Resource file replaced", resource)
}

func (h *Handler) UpdateResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}
	var req UpdateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.ResourceType != nil {
		normalized, err := NormalizeType(*req.ResourceType)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		// A type change must not orphan the stored object: the file already in
		// object storage has to satisfy the new type's upload policy (a .pdf
		// cannot become a video lecture, an .mp4 cannot become a document).
		current, err := h.service.GetResource(uint(id))
		if err != nil {
			response.Error(c, http.StatusNotFound, "Resource not found")
			return
		}
		currentType, _ := NormalizeType(current.ResourceType)
		if currentType != normalized {
			if err := ValidateTypeChange(current.FileName, current.FilePath, normalized); err != nil {
				response.Error(c, http.StatusBadRequest, err.Error())
				return
			}
		}
		req.ResourceType = &normalized
	}
	if req.Description != nil {
		cleaned := sanitizeDescription(*req.Description)
		req.Description = &cleaned
	}
	if req.DurationSeconds != nil && *req.DurationSeconds < 0 {
		response.Error(c, http.StatusBadRequest, "duration_seconds cannot be negative")
		return
	}
	resource, err := h.service.UpdateResource(uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Resource updated", resource)
}

func (h *Handler) DeleteResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}

	resource, err := h.service.GetResource(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Resource not found")
		return
	}

	// Soft-delete the model first; object cleanup is best-effort afterwards.
	if err := h.service.DeleteResource(uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete resource")
		return
	}

	if resource.FilePath != "" {
		_ = storage.DeleteObject(normalizeObjectKey(resource.FilePath))
	}

	response.Success(c, http.StatusOK, "Resource deleted", nil)
}

// normalizeObjectKey maps legacy stored values to a raw object key. The
// canonical FilePath is the bare MinIO key (e.g. "study-resources/x.pdf"), but
// older rows may hold a "/uploads/..." URL-style path — trim that prefix the
// same way scholarship/service.go does.
func normalizeObjectKey(v string) string {
	if strings.HasPrefix(v, "/uploads/") {
		return v[len("/uploads/"):]
	}
	return v
}

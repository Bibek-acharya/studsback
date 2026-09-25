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
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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

	upload, err := uploadFile(c.Request.Context(), fileHeader, resourceType)
	if err != nil {
		response.Error(c, statusForUploadError(err), err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uploadedBy, _ := userID.(uint)

	resource := newResourceFromRequest(req, fileHeader, upload, resourceType, uploadedBy)
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

package studyresources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

// StreamResource handles GET /api/v1/study-resources/:id/stream for video
// lectures. It serves the stored object inline with HTTP range support so
// <video> can seek without downloading the whole file, and counts the play in
// the resource's Views counter. The four document types keep using
// /:id/download unchanged.
//
// Video bytes are only ever released to a caller holding a playback token that
// is bound to THIS resource. The token is minted by IssuePlaybackToken, which
// sits behind the session Auth middleware: the normal session JWT is not
// accepted here, and an anonymous request never reaches object storage.
func (h *Handler) StreamResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}

	// Authorize BEFORE touching the database or object storage, so an
	// unauthorized caller cannot probe which resource ids exist.
	claims, err := utils.ParsePlaybackToken(c.Query(playbackTokenQueryParam))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "A valid playback token is required to stream this video")
		return
	}
	if claims.ResourceID != uint(id) {
		response.Error(c, http.StatusForbidden, "Playback token is not valid for this resource")
		return
	}

	// Only published, video, stored-object-backed rows are streamable, so
	// drafts never leak publicly.
	resource, err := h.service.GetPlayableVideoResource(uint(id))
	if err != nil {
		if errors.Is(err, ErrNotAVideoResource) {
			response.Error(c, http.StatusBadRequest, "Only video lectures can be streamed")
			return
		}
		response.Error(c, http.StatusNotFound, "Resource not found")
		return
	}

	objectKey := normalizeObjectKey(resource.FilePath)
	if objectKey == "" {
		response.Error(c, http.StatusNotFound, "File not found")
		return
	}

	// No artificial deadline: the request context bounds the stream.
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	reader, info, err := storage.GetWithContext(ctx, objectKey)
	if err != nil {
		response.Error(c, http.StatusNotFound, "File not found")
		return
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	objectSize := int64(0)
	if info != nil {
		objectSize = info.Size
	}
	h.streamVideo(c, resource, reader, objectSize)
}

// streamVideo writes a range-aware inline video response. It is separated from
// StreamResource so the range logic can be exercised without object storage.
func (h *Handler) streamVideo(c *gin.Context, resource *StudyResource, reader io.Reader, objectSize int64) {
	total := objectSize
	if total <= 0 {
		total = resource.FileSize
	}
	if total <= 0 {
		response.Error(c, http.StatusNotFound, "File not found")
		return
	}

	c.Header("Accept-Ranges", "bytes")
	// Video bytes are private to a signed-in user. The object key is replaced
	// with a fresh UUID on every replace, so the artifact itself never changes
	// and a short private cache is safe.
	c.Header("Cache-Control", "private, immutable, max-age=300")
	// Never let a browser sniff a different type out of the bytes.
	c.Header("X-Content-Type-Options", "nosniff")

	// Validate the range BEFORE advertising a video content type: a rejected
	// range must answer 416 without a Content-Type the client would try to play.
	start, end, partial, ok := ParseByteRange(c.GetHeader("Range"), total)
	if !ok {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", total))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		// Flush the status now: nothing is written for a rejected range, and
		// gin would otherwise only emit it when the handler chain unwinds.
		c.Writer.WriteHeaderNow()
		return
	}

	filename := resource.FileName
	if filename == "" {
		filename = resource.FilePath
	}
	c.Header("Content-Type", videoContentType(resource))
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sanitizeHeaderValue(filename)))

	length := end - start + 1
	c.Header("Content-Length", strconv.FormatInt(length, 10))
	if partial {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}

	// Prefer random access (MinIO objects implement io.ReaderAt) so a seek
	// never re-reads from the start; fall back to skipping sequentially for
	// plain streams.
	var body io.Reader
	if randomAccess, ok := reader.(io.ReaderAt); ok {
		body = io.NewSectionReader(randomAccess, start, length)
	} else {
		if start > 0 {
			if _, err := io.CopyN(io.Discard, reader, start); err != nil {
				return
			}
		}
		body = io.LimitReader(reader, length)
	}
	if _, err := io.Copy(c.Writer, body); err != nil {
		// The response is already committed; nothing useful is left to send.
		return
	}

	// View counting is best-effort and only counts the INITIAL full request.
	// A player issues one request per seek, so counting range requests would
	// inflate the counter for a single view.
	if !partial {
		h.service.IncrementViews(resource.ID)
	}
}

// ParseByteRange parses a single-range `Range: bytes=...` header against a
// known object size. It supports "start-end", "start-" and the suffix form
// "-length". The returned partial flag reports whether a 206 is required;
// ok=false means the range is unsatisfiable and the caller must answer 416.
func ParseByteRange(rangeHeader string, total int64) (start, end int64, partial, ok bool) {
	if total <= 0 {
		return 0, 0, false, false
	}

	header := strings.TrimSpace(rangeHeader)
	if header == "" {
		return 0, total - 1, false, true
	}

	const prefix = "bytes="
	if !strings.HasPrefix(strings.ToLower(header), prefix) {
		// Multi-range and unknown units are not supported; fall back to the
		// full object so playback still works.
		return 0, total - 1, false, true
	}
	spec := strings.TrimSpace(header[len(prefix):])
	if strings.Contains(spec, ",") {
		return 0, total - 1, false, true
	}

	dash := strings.Index(spec, "-")
	if dash < 0 {
		return 0, 0, false, false
	}
	startRaw := strings.TrimSpace(spec[:dash])
	endRaw := strings.TrimSpace(spec[dash+1:])

	if startRaw == "" {
		// Suffix range: the last N bytes.
		suffix, err := strconv.ParseInt(endRaw, 10, 64)
		if err != nil || suffix <= 0 {
			return 0, 0, false, false
		}
		if suffix > total {
			suffix = total
		}
		return total - suffix, total - 1, true, true
	}

	parsedStart, err := strconv.ParseInt(startRaw, 10, 64)
	if err != nil || parsedStart < 0 || parsedStart >= total {
		return 0, 0, false, false
	}
	start = parsedStart
	end = total - 1
	if endRaw != "" {
		parsedEnd, err := strconv.ParseInt(endRaw, 10, 64)
		if err != nil {
			return 0, 0, false, false
		}
		if parsedEnd < start {
			return 0, 0, false, false
		}
		if parsedEnd < end {
			end = parsedEnd
		}
	}
	return start, end, true, true
}

// videoContentType resolves the inline content type of a stream. Normalized
// uploads are always .mp4; a row that predates the normalization path is
// resolved from its file extension, and a stored MimeType is only trusted when
// it already is a video/* type, so nothing can be served inline as text/html.
func videoContentType(resource *StudyResource) string {
	ext := strings.ToLower(filepath.Ext(resource.FileName))
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(resource.FilePath))
	}
	if ext == NormalizedVideoExtension {
		return NormalizedVideoContentType
	}
	if mime, ok := videoSourceExtensions[ext]; ok {
		return mime
	}
	if strings.HasPrefix(strings.ToLower(resource.MimeType), "video/") {
		return resource.MimeType
	}
	return NormalizedVideoContentType
}

// sanitizeHeaderValue strips characters that would allow a header injection
// through the stored file name.
func sanitizeHeaderValue(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r == '"' || r == '\\' || r == '\r' || r == '\n':
			b.WriteByte('-')
		case r < 0x20 || r == 0x7f:
			b.WriteByte('-')
		default:
			b.WriteRune(r)
		}
	}
	cleaned := strings.TrimSpace(b.String())
	if cleaned == "" {
		return "video"
	}
	return cleaned
}

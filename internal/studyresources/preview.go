// internal/studyresources/preview.go
//
// The document sample: GET /api/v1/study-resources/:id/preview serves the
// FIRST FEW PAGES of a published PDF inline, so a student can read enough of a
// resource to decide whether it is worth unlocking before any coins move.
//
// ── why a sample and not the file ────────────────────────────────────────────
//
// The download route releases the whole artifact, and the coin gate there is
// what makes that safe. A preview route has the same obligation with none of
// the download route's plumbing: it is public on purpose (a buying decision
// cannot require a purchase), so it must never be a second path to the full
// bytes. Two rules enforce that, and they are the whole security model:
//
//  1. At most PreviewSamplePages pages are served, AND at least one page is
//     always withheld. A two-page document serves its first page; a one-page
//     document serves nothing at all (422), because any sample of a one-page
//     document IS the document. samplePageCount is the pure function both
//     rules live in, and it is what the tests pin.
//  2. Only PDFs are previewable. Office formats, archives and text files have
//     no cheap server-side page extraction, and images cannot be "sampled" —
//     serving an image inline is serving it. Everything that is not a
//     published PDF document gets the same 404 a draft gets, so the route
//     cannot be used to probe which ids exist or what type they are.
//
// The response is Content-Disposition: inline — the opposite of the download
// route's attachment — because the whole point is that the browser renders it
// in a viewer rather than saving it.
//
// ── why no auth, no gate, no counters ────────────────────────────────────────
//
// The sample is a shop window, not the goods: it is deliberately available to
// anonymous visitors exactly as the public metadata list is, it never charges
// and never unlocks (the download route keeps both), and it touches no
// counters — a preview is neither a download nor a view, and counting it
// under either would inflate engagement metrics with purchase indecision.
package studyresources

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/storage"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// PreviewSamplePages is the maximum number of pages the preview route serves.
// Three is enough of a past paper or a notes chapter to judge it by, and few
// enough that the withheld remainder stays worth the unlock.
const PreviewSamplePages = 3

// previewUnavailableCode answers the one non-404 refusal: a PDF too short to
// sample. The frontend keys the "unlock to read it" message off this code.
const previewUnavailableCode = "PREVIEW_UNAVAILABLE"

// The pdfcpu seam, as package-level function values: the handler tests stub
// these to exercise every refusal without parsing real PDFs, and the trim
// test exercises the real ones against the committed fixture.
var (
	pdfPageCount = func(rs io.ReadSeeker) (int, error) {
		return api.PageCount(rs, model.NewDefaultConfiguration())
	}
	pdfTrimPages = func(rs io.ReadSeeker, w io.Writer, pageSelection string) error {
		return api.Trim(rs, w, []string{pageSelection}, model.NewDefaultConfiguration())
	}
)

// samplePageCount applies the two withholding rules: never more than
// PreviewSamplePages, and never the whole document. Zero means "no honest
// sample exists" and maps to 422, not to an empty PDF.
func samplePageCount(totalPages int) int {
	if totalPages <= 1 {
		return 0
	}
	if rest := totalPages - 1; rest < PreviewSamplePages {
		return rest
	}
	return PreviewSamplePages
}

// isPreviewablePDF reports whether the row is a document whose stored file the
// preview can page through. The extension leads (it is set from the validated
// upload), and a stored PDF mime type is trusted only when it is exactly
// application/pdf — nothing else is ever served inline from this route.
func isPreviewablePDF(resource *StudyResource) bool {
	if resource == nil || IsVideoType(resource.ResourceType) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(resource.FileName))
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(resource.FilePath))
	}
	if ext == ".pdf" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(resource.MimeType), "application/pdf")
}

// PreviewResource handles GET /api/v1/study-resources/:id/preview.
func (h *Handler) PreviewResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}

	resource, err := h.service.GetPublishedResource(uint(id))
	if err != nil || !isPreviewablePDF(resource) {
		// One answer for "no such row", "draft", "video" and "not a PDF": the
		// route discloses nothing about which it was.
		response.Error(c, http.StatusNotFound, "Preview not available for this resource")
		return
	}

	objectKey := normalizeObjectKey(resource.FilePath)
	if objectKey == "" {
		response.Error(c, http.StatusNotFound, "File not found")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	reader, _, err := storage.GetWithContext(ctx, objectKey)
	if err != nil {
		response.Error(c, http.StatusNotFound, "File not found")
		return
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	// pdfcpu needs random access, and document uploads are capped at 20MB, so
	// buffering the object once is bounded and simpler than adapting every
	// storage backend to io.ReadSeeker.
	pdfBytes, err := io.ReadAll(reader)
	if err != nil {
		response.Error(c, http.StatusNotFound, "File not found")
		return
	}

	h.servePreview(c, resource, pdfBytes)
}

// servePreview writes the sampled inline response. It is separated from
// PreviewResource — the same split as StreamResource/streamVideo — so the
// page arithmetic, the trim and every refusal are exercised without object
// storage.
func (h *Handler) servePreview(c *gin.Context, resource *StudyResource, pdfBytes []byte) {
	total, err := pdfPageCount(bytes.NewReader(pdfBytes))
	if err != nil || total <= 0 {
		// A stored object that does not parse as a PDF is a data problem, not
		// a client one — but it is still not a preview, so it is the same 404
		// as every other non-previewable row.
		response.Error(c, http.StatusNotFound, "Preview not available for this resource")
		return
	}

	pages := samplePageCount(total)
	if pages == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"code":    previewUnavailableCode,
			"message": "This document is too short to sample. Unlock it to read the whole thing.",
		})
		return
	}

	var out bytes.Buffer
	if err := pdfTrimPages(bytes.NewReader(pdfBytes), &out, fmt.Sprintf("1-%d", pages)); err != nil {
		response.Error(c, http.StatusNotFound, "Preview not available for this resource")
		return
	}

	filename := resource.FileName
	if filename == "" {
		filename = "document.pdf"
	}
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	previewName := sanitizeHeaderValue(base) + "-preview.pdf"

	// The sample is identical for every caller and the object key rotates on
	// file replace, so a short public cache is safe and keeps repeat opens off
	// the trim path.
	c.Header("Cache-Control", "public, max-age=300")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, previewName))
	c.Data(http.StatusOK, "application/pdf", out.Bytes())
}

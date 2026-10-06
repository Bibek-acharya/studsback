// internal/studyresources/preview_test.go
//
// The preview route's contract, without object storage:
//
//   - the page arithmetic never serves the whole document (samplePageCount),
//   - the served sample is a real, smaller PDF (the committed fixtures run
//     through the REAL pdfcpu trim, so the integration the route depends on is
//     what is proven here, not a mock of it),
//   - every refusal — draft, video, non-PDF, undecodable bytes, too-short
//     document — is the one it promised, and none of them touches storage.
package studyresources

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ── the arithmetic ───────────────────────────────────────────────────────────

func TestSamplePageCountNeverServesTheWholeDocument(t *testing.T) {
	cases := []struct {
		total int
		want  int
	}{
		{-1, 0}, // garbage in is a refusal, not a panic
		{0, 0},
		{1, 0}, // a one-page document has no honest sample
		{2, 1},
		{3, 2},
		{4, 3}, // the cap binds from here on
		{5, 3},
		{100, 3},
	}
	for _, tc := range cases {
		if got := samplePageCount(tc.total); got != tc.want {
			t.Errorf("samplePageCount(%d) = %d, want %d", tc.total, got, tc.want)
		}
		if tc.total > 0 && samplePageCount(tc.total) >= tc.total {
			t.Errorf("samplePageCount(%d) served the whole document", tc.total)
		}
	}
}

// ── what counts as previewable ───────────────────────────────────────────────

func TestIsPreviewablePDF(t *testing.T) {
	cases := []struct {
		name     string
		resource *StudyResource
		want     bool
	}{
		{"nil row", nil, false},
		{"pdf by filename", &StudyResource{ResourceType: TypePastQuestions, FileName: "notes.pdf"}, true},
		{"pdf by uppercase extension", &StudyResource{ResourceType: TypePastQuestions, FileName: "NOTES.PDF"}, true},
		{"pdf by stored mime only", &StudyResource{ResourceType: TypePastQuestions, FileName: "notes.bin", MimeType: "application/pdf"}, true},
		{"word document", &StudyResource{ResourceType: TypePastQuestions, FileName: "notes.docx"}, false},
		{"image", &StudyResource{ResourceType: TypePastQuestions, FileName: "scan.png"}, false},
		{"archive", &StudyResource{ResourceType: TypePastQuestions, FileName: "bundle.zip"}, false},
		{"a video is never previewable here", &StudyResource{ResourceType: TypeVideoLectures, FileName: "lecture.pdf", MimeType: "application/pdf"}, false},
	}
	for _, tc := range cases {
		if got := isPreviewablePDF(tc.resource); got != tc.want {
			t.Errorf("%s: isPreviewablePDF = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// ── the serve path, with the real pdfcpu behind it ───────────────────────────

func previewRequest(t *testing.T, h *Handler, resource *StudyResource, pdf []byte) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/1/preview", nil)
	h.servePreview(c, resource, pdf)
	return rec
}

func fixturePDF(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestServePreviewTrimsAFourPageDocumentToTheSample(t *testing.T) {
	resource := &StudyResource{ID: 7, ResourceType: TypePastQuestions, FileName: "physics.pdf"}
	rec := previewRequest(t, NewHandler(nil), resource, fixturePDF(t, "preview-4pages.pdf"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline;") || !strings.Contains(cd, "physics-preview.pdf") {
		t.Errorf("Content-Disposition = %q, want an inline physics-preview.pdf", cd)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("the sample must not be sniffable into another type")
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "public") {
		t.Errorf("Cache-Control = %q, want a public sample cache", cc)
	}

	// The body must be a real PDF of exactly the sample size: this is the
	// assertion that proves "first N pages, rest withheld" rather than trusting
	// the trim call to have meant it.
	pages, err := pdfPageCount(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("the served sample does not parse as a PDF: %v", err)
	}
	if pages != PreviewSamplePages {
		t.Errorf("sample has %d pages, want the configured %d", pages, PreviewSamplePages)
	}
}

func TestServePreviewRefusesADocumentTooShortToSample(t *testing.T) {
	resource := &StudyResource{ID: 8, ResourceType: TypePastQuestions, FileName: "one-pager.pdf"}
	rec := previewRequest(t, NewHandler(nil), resource, fixturePDF(t, "preview-1page.pdf"))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), previewUnavailableCode) {
		t.Errorf("body = %s, want the %s code the frontend keys off", rec.Body.String(), previewUnavailableCode)
	}
}

func TestServePreviewRejectsBytesThatAreNotAPDF(t *testing.T) {
	resource := &StudyResource{ID: 9, ResourceType: TypePastQuestions, FileName: "broken.pdf"}
	rec := previewRequest(t, NewHandler(nil), resource, []byte("this is not a pdf at all"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

func TestServePreviewMapsATrimFailureToNotFound(t *testing.T) {
	original := pdfTrimPages
	t.Cleanup(func() { pdfTrimPages = original })
	pdfTrimPages = func(_ io.ReadSeeker, _ io.Writer, _ string) error {
		return errors.New("boom")
	}

	resource := &StudyResource{ID: 10, ResourceType: TypePastQuestions, FileName: "physics.pdf"}
	rec := previewRequest(t, NewHandler(nil), resource, fixturePDF(t, "preview-4pages.pdf"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

// ── the route, up to the storage boundary ────────────────────────────────────
//
// Storage is not available in tests, so the PDF case is asserted exactly up to
// the object fetch: the 404 there is "File not found", which can only be
// reached AFTER the row resolved as published AND previewable — the ordering
// proof that no refusal leaks ahead of it.

func TestPreviewRouteRefusals(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	pdf := &StudyResource{
		Title: "Physics Past Questions 2081", ResourceType: TypePastQuestions,
		FileName: "physics.pdf", FilePath: "study-resources/physics.pdf",
		FileURL: "/uploads/study-resources/physics.pdf", FileSize: 10,
		MimeType: "application/pdf", IsPublished: true,
	}
	docx := &StudyResource{
		Title: "Word notes", ResourceType: TypePastQuestions,
		FileName: "notes.docx", FilePath: "study-resources/notes.docx",
		FileURL: "/uploads/study-resources/notes.docx", FileSize: 10,
		IsPublished: true,
	}
	draft := &StudyResource{
		Title: "Draft physics", ResourceType: TypePastQuestions,
		FileName: "draft.pdf", FilePath: "study-resources/draft.pdf",
		FileURL: "/uploads/study-resources/draft.pdf", FileSize: 10,
		MimeType: "application/pdf", IsPublished: false,
	}
	repo := NewRepository(db)
	for _, resource := range []*StudyResource{pdf, docx, draft} {
		if err := repo.CreateResource(resource); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(repo))
	r.GET("/api/v1/study-resources/:id/preview", h.PreviewResource)

	path := func(id uint) string {
		return "/api/v1/study-resources/" + strconv.FormatUint(uint64(id), 10) + "/preview"
	}

	if rec := get(t, r, "/api/v1/study-resources/abc/preview"); rec.Code != http.StatusBadRequest {
		t.Errorf("a non-numeric id: status = %d, want 400", rec.Code)
	}
	if rec := get(t, r, path(999999)); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown id: status = %d, want 404", rec.Code)
	}
	if rec := get(t, r, path(draft.ID)); rec.Code != http.StatusNotFound {
		t.Errorf("a draft: status = %d, want 404 — a draft must never preview", rec.Code)
	}
	if rec := get(t, r, path(docx.ID)); rec.Code != http.StatusNotFound {
		t.Errorf("a non-PDF document: status = %d, want 404", rec.Code)
	}
	if rec := get(t, r, path(pdf.ID)); rec.Code != http.StatusNotFound ||
		!strings.Contains(rec.Body.String(), "File not found") {
		t.Errorf("a published PDF with no stored object: status = %d body = %s, want the storage 404",
			rec.Code, rec.Body.String())
	}
}

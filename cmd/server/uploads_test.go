package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/storage"

	"github.com/gin-gonic/gin"
)

// fakeFetch records the keys the public /uploads route asks for, so the test can
// prove that private keys never reach object storage at all.
type fakeFetch struct {
	requested []string
	body      string
}

func (f *fakeFetch) get(key string) (io.Reader, *storage.ObjectInfo, error) {
	f.requested = append(f.requested, key)
	return bytes.NewReader([]byte(f.body)), &storage.ObjectInfo{
		Key:         key,
		ContentType: "application/pdf",
		Size:        int64(len(f.body)),
	}, nil
}

func uploadsRouter(fetch objectFetcher) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/uploads/*filepath", newUploadsHandler(fetch))
	return r
}

// Study-resource objects must not be addressable through the global public
// object route: that would hand out video bytes without the playback gate.
func TestUploadsRejectsStudyResourceObjects(t *testing.T) {
	fetch := &fakeFetch{body: "%PDF-1.4 secret"}
	r := uploadsRouter(fetch.get)

	privateKeys := []string{
		"study-resources/lecture.mp4",
		"study-resources/syllabus.pdf",
		"private/study-resources/lecture.mp4",
		"private/anything.txt",
		// Traversal that would otherwise resolve into a private prefix.
		"scholarship/../private/study-resources/lecture.mp4",
		"../private/x.mp4",
	}

	for _, key := range privateKeys {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/uploads/"+key, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET /uploads/%s = %d, want 404 (private key leaked)", key, rec.Code)
		}
	}

	if len(fetch.requested) != 0 {
		t.Errorf("private keys reached object storage: %v", fetch.requested)
	}
}

// Unrelated assets keep working exactly as before.
func TestUploadsStillServesUnrelatedAssets(t *testing.T) {
	fetch := &fakeFetch{body: "%PDF-1.4 public"}
	r := uploadsRouter(fetch.get)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/uploads/scholarship/documents/brochure.pdf", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "%PDF-1.4 public" {
		t.Errorf("body = %q, want the stored object", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Errorf("content type = %q, want application/pdf", got)
	}
	if len(fetch.requested) != 1 || fetch.requested[0] != "scholarship/documents/brochure.pdf" {
		t.Errorf("requested keys = %v, want the unrelated asset", fetch.requested)
	}

	// The ?dl=1 attachment behavior is preserved.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/uploads/scholarship/documents/brochure.pdf?dl=1", nil))
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="brochure.pdf"` {
		t.Errorf("content-disposition = %q, want an attachment", got)
	}
}

// A prefix that merely starts with the same characters is not private.
func TestUploadsAllowsSimilarLookingPrefixes(t *testing.T) {
	fetch := &fakeFetch{body: "ok"}
	r := uploadsRouter(fetch.get)

	for _, key := range []string{"study-resources-legacy/x.pdf", "privately/x.pdf", "publications/x.pdf"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/uploads/"+key, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET /uploads/%s = %d, want 200 (unrelated asset blocked)", key, rec.Code)
		}
	}
}

// The document download path must stay reachable through the domain API; this
// documents the routing contract the private /uploads gate relies on.
func TestUploadsGateDoesNotAffectDocumentDownloadContract(t *testing.T) {
	// /api/v1/study-resources/:id/download is a separate, published-only
	// route (see internal/studyresources). The global route must therefore not
	// be the way documents are fetched.
	if strings.Contains(storage.StudyResourcePrefix, "private") {
		t.Fatal("study-resource document objects must not move under the private prefix")
	}
	if !strings.HasPrefix(storage.PrivateVideoPrefix, storage.PrivatePrefix) {
		t.Fatal("video objects must live under the private prefix")
	}
}

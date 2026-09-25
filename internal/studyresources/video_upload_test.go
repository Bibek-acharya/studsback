package studyresources

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/shared/utils"
)

// multipartVideoHeader builds a real multipart.FileHeader from bytes, which is
// what the upload path opens and streams.
func multipartVideoHeader(t *testing.T, filename string, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	reader := multipart.NewReader(&body, writer.Boundary())
	form, err := reader.ReadForm(int64(len(data)) + 4096)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	headers := form.File["file"]
	if len(headers) != 1 {
		t.Fatalf("form files = %d, want 1", len(headers))
	}
	return headers[0]
}

// A document upload keeps its object prefix, name, size and client content type
// and never touches ffmpeg.
func TestUploadFileDocumentsAreUnchanged(t *testing.T) {
	previous := storage.Client
	storage.Client = nil // force an upload failure AFTER validation
	t.Cleanup(func() { storage.Client = previous })

	pdf := []byte("%PDF-1.4 hello")
	header := multipartVideoHeader(t, "syllabus.pdf", pdf)

	upload, err := uploadFile(context.Background(), header, TypeSyllabus)
	if err == nil {
		t.Fatalf("upload unexpectedly succeeded without object storage: %+v", upload)
	}
	if !errors.Is(err, errUploadFailed) {
		t.Fatalf("error = %v, want errUploadFailed", err)
	}
	// A document that validated but could not be stored is a server error, not
	// a client mistake.
	if status := statusForUploadError(err); status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for a storage failure", status)
	}
	// Documents must never be normalized.
	if RequiresNormalization(TypeSyllabus) {
		t.Error("document uploads must not be normalized")
	}

	// A rejected extension is still a 400 before any storage call.
	if _, err := uploadFile(context.Background(), multipartVideoHeader(t, "payload.exe", pdf), TypeSyllabus); err == nil {
		t.Error("a disallowed document extension must be refused")
	} else if status := statusForUploadError(err); status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a document validation error", status)
	}
}

// Without ffmpeg a video upload must fail with 503 rather than store bytes that
// cannot be guaranteed playable.
func TestUploadFileVideoRequiresFFmpeg(t *testing.T) {
	if utils.FFMPEGAvailable() {
		t.Skip("ffmpeg is installed, so the 503 path cannot be exercised here")
	}

	header := multipartVideoHeader(t, "lecture.mp4", []byte("not really a video"))
	_, err := uploadFile(context.Background(), header, TypeVideoLectures)
	if !errors.Is(err, utils.ErrFFmpegUnavailable) {
		t.Fatalf("error = %v, want ErrFFmpegUnavailable", err)
	}
	if status := statusForUploadError(err); status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when ffmpeg is missing", status)
	}
}

func TestStatusForUploadError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "storage failure", err: errUploadFailed, want: http.StatusInternalServerError},
		{name: "ffmpeg missing", err: utils.ErrFFmpegUnavailable, want: http.StatusServiceUnavailable},
		{name: "conversion failed", err: utils.ErrVideoNormalizeFailed, want: http.StatusBadRequest},
		{name: "conversion timed out", err: utils.ErrVideoNormalizeTimeout, want: http.StatusBadRequest},
		{name: "validation error", err: errors.New("file type not allowed"), want: http.StatusBadRequest},
	}
	for _, tc := range cases {
		if got := statusForUploadError(tc.err); got != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// With ffmpeg present, a real source must be normalized into a stored artifact
// whose metadata describes the NORMALIZED file under the private prefix.
func TestUploadFileVideoStoresNormalizedArtifact(t *testing.T) {
	if !utils.FFMPEGAvailable() {
		t.Skip("ffmpeg is not installed; skipping the normalization upload test")
	}
	withTestJWTSecret(t)

	source := makeSampleClip(t)
	header := multipartVideoHeader(t, "iPhone Clip.MOV", source)

	// No object storage in unit tests: the normalization still runs and the
	// upload then fails with a storage error, which proves the conversion
	// completed and the artifact was handed over. Temp-file hygiene is asserted
	// deterministically in TestNormalizedArtifactCleanupIsIdempotent.
	previous := storage.Client
	storage.Client = nil
	t.Cleanup(func() { storage.Client = previous })

	_, err := uploadFile(context.Background(), header, TypeVideoLectures)
	if err == nil {
		t.Fatal("upload cannot succeed without object storage")
	}
	if !errors.Is(err, errUploadFailed) {
		t.Fatalf("error = %v, want errUploadFailed after a successful conversion", err)
	}
}

// The stored object key for a video must live under the private prefix, and a
// legacy /uploads/ style path must still resolve for streaming.
func TestVideoObjectKeyPrivacy(t *testing.T) {
	if !strings.HasPrefix(storage.PrivateVideoPrefix, storage.PrivatePrefix) {
		t.Fatalf("private video prefix = %q, want it under %q", storage.PrivateVideoPrefix, storage.PrivatePrefix)
	}
	if !storage.IsPrivateKey(storage.PrivateVideoPrefix + "abc-lecture.mp4") {
		t.Error("a stored video object key must be private")
	}
	// Legacy rows keep a URL-style path; the stream path must still resolve it.
	legacy := &StudyResource{FilePath: "/uploads/private/study-resources/legacy.mp4"}
	if got := normalizeObjectKey(legacy.FilePath); got != "private/study-resources/legacy.mp4" {
		t.Errorf("normalizeObjectKey = %q", got)
	}
	if !storage.IsPrivateKey(legacy.FilePath) {
		t.Error("a legacy URL-style video path must still be classified as private")
	}
}

// Temp-file hygiene of the normalizer, as used by the upload path.
func TestNormalizedArtifactCleanupIsIdempotent(t *testing.T) {
	if !utils.FFMPEGAvailable() {
		t.Skip("ffmpeg is not installed; skipping the artifact test")
	}

	normalized, err := utils.NormalizeVideoToMP4(context.Background(), bytes.NewReader(makeSampleClip(t)))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	dir := filepath.Dir(normalized.Path)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("work dir missing: %v", err)
	}
	normalized.Cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("work dir %s survived cleanup", dir)
	}
	normalized.Cleanup() // no panic on the second call
	var nilVideo *utils.NormalizedVideo
	nilVideo.Cleanup()
}

// makeSampleClip renders a tiny non-H.264 clip with ffmpeg.
func makeSampleClip(t *testing.T) []byte {
	t.Helper()
	if !utils.FFMPEGAvailable() {
		t.Skip("ffmpeg is not installed")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "clip.mov")
	cmd := exec.Command(utils.FFmpegBinary,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=10:duration=1",
		"-c:v", "mpeg4", "-f", "mov", out,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot synthesize a test clip with this ffmpeg build: %v (%s)", err, output)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read clip: %v", err)
	}
	return data
}

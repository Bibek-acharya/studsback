package studyresources

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ------------------------------------------------------------------ fixtures

// legacySchema is the study_resources shape from before video lectures: no
// is_published, duration_seconds or views column.
const legacySchema = `
CREATE TABLE study_resources (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME,
	updated_at DATETIME,
	deleted_at DATETIME,
	title TEXT NOT NULL,
	description TEXT DEFAULT '',
	resource_type TEXT NOT NULL DEFAULT '',
	course TEXT DEFAULT '',
	year TEXT DEFAULT '',
	file_name TEXT NOT NULL,
	file_path TEXT NOT NULL,
	file_url TEXT NOT NULL,
	file_size INTEGER NOT NULL DEFAULT 0,
	mime_type TEXT DEFAULT '',
	downloads INTEGER NOT NULL DEFAULT 0,
	uploaded_by INTEGER NOT NULL DEFAULT 0
)`

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func fileHeader(name string, size int64, contentType string) *multipart.FileHeader {
	return &multipart.FileHeader{
		Filename: name,
		Size:     size,
		Header:   map[string][]string{"Content-Type": {contentType}},
	}
}

// ------------------------------------------------- legacy publish backfill

// A study resource created before the is_published column exists must still be
// publicly visible once the column is added: this is the regression guard for
// the DEFAULT TRUE backfill. The ALTER below is the production DDL (the
// migration adds IF NOT EXISTS for Postgres); SQLite cannot run GORM's
// table-rebuild migrator against this legacy shape, so the column is added the
// same way production does it.
func TestLegacyResourcesRemainPublished(t *testing.T) {
	db := openTestDB(t)
	if err := db.Exec(legacySchema).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	legacyTitles := []string{"Legacy Physics", "Legacy Maths", "Legacy Syllabus"}
	for i, title := range legacyTitles {
		if err := db.Exec(
			`INSERT INTO study_resources (title, description, resource_type, course, year, file_name, file_path, file_url, file_size, mime_type, downloads, uploaded_by)
			 VALUES (?, '', 'Past Questions', 'Physics', '2081', ?, ?, ?, 10, 'application/pdf', 0, 0)`,
			title, "f"+string(rune('a'+i))+".pdf", "study-resources/a.pdf", "/uploads/study-resources/a.pdf",
		).Error; err != nil {
			t.Fatalf("insert legacy row: %v", err)
		}
	}

	for _, stmt := range []string{
		`ALTER TABLE study_resources ADD COLUMN is_published BOOLEAN NOT NULL DEFAULT TRUE`,
		`ALTER TABLE study_resources ADD COLUMN duration_seconds INT NOT NULL DEFAULT 0`,
		`ALTER TABLE study_resources ADD COLUMN views INT NOT NULL DEFAULT 0`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("apply %q: %v", stmt, err)
		}
	}

	var count int64
	if err := db.Model(&StudyResource{}).Where("is_published = ?", true).Count(&count).Error; err != nil {
		t.Fatalf("count published: %v", err)
	}
	if count != int64(len(legacyTitles)) {
		t.Errorf("published legacy rows = %d, want %d", count, len(legacyTitles))
	}

	svc := NewService(NewRepository(db))

	public, total, err := svc.GetResources(ResourceFilters{PublishedOnly: true}, 1, 20)
	if err != nil {
		t.Fatalf("public list error: %v", err)
	}
	if total != int64(len(legacyTitles)) || len(public) != len(legacyTitles) {
		t.Errorf("public list total=%d len=%d, want %d", total, len(public), len(legacyTitles))
	}
	for _, resource := range public {
		if !resource.IsPublished {
			t.Errorf("legacy row %q is not published", resource.Title)
		}
	}

	// The admin list sees everything, published or not.
	if err := NewRepository(db).CreateResource(&StudyResource{
		Title: "Draft syllabus", ResourceType: TypeSyllabus, FileName: "d.pdf",
		FilePath: "study-resources/d.pdf", FileURL: "/uploads/study-resources/d.pdf",
		IsPublished: false,
	}); err != nil {
		t.Fatalf("insert draft: %v", err)
	}
	_, adminTotal, err := svc.GetResources(ResourceFilters{}, 1, 20)
	if err != nil {
		t.Fatalf("admin list error: %v", err)
	}
	if adminTotal != int64(len(legacyTitles))+1 {
		t.Errorf("admin total = %d, want %d", adminTotal, len(legacyTitles)+1)
	}
	_, publicTotal, err := svc.GetResources(ResourceFilters{PublishedOnly: true}, 1, 20)
	if err != nil {
		t.Fatalf("public list error: %v", err)
	}
	if publicTotal != int64(len(legacyTitles)) {
		t.Errorf("public total after draft = %d, want %d", publicTotal, len(legacyTitles))
	}
}

// An explicit is_published=false must survive the insert even though the
// column carries a DEFAULT TRUE tag.
func TestCreateResourcePersistsExplicitDraft(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := NewRepository(db)

	draft := &StudyResource{
		Title: "Draft video", ResourceType: TypeVideoLectures, FileName: "v.mp4",
		FilePath: "study-resources/v.mp4", FileURL: "/uploads/study-resources/v.mp4",
		MimeType: "video/mp4", IsPublished: false,
	}
	if err := repo.CreateResource(draft); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	var reloaded StudyResource
	if err := db.First(&reloaded, draft.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.IsPublished {
		t.Error("explicit draft was published by the column default")
	}
}

// --------------------------------------------------------- type validation

func TestNormalizeType(t *testing.T) {
	cases := map[string]string{
		"past-questions":  TypePastQuestions,
		"Past Questions":  TypePastQuestions,
		"PAST_QUESTIONS":  TypePastQuestions,
		"  study notes ":  TypeStudyNotes,
		"Study Notes":     TypeStudyNotes,
		"model-questions": TypeModelQuestions,
		"Model Questions": TypeModelQuestions,
		"syllabus":        TypeSyllabus,
		"video-lectures":  TypeVideoLectures,
		"Video Lectures":  TypeVideoLectures,
		"video_lecture":   TypeVideoLectures,
		"VIDEO":           TypeVideoLectures,
	}
	for input, want := range cases {
		got, err := NormalizeType(input)
		if err != nil {
			t.Errorf("NormalizeType(%q) error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeTypeRejectsMockTests(t *testing.T) {
	for _, input := range []string{"mock-test", "Mock Test", "mock_test", "mocktest", "MOCK"} {
		if _, err := NormalizeType(input); !errors.Is(err, ErrMockTestNotAStudyResource) {
			t.Errorf("NormalizeType(%q) error = %v, want ErrMockTestNotAStudyResource", input, err)
		}
		if IsValidType(input) {
			t.Errorf("IsValidType(%q) = true, want false", input)
		}
	}
}

func TestNormalizeTypeRejectsUnknown(t *testing.T) {
	for _, input := range []string{"", "   ", "podcast", "past-papers", "mcq"} {
		if got, err := NormalizeType(input); err == nil {
			t.Errorf("NormalizeType(%q) = %q, want error", input, got)
		}
	}
}

// A canonical filter must keep matching rows that were stored with a legacy
// spelling, otherwise existing content disappears from filtered lists.
func TestTypeFilterMatchesLegacyStoredSpellings(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := NewRepository(db)

	stored := []string{"Past Questions", "past-questions", "PAST_QUESTIONS", "Past Questions "}
	for i, resourceType := range stored {
		name := "f" + string(rune('a'+i)) + ".pdf"
		if err := db.Create(&StudyResource{
			Title: "row " + string(rune('a'+i)), ResourceType: resourceType, FileName: name,
			FilePath: "study-resources/" + name, FileURL: "/uploads/study-resources/" + name,
		}).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := db.Create(&StudyResource{
		Title: "notes", ResourceType: "Study Notes", FileName: "n.pdf",
		FilePath: "study-resources/n.pdf", FileURL: "/uploads/study-resources/n.pdf",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	resources, total, err := repo.FindAll(ResourceFilters{Type: TypePastQuestions}, 1, 20)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != int64(len(stored)) {
		t.Errorf("total = %d, want %d (legacy spellings must still match)", total, len(stored))
	}
	if len(resources) != len(stored) {
		t.Errorf("len = %d, want %d", len(resources), len(stored))
	}

	matches := MatchingStoredTypes(TypePastQuestions)
	found := false
	for _, match := range matches {
		if match == "Past Questions" {
			found = true
		}
	}
	if !found {
		t.Errorf("MatchingStoredTypes(%q) = %v, want it to include the legacy spelling", TypePastQuestions, matches)
	}
	if got := MatchingStoredTypes("mock-test"); got != nil {
		t.Errorf("MatchingStoredTypes(mock-test) = %v, want nil", got)
	}
}

func TestRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, func(c *gin.Context) { c.Next() }, nil, NewHandler(NewService(NewRepository(openTestDB(t)))))

	registered := map[string]bool{}
	for _, route := range r.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /api/v1/study-resources",
		"GET /api/v1/study-resources/:id",
		"GET /api/v1/study-resources/:id/download",
		"GET /api/v1/study-resources/:id/playback-token",
		"GET /api/v1/study-resources/:id/stream",
		"POST /api/v1/admin/study-resources",
		"GET /api/v1/admin/study-resources",
		"PUT /api/v1/admin/study-resources/:id",
		"POST /api/v1/admin/study-resources/:id/file",
		"DELETE /api/v1/admin/study-resources/:id",
	} {
		if !registered[want] {
			t.Errorf("missing route %s", want)
		}
	}
}

func TestAllTypesAndVideoDetection(t *testing.T) {
	if len(AllTypes()) != 5 {
		t.Errorf("AllTypes() = %v, want 5 canonical types", AllTypes())
	}
	if !IsVideoType("video-lectures") || !IsVideoType("Video Lectures") {
		t.Error("video-lectures must be detected as a video type")
	}
	for _, other := range []string{TypePastQuestions, TypeStudyNotes, TypeModelQuestions, TypeSyllabus} {
		if IsVideoType(other) {
			t.Errorf("%q must not be a video type", other)
		}
	}
}

// ------------------------------------------------------------ upload policy

func TestUploadPolicyPerType(t *testing.T) {
	docPolicy := UploadPolicyForType(TypeSyllabus)
	if docPolicy.maxSize != maxFileSize {
		t.Errorf("document cap = %d, want %d", docPolicy.maxSize, int64(maxFileSize))
	}
	if len(docPolicy.exts) != len(documentExtensions) {
		t.Errorf("document whitelist changed: %d entries", len(docPolicy.exts))
	}
	for _, ext := range []string{".pdf", ".docx", ".pptx", ".xlsx", ".txt", ".csv", ".zip", ".rar", ".7z", ".jpg", ".jpeg", ".png", ".webp"} {
		if !docPolicy.exts[ext] {
			t.Errorf("document whitelist lost %q", ext)
		}
	}
	if docPolicy.mimeByExt != nil {
		t.Error("document uploads must keep trusting the client content type")
	}
	if docPolicy.normalized {
		t.Error("document uploads must never be transcoded")
	}

	videoPolicy := UploadPolicyForType(TypeVideoLectures)
	if videoPolicy.maxSize != MaxVideoSizeBytes() {
		t.Errorf("video cap = %d, want %d", videoPolicy.maxSize, MaxVideoSizeBytes())
	}
	if videoPolicy.maxSize != 200*1024*1024 {
		t.Errorf("default video cap = %d, want 200MB", videoPolicy.maxSize)
	}
	// Every common camera/container format is accepted as a SOURCE.
	wantSourceExts := []string{".mp4", ".webm", ".mov", ".m4v", ".mkv", ".avi"}
	for _, ext := range wantSourceExts {
		if !videoPolicy.exts[ext] {
			t.Errorf("video source whitelist lost %q", ext)
		}
	}
	if len(videoPolicy.exts) != len(wantSourceExts) {
		t.Errorf("video whitelist = %v, want exactly %v", videoPolicy.exts, wantSourceExts)
	}
	if !videoPolicy.normalized {
		t.Error("video uploads must be normalized to a playable MP4")
	}
	if videoPolicy.storeAsType != NormalizedVideoContentType {
		t.Errorf("stored video type = %q, want %q", videoPolicy.storeAsType, NormalizedVideoContentType)
	}
	if !RequiresNormalization(TypeVideoLectures) {
		t.Error("RequiresNormalization(video-lectures) = false")
	}
	for _, other := range []string{TypePastQuestions, TypeStudyNotes, TypeModelQuestions, TypeSyllabus} {
		if RequiresNormalization(other) {
			t.Errorf("RequiresNormalization(%q) = true, want false", other)
		}
	}
	if got := VideoSourceExtensions(); len(got) != len(wantSourceExts) || got[0] != ".avi" {
		t.Errorf("VideoSourceExtensions() = %v, want a sorted list of %v", got, wantSourceExts)
	}
}

func TestVideoCapIsConfigurable(t *testing.T) {
	previous := config.AppConfig
	t.Cleanup(func() { config.AppConfig = previous })

	if got := MaxVideoSizeBytes(); got != 200*1024*1024 {
		t.Errorf("cap without config = %d, want 200MB", got)
	}
	config.AppConfig = &config.Config{StudyResourceVideoMaxSizeMB: 512}
	if got := MaxVideoSizeBytes(); got != 512*1024*1024 {
		t.Errorf("configured cap = %d, want 512MB", got)
	}
	if got := UploadPolicyForType(TypeVideoLectures).maxSize; got != 512*1024*1024 {
		t.Errorf("policy cap = %d, want 512MB", got)
	}
}

func TestValidateUploadForType(t *testing.T) {
	cases := []struct {
		name         string
		resourceType string
		header       *multipart.FileHeader
		wantExt      string
		wantMIME     string
		wantNorm     bool
		wantErr      string
	}{
		{
			name:         "document keeps client header",
			resourceType: TypeStudyNotes,
			header:       fileHeader("notes.PDF", 1024, "application/pdf"),
			wantExt:      ".pdf",
			wantMIME:     "",
		},
		{
			name:         "video mp4 derives mime and needs normalization",
			resourceType: TypeVideoLectures,
			header:       fileHeader("lecture.MP4", 5*1024*1024, "application/octet-stream"),
			wantExt:      ".mp4",
			wantMIME:     "video/mp4",
			wantNorm:     true,
		},
		{
			name:         "video webm derives mime and ignores spoofed header",
			resourceType: TypeVideoLectures,
			header:       fileHeader("lecture.webm", 1024, "text/html"),
			wantExt:      ".webm",
			wantMIME:     "video/webm",
			wantNorm:     true,
		},
		{
			name:         "mov source accepted",
			resourceType: TypeVideoLectures,
			header:       fileHeader("iphone clip.MOV", 1024, ""),
			wantExt:      ".mov",
			wantMIME:     "video/quicktime",
			wantNorm:     true,
		},
		{
			name:         "m4v source accepted",
			resourceType: TypeVideoLectures,
			header:       fileHeader("clip.m4v", 1024, ""),
			wantExt:      ".m4v",
			wantMIME:     "video/x-m4v",
			wantNorm:     true,
		},
		{
			name:         "mkv source accepted",
			resourceType: TypeVideoLectures,
			header:       fileHeader("clip.mkv", 1024, ""),
			wantExt:      ".mkv",
			wantMIME:     "video/x-matroska",
			wantNorm:     true,
		},
		{
			name:         "avi source accepted",
			resourceType: TypeVideoLectures,
			header:       fileHeader("clip.avi", 1024, ""),
			wantExt:      ".avi",
			wantMIME:     "video/x-msvideo",
			wantNorm:     true,
		},
		{
			name:         "video rejects document extension",
			resourceType: TypeVideoLectures,
			header:       fileHeader("notes.pdf", 1024, "application/pdf"),
			wantErr:      "video lectures must be uploaded as",
		},
		{
			name:         "document rejects video extension",
			resourceType: TypePastQuestions,
			header:       fileHeader("lecture.mp4", 1024, "video/mp4"),
			wantErr:      "file type not allowed",
		},
		{
			name:         "video rejects exe",
			resourceType: TypeVideoLectures,
			header:       fileHeader("payload.exe", 1024, "application/octet-stream"),
			wantErr:      "video lectures must be uploaded as",
		},
		{
			name:         "document over 20MB rejected",
			resourceType: TypeSyllabus,
			header:       fileHeader("big.pdf", maxFileSize+1, "application/pdf"),
			wantErr:      "file size exceeds limit of 20MB",
		},
		{
			name:         "document at exactly 20MB accepted",
			resourceType: TypeSyllabus,
			header:       fileHeader("ok.pdf", maxFileSize, "application/pdf"),
			wantExt:      ".pdf",
		},
		{
			name:         "video over cap rejected",
			resourceType: TypeVideoLectures,
			header:       fileHeader("big.mp4", MaxVideoSizeBytes()+1, "video/mp4"),
			wantErr:      "file size exceeds limit of 200MB",
		},
		{
			name:         "video over document cap accepted",
			resourceType: TypeVideoLectures,
			header:       fileHeader("big.mp4", maxFileSize+1, "video/mp4"),
			wantExt:      ".mp4",
			wantMIME:     "video/mp4",
			wantNorm:     true,
		},
		{
			name:         "no extension rejected",
			resourceType: TypeStudyNotes,
			header:       fileHeader("README", 10, "text/plain"),
			wantErr:      "file has no extension",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext, mime, normalized, err := validateUploadForType(tc.header, tc.resourceType)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ext != tc.wantExt {
				t.Errorf("ext = %q, want %q", ext, tc.wantExt)
			}
			if mime != tc.wantMIME {
				t.Errorf("mime = %q, want %q", mime, tc.wantMIME)
			}
			if normalized != tc.wantNorm {
				t.Errorf("normalized = %v, want %v", normalized, tc.wantNorm)
			}
		})
	}
}

// ------------------------------------------------- create mapping + sanitize

// A normalized video upload persists the STORED artifact metadata: the .mp4
// name, video/mp4, the normalized size and the private object prefix — never
// the source container the admin uploaded.
func TestNewResourceFromRequestUsesStoredArtifactMetadata(t *testing.T) {
	header := fileHeader("iPhone Clip.MOV", 40*1024*1024, "video/quicktime")
	duration := 615
	req := CreateResourceRequest{
		Title:           "Kinematics lecture",
		Description:     `<p>Intro</p><script>alert(1)</script><p onclick="x()">Body</p>`,
		ResourceType:    TypeVideoLectures,
		Course:          "Physics",
		Year:            "2081",
		DurationSeconds: &duration,
	}

	upload := storedUpload{
		ObjectPath:  "private/study-resources/abc-lecture.mp4",
		ContentType: NormalizedVideoContentType,
		Size:        1234,
		FileName:    "lecture.mp4",
	}
	resource := newResourceFromRequest(req, header, upload, TypeVideoLectures, 7)

	if !resource.IsPublished {
		t.Error("a new resource must default to published")
	}
	// The admin-supplied duration is kept as a UI hint only.
	if resource.DurationSeconds != 615 {
		t.Errorf("duration = %d, want the admin-supplied 615", resource.DurationSeconds)
	}
	if resource.MimeType != NormalizedVideoContentType {
		t.Errorf("mime = %q, want %q", resource.MimeType, NormalizedVideoContentType)
	}
	if resource.FileName != "lecture.mp4" {
		t.Errorf("file name = %q, want the normalized artifact name", resource.FileName)
	}
	if resource.FileSize != 1234 {
		t.Errorf("file size = %d, want the normalized artifact size", resource.FileSize)
	}
	if resource.FilePath != "private/study-resources/abc-lecture.mp4" {
		t.Errorf("file path = %q, want the private video prefix", resource.FilePath)
	}
	if !strings.HasPrefix(resource.FilePath, "private/") {
		t.Errorf("video objects must live under a private prefix, got %q", resource.FilePath)
	}
	if resource.UploadedBy != 7 {
		t.Errorf("uploaded_by = %d, want 7", resource.UploadedBy)
	}
	if resource.FileURL != "/uploads/private/study-resources/abc-lecture.mp4" {
		t.Errorf("file_url = %q", resource.FileURL)
	}
	if strings.Contains(resource.Description, "<script") || strings.Contains(resource.Description, "onclick") {
		t.Errorf("description not sanitized: %q", resource.Description)
	}
	if !strings.Contains(resource.Description, "Intro") {
		t.Errorf("description lost content: %q", resource.Description)
	}
}

// The four document types keep their original upload metadata untouched.
func TestNewResourceFromRequestKeepsDocumentMetadata(t *testing.T) {
	header := fileHeader("notes.pdf", 2048, "application/pdf")
	upload := storedUpload{
		ObjectPath:  "study-resources/abc-notes.pdf",
		ContentType: "application/pdf",
		Size:        2048,
		FileName:    "notes.pdf",
	}
	resource := newResourceFromRequest(CreateResourceRequest{
		Title: "Notes", ResourceType: TypeStudyNotes,
	}, header, upload, TypeStudyNotes, 3)

	if resource.FileName != "notes.pdf" || resource.FileSize != 2048 || resource.MimeType != "application/pdf" {
		t.Errorf("document metadata changed: %+v", resource)
	}
	if !strings.HasPrefix(resource.FilePath, "study-resources/") {
		t.Errorf("document objects must keep the study-resources prefix, got %q", resource.FilePath)
	}
}

func TestNewResourceFromRequestHonorsDraftFlag(t *testing.T) {
	draft := false
	req := CreateResourceRequest{Title: "Draft", ResourceType: TypeSyllabus, IsPublished: &draft}
	resource := newResourceFromRequest(req, fileHeader("s.pdf", 10, "application/pdf"), storedUpload{
		ObjectPath:  "study-resources/s.pdf",
		ContentType: "application/pdf",
		Size:        10,
		FileName:    "s.pdf",
	}, TypeSyllabus, 0)
	if resource.IsPublished {
		t.Error("is_published=false must be honored")
	}
}

// -------------------------------------------------------------- video stream

func streamFixture(t *testing.T) (*Handler, *gorm.DB, *StudyResource, []byte) {
	t.Helper()
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	body := []byte("0123456789abcdefghij") // 20 bytes of "video"
	resource := &StudyResource{
		Title:           "Kinematics lecture",
		ResourceType:    TypeVideoLectures,
		FileName:        "lecture.mp4",
		FilePath:        "study-resources/lecture.mp4",
		FileURL:         "/uploads/study-resources/lecture.mp4",
		FileSize:        int64(len(body)),
		MimeType:        "video/mp4",
		DurationSeconds: 20,
		IsPublished:     true,
	}
	if err := db.Create(resource).Error; err != nil {
		t.Fatalf("seed video: %v", err)
	}
	return NewHandler(NewService(NewRepository(db))), db, resource, body
}

func TestStreamVideoFullRequest(t *testing.T) {
	h, db, resource, body := streamFixture(t)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/1/stream", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.streamVideo(c, resource, bytes.NewReader(body), int64(len(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "video/mp4" {
		t.Errorf("content type = %q, want video/mp4", got)
	}
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("accept-ranges = %q, want bytes", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline;") {
		t.Errorf("content-disposition = %q, want inline", got)
	}
	if rec.Body.String() != string(body) {
		t.Errorf("body = %q, want %q", rec.Body.String(), body)
	}

	var refreshed StudyResource
	if err := db.First(&refreshed, resource.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if refreshed.Views != 1 {
		t.Errorf("views = %d, want 1", refreshed.Views)
	}
}

func TestStreamVideoRangeRequests(t *testing.T) {
	h, db, resource, body := streamFixture(t)
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		rangeHeader string
		wantStatus  int
		wantBody    string
		wantRange   string
	}{
		{name: "prefix", rangeHeader: "bytes=0-3", wantStatus: http.StatusPartialContent, wantBody: "0123", wantRange: "bytes 0-3/20"},
		{name: "open ended", rangeHeader: "bytes=15-", wantStatus: http.StatusPartialContent, wantBody: "fghij", wantRange: "bytes 15-19/20"},
		{name: "suffix", rangeHeader: "bytes=-5", wantStatus: http.StatusPartialContent, wantBody: "fghij", wantRange: "bytes 15-19/20"},
		{name: "clamped end", rangeHeader: "bytes=18-999", wantStatus: http.StatusPartialContent, wantBody: "ij", wantRange: "bytes 18-19/20"},
		{name: "unsatisfiable", rangeHeader: "bytes=50-60", wantStatus: http.StatusRequestedRangeNotSatisfiable, wantRange: "bytes */20"},
		{name: "reversed", rangeHeader: "bytes=10-5", wantStatus: http.StatusRequestedRangeNotSatisfiable, wantRange: "bytes */20"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/1/stream", nil)
			c.Request.Header.Set("Range", tc.rangeHeader)
			c.Params = gin.Params{{Key: "id", Value: "1"}}

			h.streamVideo(c, resource, bytes.NewReader(body), int64(len(body)))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantRange != "" && rec.Header().Get("Content-Range") != tc.wantRange {
				t.Errorf("content-range = %q, want %q", rec.Header().Get("Content-Range"), tc.wantRange)
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}

	var refreshed StudyResource
	if err := db.First(&refreshed, resource.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	// Only the INITIAL full request counts a view. Seeks issue one range
	// request each and a rejected range never counts, so none of these six
	// requests may increment the counter.
	if refreshed.Views != 0 {
		t.Errorf("views = %d, want 0: range seeks must not count as views", refreshed.Views)
	}
}

// A rejected range must not advertise a video content type either.
func TestStreamVideoRejectsUnsatisfiableRangeWithoutVideoContentType(t *testing.T) {
	h, db, resource, body := streamFixture(t)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/1/stream", nil)
	c.Request.Header.Set("Range", "bytes=50-60")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.streamVideo(c, resource, bytes.NewReader(body), int64(len(body)))

	if rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("status = %d, want 416", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); strings.HasPrefix(got, "video/") {
		t.Errorf("content-type = %q, want no video content type on a 416", got)
	}
	if got := rec.Header().Get("Content-Disposition"); strings.HasPrefix(got, "inline") {
		t.Errorf("content-disposition = %q, want no inline disposition on a 416", got)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes */20" {
		t.Errorf("content-range = %q, want bytes */20", got)
	}
	// Accept-Ranges stays: it advertises capability, not content.
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("accept-ranges = %q, want bytes", got)
	}

	var refreshed StudyResource
	if err := db.First(&refreshed, resource.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if refreshed.Views != 0 {
		t.Errorf("views = %d, want 0 for a rejected range", refreshed.Views)
	}
}

// One full play plus several seeks must produce exactly one view.
func TestStreamVideoCountsOnlyTheInitialRequest(t *testing.T) {
	h, db, resource, body := streamFixture(t)
	gin.SetMode(gin.TestMode)

	requests := []string{"", "bytes=0-99", "bytes=5-9", "bytes=15-", "bytes=-3", "bytes=0-0"}
	for _, rangeHeader := range requests {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/1/stream", nil)
		if rangeHeader != "" {
			c.Request.Header.Set("Range", rangeHeader)
		}
		c.Params = gin.Params{{Key: "id", Value: "1"}}

		h.streamVideo(c, resource, bytes.NewReader(body), int64(len(body)))
		if rec.Code != http.StatusOK && rec.Code != http.StatusPartialContent {
			t.Fatalf("Range %q: status = %d", rangeHeader, rec.Code)
		}
	}

	var refreshed StudyResource
	if err := db.First(&refreshed, resource.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if refreshed.Views != 1 {
		t.Errorf("views = %d, want 1 for one play with five seeks", refreshed.Views)
	}
}

func TestStreamVideoFallsBackToStoredFileSize(t *testing.T) {
	h, _, resource, body := streamFixture(t)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/1/stream", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	// objectSize 0 forces the FileSize fallback.
	h.streamVideo(c, resource, bytes.NewReader(body), 0)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != string(body) {
		t.Errorf("body = %q, want %q", rec.Body.String(), body)
	}
}

func TestParseByteRange(t *testing.T) {
	cases := []struct {
		header      string
		total       int64
		wantStart   int64
		wantEnd     int64
		wantPartial bool
		wantOK      bool
	}{
		{header: "", total: 100, wantStart: 0, wantEnd: 99, wantOK: true},
		{header: "bytes=0-9", total: 100, wantStart: 0, wantEnd: 9, wantPartial: true, wantOK: true},
		{header: "bytes=90-", total: 100, wantStart: 90, wantEnd: 99, wantPartial: true, wantOK: true},
		{header: "bytes=-10", total: 100, wantStart: 90, wantEnd: 99, wantPartial: true, wantOK: true},
		{header: "bytes=-500", total: 100, wantStart: 0, wantEnd: 99, wantPartial: true, wantOK: true},
		{header: "bytes=100-", total: 100, wantOK: false},
		{header: "bytes=abc", total: 100, wantOK: false},
		{header: "bytes=0-", total: 0, wantOK: false},
		{header: "items=0-1", total: 100, wantStart: 0, wantEnd: 99, wantOK: true},
		{header: "bytes=0-9,20-29", total: 100, wantStart: 0, wantEnd: 99, wantOK: true},
	}

	for _, tc := range cases {
		start, end, partial, ok := ParseByteRange(tc.header, tc.total)
		if ok != tc.wantOK {
			t.Errorf("ParseByteRange(%q, %d) ok = %v, want %v", tc.header, tc.total, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if start != tc.wantStart || end != tc.wantEnd || partial != tc.wantPartial {
			t.Errorf("ParseByteRange(%q, %d) = (%d,%d,%v), want (%d,%d,%v)",
				tc.header, tc.total, start, end, partial, tc.wantStart, tc.wantEnd, tc.wantPartial)
		}
	}
}

// withTestJWTSecret installs a deterministic JWT secret for the duration of a
// test so playback tokens can be minted and parsed.
func withTestJWTSecret(t *testing.T) {
	t.Helper()
	previous := config.AppConfig
	t.Cleanup(func() { config.AppConfig = previous })
	config.AppConfig = &config.Config{
		JWTSecret:                          "test-jwt-secret",
		StudyResourceVideoMaxSizeMB:        200,
		StudyResourceVideoTranscodeTimeout: utils.DefaultVideoTranscodeTimeout,
	}
}

// playbackTokenFor mints a valid playback token for a user/resource pair.
func playbackTokenFor(t *testing.T, userID, resourceID uint) string {
	t.Helper()
	token, expiresAt, err := utils.IssuePlaybackToken(userID, resourceID, utils.DefaultPlaybackTokenTTL)
	if err != nil {
		t.Fatalf("issue playback token: %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("playback token already expired at %s", expiresAt)
	}
	return token
}

// The stream endpoint itself must reject non-video rows and unpublished rows
// before it ever touches object storage — but only after the caller proves it
// holds a valid playback token for that resource.
func TestStreamResourceRejectsNonVideoAndDrafts(t *testing.T) {
	withTestJWTSecret(t)
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	doc := &StudyResource{
		Title: "Syllabus pdf", ResourceType: TypeSyllabus, FileName: "s.pdf",
		FilePath: "study-resources/s.pdf", FileURL: "/uploads/study-resources/s.pdf",
		MimeType: "application/pdf", IsPublished: true,
	}
	draftVideo := &StudyResource{
		Title: "Draft video", ResourceType: TypeVideoLectures, FileName: "v.mp4",
		FilePath: "private/study-resources/v.mp4", FileURL: "/uploads/private/study-resources/v.mp4",
		MimeType: "video/mp4", IsPublished: false,
	}
	if err := db.Create(doc).Error; err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	if err := db.Create(draftVideo).Error; err != nil {
		t.Fatalf("seed draft: %v", err)
	}

	h := NewHandler(NewService(NewRepository(db)))
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name       string
		id         string
		wantStatus int
	}{
		{name: "document is not streamable", id: "1", wantStatus: http.StatusBadRequest},
		{name: "draft video is hidden", id: "2", wantStatus: http.StatusNotFound},
		{name: "unknown id", id: "999", wantStatus: http.StatusNotFound},
		{name: "invalid id", id: "abc", wantStatus: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/"+tc.id+"/stream", nil)
			c.Params = gin.Params{{Key: "id", Value: tc.id}}
			if id, err := strconv.ParseUint(tc.id, 10, 64); err == nil {
				c.Request = httptest.NewRequest(http.MethodGet,
					"/api/v1/study-resources/"+tc.id+"/stream?pt="+playbackTokenFor(t, 7, uint(id)), nil)
				c.Params = gin.Params{{Key: "id", Value: tc.id}}
			}

			h.StreamResource(c)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// Object storage is unavailable in unit tests; guard against a nil client
// panic turning into a 500 instead of a clean 404 when the bucket is down.
func TestStreamResourceMissingObjectReturnsNotFound(t *testing.T) {
	withTestJWTSecret(t)
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	video := &StudyResource{
		Title: "Lecture", ResourceType: TypeVideoLectures, FileName: "v.mp4",
		FilePath: "private/study-resources/v.mp4", FileURL: "/uploads/private/study-resources/v.mp4",
		MimeType: "video/mp4", FileSize: 100, IsPublished: true,
	}
	if err := db.Create(video).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	previous := storage.Client
	storage.Client = nil
	t.Cleanup(func() { storage.Client = previous })

	h := NewHandler(NewService(NewRepository(db)))
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet,
		"/api/v1/study-resources/1/stream?pt="+playbackTokenFor(t, 7, 1), nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.StreamResource(c)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when the object cannot be read", rec.Code)
	}
}

// ------------------------------------------------- public visibility of drafts

// publicHandlerFixture seeds one published document and one unpublished draft
// and returns a router with the real public routes registered.
func publicHandlerFixture(t *testing.T) (*gin.Engine, *gorm.DB, *StudyResource, *StudyResource) {
	t.Helper()
	withTestJWTSecret(t)
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	published := &StudyResource{
		Title: "Published syllabus", ResourceType: TypeSyllabus, FileName: "published.pdf",
		FilePath: "study-resources/published.pdf", FileURL: "/uploads/study-resources/published.pdf",
		FileSize: 10, MimeType: "application/pdf", IsPublished: true,
	}
	draft := &StudyResource{
		Title: "Draft syllabus", ResourceType: TypeSyllabus, FileName: "draft.pdf",
		FilePath: "study-resources/draft.pdf", FileURL: "/uploads/study-resources/draft.pdf",
		FileSize: 10, MimeType: "application/pdf", IsPublished: false,
	}
	for _, resource := range []*StudyResource{published, draft} {
		if err := NewRepository(db).CreateResource(resource); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(NewService(NewRepository(db)))
	r.GET("/api/v1/study-resources/:id", h.GetResource)
	r.GET("/api/v1/study-resources/:id/download", h.DownloadResource)
	r.GET("/api/v1/study-resources/:id/stream", h.StreamResource)
	return r, db, published, draft
}

// An unpublished resource must be unreachable through ANY public path.
func TestPublicPathsHideUnpublishedResource(t *testing.T) {
	withTestJWTSecret(t)
	r, db, published, draft := publicHandlerFixture(t)

	// Metadata paths hide the draft with 404.
	for _, path := range []string{
		"/api/v1/study-resources/" + itoaResource(draft.ID),
		"/api/v1/study-resources/" + itoaResource(draft.ID) + "/download",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 for an unpublished resource (body %s)", path, rec.Code, rec.Body.String())
		}
	}

	// The stream path rejects an anonymous caller outright, and even a token
	// that IS bound to the draft must not unlock it.
	draftToken := playbackTokenFor(t, 7, draft.ID)
	for _, tc := range []struct {
		query      string
		wantStatus int
	}{
		{query: "", wantStatus: http.StatusUnauthorized},
		{query: "?pt=" + draftToken, wantStatus: http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
			"/api/v1/study-resources/"+itoaResource(draft.ID)+"/stream"+tc.query, nil))
		if rec.Code != tc.wantStatus {
			t.Errorf("stream%s = %d, want %d (body %s)", tc.query, rec.Code, tc.wantStatus, rec.Body.String())
		}
		// Only the error envelope may come back — never video bytes.
		if body := rec.Body.String(); body != "" && !strings.Contains(body, `"success":false`) {
			t.Errorf("stream%s returned a non-error body: %s", tc.query, body)
		}
	}

	// The published resource is still served on the detail path.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/study-resources/"+itoaResource(published.ID), nil))
	if rec.Code != http.StatusOK {
		t.Errorf("published detail = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Published syllabus") {
		t.Errorf("published detail lost the title: %s", rec.Body.String())
	}

	// A hidden download must not even bump the download counter.
	var reloaded StudyResource
	if err := db.First(&reloaded, draft.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Downloads != 0 {
		t.Errorf("draft downloads = %d, want 0: a hidden file must not be counted", reloaded.Downloads)
	}
}

// The admin write paths must stay unfiltered: an unpublished row can still be
// replaced and deleted.
func TestAdminPathsStillReachUnpublishedResource(t *testing.T) {
	_, db, _, draft := publicHandlerFixture(t)
	svc := NewService(NewRepository(db))
	h := NewHandler(svc)

	// DeleteResource (admin) uses the unfiltered lookup.
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/study-resources/"+itoaResource(draft.ID), nil)
	c.Params = gin.Params{{Key: "id", Value: itoaResource(draft.ID)}}
	// Object storage is not available in unit tests; the row delete must still
	// happen, so a storage error is irrelevant here.
	previous := storage.Client
	storage.Client = nil
	h.DeleteResource(c)
	storage.Client = previous

	if rec.Code != http.StatusOK {
		t.Fatalf("admin delete of a draft = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if _, err := svc.GetResource(draft.ID); err == nil {
		t.Error("draft still readable after the admin delete")
	}
}

// UpdateResource with a type change must revalidate the STORED file against the
// new type's upload policy.
func TestUpdateResourceTypeChangeValidatesStoredFile(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&StudyResource{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := NewRepository(db)
	svc := NewService(repo)
	h := NewHandler(svc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/v1/admin/study-resources/:id", h.UpdateResource)

	put := func(id uint, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/study-resources/"+itoaResource(id), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec
	}

	// A stored .pdf may become a document category but never a video lecture.
	pdf := &StudyResource{
		Title: "Syllabus", ResourceType: TypeSyllabus, FileName: "syllabus.pdf",
		FilePath: "study-resources/syllabus.pdf", FileURL: "/uploads/study-resources/syllabus.pdf",
		MimeType: "application/pdf", IsPublished: true,
	}
	if err := repo.CreateResource(pdf); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := put(pdf.ID, `{"resource_type":"video-lectures"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("pdf -> video-lectures = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not a valid video lecture") {
		t.Errorf("unclear error message: %s", rec.Body.String())
	}
	if reloaded, _ := svc.GetResource(pdf.ID); reloaded.ResourceType != TypeSyllabus {
		t.Errorf("type changed to %q despite the rejected update", reloaded.ResourceType)
	}

	// A valid document-category change is preserved.
	rec = put(pdf.ID, `{"resource_type":"past-questions"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("pdf -> past-questions = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if reloaded, _ := svc.GetResource(pdf.ID); reloaded.ResourceType != TypePastQuestions {
		t.Errorf("valid type change not persisted: %q", reloaded.ResourceType)
	}

	// A stored .mp4 may become a video lecture, never a document category.
	mp4 := &StudyResource{
		Title: "Lecture", ResourceType: TypeVideoLectures, FileName: "lecture.mp4",
		FilePath: "study-resources/lecture.mp4", FileURL: "/uploads/study-resources/lecture.mp4",
		MimeType: "video/mp4", IsPublished: true,
	}
	if err := repo.CreateResource(mp4); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec = put(mp4.ID, `{"resource_type":"syllabus"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mp4 -> syllabus = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if reloaded, _ := svc.GetResource(mp4.ID); reloaded.ResourceType != TypeVideoLectures {
		t.Errorf("video type changed despite the rejected update: %q", reloaded.ResourceType)
	}

	// Re-submitting the SAME type (or a legacy spelling of it) is a no-op and
	// never blocked, even for a stored file the policy would reject.
	rec = put(mp4.ID, `{"resource_type":"Video Lectures"}`)
	if rec.Code != http.StatusOK {
		t.Errorf("same-type update = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	rec = put(pdf.ID, `{"resource_type":"Syllabus"}`)
	if rec.Code != http.StatusOK {
		t.Errorf("legacy same-type update = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	// An unknown type is still rejected as an invalid type, not a file problem.
	rec = put(pdf.ID, `{"resource_type":"podcast"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("pdf -> podcast = %d, want 400", rec.Code)
	}
}

func TestValidateTypeChange(t *testing.T) {
	cases := []struct {
		name       string
		fileName   string
		objectKey  string
		newType    string
		wantErr    bool
		wantDetail string
	}{
		{name: "pdf to document category", fileName: "a.pdf", objectKey: "study-resources/a.pdf", newType: TypeStudyNotes},
		{name: "pdf to video", fileName: "a.pdf", newType: TypeVideoLectures, wantErr: true, wantDetail: "not a valid video lecture"},
		{name: "mp4 to video", fileName: "a.mp4", newType: TypeVideoLectures},
		{name: "webm to video", fileName: "a.webm", newType: TypeVideoLectures},
		{name: "mp4 to document category", fileName: "a.mp4", newType: TypeSyllabus, wantErr: true, wantDetail: "not allowed for this category"},
		{name: "extension from the object key", fileName: "", objectKey: "study-resources/a.pdf", newType: TypeVideoLectures, wantErr: true},
		{name: "no extension anywhere is not blocked", fileName: "", objectKey: "", newType: TypeVideoLectures},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTypeChange(tc.fileName, tc.objectKey, tc.newType)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error")
				}
				if !strings.Contains(err.Error(), tc.wantDetail) {
					t.Errorf("error = %v, want it to mention %q", err, tc.wantDetail)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func itoaResource(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func TestVideoContentTypeDerivesFromExtension(t *testing.T) {
	cases := []struct {
		name     string
		resource StudyResource
		want     string
	}{
		{
			name:     "mp4 by extension",
			resource: StudyResource{FileName: "lecture.mp4", FilePath: "study-resources/x.mp4", MimeType: "video/mp4"},
			want:     "video/mp4",
		},
		{
			name:     "webm by extension overrides a stale stored mime",
			resource: StudyResource{FileName: "lecture.webm", FilePath: "study-resources/x.webm", MimeType: "video/mp4"},
			want:     "video/webm",
		},
		{
			name:     "spoofed stored mime is never served inline",
			resource: StudyResource{FileName: "lecture.mp4", FilePath: "study-resources/x.mp4", MimeType: "text/html"},
			want:     "video/mp4",
		},
		{
			name:     "unknown extension falls back to a video type",
			resource: StudyResource{FileName: "lecture.bin", FilePath: "study-resources/x.bin", MimeType: "application/octet-stream"},
			want:     "video/mp4",
		},
		{
			name:     "stored video mime is kept for unknown extensions",
			resource: StudyResource{FileName: "lecture", FilePath: "study-resources/x", MimeType: "video/webm"},
			want:     "video/webm",
		},
	}
	for _, tc := range cases {
		if got := videoContentType(&tc.resource); got != tc.want {
			t.Errorf("%s: videoContentType = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeHeaderValue(t *testing.T) {
	cases := map[string]string{
		"lecture.mp4":          "lecture.mp4",
		"bad\"name\r\n.mp4":    "bad-name--.mp4",
		"":                     "video",
		"a\nb":                 "a-b",
		"quote\"injection.mp4": "quote-injection.mp4",
	}
	for in, want := range cases {
		if got := sanitizeHeaderValue(in); got != want {
			t.Errorf("sanitizeHeaderValue(%q) = %q, want %q", in, got, want)
		}
	}
}

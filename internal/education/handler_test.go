package education

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Course{}, &Affiliation{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedHandlerData(db *gorm.DB) {
	aff := Affiliation{Name: "Tribhuvan University"}
	db.Create(&aff)

	courses := []Course{
		{Title: "BSc CS", Level: "Bachelor", AffiliationID: &aff.ID, IsGlobal: true, Status: "published", Field: "Science", Duration: "4 years"},
		{Title: "+2 Science", Level: "+2", IsGlobal: true, Status: "published", Field: "Science", Duration: "2 years"},
		{Title: "MBA", Level: "Master", AffiliationID: &aff.ID, IsGlobal: true, Status: "published", Field: "Management", Duration: "2 years"},
		{Title: "BA", Level: "Bachelor", IsGlobal: true, Status: "published", Field: "Arts", Duration: "3 years"},
	}
	db.Create(&courses)
}

func setupHandlerRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/education/courses/by-level/:level", h.GetCoursesByLevel)
	r.GET("/api/v1/education/courses/by-affiliation/:id", h.GetCoursesByAffiliation)
	r.GET("/api/v1/education/courses/secondary", h.GetSecondaryCourses)
	return r
}

func TestGetCoursesByLevel(t *testing.T) {
	db := setupHandlerTestDB(t)
	seedHandlerData(db)
	service := NewService(NewRepository(db), (*testInstProgramRepo)(nil), nil)
	h := NewHandler(service)
	router := setupHandlerRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/education/courses/by-level/Bachelor", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Courses []Course    `json:"courses"`
			Meta    interface{} `json:"meta"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatal("expected success=true")
	}
	if len(body.Data.Courses) != 2 {
		t.Fatalf("courses count = %d, want 2", len(body.Data.Courses))
	}
}

func TestGetCoursesByLevelPagination(t *testing.T) {
	db := setupHandlerTestDB(t)
	seedHandlerData(db)
	service := NewService(NewRepository(db), (*testInstProgramRepo)(nil), nil)
	h := NewHandler(service)
	router := setupHandlerRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/education/courses/by-level/Bachelor?page=1&limit=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Courses []Course `json:"courses"`
			Meta    struct {
				Total int64 `json:"total"`
				Page  int   `json:"page"`
				Limit int   `json:"limit"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Courses) != 1 {
		t.Fatalf("courses count = %d, want 1", len(body.Data.Courses))
	}
	if body.Data.Meta.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Data.Meta.Total)
	}
}

func TestGetCoursesByAffiliation(t *testing.T) {
	db := setupHandlerTestDB(t)
	seedHandlerData(db)
	service := NewService(NewRepository(db), (*testInstProgramRepo)(nil), nil)
	h := NewHandler(service)
	router := setupHandlerRouter(h)

	var aff Affiliation
	db.First(&aff)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/education/courses/by-affiliation/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Courses []Course `json:"courses"`
			Meta    struct {
				Total int64 `json:"total"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatal("expected success=true")
	}
	if len(body.Data.Courses) != 2 {
		t.Fatalf("courses count = %d, want 2", len(body.Data.Courses))
	}
}

func TestGetCoursesByAffiliationInvalidID(t *testing.T) {
	db := setupHandlerTestDB(t)
	service := NewService(NewRepository(db), (*testInstProgramRepo)(nil), nil)
	h := NewHandler(service)
	router := setupHandlerRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/education/courses/by-affiliation/abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetSecondaryCourses(t *testing.T) {
	db := setupHandlerTestDB(t)
	seedHandlerData(db)
	service := NewService(NewRepository(db), (*testInstProgramRepo)(nil), nil)
	h := NewHandler(service)
	router := setupHandlerRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/education/courses/secondary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Courses []Course    `json:"courses"`
			Meta    interface{} `json:"meta"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatal("expected success=true")
	}
	// Courses with no affiliation (BA and +2 Science)
	if len(body.Data.Courses) != 2 {
		t.Fatalf("courses count = %d, want 2", len(body.Data.Courses))
	}
}

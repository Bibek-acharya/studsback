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

func setupBlogBySlugTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Blog{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	blogs := []Blog{
		{Title: "Published Post", Slug: "published-post", Category: "Tech", Published: true},
		{Title: "Draft Post", Slug: "draft-post", Category: "Tech", Published: true},
	}
	for i := range blogs {
		if err := db.Create(&blogs[i]).Error; err != nil {
			t.Fatalf("seed blog: %v", err)
		}
	}
	// Published:false is a zero value, so the model's default:true tag wins on
	// create — force the draft row down afterwards.
	if err := db.Model(&Blog{}).Where("slug = ?", "draft-post").
		Update("published", false).Error; err != nil {
		t.Fatalf("mark draft unpublished: %v", err)
	}

	service := NewService(NewRepository(db), (*testInstProgramRepo)(nil), nil, nil)
	h := NewHandler(service)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Register the same route pair used by RegisterRoutes — this also
	// guards against Gin panicking on /blogs/:id vs /blogs/by-slug/:slug.
	r.GET("/api/v1/education/blogs/:id", h.GetEducationBlogByID)
	r.GET("/api/v1/education/blogs/by-slug/:slug", h.GetBlogBySlug)
	return r, db
}

func TestGetBlogBySlugFound(t *testing.T) {
	router, _ := setupBlogBySlugTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/education/blogs/by-slug/published-post", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Blog struct {
				ID      uint   `json:"id"`
				Title   string `json:"title"`
				Slug    string `json:"slug"`
				Publish bool   `json:"published"`
			} `json:"blog"`
			Related []BlogResponse `json:"related"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatal("expected success=true")
	}
	if body.Data.Blog.Slug != "published-post" {
		t.Errorf("slug = %q, want %q", body.Data.Blog.Slug, "published-post")
	}
	if body.Data.Blog.Title != "Published Post" {
		t.Errorf("title = %q, want %q", body.Data.Blog.Title, "Published Post")
	}
	if body.Data.Blog.ID == 0 {
		t.Error("expected numeric blog id in response")
	}
	if body.Data.Related == nil {
		t.Error("expected related array in response")
	}
}

func TestGetBlogBySlugNotFound(t *testing.T) {
	router, _ := setupBlogBySlugTest(t)

	for _, slug := range []string{"missing-slug", "draft-post"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/education/blogs/by-slug/"+slug, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("slug %q: status = %d, want %d", slug, rec.Code, http.StatusNotFound)
		}

		var body struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Success {
			t.Errorf("slug %q: expected success=false", slug)
		}
		if body.Error != "Blog not found" {
			t.Errorf("slug %q: error = %q, want %q", slug, body.Error, "Blog not found")
		}
	}
}

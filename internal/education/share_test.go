package education

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupShareTest builds an in-memory router exposing both share endpoints,
// seeded with one published blog and one news article.
func setupShareTest(t *testing.T) (*gin.Engine, *gorm.DB, uint, uint) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Blog{}, &News{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	blog := Blog{Title: "Shareable Post", Slug: "shareable-post", Category: "Tech", Published: true}
	if err := db.Create(&blog).Error; err != nil {
		t.Fatalf("seed blog: %v", err)
	}

	news := News{Title: "Shareable News", Category: "Tech", Published: true}
	if err := db.Create(&news).Error; err != nil {
		t.Fatalf("seed news: %v", err)
	}

	service := NewService(NewRepository(db), (*testInstProgramRepo)(nil), nil, nil)
	h := NewHandler(service)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Same routes as RegisterRoutes — guards against :id/share vs :id/view
	// wildcard conflicts in Gin.
	r.POST("/api/v1/education/blogs/:id/share", h.IncrementBlogShare)
	r.POST("/api/v1/education/news/:id/share", h.IncrementNewsShare)
	return r, db, blog.ID, news.ID
}

func postShare(t *testing.T, r *gin.Engine, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, url, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decodeShareBody(t *testing.T, rec *httptest.ResponseRecorder) (bool, int, string) {
	t.Helper()
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Shares int `json:"shares"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body.Success, body.Data.Shares, body.Error
}

func TestIncrementBlogShare(t *testing.T) {
	router, _, blogID, _ := setupShareTest(t)
	url := fmt.Sprintf("/api/v1/education/blogs/%d/share", blogID)

	for want := 1; want <= 2; want++ {
		rec := postShare(t, router, url)
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d: status = %d, want %d; body = %s", want, rec.Code, http.StatusOK, rec.Body.String())
		}

		success, shares, _ := decodeShareBody(t, rec)
		if !success {
			t.Fatalf("call %d: expected success=true", want)
		}
		if shares != want {
			t.Errorf("call %d: shares = %d, want %d", want, shares, want)
		}
	}
}

func TestIncrementBlogShareNotFound(t *testing.T) {
	router, _, _, _ := setupShareTest(t)

	for _, id := range []string{"9999", "not-a-number"} {
		rec := postShare(t, router, "/api/v1/education/blogs/"+id+"/share")
		if rec.Code != http.StatusNotFound {
			t.Errorf("id %q: status = %d, want %d", id, rec.Code, http.StatusNotFound)
		}

		success, _, errMsg := decodeShareBody(t, rec)
		if success {
			t.Errorf("id %q: expected success=false", id)
		}
		if errMsg != "Blog not found" {
			t.Errorf("id %q: error = %q, want %q", id, errMsg, "Blog not found")
		}
	}
}

func TestIncrementNewsShare(t *testing.T) {
	router, _, _, newsID := setupShareTest(t)
	url := fmt.Sprintf("/api/v1/education/news/%d/share", newsID)

	for want := 1; want <= 2; want++ {
		rec := postShare(t, router, url)
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d: status = %d, want %d; body = %s", want, rec.Code, http.StatusOK, rec.Body.String())
		}

		success, shares, _ := decodeShareBody(t, rec)
		if !success {
			t.Fatalf("call %d: expected success=true", want)
		}
		if shares != want {
			t.Errorf("call %d: shares = %d, want %d", want, shares, want)
		}
	}
}

func TestIncrementNewsShareNotFound(t *testing.T) {
	router, _, _, _ := setupShareTest(t)

	for _, id := range []string{"9999", "not-a-number"} {
		rec := postShare(t, router, "/api/v1/education/news/"+id+"/share")
		if rec.Code != http.StatusNotFound {
			t.Errorf("id %q: status = %d, want %d", id, rec.Code, http.StatusNotFound)
		}

		success, _, errMsg := decodeShareBody(t, rec)
		if success {
			t.Errorf("id %q: expected success=false", id)
		}
		if errMsg != "News not found" {
			t.Errorf("id %q: error = %q, want %q", id, errMsg, "News not found")
		}
	}
}

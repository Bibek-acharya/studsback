package pressmedia

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// These tests cover publication gating on the public detail route.
//
// The module had no test file at all, which is why an unpublished item was
// readable by id without anyone noticing. GetItem is shared with the admin
// route on purpose, so the regression to guard is route-level: the public path
// must use the published-only accessor.

func newPressMediaTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&PressMediaItem{}); err != nil {
		t.Fatalf("auto migrate pressmedia: %v", err)
	}
	return NewService(NewRepository(db)), db
}

func seedPressMediaItem(t *testing.T, db *gorm.DB, slug string, published bool) *PressMediaItem {
	t.Helper()

	item := &PressMediaItem{
		Title:       slug,
		Slug:        slug,
		Category:    "press",
		IsPublished: published,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("seed press media item: %v", err)
	}
	return item
}

func TestGetPublishedItemReturnsPublishedItem(t *testing.T) {
	svc, db := newPressMediaTestService(t)
	item := seedPressMediaItem(t, db, "published-item", true)

	got, err := svc.GetPublishedItem(item.ID)
	if err != nil {
		t.Fatalf("GetPublishedItem on a published item: %v", err)
	}
	if got.ID != item.ID {
		t.Fatalf("ID = %d, want %d", got.ID, item.ID)
	}
}

func TestGetPublishedItemRejectsUnpublishedItem(t *testing.T) {
	svc, db := newPressMediaTestService(t)
	item := seedPressMediaItem(t, db, "draft-item", false)

	if _, err := svc.GetPublishedItem(item.ID); !errors.Is(err, ErrItemNotPublished) {
		t.Fatalf("err = %v, want ErrItemNotPublished", err)
	}
}

func TestGetPublishedItemMissingIDIsNotReportedAsUnpublished(t *testing.T) {
	svc, _ := newPressMediaTestService(t)

	_, err := svc.GetPublishedItem(4242)
	if err == nil {
		t.Fatal("expected an error for a missing id")
	}
	// Both cases surface as 404 to the caller, but the error types stay distinct
	// so a real lookup failure is not silently read as "it's a draft".
	if errors.Is(err, ErrItemNotPublished) {
		t.Fatalf("a missing id must not report as unpublished, got %v", err)
	}
}

// Guards the fix against over-reach: the admin detail route shares GetItem and
// must keep seeing drafts in order to edit them.
func TestGetItemStillReturnsUnpublishedForAdmin(t *testing.T) {
	svc, db := newPressMediaTestService(t)
	item := seedPressMediaItem(t, db, "admin-visible-draft", false)

	got, err := svc.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem must still return drafts for the admin edit path: %v", err)
	}
	if got.ID != item.ID {
		t.Fatalf("ID = %d, want %d", got.ID, item.ID)
	}
}

func TestPublicDetailRouteHidesUnpublishedItem(t *testing.T) {
	svc, db := newPressMediaTestService(t)
	draft := seedPressMediaItem(t, db, "route-hidden-draft", false)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(
		r,
		func(c *gin.Context) { c.Next() },
		func(c *gin.Context) { c.Abort() },
		NewHandler(svc),
	)

	rec := httptest.NewRecorder()
	url := "/api/v1/media-press/" + strconv.FormatUint(uint64(draft.ID), 10)
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an unpublished item", rec.Code)
	}
}

func TestPublicDetailRouteRejectsMissingID(t *testing.T) {
	svc, _ := newPressMediaTestService(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(
		r,
		func(c *gin.Context) { c.Next() },
		func(c *gin.Context) { c.Abort() },
		NewHandler(svc),
	)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/media-press/4242", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a missing id", rec.Code)
	}
}

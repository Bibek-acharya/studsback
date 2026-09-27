package downloadcenter

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

// These tests cover publication gating on the public download path.
//
// The module previously had no test file at all, which is why an unpublished
// file was downloadable by id without anyone noticing. The regression that
// matters is TestPublicDownloadRouteHidesUnpublishedItem: it exercises the
// route, not just the service, because the bug lived in the handler choosing
// the wrong accessor.

func newDownloadCenterTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&DownloadItem{}); err != nil {
		t.Fatalf("auto migrate downloadcenter: %v", err)
	}
	return NewService(NewRepository(db)), db
}

func seedDownloadItem(t *testing.T, db *gorm.DB, title string, published bool) *DownloadItem {
	t.Helper()

	item := &DownloadItem{
		Title:       title,
		Category:    "notes",
		FilePath:    "downloads/test.pdf",
		FileURL:     "/uploads/downloads/test.pdf",
		FileName:    "test.pdf",
		IsPublished: published,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("seed download item: %v", err)
	}
	return item
}

func TestGetPublishedItemReturnsPublishedItem(t *testing.T) {
	svc, db := newDownloadCenterTestService(t)
	item := seedDownloadItem(t, db, "Published notes", true)

	got, err := svc.GetPublishedItem(item.ID)
	if err != nil {
		t.Fatalf("GetPublishedItem on a published item: %v", err)
	}
	if got.ID != item.ID {
		t.Fatalf("ID = %d, want %d", got.ID, item.ID)
	}
}

func TestGetPublishedItemRejectsUnpublishedItem(t *testing.T) {
	svc, db := newDownloadCenterTestService(t)
	item := seedDownloadItem(t, db, "Draft notes", false)

	if _, err := svc.GetPublishedItem(item.ID); !errors.Is(err, ErrItemNotPublished) {
		t.Fatalf("err = %v, want ErrItemNotPublished", err)
	}
}

func TestGetPublishedItemMissingIDIsNotReportedAsUnpublished(t *testing.T) {
	svc, _ := newDownloadCenterTestService(t)

	_, err := svc.GetPublishedItem(4242)
	if err == nil {
		t.Fatal("expected an error for a missing id")
	}
	// Both cases surface as 404 to the caller, but the error types must stay
	// distinct so a real lookup failure is not silently read as "it's a draft".
	if errors.Is(err, ErrItemNotPublished) {
		t.Fatalf("a missing id must not report as unpublished, got %v", err)
	}
}

// Guards the fix against over-reach: GetItem is the admin detail and update
// path, which legitimately needs to see drafts so they can be edited.
func TestGetItemStillReturnsUnpublishedForAdmin(t *testing.T) {
	svc, db := newDownloadCenterTestService(t)
	item := seedDownloadItem(t, db, "Draft notes", false)

	got, err := svc.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem must still return drafts for the admin edit path: %v", err)
	}
	if got.ID != item.ID {
		t.Fatalf("ID = %d, want %d", got.ID, item.ID)
	}
}

func TestPublicDownloadRouteHidesUnpublishedItem(t *testing.T) {
	svc, db := newDownloadCenterTestService(t)
	draft := seedDownloadItem(t, db, "Draft notes", false)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// The public route carries no auth middleware in production either, which
	// is precisely why the handler had to enforce publication itself.
	RegisterRoutes(
		r,
		func(c *gin.Context) { c.Next() },
		func(c *gin.Context) { c.Abort() },
		NewHandler(svc),
	)

	rec := httptest.NewRecorder()
	url := "/api/v1/downloads/" + strconv.FormatUint(uint64(draft.ID), 10) + "/download"
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))

	// The handler returns before touching object storage, so this test needs
	// no MinIO. Reaching storage would mean publication was not enforced.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an unpublished item", rec.Code)
	}
}

func TestPublicDownloadRouteRejectsMissingID(t *testing.T) {
	svc, _ := newDownloadCenterTestService(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(
		r,
		func(c *gin.Context) { c.Next() },
		func(c *gin.Context) { c.Abort() },
		NewHandler(svc),
	)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/downloads/4242/download", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a missing id", rec.Code)
	}
}

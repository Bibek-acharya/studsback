package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// studyResourcesPage is the non-landing page the superadmin UI lists on its own.
const studyResourcesPage = "study-resources"

func testCarouselDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&CarouselSlide{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// seedCarouselSlides inserts two landing hero slides and two study-resources
// slides (one inactive) so page filtering is observable.
func seedCarouselSlides(t *testing.T, db *gorm.DB) {
	t.Helper()
	slides := []CarouselSlide{
		{Page: CarouselPageLanding, Title: "Landing hero 2", Order: 2, Active: true},
		{Page: CarouselPageLanding, Title: "Landing hero 1", Order: 1, Active: true},
		{Page: studyResourcesPage, Title: "Resources hero 1", Order: 1, Active: true},
		{Page: studyResourcesPage, Title: "Resources hero 2", Order: 2, Active: true},
	}
	if err := db.Create(&slides).Error; err != nil {
		t.Fatalf("seed slides: %v", err)
	}
	// CarouselSlide.Active carries `gorm:"default:true"`, so a false value is
	// dropped on insert; force the inactive row here.
	if err := db.Model(&CarouselSlide{}).Where("title = ?", "Resources hero 2").
		Update("active", false).Error; err != nil {
		t.Fatalf("deactivate seed slide: %v", err)
	}
}

func slideTitles(slides []CarouselSlide) []string {
	titles := make([]string, 0, len(slides))
	for _, s := range slides {
		titles = append(titles, s.Title)
	}
	return titles
}

func assertTitles(t *testing.T, got []CarouselSlide, want []string) {
	t.Helper()
	gotTitles := slideTitles(got)
	if len(gotTitles) != len(want) {
		t.Fatalf("got %v, want %v", gotTitles, want)
	}
	for i := range want {
		if gotTitles[i] != want[i] {
			t.Fatalf("got %v, want %v", gotTitles, want)
		}
	}
}

func TestGetCarouselsPageFilterReturnsOnlyRequestedPage(t *testing.T) {
	db := testCarouselDB(t)
	seedCarouselSlides(t, db)
	svc := NewService(NewRepository(db), nil)

	// ?page=study-resources style call: only that page, in display order.
	slides, err := svc.GetCarousels(studyResourcesPage, nil)
	if err != nil {
		t.Fatalf("get carousels: %v", err)
	}
	assertTitles(t, slides, []string{"Resources hero 1", "Resources hero 2"})
	for _, s := range slides {
		if s.Page != studyResourcesPage {
			t.Fatalf("page=%q leaked into a study-resources list", s.Page)
		}
	}

	// The landing page stays independently listable.
	landing, err := svc.GetCarousels(CarouselPageLanding, nil)
	if err != nil {
		t.Fatalf("get landing carousels: %v", err)
	}
	assertTitles(t, landing, []string{"Landing hero 1", "Landing hero 2"})
}

func TestGetCarouselsOmittedPageKeepsLandingDefault(t *testing.T) {
	db := testCarouselDB(t)
	seedCarouselSlides(t, db)
	svc := NewService(NewRepository(db), nil)

	// Omitted / blank page keeps the pre-existing default (landing only) —
	// it must not start returning every page.
	for _, page := range []string{"", "   "} {
		slides, err := svc.GetCarousels(page, nil)
		if err != nil {
			t.Fatalf("get carousels (page=%q): %v", page, err)
		}
		assertTitles(t, slides, []string{"Landing hero 1", "Landing hero 2"})
	}
}

func TestGetCarouselsPageFilterComposesWithActiveFilter(t *testing.T) {
	db := testCarouselDB(t)
	seedCarouselSlides(t, db)
	svc := NewService(NewRepository(db), nil)

	active, err := svc.GetCarousels(studyResourcesPage, boolPtr(true))
	if err != nil {
		t.Fatalf("get active slides: %v", err)
	}
	assertTitles(t, active, []string{"Resources hero 1"})

	inactive, err := svc.GetCarousels(studyResourcesPage, boolPtr(false))
	if err != nil {
		t.Fatalf("get inactive slides: %v", err)
	}
	assertTitles(t, inactive, []string{"Resources hero 2"})
}

func TestCreateCarouselSlidePageDefaultsRemainUnchanged(t *testing.T) {
	db := testCarouselDB(t)
	svc := NewService(NewRepository(db), nil)

	// No page in the payload keeps the landing default.
	landing, err := svc.CreateCarouselSlide(CarouselSlideRequest{Title: "Landing hero 1"})
	if err != nil {
		t.Fatalf("create landing slide: %v", err)
	}
	if landing.Page != CarouselPageLanding {
		t.Fatalf("landing default page = %q, want %q", landing.Page, CarouselPageLanding)
	}

	// An explicit page is honoured and ordered within that page.
	first, err := svc.CreateCarouselSlide(CarouselSlideRequest{Page: studyResourcesPage, Title: "Resources hero 1"})
	if err != nil {
		t.Fatalf("create study-resources slide: %v", err)
	}
	if first.Page != studyResourcesPage || first.Order != 1 {
		t.Fatalf("study-resources slide = page %q order %d, want page %q order 1", first.Page, first.Order, studyResourcesPage)
	}
	second, err := svc.CreateCarouselSlide(CarouselSlideRequest{Page: studyResourcesPage, Title: "Resources hero 2"})
	if err != nil {
		t.Fatalf("create second study-resources slide: %v", err)
	}
	if second.Order != 2 {
		t.Fatalf("second study-resources order = %d, want 2", second.Order)
	}
}

// carouselRouter mounts the real system routes with pass-through auth/role
// middleware — the page filter must not depend on role wiring.
func carouselRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	passThrough := func(c *gin.Context) { c.Next() }
	RegisterRoutes(r, passThrough, passThrough, NewHandler(NewService(NewRepository(db), nil)))
	return r
}

type carouselListResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    []CarouselSlideResponse `json:"data"`
}

func listCarousels(t *testing.T, r *gin.Engine, path string) carouselListResponse {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, body = %s", path, w.Code, w.Body.String())
	}
	var resp carouselListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal GET %s: %v (body=%s)", path, err, w.Body.String())
	}
	return resp
}

func assertListPages(t *testing.T, resp carouselListResponse, wantPage string, wantTitles []string) {
	t.Helper()
	if !resp.Success {
		t.Fatalf("response not successful: %+v", resp)
	}
	if len(resp.Data) != len(wantTitles) {
		t.Fatalf("got %d slides, want %d (%+v)", len(resp.Data), len(wantTitles), resp.Data)
	}
	for i, want := range wantTitles {
		if resp.Data[i].Title != want {
			t.Fatalf("slide %d title = %q, want %q", i, resp.Data[i].Title, want)
		}
		if resp.Data[i].Page != wantPage {
			t.Fatalf("slide %d page = %q, want %q", i, resp.Data[i].Page, wantPage)
		}
	}
}

func TestAdminCarouselsEndpointPageFilter(t *testing.T) {
	db := testCarouselDB(t)
	seedCarouselSlides(t, db)
	r := carouselRouter(t, db)

	// Filtered: only study-resources slides, envelope unchanged.
	filtered := listCarousels(t, r, "/api/v1/admin/carousels?page="+studyResourcesPage)
	if filtered.Message != "Carousel slides retrieved successfully" {
		t.Fatalf("message = %q, want the existing success message", filtered.Message)
	}
	assertListPages(t, filtered, studyResourcesPage, []string{"Resources hero 1", "Resources hero 2"})

	// Omitted page keeps the landing-only default.
	assertListPages(t, listCarousels(t, r, "/api/v1/admin/carousels"),
		CarouselPageLanding, []string{"Landing hero 1", "Landing hero 2"})

	// Blank page behaves like an omitted one rather than matching nothing.
	assertListPages(t, listCarousels(t, r, "/api/v1/admin/carousels?page=%20%20"),
		CarouselPageLanding, []string{"Landing hero 1", "Landing hero 2"})

	// active still narrows within the requested page.
	assertListPages(t, listCarousels(t, r, "/api/v1/admin/carousels?page="+studyResourcesPage+"&active=false"),
		studyResourcesPage, []string{"Resources hero 2"})
}

func TestPublicCarouselsEndpointPageFilter(t *testing.T) {
	db := testCarouselDB(t)
	seedCarouselSlides(t, db)
	r := carouselRouter(t, db)

	// The guest route shares the filter but its default stays on landing.
	assertListPages(t, listCarousels(t, r, "/api/v1/system/carousels"),
		CarouselPageLanding, []string{"Landing hero 1", "Landing hero 2"})
	assertListPages(t, listCarousels(t, r, "/api/v1/system/carousels?page="+studyResourcesPage),
		studyResourcesPage, []string{"Resources hero 1", "Resources hero 2"})
}

func TestAdminCarouselsEndpointEmptyPageReturnsEmptyList(t *testing.T) {
	db := testCarouselDB(t)
	r := carouselRouter(t, db)

	resp := listCarousels(t, r, "/api/v1/admin/carousels?page="+studyResourcesPage)
	if !resp.Success {
		t.Fatalf("response not successful: %+v", resp)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("got %d slides for an empty page, want 0 (%+v)", len(resp.Data), resp.Data)
	}
}

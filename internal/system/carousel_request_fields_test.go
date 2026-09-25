package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func stringPtr(s string) *string { return &s }

// newCarouselSlideWithOptionals creates a slide whose four clearable fields
// hold non-empty values, and returns the persisted row.
func newCarouselSlideWithOptionals(t *testing.T, svc *Service) *CarouselSlide {
	t.Helper()
	slide, err := svc.CreateCarouselSlide(CarouselSlideRequest{
		Page:        studyResourcesPage,
		Title:       "Resources hero",
		Subtitle:    stringPtr("Past papers"),
		Description: stringPtr("Downloadable notes"),
		ImageURL:    "",
		LinkURL:     stringPtr("/study-resources"),
		ButtonText:  stringPtr("Browse"),
	})
	if err != nil {
		t.Fatalf("create slide: %v", err)
	}
	return slide
}

func assertOptionalFields(t *testing.T, slide *CarouselSlide, subtitle, description, linkURL, buttonText string) {
	t.Helper()
	if slide.Subtitle != subtitle || slide.Description != description ||
		slide.LinkURL != linkURL || slide.ButtonText != buttonText {
		t.Fatalf("optional fields = subtitle %q, description %q, link %q, button %q; want %q, %q, %q, %q",
			slide.Subtitle, slide.Description, slide.LinkURL, slide.ButtonText,
			subtitle, description, linkURL, buttonText)
	}
}

func TestCreateCarouselSlideWithoutOptionalFieldsStoresEmptyStrings(t *testing.T) {
	db := testCarouselDB(t)
	svc := NewService(NewRepository(db), nil)

	// Only the required-ish keys are sent: the four optional fields must be
	// stored as empty strings, exactly as before they became pointers.
	slide, err := svc.CreateCarouselSlide(CarouselSlideRequest{Page: studyResourcesPage, Title: "Resources hero"})
	if err != nil {
		t.Fatalf("create slide: %v", err)
	}
	assertOptionalFields(t, slide, "", "", "", "")

	reloaded, err := svc.GetCarouselSlideByID(slide.ID)
	if err != nil {
		t.Fatalf("reload slide: %v", err)
	}
	assertOptionalFields(t, reloaded, "", "", "", "")
	if reloaded.Title != "Resources hero" || reloaded.Page != studyResourcesPage {
		t.Fatalf("required fields wrong: title %q page %q", reloaded.Title, reloaded.Page)
	}
}

func TestCreateCarouselSlideStoresOptionalValues(t *testing.T) {
	db := testCarouselDB(t)
	svc := NewService(NewRepository(db), nil)

	slide, err := svc.CreateCarouselSlide(CarouselSlideRequest{
		Title:       "Resources hero",
		Subtitle:    stringPtr("Past papers"),
		Description: stringPtr("Downloadable notes"),
		ImageURL:    "https://cdn.example.com/a.png",
		LinkURL:     stringPtr("/study-resources"),
		ButtonText:  stringPtr("Browse"),
	})
	if err != nil {
		t.Fatalf("create slide: %v", err)
	}
	assertOptionalFields(t, slide, "Past papers", "Downloadable notes", "/study-resources", "Browse")

	reloaded, err := svc.GetCarouselSlideByID(slide.ID)
	if err != nil {
		t.Fatalf("reload slide: %v", err)
	}
	assertOptionalFields(t, reloaded, "Past papers", "Downloadable notes", "/study-resources", "Browse")
	if reloaded.ImageURL != "https://cdn.example.com/a.png" {
		t.Fatalf("image_url = %q, want the supplied URL", reloaded.ImageURL)
	}
	if reloaded.Page != CarouselPageLanding {
		t.Fatalf("page = %q, want the %q default", reloaded.Page, CarouselPageLanding)
	}
}

func TestUpdateCarouselSlideClearsOptionalFieldsWithEmptyStrings(t *testing.T) {
	db := testCarouselDB(t)
	svc := NewService(NewRepository(db), nil)
	created := newCarouselSlideWithOptionals(t, svc)

	// A non-nil empty pointer clears the column instead of being ignored.
	updated, err := svc.UpdateCarouselSlide(created.ID, CarouselSlideRequest{
		Subtitle:    stringPtr(""),
		Description: stringPtr(""),
		LinkURL:     stringPtr(""),
		ButtonText:  stringPtr(""),
	})
	if err != nil {
		t.Fatalf("update slide: %v", err)
	}
	assertOptionalFields(t, updated, "", "", "", "")

	// The clear is persisted, not just echoed back.
	reloaded, err := svc.GetCarouselSlideByID(created.ID)
	if err != nil {
		t.Fatalf("reload slide: %v", err)
	}
	assertOptionalFields(t, reloaded, "", "", "", "")

	// Untouched keys survive a partial clear.
	if reloaded.Title != "Resources hero" || reloaded.Page != studyResourcesPage {
		t.Fatalf("required fields changed: title %q page %q", reloaded.Title, reloaded.Page)
	}
}

func TestUpdateCarouselSlideClearsOnlyTheFieldsProvided(t *testing.T) {
	db := testCarouselDB(t)
	svc := NewService(NewRepository(db), nil)
	created := newCarouselSlideWithOptionals(t, svc)

	updated, err := svc.UpdateCarouselSlide(created.ID, CarouselSlideRequest{ButtonText: stringPtr("")})
	if err != nil {
		t.Fatalf("update slide: %v", err)
	}
	assertOptionalFields(t, updated, "Past papers", "Downloadable notes", "/study-resources", "")
}

func TestUpdateCarouselSlideNilOptionalFieldsLeaveColumnsUnchanged(t *testing.T) {
	db := testCarouselDB(t)
	svc := NewService(NewRepository(db), nil)
	created := newCarouselSlideWithOptionals(t, svc)

	// Omitted optional keys (nil) must not blank the stored values, while a
	// supplied non-empty field still applies.
	updated, err := svc.UpdateCarouselSlide(created.ID, CarouselSlideRequest{
		Title:    "Resources hero v2",
		ImageURL: "https://cdn.example.com/b.png",
	})
	if err != nil {
		t.Fatalf("update slide: %v", err)
	}
	assertOptionalFields(t, updated, "Past papers", "Downloadable notes", "/study-resources", "Browse")
	if updated.Title != "Resources hero v2" || updated.ImageURL != "https://cdn.example.com/b.png" {
		t.Fatalf("non-optional update not applied: title %q image %q", updated.Title, updated.ImageURL)
	}

	reloaded, err := svc.GetCarouselSlideByID(created.ID)
	if err != nil {
		t.Fatalf("reload slide: %v", err)
	}
	assertOptionalFields(t, reloaded, "Past papers", "Downloadable notes", "/study-resources", "Browse")
}

func TestUpdateCarouselSlideEmptyPayloadLeavesSlideIntact(t *testing.T) {
	db := testCarouselDB(t)
	svc := NewService(NewRepository(db), nil)
	created := newCarouselSlideWithOptionals(t, svc)

	before, err := svc.GetCarouselSlideByID(created.ID)
	if err != nil {
		t.Fatalf("reload slide: %v", err)
	}

	// An empty object is a no-op: every column keeps its value.
	if _, err := svc.UpdateCarouselSlide(created.ID, CarouselSlideRequest{}); err != nil {
		t.Fatalf("update slide: %v", err)
	}
	after, err := svc.GetCarouselSlideByID(created.ID)
	if err != nil {
		t.Fatalf("reload slide: %v", err)
	}
	if after.Subtitle != before.Subtitle || after.Description != before.Description ||
		after.LinkURL != before.LinkURL || after.ButtonText != before.ButtonText ||
		after.Title != before.Title || after.Page != before.Page || after.Order != before.Order ||
		after.Active != before.Active {
		t.Fatalf("empty update changed the slide: before %+v after %+v", before, after)
	}
}

func TestUpdateCarouselSlideActiveFalseStillWorks(t *testing.T) {
	db := testCarouselDB(t)
	svc := NewService(NewRepository(db), nil)
	created := newCarouselSlideWithOptionals(t, svc)

	deactivated, err := svc.UpdateCarouselSlide(created.ID, CarouselSlideRequest{Active: boolPtr(false)})
	if err != nil {
		t.Fatalf("deactivate slide: %v", err)
	}
	if deactivated.Active {
		t.Fatal("active still true after active=false update")
	}
	assertOptionalFields(t, deactivated, "Past papers", "Downloadable notes", "/study-resources", "Browse")

	// Clearing a field and deactivating in one payload both apply.
	reactivated, err := svc.UpdateCarouselSlide(created.ID, CarouselSlideRequest{
		Subtitle: stringPtr(""),
		Active:   boolPtr(true),
	})
	if err != nil {
		t.Fatalf("reactivate slide: %v", err)
	}
	if !reactivated.Active || reactivated.Subtitle != "" {
		t.Fatalf("combined update wrong: active %v subtitle %q", reactivated.Active, reactivated.Subtitle)
	}
}

// TestCarouselSlideRequestWireFormat keeps the JSON contract intact: the wire
// keys are unchanged, and omission vs. empty string is decided on the server.
func TestCarouselSlideRequestWireFormat(t *testing.T) {
	var clear CarouselSlideRequest
	if err := json.Unmarshal([]byte(`{"subtitle":"","description":"","link_url":"","button_text":""}`), &clear); err != nil {
		t.Fatalf("unmarshal clear payload: %v", err)
	}
	if clear.Subtitle == nil || *clear.Subtitle != "" || clear.Description == nil ||
		clear.LinkURL == nil || clear.ButtonText == nil {
		t.Fatalf("explicit empty strings must decode to non-nil empty pointers: %+v", clear)
	}

	var omitted CarouselSlideRequest
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatalf("unmarshal empty payload: %v", err)
	}
	if omitted.Subtitle != nil || omitted.Description != nil || omitted.LinkURL != nil || omitted.ButtonText != nil {
		t.Fatalf("omitted keys must decode to nil pointers: %+v", omitted)
	}
}

func TestAdminCarouselSlideUpdateEndpointClearsAndPreserves(t *testing.T) {
	db := testCarouselDB(t)
	r := carouselRouter(t, db)
	slide := newCarouselSlideWithOptionals(t, NewService(NewRepository(db), nil))

	put := func(path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPut, path, strings.NewReader(body)))
		if w.Code != http.StatusOK {
			t.Fatalf("PUT %s status = %d, body = %s", path, w.Code, w.Body.String())
		}
		var resp struct {
			Success bool                  `json:"success"`
			Message string                `json:"message"`
			Data    CarouselSlideResponse `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal PUT %s: %v (body=%s)", path, err, w.Body.String())
		}
		if !resp.Success || resp.Message != "Slide updated successfully" {
			t.Fatalf("update envelope wrong: %+v", resp)
		}
		return w
	}

	// `{"subtitle":""}` clears; the response shape is unchanged.
	put("/api/v1/admin/carousels/"+strconv.FormatUint(uint64(slide.ID), 10), `{"subtitle":""}`)
	cleared, err := NewService(NewRepository(db), nil).GetCarouselSlideByID(slide.ID)
	if err != nil {
		t.Fatalf("reload slide: %v", err)
	}
	if cleared.Subtitle != "" {
		t.Fatalf("subtitle = %q, want cleared", cleared.Subtitle)
	}
	assertOptionalFields(t, cleared, "", "Downloadable notes", "/study-resources", "Browse")

	// `{}` must not touch it.
	put("/api/v1/admin/carousels/"+strconv.FormatUint(uint64(slide.ID), 10), `{}`)
	untouched, err := NewService(NewRepository(db), nil).GetCarouselSlideByID(slide.ID)
	if err != nil {
		t.Fatalf("reload slide: %v", err)
	}
	assertOptionalFields(t, untouched, "", "Downloadable notes", "/study-resources", "Browse")

	// `{"active":false}` still deactivates.
	put("/api/v1/admin/carousels/"+strconv.FormatUint(uint64(slide.ID), 10), `{"active":false}`)
	deactivated, err := NewService(NewRepository(db), nil).GetCarouselSlideByID(slide.ID)
	if err != nil {
		t.Fatalf("reload slide: %v", err)
	}
	if deactivated.Active {
		t.Fatal("active still true after active=false update")
	}
}

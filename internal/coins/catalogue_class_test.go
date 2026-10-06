// internal/coins/catalogue_class_test.go
//
// The catalogue-to-economy class mapping, and the wire shape of an annotated
// item.
//
// These two things are here without a build tag on purpose. The annotation's own
// tests (catalogue_access_pg_test.go) are `coinsintegration`-gated, so they do not
// run in `go test ./...` — and they built their items with CLASS names
// ("mock_test"), which is the other half of why a catalogue-type mismatch went
// unnoticed for as long as it did. The result was that flipping
// gates_enabled.study_resource to true changed nothing a student could see: every
// document's price lookup missed the class switch, and no badge appeared. The gate
// was live and the price was invisible, which is the one combination this feature
// must never produce.
//
// Both tests here are pure: no database, no config store, no server. That is the
// point of keeping them out of the gated file — a regression in either shows up in
// the ordinary suite.
package coins

import (
	"encoding/json"
	"testing"

	"studsphere/backend/internal/studyresources"
)

// THE regression. A lister hands over the catalogue's own type; the economy prices
// a class. Getting these confused means no block, which means no price.
func TestCatalogueClassMapsCatalogueTypesOntoClasses(t *testing.T) {
	cases := []struct {
		itemType string
		want     string
	}{
		// The four document types in the catalogue, all one price.
		{"past-questions", ResourceTypeStudyResource},
		{"model-questions", ResourceTypeStudyResource},
		{"study-notes", ResourceTypeStudyResource},
		{"syllabus", ResourceTypeStudyResource},
		// The one video type, priced and drawn-down separately.
		{"video-lectures", ResourceTypeVideo},
		// Legacy and unknown types are documents: every storable type in this
		// table is either a video lecture or one of the documents.
		{"something-legacy", ResourceTypeStudyResource},
		{"", ResourceTypeStudyResource},
		// The three class names are IDENTITY, because the integration tests
		// build items with them and a double-mapped "mock_test" would be
		// priced as a document.
		{ResourceTypeStudyResource, ResourceTypeStudyResource},
		{ResourceTypeVideo, ResourceTypeVideo},
		{ResourceTypeMockTest, ResourceTypeMockTest},
	}
	for _, tc := range cases {
		if got := catalogueClass(tc.itemType); got != tc.want {
			t.Errorf("catalogueClass(%q) = %q, want %q", tc.itemType, got, tc.want)
		}
	}
}

// The composed property, because the two functions being individually correct is
// not what the card depends on: with the document gate on, an item whose type came
// straight from the catalogue must resolve a real price.
func TestAGatedDocumentResolvesAPriceThroughTheCatalogueType(t *testing.T) {
	cfg := DefaultEconomyConfig()
	cfg.Gates.StudyResource = true
	cfg.UnlockEndpointEnabled = true

	for _, itemType := range []string{"past-questions", "model-questions", "study-notes", "syllabus"} {
		price, chargeable := priceForClass(cfg, catalogueClass(itemType))
		if !chargeable {
			t.Errorf("%q resolved to no class, so no block and no price with the gate ON", itemType)
			continue
		}
		if price != cfg.Prices.StudyResource {
			t.Errorf("%q priced at %d, want the document price %d",
				itemType, price, cfg.Prices.StudyResource)
		}
	}

	// And the gate-off case still yields nothing, which is the inertness the
	// whole feature rests on.
	cfg.Gates.StudyResource = false
	if _, chargeable := priceForClass(cfg, catalogueClass("past-questions")); chargeable {
		t.Error("a document resolved a price with the gate off")
	}
}

// An unknown class is not chargeable rather than priced at the document rate: a
// gate that priced a class it cannot compute would charge a number it invented.
func TestAnUnknownClassIsNotChargeable(t *testing.T) {
	cfg := DefaultEconomyConfig()
	cfg.Gates.StudyResource = true
	cfg.Gates.Video = true
	cfg.Gates.MockTest = true
	cfg.UnlockEndpointEnabled = true
	if _, ok := priceForClass(cfg, "not-a-class"); ok {
		t.Error("an unknown class was priced")
	}
}

// The annotated item IS the catalogue for every signed-in student, so it must carry
// what a card renders. This is the second half of the same bug: a card with a price
// and no description, no course and no mime type is not a working card.
func TestAnAnnotatedItemCarriesTheFieldsACardRenders(t *testing.T) {
	row := studyresources.StudyResource{
		ID:              812,
		Title:           "Thermodynamics Notes",
		Description:     "Chapter notes",
		ResourceType:    "study-notes",
		Course:          "BSc CSIT",
		Year:            "2081",
		FileName:        "notes.pdf",
		FileURL:         "/uploads/study-resources/notes.pdf",
		FileSize:        12_345_678,
		MimeType:        "application/pdf",
		Downloads:       24,
		Views:           3,
		IsPublished:     true,
		DurationSeconds: 0,
	}

	raw, err := json.Marshal(newCatalogueItem(row))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for key, want := range map[string]any{
		"id":            float64(812),
		"title":         "Thermodynamics Notes",
		"resource_type": "study-notes", // the CATALOGUE type, not the class
		"description":   "Chapter notes",
		"course":        "BSc CSIT",
		"year":          "2081",
		"file_name":     "notes.pdf",
		"mime_type":     "application/pdf",
		"file_size":     float64(12_345_678),
		"downloads":     float64(24),
		"is_published":  true,
	} {
		if got[key] != want {
			t.Errorf("item[%q] = %v, want %v — a signed-in student renders this card from this object",
				key, got[key], want)
		}
	}

	// The moderation interior stays off a student-facing route.
	for _, key := range []string{"approval_status", "reviewed_by", "reject_reason", "file_path"} {
		if _, present := got[key]; present {
			t.Errorf("item[%q] is disclosed on the student catalogue route", key)
		}
	}
}

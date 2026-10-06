// internal/coins/catalogue_adapter.go
//
// The adapter between this package and `internal/studyresources` for the
// catalogue access block.
//
// It exists because the dependency runs ONE way: `internal/coins` imports
// `internal/studyresources`, not the reverse. So the annotation logic lives here and
// the catalogue's own model is mapped in, rather than the catalogue reaching for a
// wallet.
//
// Putting it in its own file rather than at the bottom of catalogue_access.go is so
// the MAPPING is easy to find: it is the one place in the coin system that knows what
// a study resource is, and everything else in the package is deliberately ignorant of
// the catalogue.

package coins

import (
	"context"

	"studsphere/backend/internal/studyresources"
)

// studyresourcesCatalogue adapts the studyresources service to CatalogueLister.
//
// A STRUCT rather than a bare function so it satisfies the port directly and main.go
// has nothing to write.
type studyresourcesCatalogue struct {
	svc *studyresources.Service
}

// NewCatalogueLister returns a CatalogueLister backed by the studyresources service.
func NewCatalogueLister(svc *studyresources.Service) CatalogueLister {
	return studyresourcesCatalogue{svc: svc}
}

// ListCatalogue returns one page of PUBLISHED resources.
//
// PublishedOnly is hardcoded true, and it is the load-bearing decision in this
// function rather than a default. An access block is a statement about something
// purchasable, so attaching one to a DRAFT would tell a signed-in caller the price of
// material that is not for sale and is not approved. The public list already filters
// drafts for the same reason.
//
// page and limit are passed through rather than clamped here: the handler owns
// pagination and clamping twice would mean two places to keep in step.
//
// The mapping is the rest of this file's content for one reason: this route IS the
// catalogue for every signed-in student, so an item carrying only an id and a title
// renders a card with no description, no course, no file type and no download count —
// the whole card, minus the price. Which fields travel is decided HERE, in the one
// place that knows the catalogue's shape, rather than by whatever the annotation
// logic happens to expose.
func (c studyresourcesCatalogue) ListCatalogue(
	ctx context.Context, page, limit int,
) ([]CatalogueItem, error) {
	if c.svc == nil {
		return nil, ErrNoDatabase
	}
	resources, _, err := c.svc.GetResources(
		studyresources.ResourceFilters{PublishedOnly: true}, page, limit,
	)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogueItem, 0, len(resources))
	for _, r := range resources {
		out = append(out, newCatalogueItem(r))
	}
	return out, nil
}

// newCatalogueItem maps one catalogue row onto an annotated-catalogue item.
//
// ResourceType is the row's OWN type, not the economy class: the card renders it as
// a label and the catalogue filters on it. The class the price is resolved against is
// derived by catalogueClass and reported on the block, where the frontend reads it as
// `resource_type` inside `access` — two different meanings of one field name, which is
// why the block carries it separately rather than the item overloading it.
func newCatalogueItem(r studyresources.StudyResource) CatalogueItem {
	return CatalogueItem{
		ID:              r.ID,
		ResourceType:    r.ResourceType,
		Title:           r.Title,
		Description:     r.Description,
		Course:          r.Course,
		Year:            r.Year,
		FileName:        r.FileName,
		FileURL:         r.FileURL,
		FileSize:        r.FileSize,
		MimeType:        r.MimeType,
		Downloads:       r.Downloads,
		Views:           r.Views,
		IsPublished:     r.IsPublished,
		DurationSeconds: r.DurationSeconds,
		CreatedAt:       r.CreatedAt,
	}
}

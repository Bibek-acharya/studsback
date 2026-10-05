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
		out = append(out, CatalogueItem{
			ID:           r.ID,
			ResourceType: r.ResourceType,
			Title:        r.Title,
		})
	}
	return out, nil
}

package studyresources

import (
	"sort"
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type ResourceFilters struct {
	Search string
	Type   string
	Course string
	Year   string
	// PublishedOnly restricts the query to published rows. The public list
	// sets it; the admin list leaves it false so drafts stay visible.
	PublishedOnly bool
}

func (r *Repository) buildQuery(filters ResourceFilters) *gorm.DB {
	q := r.db.Model(&StudyResource{})
	if filters.PublishedOnly {
		q = q.Where("is_published = ?", true)
	}
	if filters.Search != "" {
		lower := "%" + strings.ToLower(filters.Search) + "%"
		q = q.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR LOWER(course) LIKE ?", lower, lower, lower)
	}
	if filters.Type != "" {
		// A canonical filter must still match rows stored with a legacy
		// spelling ("Past Questions", "PAST_QUESTIONS"), so filter on every
		// stored spelling that normalizes to the requested type.
		if matches := r.resolveTypeFilter(filters.Type); len(matches) > 1 {
			q = q.Where("resource_type IN ?", matches)
		} else if len(matches) == 1 {
			q = q.Where("resource_type = ?", matches[0])
		} else {
			// Unknown type: preserve the historical exact match.
			q = q.Where("resource_type = ?", filters.Type)
		}
	}
	if filters.Course != "" {
		q = q.Where("LOWER(course) LIKE ?", "%"+strings.ToLower(filters.Course)+"%")
	}
	if filters.Year != "" {
		q = q.Where("year = ?", filters.Year)
	}
	return q
}

// resolveTypeFilter returns every stored resource_type spelling that
// normalizes to the requested type. It combines the known legacy spellings
// with the distinct values actually present in the table, so a canonical
// filter keeps matching rows written by older endpoints. An empty result
// means the requested type is unknown.
func (r *Repository) resolveTypeFilter(value string) []string {
	canonical, err := NormalizeType(value)
	if err != nil {
		return nil
	}

	matches := MatchingStoredTypes(value)
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		seen[match] = true
	}

	var stored []string
	if err := r.db.Model(&StudyResource{}).
		Where("resource_type <> ''").
		Distinct().
		Pluck("resource_type", &stored).Error; err == nil {
		for _, candidate := range stored {
			if seen[candidate] {
				continue
			}
			if got, err := NormalizeType(candidate); err == nil && got == canonical {
				seen[candidate] = true
				matches = append(matches, candidate)
			}
		}
	}

	sort.Strings(matches)
	return matches
}

func (r *Repository) FindAll(filters ResourceFilters, page, limit int) ([]StudyResource, int64, error) {
	var total int64
	if err := r.buildQuery(filters).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var resources []StudyResource
	offset := (page - 1) * limit
	err := r.buildQuery(filters).
		Order("created_at DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&resources).Error
	return resources, total, err
}

func (r *Repository) FindResourceByID(id uint) (*StudyResource, error) {
	var resource StudyResource
	err := r.db.First(&resource, id).Error
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func (r *Repository) CreateResource(resource *StudyResource) error {
	// IsPublished carries a `default:true` tag, and GORM never writes a
	// zero-valued field that has a database default. It also reads the default
	// back into the struct after the insert, so the requested value is
	// captured first, the row is fixed up inside the same transaction, and the
	// caller's struct keeps the value it asked for.
	requested := resource.IsPublished
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(resource).Error; err != nil {
			return err
		}
		if !requested {
			if err := tx.Model(&StudyResource{}).
				Where("id = ?", resource.ID).
				UpdateColumn("is_published", false).Error; err != nil {
				return err
			}
		}
		resource.IsPublished = requested
		return nil
	})
}

func (r *Repository) UpdateResource(resource *StudyResource) error {
	return r.db.Save(resource).Error
}

func (r *Repository) DeleteResource(id uint) error {
	return r.db.Delete(&StudyResource{}, id).Error
}

func (r *Repository) IncrementDownloads(id uint) error {
	return r.db.Model(&StudyResource{}).
		Where("id = ?", id).
		UpdateColumn("downloads", gorm.Expr("downloads + 1")).Error
}

func (r *Repository) IncrementViews(id uint) error {
	return r.db.Model(&StudyResource{}).
		Where("id = ?", id).
		UpdateColumn("views", gorm.Expr("views + 1")).Error
}

// DistinctValues returns distinct non-empty values of the given column across
// all non-deleted study resources. column must be a trusted identifier
// ("year" or "course"); it is never user input.
func (r *Repository) DistinctValues(column, order string) ([]string, error) {
	var values []string
	err := r.db.Model(&StudyResource{}).
		Where(column+" <> ''").
		Distinct(column).
		Order(order).
		Pluck(column, &values).Error
	return values, err
}

// DistinctPublishedValues is DistinctValues restricted to published rows. It
// powers the public filter facets so drafts never leak through a facet.
func (r *Repository) DistinctPublishedValues(column, order string) ([]string, error) {
	var values []string
	err := r.db.Model(&StudyResource{}).
		Where("is_published = ?", true).
		Where(column+" <> ''").
		Distinct(column).
		Order(order).
		Pluck(column, &values).Error
	return values, err
}

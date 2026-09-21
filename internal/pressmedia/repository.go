package pressmedia

import (
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type ItemFilters struct {
	Category      string
	PublishedOnly bool
}

func (r *Repository) buildQuery(filters ItemFilters) *gorm.DB {
	q := r.db.Model(&PressMediaItem{})
	if filters.PublishedOnly {
		q = q.Where("is_published = ?", true)
	}
	if filters.Category != "" {
		q = q.Where("category = ?", filters.Category)
	}
	return q
}

func (r *Repository) FindAll(filters ItemFilters, page, limit int) ([]PressMediaItem, int64, error) {
	var total int64
	if err := r.buildQuery(filters).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []PressMediaItem
	offset := (page - 1) * limit
	err := r.buildQuery(filters).
		Order("published_at DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error
	return items, total, err
}

func (r *Repository) FindItemByID(id uint) (*PressMediaItem, error) {
	var item PressMediaItem
	err := r.db.First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// SlugExists reports whether the given slug is already taken by another
// (non-deleted) press media item. excludeID of 0 checks against all items.
func (r *Repository) SlugExists(slug string, excludeID uint) (bool, error) {
	var count int64
	q := r.db.Model(&PressMediaItem{}).Where("slug = ?", slug)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) CreateItem(item *PressMediaItem) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateItem(item *PressMediaItem) error {
	return r.db.Save(item).Error
}

func (r *Repository) DeleteItem(id uint) error {
	return r.db.Delete(&PressMediaItem{}, id).Error
}

// sanitizeCategory normalizes user-supplied category values.
func sanitizeCategory(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

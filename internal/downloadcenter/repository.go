package downloadcenter

import (
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
	q := r.db.Model(&DownloadItem{})
	if filters.PublishedOnly {
		q = q.Where("is_published = ?", true)
	}
	if filters.Category != "" {
		q = q.Where("category = ?", filters.Category)
	}
	return q
}

func (r *Repository) FindAll(filters ItemFilters, page, limit int) ([]DownloadItem, int64, error) {
	var total int64
	if err := r.buildQuery(filters).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []DownloadItem
	offset := (page - 1) * limit
	err := r.buildQuery(filters).
		Order("created_at DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error
	return items, total, err
}

func (r *Repository) FindItemByID(id uint) (*DownloadItem, error) {
	var item DownloadItem
	err := r.db.First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateItem(item *DownloadItem) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateItem(item *DownloadItem) error {
	return r.db.Save(item).Error
}

func (r *Repository) DeleteItem(id uint) error {
	return r.db.Delete(&DownloadItem{}, id).Error
}

// IncrementDownloads bumps the download counter with a SQL-level expression so
// concurrent downloads don't clobber each other.
func (r *Repository) IncrementDownloads(id uint) error {
	return r.db.Model(&DownloadItem{}).
		Where("id = ?", id).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
}

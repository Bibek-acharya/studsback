package studyresources

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

type ResourceFilters struct {
	Search string
	Type   string
	Course string
	Year   string
}

func (r *Repository) buildQuery(filters ResourceFilters) *gorm.DB {
	q := r.db.Model(&StudyResource{})
	if filters.Search != "" {
		lower := "%" + strings.ToLower(filters.Search) + "%"
		q = q.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR LOWER(course) LIKE ?", lower, lower, lower)
	}
	if filters.Type != "" {
		q = q.Where("resource_type = ?", filters.Type)
	}
	if filters.Course != "" {
		q = q.Where("LOWER(course) LIKE ?", "%"+strings.ToLower(filters.Course)+"%")
	}
	if filters.Year != "" {
		q = q.Where("year = ?", filters.Year)
	}
	return q
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
	return r.db.Create(resource).Error
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

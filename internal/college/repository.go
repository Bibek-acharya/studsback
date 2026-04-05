package college

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(filters CollegeFilters) ([]College, int64, error) {
	var colleges []College
	var total int64

	query := r.db.Model(&College{})

	if filters.UniversityID != "" {
		query = query.Where("university_id = ?", filters.UniversityID)
	}

	if filters.Location != "" {
		query = query.Where("location ILIKE ?", "%"+filters.Location+"%")
	}

	if filters.Affiliation != "" {
		query = query.Where("affiliation ILIKE ?", "%"+filters.Affiliation+"%")
	}

	if filters.Type != "" {
		types := parseTypes(filters.Type)
		if len(types) == 1 {
			query = query.Where("college_type = ?", types[0])
		} else if len(types) > 1 {
			query = query.Where("college_type IN ?", types)
		}
	}

	if filters.Verified == "true" {
		query = query.Where("verified = ?", true)
	}

	if filters.Popular == "true" {
		query = query.Where("popular = ?", true)
	}

	if filters.MinRating != "" {
		if rating, err := parseFloat(filters.MinRating); err == nil {
			query = query.Where("rating >= ?", rating)
		}
	}

	if filters.Search != "" {
		searchLike := "%" + filters.Search + "%"
		query = query.Where(
			"name ILIKE ? OR full_name ILIKE ? OR affiliation ILIKE ? OR location ILIKE ? OR CAST(featured_programs AS TEXT) ILIKE ? OR CAST(courses AS TEXT) ILIKE ? OR CAST(programs_list AS TEXT) ILIKE ?",
			searchLike, searchLike, searchLike, searchLike, searchLike, searchLike, searchLike,
		)
	}

	if filters.CourseID != "" {
		query = query.Joins("JOIN college_university_courses ON college_university_courses.college_id = colleges.id").
			Where("college_university_courses.course_id = ?", filters.CourseID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sort := filters.Sort
	if sort != "rating" && sort != "name" && sort != "reviews" {
		sort = "rating"
	}

	order := filters.Order
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	offset := (filters.Page - 1) * filters.PageSize

	if err := query.Order(sort + " " + order).
		Offset(offset).
		Limit(filters.PageSize).
		Preload("University").
		Find(&colleges).Error; err != nil {
		return nil, 0, err
	}

	return colleges, total, nil
}

func (r *Repository) FindByID(id uint) (*College, error) {
	var college College
	err := r.db.Preload("University").First(&college, id).Error
	if err != nil {
		return nil, err
	}
	return &college, nil
}

func (r *Repository) Create(college *College) error {
	return r.db.Create(college).Error
}

func (r *Repository) Update(college *College) error {
	return r.db.Save(college).Error
}

func (r *Repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&College{}, id).Error
}

func (r *Repository) Approve(id uint) error {
	return r.db.Model(&College{}).Where("id = ?", id).Update("verified", true).Error
}

func (r *Repository) ToggleFeatured(id uint) error {
	var college College
	if err := r.db.First(&college, id).Error; err != nil {
		return err
	}
	return r.db.Model(&college).Update("featured", !college.Featured).Error
}

func (r *Repository) Count() (int64, error) {
	var total int64
	err := r.db.Model(&College{}).Count(&total).Error
	return total, err
}

func (r *Repository) FindFeatured(limit int) ([]College, error) {
	var colleges []College
	err := r.db.Where("featured = ?", true).
		Order("rating desc").
		Limit(limit).
		Find(&colleges).Error
	return colleges, err
}

func (r *Repository) UniversityExists(id uint) bool {
	var count int64
	r.db.Model(&University{}).Where("id = ?", id).Count(&count)
	return count > 0
}

func (r *Repository) FindUniversityByName(name string) (*University, error) {
	var university University
	err := r.db.Where("LOWER(name) = LOWER(?)", name).First(&university).Error
	return &university, err
}

func (r *Repository) FindUniversityByID(id uint) (*University, error) {
	var university University
	err := r.db.First(&university, id).Error
	return &university, err
}

func parseTypes(typeStr string) []string {
	typesRaw := strings.Split(typeStr, ",")
	types := make([]string, 0, len(typesRaw))
	for _, t := range typesRaw {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			types = append(types, trimmed)
		}
	}
	return types
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

package scholarship

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(search, typeFilter, locationFilter, levelFilter, sortBy, order string) ([]Scholarship, error) {
	var scholarships []Scholarship
	query := r.db.Model(&Scholarship{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("title ILIKE ? OR provider ILIKE ? OR description ILIKE ?", like, like, like)
	}

	if typeFilter != "" {
		like := "%" + typeFilter + "%"
		query = query.Where("funding_type ILIKE ? OR scholarship_type ILIKE ?", like, like)
	}

	if locationFilter != "" {
		locations := splitFilterValues(locationFilter)
		for _, location := range locations {
			query = query.Where("location ILIKE ?", "%"+location+"%")
		}
	}

	if levelFilter != "" {
		levels := splitFilterValues(levelFilter)
		for _, level := range levels {
			query = query.Where("degree_level ILIKE ?", "%"+level+"%")
		}
	}

	switch sortBy {
	case "latest":
		query = query.Order("created_at " + order)
	case "title":
		query = query.Order("title " + order)
	default:
		query = query.Order("deadline " + order)
	}

	err := query.Find(&scholarships).Error
	return scholarships, err
}

func (r *Repository) FindAllUnfiltered() ([]Scholarship, error) {
	var scholarships []Scholarship
	err := r.db.Model(&Scholarship{}).Find(&scholarships).Error
	return scholarships, err
}

func (r *Repository) FindAllAdmin() ([]Scholarship, error) {
	var scholarships []Scholarship
	err := r.db.Order("created_at DESC").Find(&scholarships).Error
	return scholarships, err
}

func (r *Repository) FindByID(id uint) (*Scholarship, error) {
	var scholarship Scholarship
	err := r.db.First(&scholarship, id).Error
	if err != nil {
		return nil, err
	}
	return &scholarship, nil
}

func (r *Repository) FindSimilar(scholarship *Scholarship) ([]Scholarship, error) {
	var similar []Scholarship
	err := r.db.Model(&Scholarship{}).
		Where("id <> ?", scholarship.ID).
		Where(
			"(degree_level ILIKE ? OR funding_type ILIKE ? OR scholarship_type ILIKE ? OR location ILIKE ?)",
			"%"+scholarship.DegreeLevel+"%",
			"%"+scholarship.FundingType+"%",
			"%"+scholarship.ScholarshipType+"%",
			"%"+scholarship.Location+"%",
		).
		Order("deadline ASC").
		Limit(5).
		Find(&similar).Error
	return similar, err
}

func (r *Repository) FindFallbackSimilar(scholarship *Scholarship) ([]Scholarship, error) {
	var similar []Scholarship
	err := r.db.Model(&Scholarship{}).
		Where("id <> ?", scholarship.ID).
		Order("deadline ASC").
		Limit(5).
		Find(&similar).Error
	return similar, err
}

func (r *Repository) Create(scholarship *Scholarship) error {
	return r.db.Create(scholarship).Error
}

func (r *Repository) Save(scholarship *Scholarship) error {
	return r.db.Save(scholarship).Error
}

func (r *Repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&Scholarship{}, id).Error
}

func (r *Repository) ApplicationCreate(app *ScholarshipApplication) error {
	return r.db.Create(app).Error
}

func (r *Repository) ApplicationFindByUserID(userID uint) ([]ScholarshipApplication, error) {
	var applications []ScholarshipApplication
	err := r.db.Where("user_id = ?", userID).Preload("Scholarship").Order("created_at DESC").Find(&applications).Error
	return applications, err
}

func (r *Repository) ApplicationFindByID(id uint) (*ScholarshipApplication, error) {
	var app ScholarshipApplication
	err := r.db.Preload("Scholarship").Preload("User").First(&app, id).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) ApplicationSave(app *ScholarshipApplication) error {
	return r.db.Save(app).Error
}

func (r *Repository) ApplicationDelete(id uint) error {
	return r.db.Unscoped().Delete(&ScholarshipApplication{}, id).Error
}

func (r *Repository) ApplicationFindAll(status string) ([]ScholarshipApplication, error) {
	var applications []ScholarshipApplication
	query := r.db.Preload("Scholarship").Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").Find(&applications).Error
	return applications, err
}

func (r *Repository) ApplicationFindByScholarshipID(scholarshipID string, status string) ([]ScholarshipApplication, error) {
	var applications []ScholarshipApplication
	query := r.db.Where("scholarship_id = ?", scholarshipID).Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").Find(&applications).Error
	return applications, err
}

func (r *Repository) ApplicationExists(scholarshipID uint, userID uint) bool {
	var count int64
	r.db.Model(&ScholarshipApplication{}).Where("scholarship_id = ? AND user_id = ?", scholarshipID, userID).Count(&count)
	return count > 0
}

package admission

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CollegeExists(id uint) bool {
	var count int64
	r.db.Model(&College{}).Where("id = ?", id).Count(&count)
	return count > 0
}

func (r *Repository) Create(admission *Admission) error {
	return r.db.Create(admission).Error
}

func (r *Repository) FindByUserID(userID uint) ([]Admission, error) {
	var admissions []Admission
	err := r.db.Where("user_id = ?", userID).Preload("College").Order("created_at DESC").Find(&admissions).Error
	return admissions, err
}

func (r *Repository) FindByID(id uint) (*Admission, error) {
	var admission Admission
	err := r.db.Preload("College").Preload("User").First(&admission, id).Error
	if err != nil {
		return nil, err
	}
	return &admission, nil
}

func (r *Repository) Save(admission *Admission) error {
	return r.db.Save(admission).Error
}

func (r *Repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&Admission{}, id).Error
}

func (r *Repository) FindByCollegeID(collegeID string, status string) ([]Admission, error) {
	var admissions []Admission
	query := r.db.Where("college_id = ?", collegeID).Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").Find(&admissions).Error
	return admissions, err
}

func (r *Repository) FindAll(status string, collegeID string) ([]Admission, error) {
	var admissions []Admission
	query := r.db.Preload("College").Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if collegeID != "" {
		query = query.Where("college_id = ?", collegeID)
	}

	err := query.Order("created_at DESC").Find(&admissions).Error
	return admissions, err
}

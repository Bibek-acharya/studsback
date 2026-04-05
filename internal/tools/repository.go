package tools

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) LoadScholarships() ([]Scholarship, error) {
	var scholarships []Scholarship
	if err := r.db.Find(&scholarships).Error; err != nil {
		return nil, err
	}
	return scholarships, nil
}

func (r *Repository) LoadColleges() ([]College, error) {
	var colleges []College
	if err := r.db.Find(&colleges).Error; err != nil {
		return nil, err
	}
	return colleges, nil
}

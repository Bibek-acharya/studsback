package models

import (
	"time"

	"gorm.io/gorm"
)

type Scholarship struct {
	ID                  uint           `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	Title               string         `gorm:"not null" json:"title" binding:"required"`
	Provider            string         `gorm:"not null" json:"provider" binding:"required"`
	Location            string         `json:"location"`
	Value               string         `json:"value"`
	Deadline            time.Time      `json:"deadline"`
	DegreeLevel         string         `json:"degree_level"`     // e.g., Masters, Bachelors
	FundingType         string         `json:"funding_type"`     // e.g., Fully Funded, Partial
	ScholarshipType     string         `json:"scholarship_type"` // e.g., Merit Based, Need Based
	Description         string         `gorm:"type:text" json:"description"`
	ImageURL            string         `json:"image_url"`
	FieldOfStudy        []byte         `gorm:"type:jsonb" json:"field_of_study"`       // Array of strings
	SelectionProcess    []byte         `gorm:"type:jsonb" json:"selection_process"`    // Array of objects {stage, description}
	EligibilityCriteria []byte         `gorm:"type:jsonb" json:"eligibility_criteria"` // Array of objects {criterion, description}
	ExcludedRegions     []byte         `gorm:"type:jsonb" json:"excluded_regions"`     // Array of strings
	RequiredDocuments   []byte         `gorm:"type:jsonb" json:"required_documents"`   // Array of objects {name, description}
	Timeline            []byte         `gorm:"type:jsonb" json:"timeline"`             // Array of objects {date, event}
	Benefits            []byte         `gorm:"type:jsonb" json:"benefits"`             // Array of objects {title, description}
	FAQs                []byte         `gorm:"type:jsonb" json:"faqs"`                 // Array of objects {question, answer}
}

type ScholarshipApplication struct {
	ID                   uint           `gorm:"primarykey" json:"id"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
	ScholarshipID        uint           `gorm:"index" json:"scholarship_id"`
	Scholarship          Scholarship    `gorm:"foreignKey:ScholarshipID" json:"scholarship,omitempty"`
	UserID               uint           `gorm:"index" json:"user_id"`
	User                 User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	NationalID           string         `json:"national_id"`
	FirstName            string         `json:"first_name"`
	LastName             string         `json:"last_name"`
	DateOfBirth          time.Time      `json:"date_of_birth"`
	Gender               string         `json:"gender"`
	StreetAddress        string         `json:"street_address"`
	City                 string         `json:"city"`
	PostCode             string         `json:"post_code"`
	Country              string         `json:"country"`
	PhoneCode            string         `json:"phone_code"`
	PhoneNumber          string         `json:"phone_number"`
	Email                string         `json:"email"`
	LatestInstitution    string         `json:"latest_institution"`
	LevelCompleted       string         `json:"level_completed"`
	GPAPercentage        string         `json:"gpa_percentage"`
	AnnualFamilyIncome   string         `json:"annual_family_income"`
	PrimaryIncomeSource  string         `json:"primary_income_source"`
	SpecialCircumstances []byte         `gorm:"type:jsonb" json:"special_circumstances"` // Array of strings
	PersonalStatement    string         `gorm:"type:text" json:"personal_statement"`
	Documents            []byte         `gorm:"type:jsonb" json:"documents"`     // Array of objects {name, url}
	Status               string         `gorm:"default:'pending'" json:"status"` // pending, approved, rejected
}

type CreateScholarshipRequest struct {
	Title           string   `json:"title" binding:"required"`
	Provider        string   `json:"provider" binding:"required"`
	Location        string   `json:"location"`
	Value           string   `json:"value"`
	Deadline        string   `json:"deadline"` // Will be parsed
	DegreeLevel     string   `json:"degree_level"`
	FundingType     string   `json:"funding_type"`
	ScholarshipType string   `json:"scholarship_type"`
	Description     string   `json:"description"`
	ImageURL        string   `json:"image_url"`
	FieldOfStudy    []string `json:"field_of_study"`
	Status          string   `json:"status"`
}

type ScholarshipApplicationRequest struct {
	NationalID           string   `json:"national_id" binding:"required"`
	FirstName            string   `json:"first_name" binding:"required"`
	LastName             string   `json:"last_name" binding:"required"`
	DateOfBirth          string   `json:"date_of_birth" binding:"required"` // Will be parsed
	Gender               string   `json:"gender" binding:"required"`
	StreetAddress        string   `json:"street_address" binding:"required"`
	City                 string   `json:"city" binding:"required"`
	PostCode             string   `json:"post_code" binding:"required"`
	Country              string   `json:"country" binding:"required"`
	PhoneCode            string   `json:"phone_code" binding:"required"`
	PhoneNumber          string   `json:"phone_number" binding:"required"`
	Email                string   `json:"email" binding:"required,email"`
	LatestInstitution    string   `json:"latest_institution" binding:"required"`
	LevelCompleted       string   `json:"level_completed" binding:"required"`
	GPAPercentage        string   `json:"gpa_percentage" binding:"required"`
	AnnualFamilyIncome   string   `json:"annual_family_income" binding:"required"`
	PrimaryIncomeSource  string   `json:"primary_income_source" binding:"required"`
	SpecialCircumstances []string `json:"special_circumstances"`
	PersonalStatement    string   `json:"personal_statement" binding:"required"`
}

type UpdateScholarshipApplicationRequest struct {
	NationalID           *string  `json:"national_id"`
	FirstName            *string  `json:"first_name"`
	LastName             *string  `json:"last_name"`
	DateOfBirth          *string  `json:"date_of_birth"`
	Gender               *string  `json:"gender"`
	StreetAddress        *string  `json:"street_address"`
	City                 *string  `json:"city"`
	PostCode             *string  `json:"post_code"`
	Country              *string  `json:"country"`
	PhoneCode            *string  `json:"phone_code"`
	PhoneNumber          *string  `json:"phone_number"`
	Email                *string  `json:"email"`
	LatestInstitution    *string  `json:"latest_institution"`
	LevelCompleted       *string  `json:"level_completed"`
	GPAPercentage        *string  `json:"gpa_percentage"`
	AnnualFamilyIncome   *string  `json:"annual_family_income"`
	PrimaryIncomeSource  *string  `json:"primary_income_source"`
	SpecialCircumstances []string `json:"special_circumstances"`
	PersonalStatement    *string  `json:"personal_statement"`
}

type UpdateScholarshipApplicationStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending under_review approved rejected shortlisted"`
	Notes  string `json:"notes"`
}

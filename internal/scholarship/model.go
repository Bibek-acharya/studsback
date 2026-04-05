package scholarship

import (
	"time"

	"gorm.io/gorm"
)

type Scholarship struct {
	ID                  uint           `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	Title               string         `gorm:"not null" json:"title"`
	Provider            string         `gorm:"not null" json:"provider"`
	Location            string         `json:"location"`
	Value               string         `json:"value"`
	Deadline            time.Time      `json:"deadline"`
	DegreeLevel         string         `json:"degree_level"`
	FundingType         string         `json:"funding_type"`
	ScholarshipType     string         `json:"scholarship_type"`
	Description         string         `gorm:"type:text" json:"description"`
	ImageURL            string         `json:"image_url"`
	FieldOfStudy        []byte         `gorm:"type:jsonb" json:"field_of_study"`
	SelectionProcess    []byte         `gorm:"type:jsonb" json:"selection_process"`
	EligibilityCriteria []byte         `gorm:"type:jsonb" json:"eligibility_criteria"`
	ExcludedRegions     []byte         `gorm:"type:jsonb" json:"excluded_regions"`
	RequiredDocuments   []byte         `gorm:"type:jsonb" json:"required_documents"`
	Timeline            []byte         `gorm:"type:jsonb" json:"timeline"`
	Benefits            []byte         `gorm:"type:jsonb" json:"benefits"`
	FAQs                []byte         `gorm:"type:jsonb" json:"faqs"`
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
	SpecialCircumstances []byte         `gorm:"type:jsonb" json:"special_circumstances"`
	PersonalStatement    string         `gorm:"type:text" json:"personal_statement"`
	Documents            []byte         `gorm:"type:jsonb" json:"documents"`
	Status               string         `gorm:"default:'pending'" json:"status"`
}

type User struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

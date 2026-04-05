package tools

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

type College struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	UniversityID     uint           `gorm:"index" json:"university_id"`
	Name             string         `gorm:"not null;index" json:"name"`
	FullName         string         `json:"full_name"`
	Location         string         `gorm:"not null" json:"location"`
	Affiliation      string         `json:"affiliation"`
	CollegeType      string         `json:"type"`
	Verified         bool           `gorm:"default:false" json:"verified"`
	Popular          bool           `gorm:"default:false" json:"popular"`
	Featured         bool           `gorm:"default:false;index" json:"featured"`
	Rating           float64        `json:"rating"`
	Reviews          int            `json:"reviews"`
	Programs         int            `json:"programs"`
	Established      string         `json:"established"`
	Students         string         `json:"students"`
	Description      string         `gorm:"type:text" json:"description"`
	Website          string         `json:"website"`
	Email            string         `json:"email"`
	Phone            string         `json:"phone"`
	ImageURL         string         `json:"image_url"`
	FeaturedPrograms []byte         `gorm:"type:jsonb" json:"featured_programs"`
	Amenities        []byte         `gorm:"type:jsonb" json:"amenities"`
	Courses          []byte         `gorm:"type:jsonb" json:"courses"`
	Scholarships     []byte         `gorm:"type:jsonb" json:"scholarships"`
	Gallery          []byte         `gorm:"type:jsonb" json:"gallery"`
	ProgramsList     []byte         `gorm:"type:jsonb" json:"programs_list"`
	About            []byte         `gorm:"type:jsonb" json:"about"`
	Admissions       []byte         `gorm:"type:jsonb" json:"admissions"`
	AdmissionCards   []byte         `gorm:"type:jsonb" json:"admission_cards"`
	OfferedPrograms  []byte         `gorm:"type:jsonb" json:"offered_programs"`
	Alumni           []byte         `gorm:"type:jsonb" json:"alumni"`
	Departments      []byte         `gorm:"type:jsonb" json:"departments"`
	CollegeReviews   []byte         `gorm:"type:jsonb" json:"college_reviews"`
	AcademicFitScore int            `gorm:"default:5" json:"academic_fit_score"`
	CampusLifeScore  int            `gorm:"default:5" json:"campus_life_score"`
	CareerFitScore   int            `gorm:"default:5" json:"career_fit_score"`
	BalancedFitScore int            `gorm:"default:5" json:"balanced_fit_score"`
	ProfileTags      []byte         `gorm:"type:jsonb" json:"profile_tags"`
}

type scoredScholarship struct {
	Item   Scholarship
	Score  int
	Reason []string
}

type scoredCollege struct {
	Item   College
	Score  int
	Reason []string
}

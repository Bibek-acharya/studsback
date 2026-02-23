package models

import (
	"time"

	"gorm.io/gorm"
)

type College struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	UniversityID     uint           `gorm:"index" json:"university_id"`
	University       University     `gorm:"foreignKey:UniversityID" json:"university,omitempty"`
	Name             string         `gorm:"not null;index" json:"name" binding:"required"`
	FullName         string         `json:"full_name,omitempty"`
	Location         string         `gorm:"not null" json:"location" binding:"required"`
	Affiliation      string         `json:"affiliation"` // e.g., TU, Pokhara Uni
	CollegeType      string         `json:"type"`        // Public or Private
	Verified         bool           `gorm:"default:false" json:"verified"`
	Popular          bool           `gorm:"default:false" json:"popular"`
	Rating           float64        `json:"rating"` // e.g., 4.2
	Reviews          int            `json:"reviews"`
	Programs         int            `json:"programs"` // number of programs offered
	Established      string         `json:"established,omitempty"`
	Students         string         `json:"students,omitempty"`
	Description      string         `gorm:"type:text" json:"description,omitempty"`
	Website          string         `json:"website,omitempty"`
	Email            string         `json:"email,omitempty"`
	Phone            string         `json:"phone,omitempty"`
	ImageURL         string         `json:"image_url,omitempty"`
	FeaturedPrograms []byte         `gorm:"type:jsonb" json:"featured_programs,omitempty"` // Array of program names
	Amenities        []byte         `gorm:"type:jsonb" json:"amenities,omitempty"`         // Array of amenities
	Courses          []byte         `gorm:"type:jsonb" json:"courses,omitempty"`           // Array of courses
	Scholarships     []byte         `gorm:"type:jsonb" json:"scholarships,omitempty"`      // Array of scholarships
	Gallery          []byte         `gorm:"type:jsonb" json:"gallery,omitempty"`           // Array of gallery images
	ProgramsList     []byte         `gorm:"type:jsonb" json:"programs_list,omitempty"`     // Array of programs with details
	About            []byte         `gorm:"type:jsonb" json:"about,omitempty"`             // About section with vision, mission, accreditations
	Admissions       []byte         `gorm:"type:jsonb" json:"admissions,omitempty"`        // Admissions info with eligibility, documents, timeline
	AdmissionCards   []byte         `gorm:"type:jsonb" json:"admission_cards,omitempty"`   // Admission cards for frontend admissions section
	OfferedPrograms  []byte         `gorm:"type:jsonb" json:"offered_programs,omitempty"`  // Offered program categories with status
	Alumni           []byte         `gorm:"type:jsonb" json:"alumni,omitempty"`            // Alumni list for frontend alumni section
	Departments      []byte         `gorm:"type:jsonb" json:"departments,omitempty"`       // Array of departments
	CollegeReviews   []byte         `gorm:"type:jsonb" json:"college_reviews,omitempty"`   // Array of reviews
}

// CreateCollegeRequest represents college creation input
type CreateCollegeRequest struct {
	UniversityID     uint     `json:"university_id" binding:"required"`
	Name             string   `json:"name" binding:"required"`
	FullName         string   `json:"full_name"`
	Location         string   `json:"location" binding:"required"`
	Affiliation      string   `json:"affiliation"`
	CollegeType      string   `json:"type"`
	Verified         bool     `json:"verified"`
	Popular          bool     `json:"popular"`
	Rating           float64  `json:"rating"`
	Reviews          int      `json:"reviews"`
	Programs         int      `json:"programs"`
	Established      string   `json:"established"`
	Students         string   `json:"students"`
	Description      string   `json:"description"`
	Website          string   `json:"website"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone"`
	ImageURL         string   `json:"image_url"`
	FeaturedPrograms []string `json:"featured_programs"`
	Amenities        []string `json:"amenities"`
}

// UpdateCollegeRequest represents college update input
type UpdateCollegeRequest struct {
	UniversityID     *uint    `json:"university_id"`
	Name             string   `json:"name"`
	FullName         string   `json:"full_name"`
	Location         string   `json:"location"`
	Affiliation      string   `json:"affiliation"`
	CollegeType      string   `json:"type"`
	Verified         *bool    `json:"verified"`
	Popular          *bool    `json:"popular"`
	Rating           *float64 `json:"rating"`
	Reviews          *int     `json:"reviews"`
	Programs         *int     `json:"programs"`
	Established      string   `json:"established"`
	Students         string   `json:"students"`
	Description      string   `json:"description"`
	Website          string   `json:"website"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone"`
	ImageURL         string   `json:"image_url"`
	FeaturedPrograms []string `json:"featured_programs"`
	Amenities        []string `json:"amenities"`
}

package college

import (
	"time"

	"gorm.io/gorm"
)

type College struct {
	ID                       uint           `gorm:"primaryKey" json:"id"`
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at"`
	DeletedAt                gorm.DeletedAt `gorm:"index" json:"-"`
	Name                     string         `gorm:"not null;index" json:"name"`
	FullName                 string         `json:"full_name,omitempty"`
	Location                 string         `gorm:"not null" json:"location"`
	Affiliation              string         `json:"affiliation"`
	CollegeType              string         `json:"type"`
	Verified                 bool           `gorm:"default:false" json:"verified"`
	Claimed                  bool           `gorm:"default:false" json:"claimed"`
	Popular                  bool           `gorm:"default:false" json:"popular"`
	Featured                 bool           `gorm:"default:false;index" json:"featured"`
	Rating                   float64        `json:"rating"`
	Reviews                  int            `json:"reviews"`
	Programs                 int            `json:"programs"`
	Established              string         `json:"established,omitempty"`
	Students                 string         `json:"students,omitempty"`
	Description              string         `gorm:"type:text" json:"description,omitempty"`
	Website                  string         `json:"website,omitempty"`
	Email                    string         `json:"email,omitempty"`
	Phone                    string         `json:"phone,omitempty"`
	ImageURL                 string         `json:"image_url,omitempty"`
	FeaturedPrograms         []byte         `gorm:"type:jsonb" json:"featured_programs,omitempty"`
	Amenities                []byte         `gorm:"type:jsonb" json:"amenities,omitempty"`
	Courses                  []byte         `gorm:"type:jsonb" json:"courses,omitempty"`
	Scholarships             []byte         `gorm:"type:jsonb" json:"scholarships,omitempty"`
	Gallery                  []byte         `gorm:"type:jsonb" json:"gallery,omitempty"`
	ProgramsList             []byte         `gorm:"type:jsonb" json:"programs_list,omitempty"`
	About                    []byte         `gorm:"type:jsonb" json:"about,omitempty"`
	Admissions               []byte         `gorm:"type:jsonb" json:"admissions,omitempty"`
	AdmissionCards           []byte         `gorm:"type:jsonb" json:"admission_cards,omitempty"`
	OfferedPrograms          []byte         `gorm:"type:jsonb" json:"offered_programs,omitempty"`
	Alumni                   []byte         `gorm:"type:jsonb" json:"alumni,omitempty"`
	Departments              []byte         `gorm:"type:jsonb" json:"departments,omitempty"`
	CollegeReviews           []byte         `gorm:"type:jsonb" json:"college_reviews,omitempty"`
	AcademicFitScore         int            `gorm:"default:5" json:"academic_fit_score"`
	CampusLifeScore          int            `gorm:"default:5" json:"campus_life_score"`
	CareerFitScore           int            `gorm:"default:5" json:"career_fit_score"`
	BalancedFitScore         int            `gorm:"default:5" json:"balanced_fit_score"`
	ProfileTags              []byte         `gorm:"type:jsonb" json:"profile_tags,omitempty"`
	Latitude                 *float64       `gorm:"index:idx_college_lat_lng,priority:1" json:"latitude,omitempty"`
	Longitude                *float64       `gorm:"index:idx_college_lat_lng,priority:2" json:"longitude,omitempty"`
	UniversityAffiliations   []byte         `gorm:"type:jsonb;default:'[]'" json:"university_affiliations,omitempty"`
	NonUniversityAffiliation string         `gorm:"default:''" json:"non_university_affiliation,omitempty"`
}

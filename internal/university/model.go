package university

import (
	"time"

	"gorm.io/gorm"
)

type University struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Name           string         `gorm:"uniqueIndex:idx_university_name;not null" json:"name"`
	Logo           string         `json:"logo,omitempty"`
	Location       string         `json:"location,omitempty"`
	Type           string         `json:"type,omitempty"`
	IsNepali       bool           `gorm:"default:true" json:"is_nepali"`
	Rank           int            `json:"rank"`
	Rating         float64        `json:"rating"`
	ReviewCount    int            `json:"review_count"`
	Verified       bool           `gorm:"default:false" json:"verified"`
	Popular        bool           `gorm:"default:false" json:"popular"`
	Status         string         `gorm:"default:'published'" json:"status,omitempty"`
	ProgramsCount      int            `gorm:"default:0" json:"programsCount"`
	CollegesCount      int            `gorm:"default:0" json:"collegesCount"`
	Description        string         `gorm:"type:text" json:"description,omitempty"`
	Established    string         `json:"established,omitempty"`
	Students       string         `json:"students,omitempty"`
	Chancellor     string         `json:"chancellor,omitempty"`
	ViceChancellor string         `json:"vice_chancellor,omitempty"`
	Founder        string         `json:"founder,omitempty"`
	Website        string         `json:"website,omitempty"`
	Cover          string         `json:"cover,omitempty"`
	About          []byte         `gorm:"type:jsonb" json:"about,omitempty"`
	Contact        []byte         `gorm:"type:jsonb" json:"contact,omitempty"`
	Quick          []byte         `gorm:"type:jsonb" json:"quick,omitempty"`
	Overview       []byte         `gorm:"type:jsonb" json:"overview,omitempty"`
	Leadership     []byte         `gorm:"type:jsonb" json:"leadership,omitempty"`
	Courses        []byte         `gorm:"type:jsonb" json:"courses,omitempty"`
	Programs       []byte         `gorm:"type:jsonb" json:"programs,omitempty"`
	Scholarships   []byte         `gorm:"type:jsonb" json:"scholarships,omitempty"`
	Events         []byte         `gorm:"type:jsonb" json:"events,omitempty"`
	News           []byte         `gorm:"type:jsonb" json:"news,omitempty"`
	Downloads      []byte         `gorm:"type:jsonb" json:"downloads,omitempty"`
	Gallery        []byte         `gorm:"type:jsonb" json:"gallery,omitempty"`
	Faculties      []byte         `gorm:"type:jsonb" json:"faculties,omitempty"`
	Admissions     []byte         `gorm:"type:jsonb" json:"admissions,omitempty"`
	OfficialNotices []byte        `gorm:"type:jsonb" json:"official_notices,omitempty"`
	LatestNews     []byte         `gorm:"type:jsonb" json:"latest_news,omitempty"`
	Reviews        []byte         `gorm:"type:jsonb" json:"reviews,omitempty"`
}

type College struct {
	ID               uint    `json:"id"`
	Name             string  `json:"name"`
	ImageURL         string  `json:"image_url"`
	Rating           float64 `json:"rating"`
	Reviews          int     `json:"reviews"`
	CollegeType      string  `json:"type"`
	Programs         int     `json:"programs"`
	FeaturedPrograms []byte  `gorm:"type:jsonb" json:"featured_programs,omitempty"`
}

type CollegeUniversityCourse struct {
	ID           uint `gorm:"primarykey" json:"id"`
	CollegeID    uint `gorm:"not null;index" json:"college_id"`
	UniversityID uint `gorm:"not null;index" json:"university_id"`
	CourseID     uint `gorm:"not null;index" json:"course_id"`
}

type AffiliatedCollege struct {
	ID          uint    `json:"id"`
	Source      string  `json:"source"`
	Name        string  `json:"name"`
	CollegeID   uint    `json:"college_id"`
	ImageURL    string  `json:"image_url"`
	Location    string  `json:"location"`
	Type        string  `json:"type"`
	Rating      float64 `json:"rating"`
	Reviews     int     `json:"reviews"`
	Programs    int     `json:"programs"`
	Verified    bool    `json:"verified"`
	Featured    bool    `json:"featured"`
	Affiliation string  `json:"affiliation"`
	Website     string  `json:"website"`
}

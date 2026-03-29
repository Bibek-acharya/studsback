package models

import (
	"time"
	"gorm.io/gorm"
)


type University struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"uniqueIndex;not null" json:"name"`
	Logo        string         `json:"logo,omitempty"`
	Location    string         `json:"location,omitempty"`
	Type        string         `json:"type,omitempty"`
	Rank           int            `json:"rank"`
	Rating         float64        `json:"rating"`
	ReviewCount    int            `json:"review_count"`
	Verified        bool           `gorm:"default:false" json:"verified"`
	Popular        bool           `gorm:"default:false" json:"popular"`
	Description    string         `gorm:"type:text" json:"description,omitempty"`
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
	Reviews        []byte         `gorm:"type:jsonb" json:"reviews,omitempty"`
	Colleges       []College      `gorm:"foreignKey:UniversityID" json:"colleges,omitempty"`
}

// CreateUniversityRequest represents university creation input
type CreateUniversityRequest struct {
	Name           string  `json:"name" binding:"required"`
	Logo           string  `json:"logo"`
	Location       string  `json:"location" binding:"required"`
	Type           string  `json:"type"`
	Rank           int     `json:"rank"`
	Rating         float64 `json:"rating"`
	ReviewCount    int     `json:"review_count"`
	Verified       bool    `json:"verified"`
	Popular        bool    `json:"popular"`
	Description    string  `json:"description"`
	Established    string  `json:"established"`
	Students       string  `json:"students"`
	Chancellor     string  `json:"chancellor"`
	ViceChancellor string  `json:"vice_chancellor"`
	Founder        string  `json:"founder"`
	Website        string  `json:"website"`
	Cover          string  `json:"cover"`
}

// UpdateUniversityRequest represents university update input
type UpdateUniversityRequest struct {
	Name           *string  `json:"name"`
	Logo           *string  `json:"logo"`
	Location       *string  `json:"location"`
	Type           *string  `json:"type"`
	Rank           *int     `json:"rank"`
	Rating         *float64 `json:"rating"`
	ReviewCount    *int     `json:"review_count"`
	Verified       *bool    `json:"verified"`
	Popular        *bool    `json:"popular"`
	Description    *string  `json:"description"`
	Established    *string  `json:"established"`
	Students       *string  `json:"students"`
	Chancellor     *string  `json:"chancellor"`
	ViceChancellor *string  `json:"vice_chancellor"`
	Founder        *string  `json:"founder"`
	Website        *string  `json:"website"`
	Cover          *string  `json:"cover"`
}

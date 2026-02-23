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
	Rank        int            `json:"rank"`
	Popular     bool           `gorm:"default:false" json:"popular"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	Established string         `json:"established,omitempty"`
	Website     string         `json:"website,omitempty"`
	Colleges    []College      `gorm:"foreignKey:UniversityID" json:"-"`
}

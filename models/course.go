package models

import (
	"time"

	"gorm.io/gorm"
)

type Course struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	Title         string         `gorm:"not null" json:"title" binding:"required"`
	ShortTitle    string         `json:"shortTitle"`
	CollegesCount int            `json:"collegesCount"`
	Affiliation   string         `json:"affiliation"`
	Badges        []byte         `gorm:"type:jsonb" json:"badges"` // Array of strings
	Level         string         `json:"level"`
	Field         string         `json:"field"`
	Duration      string         `json:"duration"`
	EstFee        string         `json:"estFee"`
	Highlights    []byte         `gorm:"type:jsonb" json:"highlights"` // Array of strings
	CareerPath    string         `json:"careerPath"`
	Description   string         `gorm:"type:text" json:"description"`
	Location      string         `json:"location"`
	GovtFee       string         `json:"govtFee"`
	PrivateFee    string         `json:"privateFee"`
	Mode          string         `json:"mode"`
	DegreeLabel   string         `json:"degreeLabel"`
	About         []byte         `gorm:"type:jsonb" json:"about"`
	Curriculum    []byte         `gorm:"type:jsonb" json:"curriculum"`
	Admissions    []byte         `gorm:"type:jsonb" json:"admissions"`
	Careers       []byte         `gorm:"type:jsonb" json:"careers"`
}

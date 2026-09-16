package system

import (
	"time"

	"studsphere/backend/internal/notification"

	"gorm.io/gorm"
)

type ContactInquiry struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID *uint          `gorm:"index" json:"institution_id"`
	Name          string         `gorm:"not null" json:"name"`
	Email         string         `gorm:"not null" json:"email"`
	Phone         string         `json:"phone"`
	Subject       string         `json:"subject"`
	Message       string         `gorm:"type:text" json:"message"`
	Type          string         `gorm:"default:'general'" json:"type"`
	Status        string         `gorm:"default:'new'" json:"status"`
}

type AdCollege struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	ImageURL string  `json:"image_url"`
	Rating   float64 `json:"rating"`
	Location string  `json:"location"`
	Website  string  `json:"website"`
}

type AdCourse struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Level       string `json:"level"`
	Duration    string `json:"duration"`
	FieldStudy  string `json:"field_of_study"`
	BannerURL   string `json:"banner_url"`
	EstFee      string `json:"est_fee"`
	Affiliation string `json:"affiliation"`
}

type Ad struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Title       string         `gorm:"not null" json:"title"`
	ImageURL    string         `json:"image_url"`
	LinkURL     string         `json:"link_url"`
	Location    string         `json:"location"`
	Page        string         `gorm:"index" json:"page"`
	Position    string         `json:"position"`
	StartDate   time.Time      `json:"start_date"`
	EndDate     time.Time      `json:"end_date"`
	Active      bool           `gorm:"default:true;index" json:"active"`
	Clicks      int            `gorm:"default:0" json:"clicks"`
	Impressions int            `gorm:"default:0" json:"impressions"`
	Priority    int            `gorm:"default:0" json:"priority"`
	CollegeID   *uint          `gorm:"index" json:"college_id"`
	CourseID    *uint          `gorm:"index" json:"course_id"`
	Description string         `gorm:"type:text" json:"description"`
	Accent      string         `gorm:"size:7" json:"accent"`
	College     *AdCollege     `gorm:"-" json:"-"`
	Course      *AdCourse      `gorm:"-" json:"-"`
}

// PublicNotification moved to internal/notification (Task 13); the alias
// keeps the system module's guest GET and the cross-module emit sites
// compiling against the same table and struct.
type PublicNotification = notification.PublicNotification

type CarouselSlide struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Page        string         `gorm:"index;default:'landing'" json:"page"`
	Title       string         `json:"title"`
	Subtitle    string         `json:"subtitle"`
	Description string         `gorm:"type:text" json:"description"`
	ImageURL    string         `json:"image_url"`
	LinkURL     string         `json:"link_url"`
	ButtonText  string         `json:"button_text"`
	Order       int            `gorm:"default:0;index" json:"order"`
	Active      bool           `gorm:"default:true;index" json:"active"`
}

type LandingCourseField struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	FieldOfStudy string    `gorm:"column:field_of_study;uniqueIndex;not null" json:"field_of_study"`
	DisplayOrder int       `gorm:"column:display_order;default:0" json:"display_order"`
	IsActive     bool      `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type LandingCourseInstitution struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	FieldID         uint      `gorm:"column:field_id;index;not null" json:"field_id"`
	InstitutionID   uint      `gorm:"column:institution_id;not null" json:"institution_id"`
	InstitutionType string    `gorm:"column:institution_type;size:20;default:'institution'" json:"institution_type"`
	InstitutionName string    `gorm:"column:institution_name;default:''" json:"institution_name"`
	InstitutionLogo string    `gorm:"column:institution_logo;type:text" json:"institution_logo"`
	Slug            string    `gorm:"column:slug;default:''" json:"slug"`
	OrderIndex      int       `gorm:"column:order_index;default:0" json:"order_index"`
	CreatedAt       time.Time `json:"created_at"`
}

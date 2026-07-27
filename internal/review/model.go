package review

import (
	"time"

	"gorm.io/gorm"
)

type Review struct {
	ID                uint           `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
	UserID            uint           `gorm:"index;not null" json:"user_id"`
	CollegeID         uint           `gorm:"index" json:"college_id"`
	UniversityID      uint           `gorm:"index" json:"university_id"`
	CollegeName       string         `json:"college_name"`
	StudentType       string         `gorm:"not null" json:"student_type"`
	Course            string         `gorm:"not null" json:"course"`
	Level             string         `gorm:"not null" json:"level"`
	BatchYear         int            `gorm:"not null" json:"batch_year"`
	Ratings           []byte         `gorm:"type:jsonb;not null" json:"ratings"`
	Pros              string         `gorm:"type:text;not null" json:"pros"`
	Cons              string         `gorm:"type:text;not null" json:"cons"`
	SummaryTitle      string         `gorm:"not null" json:"summary_title"`
	YearlyFee         *float64       `json:"yearly_fee"`
	Scholarship       *bool          `json:"scholarship"`
	InternshipOutcome *string        `json:"internship_outcome"`
	Email             string         `json:"email"`
	IsVerified        bool           `gorm:"default:false" json:"is_verified"`
	IsPublished       bool           `gorm:"default:true" json:"is_published"`
	HelpfulCount      int            `gorm:"default:0" json:"helpful_count"`
}

type ReviewHelpful struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	ReviewID  uint           `gorm:"index;not null" json:"review_id"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
}

type ReviewReport struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	ReviewID  uint           `gorm:"index;not null" json:"review_id"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	Reason    string         `gorm:"type:text;not null" json:"reason"`
}

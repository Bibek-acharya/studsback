package models

import (
	"time"

	"gorm.io/gorm"
)

type ProviderScholarship struct {
	ID                  uint           `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID          uint           `gorm:"index;not null" json:"provider_id"`
	Title               string         `gorm:"not null" json:"title" binding:"required"`
	Description         string         `gorm:"type:text" json:"description"`
	ImageURL            *string        `json:"image_url"`
	Location            string         `json:"location"`
	Value               string         `json:"value"`
	Deadline            time.Time      `json:"deadline"`
	DegreeLevel         string         `json:"degree_level"`
	FundingType         string         `json:"funding_type"`
	ScholarshipType     string         `json:"scholarship_type"`
	FieldOfStudy        []byte         `gorm:"type:jsonb" json:"field_of_study"`
	EligibilityCriteria []byte         `gorm:"type:jsonb" json:"eligibility_criteria"`
	RequiredDocuments   []byte         `gorm:"type:jsonb" json:"required_documents"`
	Status              string         `gorm:"default:'draft'" json:"status"`
	ApplicationsCount   int            `gorm:"default:0" json:"applications_count"`
}

type ProviderApplication struct {
	ID                uint                `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	DeletedAt         gorm.DeletedAt      `gorm:"index" json:"-"`
	ScholarshipID     uint                `gorm:"index" json:"scholarship_id"`
	Scholarship       ProviderScholarship `gorm:"foreignKey:ScholarshipID" json:"scholarship,omitempty"`
	UserID            uint                `gorm:"index" json:"user_id"`
	FirstName         string              `json:"first_name"`
	LastName          string              `json:"last_name"`
	Email             string              `json:"email"`
	PhoneNumber       string              `json:"phone_number"`
	Status            string              `gorm:"default:'pending'" json:"status"`
	EvaluationNotes   string              `gorm:"type:text" json:"evaluation_notes"`
	Documents         []byte              `gorm:"type:jsonb" json:"documents"`
	PersonalStatement string              `gorm:"type:text" json:"personal_statement"`
}

type ProviderInterview struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	ApplicationID uint           `gorm:"index" json:"application_id"`
	ProviderID    uint           `gorm:"index" json:"provider_id"`
	ScheduledAt   time.Time      `json:"scheduled_at"`
	Duration      int            `json:"duration"`
	Type          string         `json:"type"`
	Location      string         `json:"location"`
	Link          string         `json:"link"`
	Status        string         `gorm:"default:'scheduled'" json:"status"`
	Notes         string         `gorm:"type:text" json:"notes"`
}

type ProviderMessage struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID uint           `gorm:"index" json:"provider_id"`
	UserID     uint           `gorm:"index" json:"user_id"`
	Subject    string         `json:"subject"`
	Content    string         `gorm:"type:text" json:"content"`
	Read       bool           `gorm:"default:false" json:"read"`
	Direction  string         `json:"direction"`
}

type ProviderSettings struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProviderID  uint      `gorm:"uniqueIndex;not null" json:"provider_id"`
	EmailNotifs bool      `gorm:"default:true" json:"email_notifications"`
	SmsNotifs   bool      `gorm:"default:false" json:"sms_notifications"`
	AutoReject  bool      `gorm:"default:false" json:"auto_reject_expired"`
	Timezone    string    `gorm:"default:'UTC'" json:"timezone"`
	Language    string    `gorm:"default:'en'" json:"language"`
}

type ProviderNotification struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID uint           `gorm:"index;not null" json:"provider_id"`
	Title      string         `json:"title"`
	Message    string         `gorm:"type:text" json:"message"`
	Type       string         `json:"type"`
	Read       bool           `gorm:"default:false" json:"read"`
	Link       string         `json:"link"`
}

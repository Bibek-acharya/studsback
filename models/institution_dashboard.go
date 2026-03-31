package models

import (
	"time"

	"gorm.io/gorm"
)

type InstitutionProgram struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null" json:"institution_id"`
	Name          string         `gorm:"not null" json:"name"`
	Description   string         `gorm:"type:text" json:"description"`
	Duration      string         `json:"duration"`
	Fee           string         `json:"fee"`
	Eligibility   string         `gorm:"type:text" json:"eligibility"`
	Capacity      int            `json:"capacity"`
	Status        string         `gorm:"default:'active'" json:"status"`
}

type InstitutionMedia struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	InstitutionID uint      `gorm:"index;not null" json:"institution_id"`
	URL           string    `json:"url"`
	Type          string    `json:"type"`
	Title         string    `json:"title"`
}

type InstitutionCounsellingSession struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null" json:"institution_id"`
	Title         string         `json:"title"`
	Description   string         `gorm:"type:text" json:"description"`
	ScheduledAt   time.Time      `json:"scheduled_at"`
	Duration      int            `json:"duration"`
	MaxSeats      int            `json:"max_seats"`
	BookedSeats   int            `gorm:"default:0" json:"booked_seats"`
	Status        string         `gorm:"default:'scheduled'" json:"status"`
}

type InstitutionCounsellingBooking struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	SessionID uint           `gorm:"index" json:"session_id"`
	UserID    uint           `gorm:"index" json:"user_id"`
	Status    string         `gorm:"default:'pending'" json:"status"`
	Notes     string         `gorm:"type:text" json:"notes"`
}

type InstitutionEntrance struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null" json:"institution_id"`
	Title         string         `gorm:"not null" json:"title"`
	Description   string         `gorm:"type:text" json:"description"`
	Date          time.Time      `json:"date"`
	Duration      int            `json:"duration"`
	TotalSeats    int            `json:"total_seats"`
	FilledSeats   int            `gorm:"default:0" json:"filled_seats"`
	Status        string         `gorm:"default:'upcoming'" json:"status"`
}

type InstitutionEntranceApplicant struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	EntranceID uint           `gorm:"index" json:"entrance_id"`
	UserID     uint           `gorm:"index" json:"user_id"`
	Status     string         `gorm:"default:'registered'" json:"status"`
	Score      float64        `json:"score"`
	Rank       int            `json:"rank"`
}

type InstitutionEvent struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null" json:"institution_id"`
	Title         string         `gorm:"not null" json:"title"`
	Description   string         `gorm:"type:text" json:"description"`
	Date          time.Time      `json:"date"`
	Location      string         `json:"location"`
	Image         string         `json:"image"`
	Status        string         `gorm:"default:'upcoming'" json:"status"`
}

type InstitutionNews struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null" json:"institution_id"`
	Title         string         `gorm:"not null" json:"title"`
	Content       string         `gorm:"type:text" json:"content"`
	Excerpt       string         `json:"excerpt"`
	Image         string         `json:"image"`
	Category      string         `json:"category"`
	Published     bool           `gorm:"default:true" json:"published"`
}

type InstitutionQMS struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null" json:"institution_id"`
	Title         string         `gorm:"not null" json:"title"`
	Description   string         `gorm:"type:text" json:"description"`
	Category      string         `json:"category"`
	Status        string         `gorm:"default:'pending'" json:"status"`
	Score         float64        `json:"score"`
	Documents     []byte         `gorm:"type:jsonb" json:"documents"`
}

type InstitutionMessage struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index" json:"institution_id"`
	UserID        uint           `gorm:"index" json:"user_id"`
	Subject       string         `json:"subject"`
	Content       string         `gorm:"type:text" json:"content"`
	Read          bool           `gorm:"default:false" json:"read"`
	Direction     string         `json:"direction"`
}

type InstitutionSettings struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	InstitutionID uint      `gorm:"uniqueIndex;not null" json:"institution_id"`
	EmailNotifs   bool      `gorm:"default:true" json:"email_notifications"`
	Timezone      string    `gorm:"default:'UTC'" json:"timezone"`
	Language      string    `gorm:"default:'en'" json:"language"`
	PublicProfile bool      `gorm:"default:true" json:"public_profile"`
}

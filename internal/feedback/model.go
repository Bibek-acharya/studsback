package feedback

import (
	"time"

	"gorm.io/gorm"
)

type Feedback struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	UserID      uint           `gorm:"index;not null" json:"user_id"`
	Rating      int            `gorm:"not null" json:"rating"`
	Experience  string         `gorm:"type:text;default:''" json:"experience"`
	Designation string         `gorm:"default:''" json:"designation"`
	Email       string         `gorm:"default:''" json:"email"`
}

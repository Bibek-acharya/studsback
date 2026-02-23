package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Preferences struct {
	Role           string                 `json:"role"`            // student or job_seeker
	PreferenceFlow string                 `json:"preference_flow"` // flowA, flowB, flowFresher, flowExperienced
	Preferences    map[string]interface{} `json:"preferences"`     // key-value pair of preference ID and answer
	CompletedAt    *time.Time             `json:"completed_at"`
}

type User struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Email       string         `gorm:"uniqueIndex;not null" json:"email" binding:"required,email"`
	Password    string         `gorm:"not null" json:"-"`
	FirstName   string         `gorm:"not null" json:"first_name" binding:"required"`
	LastName    string         `gorm:"not null" json:"last_name" binding:"required"`
	Role        string         `gorm:"default:'student'" json:"role"`                                          // student, job_seeker, teacher, admin
	Preferences *Preferences   `gorm:"type:jsonb;serializer:json;default:'null'" json:"preferences,omitempty"` // user preferences from onboarding
}

// HashPassword hashes the user password
func (u *User) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword verifies the password
func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
}

// RegisterRequest represents registration input
type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Role      string `json:"role"`
}

// LoginRequest represents login input
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// SavePreferencesRequest represents preference save input
type SavePreferencesRequest struct {
	PreferenceRole string                 `json:"preference_role" binding:"required"`
	PreferenceFlow string                 `json:"preference_flow" binding:"required"`
	Preferences    map[string]interface{} `json:"preferences" binding:"required"`
}

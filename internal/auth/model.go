package auth

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Preferences struct {
	Role                string                 `json:"role"`
	PreferenceFlow      string                 `json:"preference_flow"`
	Preferences         map[string]interface{} `json:"preferences"`
	CompletedAt         *time.Time             `json:"completed_at"`
	OnboardingCompleted bool                   `json:"onboarding_completed"`
}

type User struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Email       string         `gorm:"uniqueIndex;not null" json:"email" binding:"required,email"`
	Password    *string        `json:"-"`
	FirstName   string         `gorm:"not null" json:"first_name" binding:"required"`
	LastName    string         `gorm:"not null" json:"last_name" binding:"required"`
	GoogleID    *string        `gorm:"uniqueIndex;default:null" json:"google_id"`
	Role        string         `gorm:"default:'student'" json:"role"`
	Preferences *Preferences   `gorm:"type:jsonb;serializer:json;default:'null'" json:"preferences,omitempty"`
}

func (u *User) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hashedStr := string(hashedPassword)
	u.Password = &hashedStr
	return nil
}

func (u *User) CheckPassword(password string) error {
	if u.Password == nil {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(password))
}

type InstitutionUser struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionName    string         `gorm:"not null" json:"institution_name" binding:"required"`
	RegistrationNumber string         `gorm:"uniqueIndex;not null" json:"registration_number" binding:"required"`
	Email              string         `gorm:"uniqueIndex;not null" json:"email" binding:"required,email"`
	GoogleID           *string        `gorm:"uniqueIndex;default:null" json:"google_id"`
	Password           *string        `json:"-"`
	Role               string         `gorm:"default:'institution'" json:"role"`
}

func (u *InstitutionUser) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hashedStr := string(hashedPassword)
	u.Password = &hashedStr
	return nil
}

func (u *InstitutionUser) CheckPassword(password string) error {
	if u.Password == nil {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(password))
}

type ScholarshipProviderUser struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderName       string         `gorm:"not null" json:"provider_name" binding:"required"`
	RegistrationNumber string         `gorm:"uniqueIndex;not null" json:"registration_number" binding:"required"`
	Email              string         `gorm:"uniqueIndex;not null" json:"email" binding:"required,email"`
	GoogleID           *string        `gorm:"uniqueIndex;default:null" json:"google_id"`
	Password           *string        `json:"-"`
	Role               string         `gorm:"default:'scholarship_provider'" json:"role"`
}

func (u *ScholarshipProviderUser) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hashedStr := string(hashedPassword)
	u.Password = &hashedStr
	return nil
}

func (u *ScholarshipProviderUser) CheckPassword(password string) error {
	if u.Password == nil {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(password))
}

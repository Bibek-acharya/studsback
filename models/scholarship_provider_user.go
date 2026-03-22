package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ScholarshipProviderUser struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderName       string         `gorm:"not null" json:"provider_name" binding:"required"`
	RegistrationNumber string         `gorm:"uniqueIndex;not null" json:"registration_number" binding:"required"`
	Email              string         `gorm:"uniqueIndex;not null" json:"email" binding:"required,email"`
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

type ScholarshipProviderRegisterRequest struct {
	ProviderName       string `json:"provider_name" binding:"required"`
	RegistrationNumber string `json:"registration_number" binding:"required"`
	Email              string `json:"email" binding:"required,email"`
	Password           string `json:"password" binding:"required,min=6"`
}

type ScholarshipProviderLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

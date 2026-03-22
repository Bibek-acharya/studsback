package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type InstitutionUser struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionName    string         `gorm:"not null" json:"institution_name" binding:"required"`
	RegistrationNumber string         `gorm:"uniqueIndex;not null" json:"registration_number" binding:"required"`
	Email              string         `gorm:"uniqueIndex;not null" json:"email" binding:"required,email"`
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

type InstitutionRegisterRequest struct {
	InstitutionName    string `json:"institution_name" binding:"required"`
	RegistrationNumber string `json:"registration_number" binding:"required"`
	Email              string `json:"email" binding:"required,email"`
	Password           string `json:"password" binding:"required,min=6"`
}

type InstitutionLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

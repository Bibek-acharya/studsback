package seeder

import (
	"errors"
	"log"
	"strings"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/shared/config"

	"gorm.io/gorm"
)

// SeedSuperAdmin ensures an admin credential exists for the dedicated /admin login flow.
func SeedSuperAdmin(db *gorm.DB) error {
	email := strings.TrimSpace(config.AppConfig.SuperAdminEmail)
	password := config.AppConfig.SuperAdminPassword

	if email == "" || password == "" {
		log.Println("Super admin bootstrap skipped (SUPER_ADMIN_EMAIL or SUPER_ADMIN_PASSWORD missing)")
		return nil
	}

	role := strings.TrimSpace(config.AppConfig.SuperAdminRole)
	if role == "" {
		role = "super_admin"
	}

	firstName := strings.TrimSpace(config.AppConfig.SuperAdminFirst)
	if firstName == "" {
		firstName = "Super"
	}

	lastName := strings.TrimSpace(config.AppConfig.SuperAdminLast)
	if lastName == "" {
		lastName = "Admin"
	}

	var user auth.User
	err := db.Where("email = ?", email).First(&user).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		newUser := auth.User{
			Email:     email,
			FirstName: firstName,
			LastName:  lastName,
			Role:      role,
		}
		if hashErr := newUser.HashPassword(password); hashErr != nil {
			return hashErr
		}

		if createErr := db.Create(&newUser).Error; createErr != nil {
			return createErr
		}

		log.Printf("Super admin bootstrap completed for %s", email)
		return nil
	}

	user.FirstName = firstName
	user.LastName = lastName
	user.Role = role
	if hashErr := user.HashPassword(password); hashErr != nil {
		return hashErr
	}

	if saveErr := db.Save(&user).Error; saveErr != nil {
		return saveErr
	}

	log.Printf("Super admin credential refreshed for %s", email)
	return nil
}

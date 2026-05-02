package seeder

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/shared/config"
)

func TestSeedSuperAdminCreatesAndRefreshesCredential(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&auth.User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	config.AppConfig = &config.Config{
		SuperAdminEmail:    "admin@example.com",
		SuperAdminPassword: "InitialPass123!",
		SuperAdminRole:     "super_admin",
		SuperAdminFirst:    "Super",
		SuperAdminLast:     "Admin",
	}

	if err := SeedSuperAdmin(db); err != nil {
		t.Fatalf("SeedSuperAdmin() create = %v", err)
	}

	var user auth.User
	if err := db.Where("email = ?", "admin@example.com").First(&user).Error; err != nil {
		t.Fatalf("find seeded admin: %v", err)
	}

	if user.Role != "super_admin" {
		t.Fatalf("Role = %q, want %q", user.Role, "super_admin")
	}

	config.AppConfig.SuperAdminPassword = "UpdatedPass456!"
	config.AppConfig.SuperAdminFirst = "Updated"
	config.AppConfig.SuperAdminLast = "Owner"

	if err := SeedSuperAdmin(db); err != nil {
		t.Fatalf("SeedSuperAdmin() refresh = %v", err)
	}

	if err := db.Where("email = ?", "admin@example.com").First(&user).Error; err != nil {
		t.Fatalf("refetch seeded admin: %v", err)
	}

	if user.FirstName != "Updated" {
		t.Fatalf("FirstName = %q, want %q", user.FirstName, "Updated")
	}
	if user.LastName != "Owner" {
		t.Fatalf("LastName = %q, want %q", user.LastName, "Owner")
	}
}

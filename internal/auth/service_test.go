package auth

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"studsphere/backend/internal/shared/config"
)

func TestRejectScholarshipProviderHardDeletesRecord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&User{}, &InstitutionUser{}, &ScholarshipProviderUser{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewRepository(db)
	service := NewService(repo)
	config.AppConfig = &config.Config{
		SMTPHost: "localhost",
		SMTPPort: "1",
		SMTPUser: "test@example.com",
		SMTPPass: "password",
	}

	provider := ScholarshipProviderUser{
		ProviderName:       "Scholarship Nepal",
		RegistrationNumber: "REG-12345",
		Email:              "provider@example.com",
		Status:             "pending",
		Role:               "scholarship_provider",
	}

	if err := repo.CreateScholarshipProviderUser(&provider); err != nil {
		t.Fatalf("create scholarship provider: %v", err)
	}

	if err := service.RejectScholarshipProvider(provider.ID); err != nil {
		t.Fatalf("RejectScholarshipProvider() error = %v", err)
	}

	if _, err := repo.FindScholarshipProviderUserByEmail(provider.Email); err == nil {
		t.Fatalf("expected provider to be deleted, but it still exists")
	}

	if _, err := service.ScholarshipProviderRegister(ScholarshipProviderRegisterRequest{
		ProviderName:       "Scholarship Nepal",
		RegistrationNumber: "REG-12345",
		Email:              "provider@example.com",
	}); err != nil {
		t.Fatalf("ScholarshipProviderRegister() after rejection = %v", err)
	}
}

func TestGoogleLoginOrRegisterWithPicture(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewRepository(db)
	service := NewService(repo)

	pictureURL := "https://lh3.googleusercontent.com/a/test123"

	token, err := service.GoogleLoginOrRegister("google-id-1", "test@example.com", "John", "Doe", pictureURL)
	if err != nil {
		t.Fatalf("GoogleLoginOrRegister() error = %v", err)
	}
	if token == nil {
		t.Fatal("expected non-empty token")
	}

	user, err := repo.FindUserByEmail("test@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail() error = %v", err)
	}
	if user.ImageURL != pictureURL {
		t.Fatalf("ImageURL = %q, want %q", user.ImageURL, pictureURL)
	}
	if user.FirstName != "John" {
		t.Fatalf("FirstName = %q, want %q", user.FirstName, "John")
	}
}

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
	Phone       string         `gorm:"default:''" json:"phone"`
	DateOfBirth string         `gorm:"default:''" json:"date_of_birth"`
	Gender      string         `gorm:"default:''" json:"gender"`
	Nationality string         `gorm:"default:''" json:"nationality"`
	Address     string         `gorm:"type:text;default:''" json:"address"`
	Bio         string         `gorm:"type:text;default:''" json:"bio"`
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
	ContactNumber      string         `json:"contact_number"`
	PANNumber          string         `json:"pan_number"`
	WebsiteURL         string         `json:"website_url"`
	LogoURL            *string        `gorm:"default:null" json:"logo_url"`
	Address            string         `gorm:"default:''" json:"address"`
	AboutText          string         `gorm:"type:text;default:''" json:"about_text"`
	Mission            string         `gorm:"type:text;default:''" json:"mission"`
	Values             string         `gorm:"type:text;default:''" json:"values"`
	GoogleID           *string        `gorm:"uniqueIndex;default:null" json:"google_id"`
	Password           *string        `json:"-"`
	Status             string         `gorm:"default:'pending'" json:"status"`
	Role               string         `gorm:"default:'scholarship_provider'" json:"role"`
	FounderName        string         `gorm:"column:founder_name;default:''" json:"founder_name"`
	FounderRole        string         `gorm:"column:founder_role;default:''" json:"founder_role"`
	FounderMessage     string         `gorm:"column:founder_message;type:text;default:''" json:"founder_message"`
	FounderImageURL    string         `gorm:"column:founder_image_url;default:''" json:"founder_image_url"`
	FacebookURL        string         `gorm:"column:facebook_url;default:''" json:"facebook_url"`
	InstagramURL       string         `gorm:"column:instagram_url;default:''" json:"instagram_url"`
	YoutubeURL         string         `gorm:"column:youtube_url;default:''" json:"youtube_url"`
	LinkedInURL        string         `gorm:"column:linkedin_url;default:''" json:"linkedin_url"`
	MapURL             string         `gorm:"column:map_url;type:text;default:''" json:"map_url"`
	BrochureURL        string         `gorm:"column:brochure_url;default:''" json:"brochure_url"`
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

type EducationEntry struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	UserID           uint           `gorm:"index;not null" json:"user_id"`
	Level            string         `json:"level"`
	InstitutionName  string         `json:"institution_name"`
	BoardUniversity  string         `json:"board_university"`
	Country          string         `json:"country"`
	Stream           string         `json:"stream"`
	StartYear        string         `json:"start_year"`
	EndYear          string         `json:"end_year"`
	GradingSystem    string         `json:"grading_system"`
	Grade            string         `json:"grade"`
}

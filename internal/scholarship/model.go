package scholarship

import (
	"time"

	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

type Scholarship struct {
	ID                       uint             `gorm:"primarykey" json:"id"`
	CreatedAt                time.Time        `json:"created_at"`
	UpdatedAt                time.Time        `json:"updated_at"`
	DeletedAt                gorm.DeletedAt   `gorm:"index" json:"-"`
	Title                    string           `gorm:"not null" json:"title"`
	Provider                 string           `gorm:"not null" json:"provider"`
	Location                 string           `json:"location"`
	Value                    string           `json:"value"`
	Deadline                 time.Time        `json:"deadline"`
	DegreeLevel              string           `json:"degree_level"`
	FundingType              string           `json:"funding_type"`
	ScholarshipType          string           `json:"scholarship_type"`
	Description              string           `gorm:"type:text" json:"description"`
	ImageURL                 string           `json:"image_url"`
	BannerBackgroundImageURL string           `json:"banner_background_image_url"`
	FieldOfStudy             []byte           `gorm:"type:jsonb" json:"field_of_study"`
	SelectionProcess         []byte           `gorm:"type:jsonb" json:"selection_process"`
	EligibilityCriteria      []byte           `gorm:"type:jsonb" json:"eligibility_criteria"`
	ExcludedRegions          []byte           `gorm:"type:jsonb" json:"excluded_regions"`
	RequiredDocuments        []byte           `gorm:"type:jsonb" json:"required_documents"`
	Timeline                 []byte           `gorm:"type:jsonb" json:"timeline"`
	Benefits                 []byte           `gorm:"type:jsonb" json:"benefits"`
	FAQs                     []byte           `gorm:"type:jsonb" json:"faqs"`
	Status                   string           `json:"status" gorm:"default:draft"`
	FormConfig               []byte           `gorm:"type:jsonb" json:"form_config"`
	PaymentConfig            []byte           `gorm:"type:jsonb" json:"payment_config"`
	BankAccountName          string           `json:"bank_account_name"`
	BankAccountNo            string           `json:"bank_account_no"`
	BankName                 string           `json:"bank_name"`
	BankBranch               string           `json:"bank_branch"`
	ProviderScholarshipID    *uint            `gorm:"index" json:"-"`
	Embedding                *pgvector.Vector `gorm:"type:vector(1536)" json:"-"`
}

type ScholarshipApplication struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	ScholarshipID uint           `gorm:"index" json:"scholarship_id"`
	Scholarship   Scholarship    `gorm:"foreignKey:ScholarshipID" json:"scholarship,omitempty"`
	UserID        *uint          `gorm:"index" json:"user_id,omitempty"`
	User          User           `gorm:"foreignKey:UserID" json:"user,omitempty"`

	FullName       string    `gorm:"not null" json:"full_name"`
	Gender         string    `gorm:"not null" json:"gender"`
	Ethnicity      string    `json:"ethnicity"`
	EthnicityOther string    `json:"ethnicity_other"`
	DateOfBirthBS  string    `json:"date_of_birth_bs"`
	DateOfBirthAD  time.Time `json:"date_of_birth_ad"`
	Age            int       `json:"age"`
	PhoneNumber    string    `json:"phone_number"`
	Email          string    `json:"email"`
	PhotoURL       string    `json:"photo_url"`

	SEEGPA             string `json:"see_gpa"`
	SchoolType         string `json:"school_type"`
	SchoolName         string `json:"school_name"`
	SchoolProvince     string `json:"school_province"`
	SchoolDistrict     string `json:"school_district"`
	SchoolMunicipality string `json:"school_municipality"`
	SchoolTole         string `json:"school_tole"`

	Documents []byte `gorm:"type:jsonb" json:"documents"`

	PermanentProvince     string `json:"permanent_province"`
	PermanentDistrict     string `json:"permanent_district"`
	PermanentMunicipality string `json:"permanent_municipality"`
	PermanentWard         string `json:"permanent_ward"`
	PermanentTole         string `json:"permanent_tole"`

	TemporaryProvince     string `json:"temporary_province"`
	TemporaryDistrict     string `json:"temporary_district"`
	TemporaryMunicipality string `json:"temporary_municipality"`
	TemporaryWard         string `json:"temporary_ward"`
	TemporaryTole         string `json:"temporary_tole"`

	GuardianName          string  `json:"guardian_name"`
	GuardianPhone         string  `json:"guardian_phone"`
	GuardianEmail         string  `json:"guardian_email"`
	FatherOccupation      string  `json:"father_occupation"`
	FatherOccupationOther string  `json:"father_occupation_other"`
	MotherOccupation      string  `json:"mother_occupation"`
	MotherOccupationOther string  `json:"mother_occupation_other"`
	FamilyMonthlyIncome   float64 `json:"family_monthly_income"`
	FamilyMembersCount    int     `json:"family_members_count"`

	Stream     string `json:"stream"`
	ExamCenter string `json:"exam_center"`

	Status string `gorm:"default:'pending'" json:"status"`
}

type User struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

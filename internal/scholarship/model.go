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
	TotalSeats               int              `json:"total_seats"`
	AmountPerStudent         float64          `json:"amount_per_student"`
	ApplicationStartDate     time.Time        `json:"application_start_date"`
	ResultPublicationDate    time.Time        `json:"result_publication_date"`
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

	// New fields from ProviderScholarship
	ProviderName             string `gorm:"column:provider_name" json:"provider_name"`
	FundingTypeOther         string `gorm:"column:funding_type_other" json:"funding_type_other"`
	ScholarshipTypeOther     string `gorm:"column:scholarship_type_other" json:"scholarship_type_other"`
	EducationLevel           string `gorm:"column:education_level" json:"education_level"`
	EducationLevelOther      string `gorm:"column:education_level_other" json:"education_level_other"`
	ApplyLink                string `json:"apply_link"`
	CoverageArea             string `json:"coverage_area"`
	ContactEmail             string `json:"contact_email"`
	PrimaryPhone             string `json:"primary_phone"`
	SecondaryPhone           string `json:"secondary_phone"`
	WebsiteUrl               string `json:"website_url"`
	OfficeAddress            string `json:"office_address"`
	MapUrl                   string `json:"map_url"`
	AboutParagraph1          string `gorm:"type:text;column:about_paragraph_1" json:"about_paragraph_1"`
	VideoTutorials           []byte `gorm:"type:jsonb;column:video_tutorials" json:"video_tutorials"`
	JourneyTimeline          []byte `gorm:"type:jsonb;column:journey_timeline" json:"journey_timeline"`
	ScholarshipSectionTitle  string `gorm:"column:scholarship_section_title" json:"scholarship_section_title"`
	ScholarshipSubtitle      string `gorm:"column:scholarship_subtitle" json:"scholarship_subtitle"`
	ScholarshipDescription1  string `gorm:"type:text;column:scholarship_description_1" json:"scholarship_description_1"`
	ScholarshipDescription2  string `gorm:"type:text;column:scholarship_description_2" json:"scholarship_description_2"`
	ScholarshipTypes         []byte `gorm:"type:jsonb;column:scholarship_types" json:"scholarship_types"`
	ScholarshipTypesNew      []byte `gorm:"type:jsonb;column:scholarship_types_new" json:"scholarship_types_new"`
	SelectionRubric          []byte `gorm:"type:jsonb;column:selection_rubric" json:"selection_rubric"`
	SelectionRubricNew       []byte `gorm:"type:jsonb;column:selection_rubric_new" json:"selection_rubric_new"`
	EligibilitySectionTitle  string `gorm:"column:eligibility_section_title" json:"eligibility_section_title"`
	EligibilitySubtitle      string `gorm:"column:eligibility_subtitle" json:"eligibility_subtitle"`
	BasicEligibilityCriteria []byte `gorm:"type:jsonb;column:basic_eligibility_criteria" json:"basic_eligibility_criteria"`
	FullyFundedCriteria      []byte `gorm:"type:jsonb;column:fully_funded_criteria" json:"fully_funded_criteria"`
	PartiallyFundedCriteria  []byte `gorm:"type:jsonb;column:partially_funded_criteria" json:"partially_funded_criteria"`
	SelectionProcessSteps    []byte `gorm:"type:jsonb;column:selection_process_steps" json:"selection_process_steps"`
	FAQsNew                  []byte `gorm:"type:jsonb;column:faqs_new" json:"faqs_new"`
	GalleryImages            []byte `gorm:"type:jsonb;column:gallery_images" json:"gallery_images"`
	GalleryImagesNew         []byte `gorm:"type:jsonb;column:gallery_images_new" json:"gallery_images_new"`
	PartnerGroups            []byte `gorm:"type:jsonb;column:partner_groups" json:"partner_groups"`
	ExamCenters              []byte `gorm:"type:jsonb;column:exam_centers" json:"exam_centers"`
	ExamCentersNew           []byte `gorm:"type:jsonb;column:exam_centers_new" json:"exam_centers_new"`
	Downloads                []byte `gorm:"type:jsonb;column:downloads" json:"downloads"`

	Embedding *pgvector.Vector `gorm:"type:vector(1536)" json:"-"`
}

type ScholarshipApplication struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	DeletedAt     gorm.DeletedAt   `gorm:"index" json:"-"`
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

	PersonalStatement string `gorm:"type:text" json:"personal_statement"`
	Status string `gorm:"default:'pending'" json:"status"`
	Payment *Payment `gorm:"-"`
}

type User struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

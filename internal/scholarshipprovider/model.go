package scholarshipprovider

import (
	"time"

	"gorm.io/gorm"
)

type ProviderScholarship struct {
	ID                  uint           `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID          uint           `gorm:"index;not null" json:"provider_id"`
	Title               string         `gorm:"not null" json:"title"`
	Description         string         `gorm:"type:text" json:"description"`
	ImageURL            *string        `json:"image_url"`
	Location            string         `json:"location"`
	Value               string         `json:"value"`
	Deadline            time.Time      `json:"deadline"`
	DegreeLevel         string         `json:"degree_level"`
	FundingType         string         `json:"funding_type"`
	ScholarshipType     string         `json:"scholarship_type"`
	FieldOfStudy        []byte         `gorm:"type:jsonb" json:"field_of_study"`
	EligibilityCriteria []byte         `gorm:"type:jsonb" json:"eligibility_criteria"`
	RequiredDocuments   []byte         `gorm:"type:jsonb" json:"required_documents"`
	Status              string         `gorm:"default:'draft'" json:"status"`
	ApplicationsCount   int            `gorm:"default:0" json:"applications_count"`

	// Prototype extended fields
	BannerBackgroundImageURL *string   `gorm:"type:text;column:banner_background_image_url" json:"banner_background_image_url"`
	AboutParagraph1          string    `gorm:"type:text;column:about_paragraph1" json:"about_paragraph_1"`
	AboutParagraph2          string    `gorm:"type:text;column:about_paragraph2" json:"about_paragraph_2"`
	VideoTutorials           []byte    `gorm:"type:jsonb;column:video_tutorials" json:"video_tutorials"`
	JourneyTimeline          []byte    `gorm:"type:jsonb;column:journey_timeline" json:"journey_timeline"`
	ScholarshipSectionTitle  string    `gorm:"column:scholarship_section_title" json:"scholarship_section_title"`
	ScholarshipSubtitle      string    `gorm:"column:scholarship_subtitle" json:"scholarship_subtitle"`
	ScholarshipDescription1  string    `gorm:"type:text;column:scholarship_description1" json:"scholarship_description_1"`
	ScholarshipDescription2  string    `gorm:"type:text;column:scholarship_description2" json:"scholarship_description_2"`
	ScholarshipTypes         []byte    `gorm:"type:jsonb;column:scholarship_types" json:"scholarship_types"`
	ScholarshipTypesNew      []byte    `gorm:"type:jsonb;column:scholarship_types_new" json:"scholarship_types_new"`
	SelectionRubric          []byte    `gorm:"type:jsonb;column:selection_rubric" json:"selection_rubric"`
	SelectionRubricNew       []byte    `gorm:"type:jsonb;column:selection_rubric_new" json:"selection_rubric_new"`
	EligibilitySectionTitle  string    `gorm:"column:eligibility_section_title" json:"eligibility_section_title"`
	EligibilitySubtitle      string    `gorm:"column:eligibility_subtitle" json:"eligibility_subtitle"`
	BasicEligibilityCriteria []byte    `gorm:"type:jsonb;column:basic_eligibility_criteria" json:"basic_eligibility_criteria"`
	FullyFundedCriteria      []byte    `gorm:"type:jsonb;column:fully_funded_criteria" json:"fully_funded_criteria"`
	PartiallyFundedCriteria  []byte    `gorm:"type:jsonb;column:partially_funded_criteria" json:"partially_funded_criteria"`
	SelectionProcessSteps    []byte    `gorm:"type:jsonb;column:selection_process_steps" json:"selection_process_steps"`
	FAQs                     []byte    `gorm:"type:jsonb;column:fa_qs" json:"faqs"`
	FAQsNew                  []byte    `gorm:"type:jsonb;column:faqs_new" json:"faqs_new"`
	GalleryImages            []byte    `gorm:"type:jsonb;column:gallery_images" json:"gallery_images"`
	GalleryImagesNew         []byte    `gorm:"type:jsonb;column:gallery_images_new" json:"gallery_images_new"`
	PartnerGroups            []byte    `gorm:"type:jsonb;column:partner_groups" json:"partner_groups"`
	ExamCenters              []byte    `gorm:"type:jsonb;column:exam_centers" json:"exam_centers"`
	ExamCentersNew           []byte    `gorm:"type:jsonb;column:exam_centers_new" json:"exam_centers_new"`
	Downloads                []byte    `gorm:"type:jsonb;column:downloads" json:"downloads"`

	// New fields from prototype
	ProviderName          string    `gorm:"column:provider_name" json:"provider_name"`
	FundingTypeOther     string    `gorm:"column:funding_type_other" json:"funding_type_other"`
	ScholarshipTypeOther string    `gorm:"column:scholarship_type_other" json:"scholarship_type_other"`
	EducationLevel       string    `gorm:"column:education_level" json:"education_level"`
	EducationLevelOther  string    `gorm:"column:education_level_other" json:"education_level_other"`
	ApplyLink            string    `gorm:"column:apply_link" json:"apply_link"`

	// Contact Details
	CoverageArea   string `gorm:"column:coverage_area" json:"coverage_area"`
	ContactEmail   string `gorm:"column:contact_email" json:"contact_email"`
	PrimaryPhone   string `gorm:"column:primary_phone" json:"primary_phone"`
	SecondaryPhone string `gorm:"column:secondary_phone" json:"secondary_phone"`
	WebsiteUrl     string `gorm:"column:website_url" json:"website_url"`
	OfficeAddress  string `gorm:"column:office_address" json:"office_address"`
	MapUrl         string `gorm:"column:map_url" json:"map_url"`

	// Payment Configuration
	PaymentConfig []byte `gorm:"type:jsonb;column:payment_config" json:"payment_config"`
	TotalSeats               int       `json:"total_seats"`
	AmountPerStudent         float64   `json:"amount_per_student"`
	DisbursementType         string    `json:"disbursement_type"`
	ApplicationStartDate     time.Time `json:"application_start_date"`
	ResultPublicationDate    time.Time `json:"result_publication_date"`
	MinGPA                   float64   `json:"min_gpa"`
	EligibleProvinces        []byte    `gorm:"type:jsonb" json:"eligible_provinces"`
	SelectionCriteria        []byte    `gorm:"type:jsonb" json:"selection_criteria"`
	InterviewRounds          int       `json:"interview_rounds"`
	Timeline                 []byte    `gorm:"type:jsonb" json:"timeline"`
	Achievements             []byte    `gorm:"type:jsonb" json:"achievements"`
	SocialLinks              []byte    `gorm:"type:jsonb" json:"social_links"`
	MapEmbedURL              string    `json:"map_embed_url"`
	GuidelinesURL            string    `json:"guidelines_url"`
}

type ProviderApplication struct {
	ID                    uint                `gorm:"primarykey" json:"id"`
	CreatedAt             time.Time           `json:"created_at"`
	UpdatedAt             time.Time           `json:"updated_at"`
	DeletedAt             gorm.DeletedAt      `gorm:"index" json:"-"`
	ScholarshipID         uint                `gorm:"index" json:"scholarship_id"`
	Scholarship           ProviderScholarship `gorm:"foreignKey:ScholarshipID" json:"scholarship,omitempty"`
	UserID                *uint               `gorm:"index" json:"user_id,omitempty"`
	FullName              string              `json:"full_name"`
	FirstName             string              `json:"first_name"`
	LastName              string              `json:"last_name"`
	Email                 string              `json:"email"`
	PhoneNumber           string              `json:"phone_number"`
	Gender                string              `json:"gender"`
	Ethnicity             string              `json:"ethnicity"`
	EthnicityOther        string              `json:"ethnicity_other"`
	DateOfBirthBS         string              `json:"date_of_birth_bs"`
	DateOfBirthAD         time.Time           `json:"date_of_birth_ad"`
	Age                   int                 `json:"age"`
	PhotoURL              string              `json:"photo_url"`
	SEEGPA                string              `json:"see_gpa"`
	SchoolName            string              `json:"school_name"`
	SchoolProvince        string              `json:"school_province"`
	SchoolDistrict        string              `json:"school_district"`
	SchoolMunicipality    string              `json:"school_municipality"`
	SchoolTole            string              `json:"school_tole"`
	PermanentProvince     string              `json:"permanent_province"`
	PermanentDistrict     string              `json:"permanent_district"`
	PermanentMunicipality string              `json:"permanent_municipality"`
	PermanentWard         string              `json:"permanent_ward"`
	PermanentTole         string              `json:"permanent_tole"`
	TemporaryProvince     string              `json:"temporary_province"`
	TemporaryDistrict     string              `json:"temporary_district"`
	TemporaryMunicipality string              `json:"temporary_municipality"`
	TemporaryWard         string              `json:"temporary_ward"`
	TemporaryTole         string              `json:"temporary_tole"`
	GuardianName          string              `json:"guardian_name"`
	GuardianPhone         string              `json:"guardian_phone"`
	GuardianEmail         string              `json:"guardian_email"`
	FatherOccupation      string              `json:"father_occupation"`
	FatherOccupationOther string              `json:"father_occupation_other"`
	MotherOccupation      string              `json:"mother_occupation"`
	MotherOccupationOther string              `json:"mother_occupation_other"`
	FamilyMonthlyIncome   float64             `json:"family_monthly_income"`
	FamilyMembersCount    int                 `json:"family_members_count"`
	Status                string              `gorm:"default:'pending'" json:"status"`
	EvaluationNotes       string              `gorm:"type:text" json:"evaluation_notes"`
	Documents             []byte              `gorm:"type:jsonb" json:"documents"`
	PersonalStatement     string              `gorm:"type:text" json:"personal_statement"`
	Province              string              `json:"province"`
	District              string              `json:"district"`
	Stream                string              `json:"stream"`
	GPA                   float64             `json:"gpa"`
	SchoolType            string              `json:"school_type"`
	ExamCenter            string              `json:"exam_center"`
}

type ProviderInterview struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	ApplicationID uint           `gorm:"index" json:"application_id"`
	ProviderID    uint           `gorm:"index" json:"provider_id"`
	ScheduledAt   time.Time      `json:"scheduled_at"`
	Duration      int            `json:"duration"`
	Type          string         `json:"type"`
	Location      string         `json:"location"`
	Link          string         `json:"link"`
	Status        string         `gorm:"default:'scheduled'" json:"status"`
	Notes         string         `gorm:"type:text" json:"notes"`
}

type ProviderMessage struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID uint           `gorm:"index" json:"provider_id"`
	UserID     uint           `gorm:"index" json:"user_id"`
	Subject    string         `json:"subject"`
	Content    string         `gorm:"type:text" json:"content"`
	Read       bool           `gorm:"default:false" json:"read"`
	Direction  string         `json:"direction"`
}

type ProviderSettings struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProviderID  uint      `gorm:"uniqueIndex;not null" json:"provider_id"`
	EmailNotifs bool      `gorm:"default:true" json:"email_notifications"`
	SmsNotifs   bool      `gorm:"default:false" json:"sms_notifications"`
	AutoReject  bool      `gorm:"default:false" json:"auto_reject_expired"`
	Timezone    string    `gorm:"default:'UTC'" json:"timezone"`
	Language    string    `gorm:"default:'en'" json:"language"`
}

type ProviderNotification struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID uint           `gorm:"index;not null" json:"provider_id"`
	Title      string         `json:"title"`
	Message    string         `gorm:"type:text" json:"message"`
	Type       string         `json:"type"`
	Read       bool           `gorm:"default:false" json:"read"`
	Link       string         `json:"link"`
}

type ScholarshipProviderUser struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderName       string         `gorm:"not null" json:"provider_name"`
	RegistrationNumber string         `gorm:"uniqueIndex;not null" json:"registration_number"`
	Email              string         `gorm:"uniqueIndex;not null" json:"email"`
	ContactNumber      string         `json:"contact_number"`
	PANNumber          string         `json:"pan_number"`
	WebsiteURL         string         `json:"website_url"`
	GoogleID           *string        `gorm:"uniqueIndex;default:null" json:"google_id"`
	Password           *string        `json:"-"`
	Status             string         `gorm:"default:'pending'" json:"status"`
	Role               string         `gorm:"default:'scholarship_provider'" json:"role"`
}

type ProviderNews struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID    uint           `gorm:"index;not null" json:"provider_id"`
	Title         string         `gorm:"not null" json:"title"`
	ShortDesc     string         `gorm:"type:text" json:"short_desc"`
	Content       string         `gorm:"type:text" json:"content"`
	ImageURL      *string        `json:"image_url"`
	NewsType      string         `json:"news_type"`
	PublishedBy   string         `json:"published_by"`
	PublishDate   *time.Time     `json:"publish_date"`
	Tags          []byte         `gorm:"type:jsonb" json:"tags"`
	AllowComments bool           `gorm:"default:false" json:"allow_comments"`
	Status        string         `gorm:"default:'draft'" json:"status"`
	PublishedAt   *time.Time     `json:"published_at"`
}

type ProviderEvent struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID         uint           `gorm:"index;not null" json:"provider_id"`
	Name               string         `gorm:"not null" json:"name"`
	ShortDesc          string         `gorm:"type:text" json:"short_desc"`
	Description        string         `gorm:"type:text" json:"description"`
	ImageURL           *string        `json:"image_url"`
	EventType          string         `json:"event_type"`
	Category           string         `json:"category"`
	MaxParticipants    int            `json:"max_participants"`
	OnlineLink         string         `json:"online_link"`
	OrganizedBy        string         `json:"organized_by"`
	ContactPerson      string         `json:"contact_person"`
	ContactEmail       string         `json:"contact_email"`
	StartDate          time.Time      `json:"start_date"`
	EndDate            time.Time      `json:"end_date"`
	Location           string         `json:"location"`
	Tags               []byte          `gorm:"type:jsonb" json:"tags"`
	EnableRegistration bool           `gorm:"default:false" json:"enable_registration"`
	Status             string          `gorm:"default:'upcoming'" json:"status"`
	Attendees          int            `gorm:"default:0" json:"attendees"`
}

type ProviderBlog struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID  uint           `gorm:"index;not null" json:"provider_id"`
	Title       string         `gorm:"not null" json:"title"`
	Content     string         `gorm:"type:text" json:"content"`
	ImageURL    *string        `json:"image_url"`
	Author      string         `json:"author"`
	Status      string         `gorm:"default:'draft'" json:"status"`
	PublishedAt *time.Time     `json:"published_at"`
	Views       int            `gorm:"default:0" json:"views"`
	Likes       int            `gorm:"default:0" json:"likes"`
}

type ProviderCalendarEvent struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID  uint           `gorm:"index;not null" json:"provider_id"`
	Title       string         `gorm:"not null" json:"title"`
	Description string         `gorm:"type:text" json:"description"`
	StartDate   time.Time      `json:"start_date"`
	EndDate     time.Time      `json:"end_date"`
	Color       string         `json:"color"`
	IsAllDay    bool           `gorm:"default:false" json:"is_all_day"`
}

type ProviderResult struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID    uint           `gorm:"index;not null" json:"provider_id"`
	ScholarshipID uint           `gorm:"index" json:"scholarship_id"`
	Title         string         `gorm:"not null" json:"title"`
	Status        string         `gorm:"default:'draft'" json:"status"`
	PublishedAt   *time.Time     `json:"published_at"`
	Results       []byte         `gorm:"type:jsonb" json:"results"`
}

type ProviderAccess struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID uint           `gorm:"index;not null" json:"provider_id"`
	Email      string         `gorm:"not null" json:"email"`
	Role       string         `gorm:"default:'viewer'" json:"role"`
	Status     string         `gorm:"default:'pending'" json:"status"`
}

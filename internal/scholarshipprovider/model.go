package scholarshipprovider

import (
	"time"

	"gorm.io/gorm"

	"studsphere/backend/internal/shared/slug"
)

type ProviderScholarship struct {
	ID                       uint      `gorm:"primarykey" json:"id"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
	ProviderID               uint      `gorm:"index;not null" json:"provider_id"`
	Slug                     string    `gorm:"uniqueIndex" json:"slug"`
	Title                    string    `gorm:"not null" json:"title"`
	Provider                 string    `gorm:"column:provider" json:"provider"`
	Description              string    `gorm:"type:text" json:"description"`
	ProviderName             string    `gorm:"column:provider_name" json:"provider_name"`
	FundingTypeOther         string    `gorm:"column:funding_type_other" json:"funding_type_other"`
	ScholarshipTypeOther     string    `gorm:"column:scholarship_type_other" json:"scholarship_type_other"`
	EducationLevel           string    `gorm:"column:education_level" json:"education_level"`
	EducationLevelOther      string    `gorm:"column:education_level_other" json:"education_level_other"`
	Location                 string    `json:"location"`
	Value                    string    `json:"value"`
	Deadline                 time.Time `json:"deadline"`
	DegreeLevel              string    `json:"degree_level"`
	FundingType              string    `json:"funding_type"`
	ScholarshipType          string    `json:"scholarship_type"`
	FieldOfStudy             []byte    `gorm:"type:jsonb" json:"field_of_study"`
	Status                   string    `gorm:"default:'draft'" json:"status"`
	ApplicationStartDate     time.Time `json:"application_start_date"`
	ApplicationEndDate       time.Time `json:"application_end_date"`
	ApplyLink                string    `json:"apply_link"`
	ImageURL                 string    `json:"image_url"`
	BannerBackgroundImageURL string    `gorm:"column:banner_background_image_url" json:"banner_background_image_url"`
	CoverageArea             string    `json:"coverage_area"`
	ContactEmail             string    `json:"contact_email"`
	PrimaryPhone             string    `json:"primary_phone"`
	SecondaryPhone           string    `json:"secondary_phone"`
	WebsiteUrl               string    `json:"website_url"`
	OfficeAddress            string    `json:"office_address"`
	MapUrl                   string    `json:"map_url"`
	AboutParagraph1          string    `gorm:"type:text;column:about_paragraph_1" json:"about_paragraph_1"`
	VideoTutorials           []byte    `gorm:"type:jsonb;column:video_tutorials" json:"video_tutorials"`
	JourneyTimeline          []byte    `gorm:"type:jsonb;column:journey_timeline" json:"journey_timeline"`
	Timeline                 []byte    `gorm:"type:jsonb;column:timeline" json:"timeline"`
	ScholarshipSectionTitle  string    `gorm:"column:scholarship_section_title" json:"scholarship_section_title"`
	ScholarshipSubtitle      string    `gorm:"column:scholarship_subtitle" json:"scholarship_subtitle"`
	ScholarshipDescription1  string    `gorm:"type:text;column:scholarship_description_1" json:"scholarship_description_1"`
	ScholarshipDescription2  string    `gorm:"type:text;column:scholarship_description_2" json:"scholarship_description_2"`
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
	RequiredDocuments        []byte    `gorm:"type:jsonb;column:required_documents" json:"required_documents"`
	FAQs                     []byte    `gorm:"type:jsonb;column:faqs" json:"faqs"`
	FAQsNew                  []byte    `gorm:"type:jsonb;column:faqs_new" json:"faqs_new"`
	GalleryImages            []byte    `gorm:"type:jsonb;column:gallery_images" json:"gallery_images"`
	GalleryImagesNew         []byte    `gorm:"type:jsonb;column:gallery_images_new" json:"gallery_images_new"`
	PartnerGroups            []byte    `gorm:"type:jsonb;column:partner_groups" json:"partner_groups"`
	PartnerMessages          []byte    `gorm:"type:jsonb;column:partner_messages" json:"partner_messages"`
	ExamCenters              []byte    `gorm:"type:jsonb;column:exam_centers" json:"exam_centers"`
	ExamCentersNew           []byte    `gorm:"type:jsonb;column:exam_centers_new" json:"exam_centers_new"`
	Downloads                []byte    `gorm:"type:jsonb;column:downloads" json:"downloads"`
	PaymentConfig            []byte    `gorm:"type:jsonb;column:payment_config" json:"payment_config"`

	ExamDate string `gorm:"column:exam_date;default:''" json:"exam_date"`
	ExamTime string `gorm:"column:exam_time;default:''" json:"exam_time"`
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
	EvaluationScore       *int                `gorm:"default:null" json:"evaluation_score"`
	EvaluationPassed      bool                `gorm:"default:false" json:"evaluation_passed"`
	EvaluationNotes       string              `gorm:"type:text" json:"evaluation_notes"`
	Documents             []byte              `gorm:"type:jsonb" json:"documents"`
	PersonalStatement     string              `gorm:"type:text" json:"personal_statement"`
	Province              string              `json:"province"`
	District              string              `json:"district"`
	Stream                string              `json:"stream"`
	GPA                   float64             `json:"gpa"`
	SchoolType            string              `json:"school_type"`
	ExamCenter            string              `json:"exam_center"`
	RollNumber            string              `gorm:"size:20" json:"roll_number"`
	ScholarshipApplicationID *uint             `gorm:"column:scholarship_application_id" json:"scholarship_application_id,omitempty"`
	RejectionReason          string            `gorm:"type:text" json:"rejection_reason"`
	Payment                  *ProviderPayment  `gorm:"-" json:"payment,omitempty"`
}

type ProviderPayment struct {
	ID             uint       `json:"id"`
	Method         string     `json:"method"`
	Amount         float64    `json:"amount"`
	Status         string     `json:"status"`
	ReceiptURL     string     `json:"receipt_url"`
	TransactionID  string     `json:"transaction_id"`
	PaidAt         *time.Time `json:"paid_at"`
	DisputeStatus  string     `json:"dispute_status"`
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
	UserName   string         `gorm:"-" json:"user_name"`
	UserEmail  string         `gorm:"-" json:"user_email"`
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
	BannerURL          string         `gorm:"column:banner_url;default:''" json:"banner_url"`
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
	Tags               []byte         `gorm:"type:jsonb" json:"tags"`
	EnableRegistration bool           `gorm:"default:false" json:"enable_registration"`
	Status             string         `gorm:"default:'upcoming'" json:"status"`
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

type WrittenExam struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID    uint           `gorm:"index;not null" json:"provider_id"`
	ScholarshipID uint           `gorm:"index;not null" json:"scholarship_id"`
	Title         string         `gorm:"not null" json:"title"`
	ExamDate      string         `json:"exam_date"`
	Duration      int            `json:"duration"`
	Location      string         `json:"location"`
	TotalMarks    int            `json:"total_marks"`
	PassingMarks  int            `json:"passing_marks"`
	Status        string         `gorm:"default:'draft'" json:"status"`
	Results       []WrittenExamResult `gorm:"-" json:"results,omitempty"`
}

type WrittenExamResult struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	WrittenExamID      uint           `gorm:"uniqueIndex:idx_written_exam_result_exam_app;not null" json:"written_exam_id"`
	ApplicationID      uint           `gorm:"uniqueIndex:idx_written_exam_result_exam_app;not null" json:"application_id"`
	MarksObtained      int            `json:"marks_obtained"`
	Remarks            string         `json:"remarks"`
	InterviewLocation  string         `json:"interview_location,omitempty"`
	InterviewDate      string         `json:"interview_date,omitempty"`
	ReportingTime      string         `json:"reporting_time,omitempty"`
	RequiredDocuments  []byte         `gorm:"type:jsonb" json:"required_documents,omitempty"`
}

type WrittenExamResultWithApp struct {
	WrittenExamResult
	AppFirstName  string  `gorm:"column:app_first_name"`
	AppLastName   string  `gorm:"column:app_last_name"`
	AppRollNumber string  `gorm:"column:app_roll_number"`
	AppStream     string  `gorm:"column:app_stream"`
	AppGender     string  `gorm:"column:app_gender"`
	AppSchoolType string  `gorm:"column:app_school_type"`
	AppExamCenter string  `gorm:"column:app_exam_center"`
	AppGPA        float64 `gorm:"column:app_gpa"`
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

type ProviderService struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProviderID  uint      `gorm:"index;not null" json:"provider_id"`
	Icon        string    `gorm:"default:''" json:"icon"`
	Title       string    `gorm:"not null" json:"title"`
	Description  string    `gorm:"type:text" json:"description"`
	ExternalLink string    `gorm:"default:''" json:"external_link"`
	SortOrder    int       `gorm:"default:0" json:"sort_order"`
}

type ProviderSector struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProviderID  uint      `gorm:"index;not null" json:"provider_id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Color       string    `gorm:"default:'#2563eb'" json:"color"`
	ImageURL     string    `gorm:"default:''" json:"image_url"`
	Icon         string    `gorm:"default:''" json:"icon"`
	ExternalLink string    `gorm:"default:''" json:"external_link"`
	SortOrder    int       `gorm:"default:0" json:"sort_order"`
}

type ProviderProject struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProviderID  uint      `gorm:"index;not null" json:"provider_id"`
	Title       string    `gorm:"not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	ImageURL    string    `gorm:"default:''" json:"image_url"`
	Category     string    `gorm:"default:''" json:"category"`
	ExternalLink string    `gorm:"default:''" json:"external_link"`
	Date         time.Time `json:"date"`
	SortOrder    int       `gorm:"default:0" json:"sort_order"`
}

type ProviderGalleryImage struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ProviderID uint      `gorm:"index;not null" json:"provider_id"`
	Folder     string    `gorm:"default:''" json:"folder"`
	ImageURL   string    `gorm:"not null" json:"image_url"`
	Caption    string    `gorm:"default:''" json:"caption"`
	SortOrder  int       `gorm:"default:0" json:"sort_order"`
}

type ProviderReview struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ProviderID uint      `gorm:"index;not null" json:"provider_id"`
	AuthorName string    `gorm:"not null" json:"author_name"`
	AvatarURL  string    `gorm:"default:''" json:"avatar_url"`
	Rating     int       `gorm:"default:5" json:"rating"`
	Title      string    `gorm:"default:''" json:"title"`
	Content    string    `gorm:"type:text" json:"content"`
	Pros       string    `gorm:"type:text" json:"pros"`
	Cons       string    `gorm:"type:text" json:"cons"`
	Status     string    `gorm:"default:'published'" json:"status"`
}

type ProviderVolunteer struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID         uint           `gorm:"index;not null" json:"provider_id"`
	Slug               string         `gorm:"uniqueIndex" json:"slug"`
	Title              string         `gorm:"not null" json:"title"`
	BannerImage        string         `json:"banner_image"`
	Description        string         `gorm:"type:text" json:"description"`
	VolunteerType      string         `gorm:"default:'free'" json:"volunteer_type"`
	VolunteerPayment   string         `json:"volunteer_payment"`
	DateMode           string         `gorm:"default:'range'" json:"date_mode"`
	RangeStart         string         `json:"range_start"`
	RangeEnd           string         `json:"range_end"`
	SpecificDates      []byte         `gorm:"type:jsonb" json:"specific_dates"`
	ApplicationDeadline string        `json:"application_deadline"`
	Districts          []byte         `gorm:"type:jsonb" json:"districts"`
	Active             bool           `gorm:"default:false" json:"active"`
	ApplicantCount     int64          `gorm:"-" json:"applicant_count"`
	Location           string         `gorm:"default:''" json:"location"`
}

type VolunteerApplication struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	VolunteerID        uint           `gorm:"index;not null" json:"volunteer_id"`
	UserID             *uint          `gorm:"index" json:"user_id,omitempty"`
	FullName           string         `gorm:"not null" json:"full_name"`
	Gender             string         `json:"gender"`
	Phone              string         `json:"phone"`
	Email              string         `json:"email"`
	Designation        string         `json:"designation"`
	OtherDesignation   string         `json:"other_designation"`
	Province           string         `json:"province"`
	District           string         `json:"district"`
	Municipality       string         `json:"municipality"`
	Ward               string         `json:"ward"`
	Tole               string         `json:"tole"`
	ParticipateDistrict string        `json:"participate_district"`
	AvailableDays      []byte         `gorm:"type:jsonb" json:"available_days"`
	VolunteeredBefore  string         `json:"volunteered_before"`
	VolunteerDetails   string         `gorm:"type:text" json:"volunteer_details"`
	CVPath             string         `json:"cv_path"`
	Status             string         `gorm:"default:'pending'" json:"status"`
}

type ProviderAccessUser struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	ProviderID  uint           `gorm:"index;not null" json:"provider_id"`
	Name        string         `gorm:"not null" json:"name"`
	Email       string         `gorm:"uniqueIndex;not null" json:"email"`
	Password    string         `json:"-"`
	Role        string         `gorm:"default:'user'" json:"role"`
	RoleLabel   string         `gorm:"default:'User'" json:"role_label"`
	Status      string         `gorm:"default:'Active'" json:"status"`
	LastActive  time.Time      `json:"last_active"`
	Avatar      string         `json:"avatar"`
	Permissions []byte         `gorm:"type:jsonb" json:"permissions"`
}

func (ps *ProviderScholarship) BeforeCreate(tx *gorm.DB) error {
	if ps.Slug == "" {
		ps.Slug = slug.GenerateUnique(ps.Title, func(slugStr string) bool {
			var count int64
			tx.Model(&ProviderScholarship{}).Where("slug = ?", slugStr).Count(&count)
			return count > 0
		})
	}
	return nil
}

func (pv *ProviderVolunteer) BeforeCreate(tx *gorm.DB) error {
	if pv.Slug == "" {
		pv.Slug = slug.GenerateUnique(pv.Title, func(slugStr string) bool {
			var count int64
			tx.Model(&ProviderVolunteer{}).Where("slug = ?", slugStr).Count(&count)
			return count > 0
		})
	}
	return nil
}

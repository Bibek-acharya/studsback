package scholarshipprovider

import (
	"encoding/json"
	"time"
)

type VideoTutorial struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type JourneyTimelineItem struct {
	Year        string `json:"year"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type TimelineItem struct {
	Title       string `json:"title"`
	Date        string `json:"date"`
	Description string `json:"description"`
}

type ScholarshipTypeItem struct {
	Type        string `json:"type"`
	Seats       string `json:"seats"`
	Coverage    string `json:"coverage"`
	Eligibility string `json:"eligibility"`
}

type SelectionRubricItem struct {
	Criteria    string `json:"criteria"`
	Description string `json:"description"`
	Weight      string `json:"weight"`
}

type SelectionProcessStepItem struct {
	Step        int    `json:"step"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type FAQItem struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type GalleryImageItem struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func (g *GalleryImageItem) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	if data[0] == '"' {
		var url string
		if err := json.Unmarshal(data, &url); err != nil {
			return err
		}
		g.URL = url
		g.Title = ""
		return nil
	}

	type alias GalleryImageItem
	var item alias
	if err := json.Unmarshal(data, &item); err != nil {
		return err
	}
	*g = GalleryImageItem(item)
	return nil
}

type PartnerOrganization struct {
	GroupHeading string `json:"groupHeading"`
	Name         string `json:"name"`
	Website      string `json:"website"`
	Logo         string `json:"logo"`
}

type ExamCenterItem struct {
	Province       string `json:"province"`
	HeaderColor    string `json:"headerColor"`
	Info           string `json:"info"`
	CenterName     string `json:"centerName"`
	ContactPerson  string `json:"contactPerson"`
	PhoneNumber    string `json:"phoneNumber"`
	MapCoordinates string `json:"mapCoordinates"`
}

type DownloadItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

type BankDetails struct {
	BankName      string `json:"bank_name"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	Branch        string `json:"branch"`
}

type PaymentConfig struct {
	Enabled     bool         `json:"enabled"`
	FeeAmount   int          `json:"fee_amount"`
	Methods     []string     `json:"methods"`
	BankDetails *BankDetails `json:"bank_details,omitempty"`
	QRCode      string       `json:"qr_code,omitempty"`
}

type ScholarshipTypeWithEligibility struct {
	Type        string `json:"type"`
	Seats       string `json:"seats"`
	Coverage    string `json:"coverage"`
	Eligibility string `json:"eligibility"`
}

type CreateScholarshipRequest struct {
	Title                    string                     `json:"title"`
	Provider                 string                     `json:"provider"`
	Description              string                     `json:"description"`
	ProviderName             string                     `json:"provider_name"`
	FundingTypeOther         string                     `json:"funding_type_other"`
	ScholarshipTypeOther     string                     `json:"scholarship_type_other"`
	EducationLevel           string                     `json:"education_level"`
	EducationLevelOther      string                     `json:"education_level_other"`
	Location                 string                     `json:"location"`
	Value                    string                     `json:"value"`
	Deadline                 string                     `json:"deadline"`
	DegreeLevel              string                     `json:"degree_level"`
	FundingType              string                     `json:"funding_type"`
	ScholarshipType          string                     `json:"scholarship_type"`
	FieldOfStudy             []string                   `json:"field_of_study"`
	Status                   string                     `json:"status"`
	ApplicationStartDate     string                     `json:"application_start_date"`
	ApplicationEndDate       string                     `json:"application_end_date"`
	ApplyLink                string                     `json:"apply_link"`
	BannerBackgroundImageURL string                     `json:"banner_background_image_url"`
	CoverageArea             string                     `json:"coverage_area"`
	ContactEmail             string                     `json:"contact_email"`
	PrimaryPhone             string                     `json:"primary_phone"`
	SecondaryPhone           string                     `json:"secondary_phone"`
	WebsiteUrl               string                     `json:"website_url"`
	OfficeAddress            string                     `json:"office_address"`
	MapUrl                   string                     `json:"map_url"`
	AboutParagraph1          string                     `json:"about_paragraph_1"`
	VideoTutorials           []VideoTutorial            `json:"video_tutorials"`
	JourneyTimeline          []JourneyTimelineItem      `json:"journey_timeline"`
	Timeline                 []TimelineItem             `json:"timeline"`
	ScholarshipSectionTitle  string                     `json:"scholarship_section_title"`
	ScholarshipSubtitle      string                     `json:"scholarship_subtitle"`
	ScholarshipDescription1  string                     `json:"scholarship_description_1"`
	ScholarshipDescription2  string                     `json:"scholarship_description_2"`
	ScholarshipTypes         []ScholarshipTypeItem      `json:"scholarship_types"`
	ScholarshipTypesNew      []ScholarshipTypeItem      `json:"scholarship_types_new"`
	SelectionRubric          []SelectionRubricItem      `json:"selection_rubric"`
	SelectionRubricNew       []SelectionRubricItem      `json:"selection_rubric_new"`
	EligibilitySectionTitle  string                     `json:"eligibility_section_title"`
	EligibilitySubtitle      string                     `json:"eligibility_subtitle"`
	BasicEligibilityCriteria []string                   `json:"basic_eligibility_criteria"`
	FullyFundedCriteria      []string                   `json:"fully_funded_criteria"`
	PartiallyFundedCriteria  []string                   `json:"partially_funded_criteria"`
	SelectionProcessSteps    []SelectionProcessStepItem `json:"selection_process_steps"`
	RequiredDocuments        []string                   `json:"required_documents"`
	FAQs                     []FAQItem                  `json:"faqs"`
	FAQsNew                  []FAQItem                  `json:"faqs_new"`
	GalleryImages            []GalleryImageItem         `json:"gallery_images"`
	GalleryImagesNew         []GalleryImageItem         `json:"gallery_images_new"`
	PartnerGroups            []PartnerOrganization      `json:"partner_groups"`
	ExamCenters              []ExamCenterItem           `json:"exam_centers"`
	ExamCentersNew           []ExamCenterItem           `json:"exam_centers_new"`
	Downloads                []DownloadItem             `json:"downloads"`
	PaymentConfig            *PaymentConfig             `json:"payment_config"`
}

type ScholarshipResponse struct {
	ID                       uint                       `json:"id"`
	CreatedAt                time.Time                  `json:"created_at"`
	UpdatedAt                time.Time                  `json:"updated_at"`
	ProviderID               uint                       `json:"provider_id"`
	Title                    string                     `json:"title"`
	Provider                 string                     `json:"provider"`
	Description              string                     `json:"description"`
	ProviderName             string                     `json:"provider_name"`
	FundingTypeOther         string                     `json:"funding_type_other"`
	ScholarshipTypeOther     string                     `json:"scholarship_type_other"`
	EducationLevel           string                     `json:"education_level"`
	EducationLevelOther      string                     `json:"education_level_other"`
	Location                 string                     `json:"location"`
	Value                    string                     `json:"value"`
	Deadline                 string                     `json:"deadline"`
	DegreeLevel              string                     `json:"degree_level"`
	FundingType              string                     `json:"funding_type"`
	ScholarshipType          string                     `json:"scholarship_type"`
	FieldOfStudy             []string                   `json:"field_of_study"`
	Status                   string                     `json:"status"`
	ApplicationStartDate     string                     `json:"application_start_date"`
	ApplicationEndDate       string                     `json:"application_end_date"`
	ApplyLink                string                     `json:"apply_link"`
	BannerBackgroundImageURL string                     `json:"banner_background_image_url"`
	CoverageArea             string                     `json:"coverage_area"`
	ContactEmail             string                     `json:"contact_email"`
	PrimaryPhone             string                     `json:"primary_phone"`
	SecondaryPhone           string                     `json:"secondary_phone"`
	WebsiteUrl               string                     `json:"website_url"`
	OfficeAddress            string                     `json:"office_address"`
	MapUrl                   string                     `json:"map_url"`
	AboutParagraph1          string                     `json:"about_paragraph_1"`
	VideoTutorials           []VideoTutorial            `json:"video_tutorials"`
	JourneyTimeline          []JourneyTimelineItem      `json:"journey_timeline"`
	Timeline                 []TimelineItem             `json:"timeline"`
	ScholarshipSectionTitle  string                     `json:"scholarship_section_title"`
	ScholarshipSubtitle      string                     `json:"scholarship_subtitle"`
	ScholarshipDescription1  string                     `json:"scholarship_description_1"`
	ScholarshipDescription2  string                     `json:"scholarship_description_2"`
	ScholarshipTypes         []ScholarshipTypeItem      `json:"scholarship_types"`
	ScholarshipTypesNew      []ScholarshipTypeItem      `json:"scholarship_types_new"`
	SelectionRubric          []SelectionRubricItem      `json:"selection_rubric"`
	SelectionRubricNew       []SelectionRubricItem      `json:"selection_rubric_new"`
	EligibilitySectionTitle  string                     `json:"eligibility_section_title"`
	EligibilitySubtitle      string                     `json:"eligibility_subtitle"`
	BasicEligibilityCriteria []string                   `json:"basic_eligibility_criteria"`
	FullyFundedCriteria      []string                   `json:"fully_funded_criteria"`
	PartiallyFundedCriteria  []string                   `json:"partially_funded_criteria"`
	SelectionProcessSteps    []SelectionProcessStepItem `json:"selection_process_steps"`
	RequiredDocuments        []string                   `json:"required_documents"`
	FAQs                     []FAQItem                  `json:"faqs"`
	FAQsNew                  []FAQItem                  `json:"faqs_new"`
	GalleryImages            []GalleryImageItem         `json:"gallery_images"`
	GalleryImagesNew         []GalleryImageItem         `json:"gallery_images_new"`
	PartnerGroups            []PartnerOrganization      `json:"partner_groups"`
	ExamCenters              []ExamCenterItem           `json:"exam_centers"`
	ExamCentersNew           []ExamCenterItem           `json:"exam_centers_new"`
	Downloads                []DownloadItem             `json:"downloads"`
	PaymentConfig            *PaymentConfig             `json:"payment_config"`
}

type ScholarshipListResponse struct {
	Scholarships []ScholarshipResponse `json:"scholarships"`
	Meta         PaginationMeta        `json:"meta"`
}

type ApplicationResponse struct {
	ID                    uint                 `json:"id"`
	CreatedAt             time.Time            `json:"created_at"`
	UpdatedAt             time.Time            `json:"updated_at"`
	ScholarshipID         uint                 `json:"scholarship_id"`
	Scholarship           *ScholarshipResponse `json:"scholarship,omitempty"`
	UserID                uint                 `json:"user_id"`
	FullName              string               `json:"full_name"`
	FirstName             string               `json:"first_name"`
	LastName              string               `json:"last_name"`
	Email                 string               `json:"email"`
	PhoneNumber           string               `json:"phone_number"`
	Gender                string               `json:"gender"`
	Ethnicity             string               `json:"ethnicity"`
	EthnicityOther        string               `json:"ethnicity_other"`
	DateOfBirthBS         string               `json:"date_of_birth_bs"`
	DateOfBirthAD         time.Time            `json:"date_of_birth_ad"`
	Age                   int                  `json:"age"`
	PhotoURL              string               `json:"photo_url"`
	SEEGPA                string               `json:"see_gpa"`
	SchoolName            string               `json:"school_name"`
	SchoolProvince        string               `json:"school_province"`
	SchoolDistrict        string               `json:"school_district"`
	SchoolMunicipality    string               `json:"school_municipality"`
	SchoolTole            string               `json:"school_tole"`
	PermanentProvince     string               `json:"permanent_province"`
	PermanentDistrict     string               `json:"permanent_district"`
	PermanentMunicipality string               `json:"permanent_municipality"`
	PermanentWard         string               `json:"permanent_ward"`
	PermanentTole         string               `json:"permanent_tole"`
	TemporaryProvince     string               `json:"temporary_province"`
	TemporaryDistrict     string               `json:"temporary_district"`
	TemporaryMunicipality string               `json:"temporary_municipality"`
	TemporaryWard         string               `json:"temporary_ward"`
	TemporaryTole         string               `json:"temporary_tole"`
	GuardianName          string               `json:"guardian_name"`
	GuardianPhone         string               `json:"guardian_phone"`
	GuardianEmail         string               `json:"guardian_email"`
	FatherOccupation      string               `json:"father_occupation"`
	FatherOccupationOther string               `json:"father_occupation_other"`
	MotherOccupation      string               `json:"mother_occupation"`
	MotherOccupationOther string               `json:"mother_occupation_other"`
	FamilyMonthlyIncome   float64              `json:"family_monthly_income"`
	FamilyMembersCount    int                  `json:"family_members_count"`
	Status                string               `json:"status"`
	EvaluationScore       int                  `json:"evaluation_score"`
	EvaluationPassed      bool                 `json:"evaluation_passed"`
	EvaluationNotes       string               `json:"evaluation_notes"`
	Documents             json.RawMessage      `json:"documents"`
	PersonalStatement     string               `json:"personal_statement"`
	Province              string               `json:"province"`
	District              string               `json:"district"`
	Stream                string               `json:"stream"`
	GPA                   float64              `json:"gpa"`
	SchoolType            string               `json:"school_type"`
	ExamCenter            string               `json:"exam_center"`
}

type ApplicationListResponse struct {
	Applications []ApplicationResponse `json:"applications"`
	Meta         PaginationMeta        `json:"meta"`
}

type EvaluateApplicationRequest struct {
	Score   int    `json:"score"`
	Notes   string `json:"notes"`
	Passing bool   `json:"passing"`
}

type UpdateApplicationStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type CreateInterviewRequest struct {
	ApplicationID uint   `json:"application_id" binding:"required"`
	ScheduledAt   string `json:"scheduled_at" binding:"required"`
	Duration      int    `json:"duration"`
	Type          string `json:"type"`
	Location      string `json:"location"`
	Link          string `json:"link"`
	Notes         string `json:"notes"`
}

type UpdateInterviewRequest struct {
	ScheduledAt string `json:"scheduled_at"`
	Duration    int    `json:"duration"`
	Type        string `json:"type"`
	Location    string `json:"location"`
	Link        string `json:"link"`
	Status      string `json:"status"`
	Notes       string `json:"notes"`
}

type InterviewResponse struct {
	ID            uint      `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ApplicationID uint      `json:"application_id"`
	ProviderID    uint      `json:"provider_id"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	Duration      int       `json:"duration"`
	Type          string    `json:"type"`
	Location      string    `json:"location"`
	Link          string    `json:"link"`
	Status        string    `json:"status"`
	Notes         string    `json:"notes"`
}

type CreateMessageRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	Subject string `json:"subject" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type MessageResponse struct {
	ID         uint      `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ProviderID uint      `json:"provider_id"`
	UserID     uint      `json:"user_id"`
	Subject    string    `json:"subject"`
	Content    string    `json:"content"`
	Read       bool      `json:"read"`
	Direction  string    `json:"direction"`
}

type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
	Meta     PaginationMeta    `json:"meta"`
}

type UpdateProfileRequest struct {
	ProviderName       string `json:"provider_name"`
	RegistrationNumber string `json:"registration_number"`
	ContactNumber      string `json:"contact_number"`
	PANNumber          string `json:"pan_number"`
	WebsiteURL         string `json:"website_url"`
}

type ProfileResponse struct {
	ID                 uint   `json:"id"`
	ProviderName       string `json:"provider_name"`
	RegistrationNumber string `json:"registration_number"`
	Email              string `json:"email"`
	ContactNumber      string `json:"contact_number"`
	PANNumber          string `json:"pan_number"`
	WebsiteURL         string `json:"website_url"`
	Role               string `json:"role"`
}

type UpdateSettingsRequest struct {
	EmailNotifs bool   `json:"email_notifications"`
	SmsNotifs   bool   `json:"sms_notifications"`
	AutoReject  bool   `json:"auto_reject_expired"`
	Timezone    string `json:"timezone"`
	Language    string `json:"language"`
}

type SettingsResponse struct {
	ID          uint      `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProviderID  uint      `json:"provider_id"`
	EmailNotifs bool      `json:"email_notifications"`
	SmsNotifs   bool      `json:"sms_notifications"`
	AutoReject  bool      `json:"auto_reject_expired"`
	Timezone    string    `json:"timezone"`
	Language    string    `json:"language"`
}

type NotificationResponse struct {
	ID         uint      `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ProviderID uint      `json:"provider_id"`
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	Type       string    `json:"type"`
	Read       bool      `json:"read"`
	Link       string    `json:"link"`
}

type NotificationListResponse struct {
	Notifications []NotificationResponse `json:"notifications"`
	UnreadCount   int64                  `json:"unread_count"`
	Meta          PaginationMeta         `json:"meta"`
}

type DashboardResponse struct {
	TotalScholarships   int64 `json:"total_scholarships"`
	TotalApplications   int64 `json:"total_applications"`
	PendingApplications int64 `json:"pending_applications"`
	TotalInterviews     int64 `json:"total_interviews"`
	UnreadMessages      int64 `json:"unread_messages"`
}

type ScholarshipStat struct {
	ID           uint   `json:"id"`
	Title        string `json:"title"`
	Applications int64  `json:"applications"`
	Status       string `json:"status"`
}

type AnalyticsResponse struct {
	StatusBreakdown   map[string]int    `json:"status_breakdown"`
	TotalApplications int               `json:"total_applications"`
	ScholarshipStats  []ScholarshipStat `json:"scholarship_stats"`
}

type PaginationMeta struct {
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

type CreateNewsRequest struct {
	Title         string   `json:"title" binding:"required"`
	ShortDesc     string   `json:"short_desc"`
	Content       string   `json:"content"`
	ImageURL      string   `json:"image_url"`
	NewsType      string   `json:"news_type"`
	PublishedBy   string   `json:"published_by"`
	PublishDate   string   `json:"publish_date"`
	Tags          []string `json:"tags"`
	AllowComments bool     `json:"allow_comments"`
	Status        string   `json:"status"`
}

type NewsResponse struct {
	ID            uint        `json:"id"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	ProviderID    uint        `json:"provider_id"`
	Title         string      `json:"title"`
	ShortDesc     string      `json:"short_desc"`
	Content       string      `json:"content"`
	ImageURL      *string     `json:"image_url"`
	NewsType      string      `json:"news_type"`
	PublishedBy   string      `json:"published_by"`
	PublishDate   *time.Time  `json:"publish_date"`
	Tags          interface{} `json:"tags"`
	AllowComments bool        `json:"allow_comments"`
	Status        string      `json:"status"`
	PublishedAt   *time.Time  `json:"published_at"`
}

type NewsListResponse struct {
	News []NewsResponse `json:"news"`
	Meta PaginationMeta `json:"meta"`
}

type CreateEventRequest struct {
	Name               string   `json:"name" binding:"required"`
	ShortDesc          string   `json:"short_desc"`
	Description        string   `json:"description"`
	ImageURL           string   `json:"image_url"`
	EventType          string   `json:"event_type"`
	Category           string   `json:"category"`
	MaxParticipants    int      `json:"max_participants"`
	OnlineLink         string   `json:"online_link"`
	OrganizedBy        string   `json:"organized_by"`
	ContactPerson      string   `json:"contact_person"`
	ContactEmail       string   `json:"contact_email"`
	StartDate          string   `json:"start_date" binding:"required"`
	EndDate            string   `json:"end_date"`
	Location           string   `json:"location"`
	Tags               []string `json:"tags"`
	EnableRegistration bool     `json:"enable_registration"`
	Status             string   `json:"status"`
}

type EventResponse struct {
	ID                 uint        `json:"id"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	ProviderID         uint        `json:"provider_id"`
	Name               string      `json:"name"`
	ShortDesc          string      `json:"short_desc"`
	Description        string      `json:"description"`
	ImageURL           *string     `json:"image_url"`
	EventType          string      `json:"event_type"`
	Category           string      `json:"category"`
	MaxParticipants    int         `json:"max_participants"`
	OnlineLink         string      `json:"online_link"`
	OrganizedBy        string      `json:"organized_by"`
	ContactPerson      string      `json:"contact_person"`
	ContactEmail       string      `json:"contact_email"`
	StartDate          time.Time   `json:"start_date"`
	EndDate            time.Time   `json:"end_date"`
	Location           string      `json:"location"`
	Tags               interface{} `json:"tags"`
	EnableRegistration bool        `json:"enable_registration"`
	Status             string      `json:"status"`
	Attendees          int         `json:"attendees"`
}

type EventListResponse struct {
	Events []EventResponse `json:"events"`
	Meta   PaginationMeta  `json:"meta"`
}

type CreateBlogRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content"`
	Image   string `json:"image_url"`
	Author  string `json:"author"`
	Status  string `json:"status"`
}

type BlogResponse struct {
	ID          uint       `json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ProviderID  uint       `json:"provider_id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	ImageURL    *string    `json:"image_url"`
	Author      string     `json:"author"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"published_at"`
	Views       int        `json:"views"`
	Likes       int        `json:"likes"`
}

type BlogListResponse struct {
	Blogs []BlogResponse `json:"blogs"`
	Meta  PaginationMeta `json:"meta"`
}

type CreateCalendarEventRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	StartDate   string `json:"start_date" binding:"required"`
	EndDate     string `json:"end_date"`
	Color       string `json:"color"`
	IsAllDay    bool   `json:"is_all_day"`
}

type CalendarEventResponse struct {
	ID          uint      `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProviderID  uint      `json:"provider_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	Color       string    `json:"color"`
	IsAllDay    bool      `json:"is_all_day"`
}

type CreateResultRequest struct {
	ScholarshipID uint            `json:"scholarship_id" binding:"required"`
	Title         string          `json:"title" binding:"required"`
	Status        string          `json:"status"`
	Results       json.RawMessage `json:"results"`
}

type ResultResponse struct {
	ID            uint            `json:"id"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	ProviderID    uint            `json:"provider_id"`
	ScholarshipID uint            `json:"scholarship_id"`
	Title         string          `json:"title"`
	Status        string          `json:"status"`
	PublishedAt   *time.Time      `json:"published_at"`
	Results       json.RawMessage `json:"results"`
}

type ResultListResponse struct {
	Results []ResultResponse `json:"results"`
	Meta    PaginationMeta   `json:"meta"`
}

type CreateAccessRequest struct {
	Email string `json:"email" binding:"required"`
	Role  string `json:"role"`
}

type AccessResponse struct {
	ID         uint      `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ProviderID uint      `json:"provider_id"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
}

type AccessListResponse struct {
	Access []AccessResponse `json:"access"`
	Meta   PaginationMeta   `json:"meta"`
}

type CreateAccessUserRequest struct {
	Name        string   `json:"name" binding:"required"`
	Email       string   `json:"email" binding:"required,email"`
	Password    string   `json:"password" binding:"required,min=6"`
	Role        string   `json:"role"`
	RoleLabel   string   `json:"role_label"`
	Permissions []string `json:"permissions"`
}

type UpdateAccessUserRequest struct {
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	Role        string   `json:"role"`
	RoleLabel   string   `json:"role_label"`
	Status      string   `json:"status"`
	Permissions []string `json:"permissions"`
}

type AccessUserResponse struct {
	ID          uint      `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ProviderID  uint      `json:"provider_id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	RoleLabel   string    `json:"role_label"`
	Status      string    `json:"status"`
	LastActive  time.Time `json:"last_active"`
	Avatar      string    `json:"avatar"`
	Permissions []string  `json:"permissions"`
}

type AccessUserListResponse struct {
	Users []AccessUserResponse `json:"users"`
	Meta  PaginationMeta       `json:"meta"`
}

type UpdatePermissionsRequest struct {
	Permissions []string `json:"permissions" binding:"required"`
}

type ProviderLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ProviderLoginResponse struct {
	User        AccessUserResponse `json:"user"`
	Token       string             `json:"token"`
	Permissions []string           `json:"permissions"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

type ChangePasswordResponse struct {
	Message string `json:"message"`
}

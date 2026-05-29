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
	Icon        string `json:"icon"`
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
	Folder string `json:"folder"`
	Title  string `json:"title"`
	URL    string `json:"url"`
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

type PartnerGroup struct {
	GroupHeading string         `json:"groupHeading"`
	Partners     []PartnerEntry `json:"partners"`
}

type PartnerEntry struct {
	Name    string `json:"name"`
	Website string `json:"website"`
	Logo    string `json:"logo"`
}

type PartnerMessage struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Message string `json:"message"`
	Logo    string `json:"logo"`
}

type ExamCenterItem struct {
	Province       string `json:"province"`
	HeaderColor    string `json:"headerColor"`
	Info           string `json:"info"`
	CenterName     string `json:"centerName"`
	ContactPerson  string `json:"contactPerson"`
	PhoneNumber    string `json:"phoneNumber"`
	MapCoordinates string `json:"mapCoordinates"`
	AllocatedSeats int    `json:"allocatedSeats"`
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

type CreateVolunteerRequest struct {
	Title               string   `json:"title"`
	BannerImage         string   `json:"banner_image"`
	Description         string   `json:"description"`
	VolunteerType       string   `json:"volunteer_type"`
	VolunteerPayment    string   `json:"volunteer_payment"`
	DateMode            string   `json:"date_mode"`
	RangeStart          string   `json:"range_start"`
	RangeEnd            string   `json:"range_end"`
	SpecificDates       []string `json:"specific_dates"`
	ApplicationDeadline string   `json:"application_deadline"`
	Districts           []string `json:"districts"`
	Active              bool     `json:"active"`
	Location            string   `json:"location"`
}

type VolunteerResponse struct {
	ID                  uint     `json:"id"`
	Slug                string   `json:"slug"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
	ProviderID          uint     `json:"provider_id"`
	Title               string   `json:"title"`
	BannerImage         string   `json:"banner_image"`
	Description         string   `json:"description"`
	VolunteerType       string   `json:"volunteer_type"`
	VolunteerPayment    string   `json:"volunteer_payment"`
	DateMode            string   `json:"date_mode"`
	RangeStart          string   `json:"range_start"`
	RangeEnd            string   `json:"range_end"`
	SpecificDates       []string `json:"specific_dates"`
	ApplicationDeadline string   `json:"application_deadline"`
	Districts           []string `json:"districts"`
	Active              bool     `json:"active"`
	ApplicantCount      int64    `json:"applicant_count"`
	Organizer           string   `json:"organizer"`
	Location            string   `json:"location"`
}

type VolunteerListResponse struct {
	Volunteers []VolunteerResponse `json:"volunteers"`
	Meta       PaginationMeta      `json:"meta"`
}

type ApplyVolunteerRequest struct {
	FullName           string   `json:"full_name"`
	Gender             string   `json:"gender"`
	Phone              string   `json:"phone"`
	Email              string   `json:"email"`
	Designation        string   `json:"designation"`
	OtherDesignation   string   `json:"other_designation"`
	Province           string   `json:"province"`
	District           string   `json:"district"`
	Municipality       string   `json:"municipality"`
	Ward               string   `json:"ward"`
	Tole               string   `json:"tole"`
	ParticipateDistrict string  `json:"participate_district"`
	AvailableDays      []string `json:"available_days"`
	VolunteeredBefore  string   `json:"volunteered_before"`
	VolunteerDetails   string   `json:"volunteer_details"`
}

type VolunteerApplicationResponse struct {
	ID                 uint     `json:"id"`
	CreatedAt          string   `json:"created_at"`
	VolunteerID        uint     `json:"volunteer_id"`
	FullName           string   `json:"full_name"`
	Gender             string   `json:"gender"`
	Phone              string   `json:"phone"`
	Email              string   `json:"email"`
	Designation        string   `json:"designation"`
	OtherDesignation   string   `json:"other_designation"`
	Province           string   `json:"province"`
	District           string   `json:"district"`
	Municipality       string   `json:"municipality"`
	Ward               string   `json:"ward"`
	Tole               string   `json:"tole"`
	ParticipateDistrict string  `json:"participate_district"`
	AvailableDays      []string `json:"available_days"`
	VolunteeredBefore  string   `json:"volunteered_before"`
	VolunteerDetails   string   `json:"volunteer_details"`
	CVPath             string   `json:"cv_path"`
	Status             string   `json:"status"`
}

type VolunteerApplicationListResponse struct {
	Applications []VolunteerApplicationResponse `json:"applications"`
	Meta         PaginationMeta                 `json:"meta"`
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
	PartnerGroups            []PartnerGroup             `json:"partner_groups"`
	PartnerMessages          []PartnerMessage           `json:"partner_messages"`
	ExamCenters              []ExamCenterItem           `json:"exam_centers"`
	ExamCentersNew           []ExamCenterItem           `json:"exam_centers_new"`
	Downloads                []DownloadItem             `json:"downloads"`
	PaymentConfig            *PaymentConfig             `json:"payment_config"`

	ExamDate string `json:"exam_date"`
	ExamTime string `json:"exam_time"`
}

type ScholarshipResponse struct {
	ID                       uint                       `json:"id"`
	Slug                     string                     `json:"slug"`
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
	PartnerGroups            []PartnerGroup              `json:"partner_groups"`
	PartnerMessages          []PartnerMessage           `json:"partner_messages"`
	ExamCenters              []ExamCenterItem           `json:"exam_centers"`
	ExamCentersNew           []ExamCenterItem           `json:"exam_centers_new"`
	Downloads                []DownloadItem             `json:"downloads"`
	PaymentConfig            *PaymentConfig             `json:"payment_config"`
	Image                    string                     `json:"image"`
	ExamDate                 string                     `json:"exam_date"`
	ExamTime                 string                     `json:"exam_time"`
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
	EvaluationScore       *int                 `json:"evaluation_score"`
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
	RollNumber            string               `json:"roll_number"`
	Payment               *PaymentResponse     `json:"payment,omitempty"`
}

type PaymentResponse struct {
	ID             uint    `json:"id"`
	Method         string  `json:"method"`
	Amount         float64 `json:"amount"`
	Status         string  `json:"status"`
	ReceiptURL     string  `json:"receipt_url"`
	TransactionID  string  `json:"transaction_id"`
	PaidAt         string  `json:"paid_at,omitempty"`
	DisputeStatus  string  `json:"dispute_status"`
}

type ApplicationListResponse struct {
	Applications []ApplicationResponse `json:"applications"`
	Meta         PaginationMeta        `json:"meta"`
}

type EvaluateApplicationRequest struct {
	Score   *int   `json:"score"`
	Notes   string `json:"notes"`
	Passing bool   `json:"passing"`
}

type UpdateApplicationStatusRequest struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
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

type CreateMessageFromUserRequest struct {
	ProviderID uint   `json:"provider_id" binding:"required"`
	Subject    string `json:"subject" binding:"required"`
	Content    string `json:"content" binding:"required"`
}

type MessageResponse struct {
	ID         uint      `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ProviderID uint      `json:"provider_id"`
	UserID     uint      `json:"user_id"`
	UserName   string    `json:"user_name"`
	UserEmail  string    `json:"user_email"`
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
	LogoURL            string `json:"logo_url"`
	Address            string `json:"address"`
	AboutText          string `json:"about_text"`
	Mission            string `json:"mission"`
	Values             string `json:"values"`
	FounderName        string `json:"founder_name"`
	FounderRole        string `json:"founder_role"`
	FounderMessage     string `json:"founder_message"`
	FounderImageURL    string `json:"founder_image_url"`
	FacebookURL        string `json:"facebook_url"`
	InstagramURL       string `json:"instagram_url"`
	YoutubeURL         string `json:"youtube_url"`
	LinkedInURL        string `json:"linkedin_url"`
	MapURL             string `json:"map_url"`
	BrochureURL        string `json:"brochure_url"`
	BannerURL          string `json:"banner_url"`


}

type ProfileResponse struct {
	ID                 uint     `json:"id"`
	ProviderName       string   `json:"provider_name"`
	RegistrationNumber string   `json:"registration_number"`
	Email              string   `json:"email"`
	ContactNumber      string   `json:"contact_number"`
	PANNumber          string   `json:"pan_number"`
	WebsiteURL         string   `json:"website_url"`
	LogoURL            string   `json:"logo_url,omitempty"`
	Address            string   `json:"address,omitempty"`
	AboutText          string   `json:"about_text,omitempty"`
	Mission            string   `json:"mission,omitempty"`
	Values             string   `json:"values,omitempty"`
	FounderName        string   `json:"founder_name,omitempty"`
	FounderRole        string   `json:"founder_role,omitempty"`
	FounderMessage     string   `json:"founder_message,omitempty"`
	FounderImageURL    string   `json:"founder_image_url,omitempty"`
	FacebookURL        string   `json:"facebook_url,omitempty"`
	InstagramURL       string   `json:"instagram_url,omitempty"`
	YoutubeURL         string   `json:"youtube_url,omitempty"`
	LinkedInURL        string   `json:"linkedin_url,omitempty"`
	MapURL             string   `json:"map_url,omitempty"`
	BrochureURL        string   `json:"brochure_url,omitempty"`
	BannerURL          string   `json:"banner_url,omitempty"`
	Role               string   `json:"role"`
	IsSubUser          bool     `json:"is_sub_user"`
	Permissions        []string `json:"permissions"`
	ProviderID         uint     `json:"provider_id"`
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
	TotalScholarships    int64             `json:"total_scholarships"`
	TotalApplications    int64             `json:"total_applications"`
	PendingApplications  int64             `json:"pending_applications"`
	ApprovedApplications int64             `json:"approved_applications"`
	RejectedApplications int64             `json:"rejected_applications"`
	TotalInterviews      int64             `json:"total_interviews"`
	UnreadMessages       int64             `json:"unread_messages"`
	ScholarshipStats     []ScholarshipStat `json:"scholarship_stats"` // Add detailed scholarship stats
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

type CreateWrittenExamRequest struct {
	ScholarshipID uint   `json:"scholarship_id" binding:"required"`
	Title         string `json:"title" binding:"required"`
	ExamDate      string `json:"exam_date"`
	Duration      int    `json:"duration"`
	Location      string `json:"location"`
	TotalMarks    int    `json:"total_marks"`
	PassingMarks  int    `json:"passing_marks"`
	Status        string `json:"status"`
}

type UpdateWrittenExamRequest struct {
	Title        string `json:"title"`
	ExamDate     string `json:"exam_date"`
	Duration     int    `json:"duration"`
	Location     string `json:"location"`
	TotalMarks   int    `json:"total_marks"`
	PassingMarks int    `json:"passing_marks"`
	Status       string `json:"status"`
}

type WrittenExamResponse struct {
	ID            uint      `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ProviderID    uint      `json:"provider_id"`
	ScholarshipID uint      `json:"scholarship_id"`
	Title         string    `json:"title"`
	ExamDate      string    `json:"exam_date"`
	Duration      int       `json:"duration"`
	Location      string    `json:"location"`
	TotalMarks    int       `json:"total_marks"`
	PassingMarks  int       `json:"passing_marks"`
	Status        string    `json:"status"`
	Results       []WrittenExamResultResponse `json:"results,omitempty"`
}

type WrittenExamListResponse struct {
	Exams []WrittenExamResponse `json:"exams"`
	Meta  PaginationMeta        `json:"meta"`
}

type AddWrittenExamResultRequest struct {
	ApplicationID uint   `json:"application_id" binding:"required"`
	MarksObtained int    `json:"marks_obtained"`
	Remarks       string `json:"remarks"`
}

type UpdateWrittenExamResultRequest struct {
	MarksObtained int    `json:"marks_obtained"`
	Remarks       string `json:"remarks"`
}

type WrittenExamResultResponse struct {
	ID                 uint      `json:"id"`
	CreatedAt          time.Time `json:"created_at"`
	WrittenExamID      uint      `json:"written_exam_id"`
	ApplicationID      uint      `json:"application_id"`
	MarksObtained      int       `json:"marks_obtained"`
	Remarks            string    `json:"remarks"`
	StudentName        string    `json:"student_name,omitempty"`
	Stream             string    `json:"stream,omitempty"`
	ExamCenter         string    `json:"exam_center,omitempty"`
	RollNo             string    `json:"roll_no,omitempty"`
	InterviewLocation  string    `json:"interview_location,omitempty"`
	InterviewDate      string    `json:"interview_date,omitempty"`
	ReportingTime      string    `json:"reporting_time,omitempty"`
	RequiredDocuments  []string  `json:"required_documents,omitempty"`
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

type ChangeEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ChangeEmailResponse struct {
	Message string `json:"message"`
}

type DetailedAnalyticsFilters struct {
	Province          string `json:"province"`
	District          string `json:"district"`
	SchoolType        string `json:"school_type"`
	ScholarshipStatus string `json:"scholarship_status"`
	EthnicityProvince string `json:"ethnicity_province"`
}

type CrossMetric struct {
	Label  string        `json:"label"`
	Values []MetricCount `json:"values"`
}

type ExamCenterMetric struct {
	Name       string `json:"name"`
	Management int    `json:"management"`
	Science    int    `json:"science"`
}

type MetricCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type DetailedAnalyticsResponse struct {
	TotalApplicants    int                `json:"total_applicants"`
	Gender             []MetricCount      `json:"gender"`
	Ethnicity          []MetricCount      `json:"ethnicity"`
	GPABreakdown       []MetricCount      `json:"gpa_breakdown"`
	SchoolType         []MetricCount      `json:"school_type"`
	Stream             []MetricCount      `json:"stream"`
	Province           []MetricCount      `json:"province"`
	District           []MetricCount      `json:"district"`
	Status             []MetricCount      `json:"status"`
	AdmitCardsSent     int                `json:"admit_cards_sent"`
	AdmitCardsPending  int                `json:"admit_cards_pending"`
	PaymentMethods     []MetricCount      `json:"payment_methods"`
	GPABySchoolType    []MetricCount      `json:"gpa_by_school_type"`
	GenderByProvince   []CrossMetric      `json:"gender_by_province"`
	StreamByProvince   []CrossMetric      `json:"stream_by_province"`
	SchoolTypeByProvince []CrossMetric      `json:"school_type_by_province"`
	ExamCenters          []ExamCenterMetric `json:"exam_centers"`
	DistrictCount        int                `json:"district_count"`
	ApplicationsPerDay   []MetricCount      `json:"applications_per_day"`
}

// ─── Provider Profile (Public) ───────────────────────────────────
type PublicProviderProfileResponse struct {
	ID                 uint                 `json:"id"`
	ProviderName       string               `json:"provider_name"`
	RegistrationNumber string               `json:"registration_number,omitempty"`
	Email              string               `json:"email,omitempty"`
	ContactNumber      string               `json:"contact_number,omitempty"`
	WebsiteURL         string               `json:"website_url,omitempty"`
	LogoURL            string               `json:"logo_url,omitempty"`
	Address            string               `json:"address,omitempty"`
	AboutText          string               `json:"about_text,omitempty"`
	Mission            string               `json:"mission,omitempty"`
	Values             string               `json:"values,omitempty"`
	FounderName        string               `json:"founder_name,omitempty"`
	FounderRole        string               `json:"founder_role,omitempty"`
	FounderMessage     string               `json:"founder_message,omitempty"`
	FounderImageURL    string               `json:"founder_image_url,omitempty"`
	FacebookURL        string               `json:"facebook_url,omitempty"`
	InstagramURL       string               `json:"instagram_url,omitempty"`
	YoutubeURL         string               `json:"youtube_url,omitempty"`
	LinkedInURL        string               `json:"linkedin_url,omitempty"`
	MapURL             string               `json:"map_url,omitempty"`
	BrochureURL        string                     `json:"brochure_url,omitempty"`
	BannerURL          string                     `json:"banner_url,omitempty"`
	Services           []ServiceResponse          `json:"services,omitempty"`


	Sectors            []SectorResponse     `json:"sectors,omitempty"`
	Projects           []ProjectResponse    `json:"projects,omitempty"`
	Gallery            []GalleryImageResponse `json:"gallery,omitempty"`
	Reviews            []ReviewResponse     `json:"reviews,omitempty"`
	Scholarships       []ScholarshipResponse `json:"scholarships,omitempty"`
	News               []NewsResponse        `json:"news,omitempty"`
	ScholarshipCount   int64                `json:"scholarship_count"`
	NewsCount          int64                `json:"news_count"`
	EventCount         int64                `json:"event_count"`
	BlogCount          int64                `json:"blog_count"`
}

type ServiceResponse struct {
	ID          uint   `json:"id"`
	Icon        string `json:"icon"`
	Title       string `json:"title"`
	Description  string `json:"description"`
	ExternalLink string `json:"external_link"`
	SortOrder    int    `json:"sort_order"`
}

type SectorResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	ImageURL     string `json:"image_url"`
	Icon         string `json:"icon"`
	ExternalLink string `json:"external_link"`
	SortOrder    int    `json:"sort_order"`
}

type ProjectResponse struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	Category     string `json:"category"`
	ExternalLink string `json:"external_link"`
	Date         string `json:"date"`
	SortOrder    int    `json:"sort_order"`
}

type GalleryImageResponse struct {
	ID        uint   `json:"id"`
	Folder    string `json:"folder"`
	ImageURL  string `json:"image_url"`
	Caption   string `json:"caption"`
	SortOrder int    `json:"sort_order"`
}

type ReviewResponse struct {
	ID         uint   `json:"id"`
	AuthorName string `json:"author_name"`
	AvatarURL  string `json:"avatar_url"`
	Rating     int    `json:"rating"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Pros       string `json:"pros"`
	Cons       string `json:"cons"`
	CreatedAt  string `json:"created_at"`
}

type CreateServiceRequest struct {
	Icon        string `json:"icon"`
	Title       string `json:"title"`
	Description  string `json:"description"`
	ExternalLink string `json:"external_link"`
	SortOrder    int    `json:"sort_order"`
}

type CreateSectorRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	ImageURL     string `json:"image_url"`
	Icon         string `json:"icon"`
	ExternalLink string `json:"external_link"`
	SortOrder    int    `json:"sort_order"`
}

type CreateProjectRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	Category     string `json:"category"`
	ExternalLink string `json:"external_link"`
	Date         string `json:"date"`
	SortOrder    int    `json:"sort_order"`
}

type CreateGalleryImageRequest struct {
	Folder    string `json:"folder"`
	ImageURL  string `json:"image_url"`
	Caption   string `json:"caption"`
	SortOrder int    `json:"sort_order"`
}

type UserResponse struct {
	ID        uint   `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Gender    string `json:"gender"`
	Address   string `json:"address"`
	Bio       string `json:"bio"`
	Role      string `json:"role"`
}

type CreateReviewRequest struct {
	AuthorName string `json:"author_name"`
	AvatarURL  string `json:"avatar_url"`
	Rating     int    `json:"rating"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Pros       string `json:"pros"`
	Cons       string `json:"cons"`
	Status     string `json:"status"`
}

type BatchImportWrittenExamResultItem struct {
	RollNumber        string   `json:"roll_number" binding:"required"`
	Marks             int      `json:"marks" binding:"required"`
	InterviewLocation string   `json:"interview_location"`
	InterviewDate     string   `json:"interview_date"`
	ReportingTime     string   `json:"reporting_time"`
	RequiredDocuments []string `json:"required_documents"`
}

type BatchImportWrittenExamResultsRequest struct {
	Results []BatchImportWrittenExamResultItem `json:"results" binding:"required"`
}

type FailedRow struct {
	RollNumber string `json:"roll_number"`
	Reason     string `json:"reason"`
}

type BatchImportSummary struct {
	Imported    int `json:"imported"`
	Overwritten int `json:"overwritten"`
	Skipped     int `json:"skipped"`
}

type BatchImportResponse struct {
	Summary    BatchImportSummary `json:"summary"`
	FailedRows []FailedRow        `json:"failed_rows"`
}

package scholarshipprovider

import "time"

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

type ScholarshipTypeItem struct {
	Type     string `json:"type"`
	Seats    string `json:"seats"`
	Coverage string `json:"coverage"`
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

type PartnerOrganization struct {
	Name    string `json:"name"`
	Website string `json:"website"`
}

type PartnerGroup struct {
	Heading  string                `json:"heading"`
	Partners []PartnerOrganization `json:"partners"`
}

type ExamCenterItem struct {
	Province      string `json:"province"`
	CenterName    string `json:"center_name"`
	ContactPerson string `json:"contact_person"`
	PhoneNumber   string `json:"phone_number"`
	MapCoordinates string `json:"map_coordinates"`
}

type DownloadItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type CreateScholarshipRequest struct {
	Title           string   `json:"title" binding:"required"`
	Description     string   `json:"description"`
	ImageURL        string   `json:"image_url"`
	Location        string   `json:"location"`
	Value           string   `json:"value"`
	Deadline        string   `json:"deadline"`
	DegreeLevel     string   `json:"degree_level"`
	FundingType     string   `json:"funding_type"`
	ScholarshipType string   `json:"scholarship_type"`
	FieldOfStudy    []string `json:"field_of_study"`
	Status          string   `json:"status"`

	BannerBackgroundImageURL string                `json:"banner_background_image_url"`
	AboutParagraph1          string                `json:"about_paragraph_1"`
	AboutParagraph2          string                `json:"about_paragraph_2"`
	VideoTutorials           []VideoTutorial       `json:"video_tutorials"`
	JourneyTimeline          []JourneyTimelineItem `json:"journey_timeline"`
	ScholarshipSectionTitle  string                `json:"scholarship_section_title"`
	ScholarshipSubtitle      string                `json:"scholarship_subtitle"`
	ScholarshipDescription1  string                `json:"scholarship_description_1"`
	ScholarshipDescription2  string                `json:"scholarship_description_2"`
	ScholarshipTypes         []ScholarshipTypeItem `json:"scholarship_types"`
	SelectionRubric          []SelectionRubricItem `json:"selection_rubric"`
	EligibilitySectionTitle  string                `json:"eligibility_section_title"`
	EligibilitySubtitle      string                `json:"eligibility_subtitle"`
	BasicEligibilityCriteria []string              `json:"basic_eligibility_criteria"`
	FullyFundedCriteria      []string              `json:"fully_funded_criteria"`
	PartiallyFundedCriteria  []string              `json:"partially_funded_criteria"`
	SelectionProcessSteps    []SelectionProcessStepItem `json:"selection_process_steps"`
	RequiredDocuments        []string              `json:"required_documents"`
	FAQs                     []FAQItem             `json:"faqs"`
	GalleryImages            []GalleryImageItem    `json:"gallery_images"`
	PartnerGroups            []PartnerGroup        `json:"partner_groups"`
	ExamCenters              []ExamCenterItem      `json:"exam_centers"`
	Downloads                []DownloadItem        `json:"downloads"`
}

type ScholarshipResponse struct {
	ID                  uint        `json:"id"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
	ProviderID          uint        `json:"provider_id"`
	Title               string      `json:"title"`
	Description         string      `json:"description"`
	ImageURL            *string     `json:"image_url"`
	Location            string      `json:"location"`
	Value               string      `json:"value"`
	Deadline            time.Time   `json:"deadline"`
	DegreeLevel         string      `json:"degree_level"`
	FundingType         string      `json:"funding_type"`
	ScholarshipType     string      `json:"scholarship_type"`
	FieldOfStudy        interface{} `json:"field_of_study"`
	EligibilityCriteria interface{} `json:"eligibility_criteria"`
	RequiredDocuments   interface{} `json:"required_documents"`
	Status              string      `json:"status"`
	ApplicationsCount   int         `json:"applications_count"`

	BannerBackgroundImageURL *string     `json:"banner_background_image_url"`
	AboutParagraph1          string      `json:"about_paragraph_1"`
	AboutParagraph2          string      `json:"about_paragraph_2"`
	VideoTutorials           interface{} `json:"video_tutorials"`
	JourneyTimeline          interface{} `json:"journey_timeline"`
	ScholarshipSectionTitle  string      `json:"scholarship_section_title"`
	ScholarshipSubtitle      string      `json:"scholarship_subtitle"`
	ScholarshipDescription1  string      `json:"scholarship_description_1"`
	ScholarshipDescription2  string      `json:"scholarship_description_2"`
	ScholarshipTypes         interface{} `json:"scholarship_types"`
	SelectionRubric          interface{} `json:"selection_rubric"`
	EligibilitySectionTitle  string      `json:"eligibility_section_title"`
	EligibilitySubtitle      string      `json:"eligibility_subtitle"`
	BasicEligibilityCriteria interface{} `json:"basic_eligibility_criteria"`
	FullyFundedCriteria      interface{} `json:"fully_funded_criteria"`
	PartiallyFundedCriteria  interface{} `json:"partially_funded_criteria"`
	SelectionProcessSteps    interface{} `json:"selection_process_steps"`
	FAQs                     interface{} `json:"faqs"`
	GalleryImages            interface{} `json:"gallery_images"`
	PartnerGroups            interface{} `json:"partner_groups"`
	ExamCenters              interface{} `json:"exam_centers"`
	Downloads                interface{} `json:"downloads"`
}

type ScholarshipListResponse struct {
	Scholarships []ScholarshipResponse `json:"scholarships"`
	Meta         PaginationMeta        `json:"meta"`
}

type ApplicationResponse struct {
	ID                uint                 `json:"id"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
	ScholarshipID     uint                 `json:"scholarship_id"`
	Scholarship       *ScholarshipResponse `json:"scholarship,omitempty"`
	UserID            uint                 `json:"user_id"`
	FirstName         string               `json:"first_name"`
	LastName          string               `json:"last_name"`
	Email             string               `json:"email"`
	PhoneNumber       string               `json:"phone_number"`
	Status            string               `json:"status"`
	EvaluationNotes   string               `json:"evaluation_notes"`
	Documents         []byte               `json:"documents"`
	PersonalStatement string               `json:"personal_statement"`
	Province          string               `json:"province"`
	Stream            string               `json:"stream"`
	GPA               float64              `json:"gpa"`
	Gender            string               `json:"gender"`
	Age               int                  `json:"age"`
	SchoolType        string               `json:"school_type"`
	ExamCenter        string               `json:"exam_center"`
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
}

type ProfileResponse struct {
	ID                 uint   `json:"id"`
	ProviderName       string `json:"provider_name"`
	RegistrationNumber string `json:"registration_number"`
	Email              string `json:"email"`
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
	Title   string `json:"title" binding:"required"`
	Content string `json:"content"`
	Image   string `json:"image_url"`
	Status  string `json:"status"`
}

type NewsResponse struct {
	ID          uint       `json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ProviderID  uint       `json:"provider_id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	ImageURL    *string    `json:"image_url"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"published_at"`
}

type NewsListResponse struct {
	News []NewsResponse `json:"news"`
	Meta PaginationMeta `json:"meta"`
}

type CreateEventRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Image       string `json:"image_url"`
	EventType   string `json:"event_type"`
	StartDate   string `json:"start_date" binding:"required"`
	EndDate     string `json:"end_date"`
	Location    string `json:"location"`
	Status      string `json:"status"`
}

type EventResponse struct {
	ID          uint       `json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ProviderID  uint       `json:"provider_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ImageURL    *string    `json:"image_url"`
	EventType   string     `json:"event_type"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     time.Time  `json:"end_date"`
	Location    string     `json:"location"`
	Status      string     `json:"status"`
	Attendees   int        `json:"attendees"`
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
	ScholarshipID uint   `json:"scholarship_id" binding:"required"`
	Title         string `json:"title" binding:"required"`
	Status        string `json:"status"`
	Results       []byte `json:"results"`
}

type ResultResponse struct {
	ID            uint       `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ProviderID    uint       `json:"provider_id"`
	ScholarshipID uint       `json:"scholarship_id"`
	Title         string     `json:"title"`
	Status        string     `json:"status"`
	PublishedAt   *time.Time `json:"published_at"`
	Results       []byte     `json:"results"`
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

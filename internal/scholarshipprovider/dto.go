package scholarshipprovider

import "time"

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
}

type ScholarshipResponse struct {
	ID                  uint      `json:"id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	ProviderID          uint      `json:"provider_id"`
	Title               string    `json:"title"`
	Description         string    `json:"description"`
	ImageURL            *string   `json:"image_url"`
	Location            string    `json:"location"`
	Value               string    `json:"value"`
	Deadline            time.Time `json:"deadline"`
	DegreeLevel         string    `json:"degree_level"`
	FundingType         string    `json:"funding_type"`
	ScholarshipType     string    `json:"scholarship_type"`
	FieldOfStudy        []byte    `json:"field_of_study"`
	EligibilityCriteria []byte    `json:"eligibility_criteria"`
	RequiredDocuments   []byte    `json:"required_documents"`
	Status              string    `json:"status"`
	ApplicationsCount   int       `json:"applications_count"`
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

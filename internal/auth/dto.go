package auth

import "encoding/json"

type RegisterRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required,min=6"`
	FirstName      string `json:"first_name" binding:"required"`
	LastName       string `json:"last_name" binding:"required"`
	Role           string `json:"role"`
	EducationLevel string `json:"education_level"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type SendOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	Type  string `json:"type"` // "verification" (registration) or "password_reset" (forgot password)
}

type VerifyOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required"`
}

type ResetPasswordRequest struct {
	Email    string `json:"email" binding:"required,email"`
	OTP      string `json:"otp" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type SavePreferencesRequest struct {
	PreferenceRole string                 `json:"preference_role" binding:"required"`
	PreferenceFlow string                 `json:"preference_flow" binding:"required"`
	Preferences    map[string]interface{} `json:"preferences" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

type EducationEntryRequest struct {
	Level           string `json:"level" binding:"required"`
	InstitutionName string `json:"institution_name" binding:"required"`
	BoardUniversity string `json:"board_university" binding:"required"`
	Country         string `json:"country" binding:"required"`
	Stream          string `json:"stream"`
	StartYear       string `json:"start_year" binding:"required"`
	EndYear         string `json:"end_year" binding:"required"`
	GradingSystem   string `json:"grading_system"`
	Grade           string `json:"grade"`
}

type EducationEntryResponse struct {
	ID              uint   `json:"id"`
	Level           string `json:"level"`
	InstitutionName string `json:"institution_name"`
	BoardUniversity string `json:"board_university"`
	Country         string `json:"country"`
	Stream          string `json:"stream"`
	StartYear       string `json:"start_year"`
	EndYear         string `json:"end_year"`
	GradingSystem   string `json:"grading_system"`
	Grade           string `json:"grade"`
}

type UpdateProfileRequest struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Phone       string `json:"phone"`
	DateOfBirth string `json:"date_of_birth"`
	Gender      string `json:"gender"`
	Nationality string `json:"nationality"`
	Address     string `json:"address"`
	Bio         string `json:"bio"`
	ImageURL    string `json:"image_url"`
}

type RegisterResponse struct {
	Email       string `json:"email"`
	RequiresOTP bool   `json:"requires_otp"`
}

type LoginResponse struct {
	User  interface{} `json:"user"`
	Token string      `json:"token"`
}

type ProfileResponse struct {
	ID          uint         `json:"id"`
	Email       string       `json:"email"`
	FirstName   string       `json:"first_name"`
	LastName    string       `json:"last_name"`
	Phone       string       `json:"phone"`
	DateOfBirth string       `json:"date_of_birth"`
	Gender      string       `json:"gender"`
	Nationality string       `json:"nationality"`
	Address     string       `json:"address"`
	Bio         string       `json:"bio"`
	Role        string       `json:"role"`
	GoogleID    *string      `json:"google_id"`
	ImageURL    string       `json:"image_url"`
	Preferences *Preferences `json:"preferences,omitempty"`
}

type PreferencesResponse struct {
	User User `json:"user"`
}

type InstitutionRegisterRequest struct {
	InstitutionName          string `json:"institution_name" binding:"required"`
	RegistrationNumber       string `json:"registration_number" binding:"required"`
	Email                    string `json:"email" binding:"required,email"`
	ContactNumber            string `json:"contact_number"`
	Province                 string `json:"province"`
	District                 string `json:"district"`
	LocalBody                string `json:"local_body"`
	OrganizationType         string `json:"organization_type"`
	PANNumber                string `json:"pan_number"`
	WebsiteURL               string `json:"website_url"`
	ContactPerson            string `json:"contact_person"`
	ContactPersonDesignation string `json:"contact_person_designation"`
	ContactPersonPhone       string `json:"contact_person_phone"`
}

type UpdateInstitutionRequest struct {
	InstitutionName string      `json:"institution_name"`
	Location        string      `json:"location"`
	Website         string      `json:"website"`
	Level           string      `json:"level"`
	Affiliation     string      `json:"affiliation"`
	About           string      `json:"about"`
	Vision          string      `json:"vision"`
	Mission         string      `json:"mission"`
	LogoURL         string      `json:"logo_url"`
	BannerURL       string      `json:"banner_url"`
	ProfileData     interface{} `json:"profile_data"`
}

type InstitutionDetailResponse struct {
	ID                 uint                   `json:"id"`
	InstitutionName    string                 `json:"institution_name"`
	Email              string                 `json:"email"`
	RegistrationNumber string                 `json:"registration_number"`
	Status             string                 `json:"status"`
	Claimed            bool                   `json:"claimed"`
	Verified           bool                   `json:"verified"`
	Featured           bool                   `json:"featured"`
	District           string                 `json:"district"`
	WebsiteURL         string                 `json:"website_url"`
	LogoURL            string                 `json:"logo_url"`
	BannerURL          string                 `json:"banner_url"`
	About              string                 `json:"about"`
	Vision             string                 `json:"vision"`
	Mission            string                 `json:"mission"`
	Level              string                 `json:"level"`
	Affiliation        string                 `json:"affiliation"`
	ProfileData        map[string]interface{} `json:"profile_data"`
}

type CreateInstitutionRequest struct {
	InstitutionName string      `json:"institution_name" binding:"required"`
	Location        string      `json:"location"`
	Website         string      `json:"website"`
	Level           string      `json:"level"`
	Affiliation     string      `json:"affiliation"`
	LogoURL         string      `json:"logo_url"`
	BannerURL       string      `json:"banner_url"`
	About           string      `json:"about"`
	Vision          string      `json:"vision"`
	Mission         string      `json:"mission"`
	Videos          interface{} `json:"videos"`
	OverviewData    interface{} `json:"overview_data"`
	LeadershipData  interface{} `json:"leadership_data"`
	CoursesData     interface{} `json:"courses_data"`
	ProgramsData    interface{} `json:"programs_data"`
	FacilitiesData  interface{} `json:"facilities_data"`
	AlumniData      interface{} `json:"alumni_data"`
	GalleryData     interface{} `json:"gallery_data"`
	DownloadsData   interface{} `json:"downloads_data"`
}

type InstitutionApprovalRequest struct {
	InstitutionID   uint   `json:"institution_id" binding:"required"`
	Action          string `json:"action" binding:"required"` // "approved" or "rejected"
	RejectionReason string `json:"rejection_reason"`
}

type InstitutionLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ScholarshipProviderRegisterRequest struct {
	ProviderName       string `json:"provider_name" binding:"required"`
	RegistrationNumber string `json:"registration_number" binding:"required"`
	Email              string `json:"email" binding:"required,email"`
	ContactNumber      string `json:"contact_number"`
	PANNumber          string `json:"pan_number" binding:"omitempty,len=9,numeric"`
	WebsiteURL         string `json:"website_url"`
}

type ScholarshipProviderApprovalRequest struct {
	ProviderID uint   `json:"provider_id" binding:"required"`
	Action     string `json:"action" binding:"required"` // "approved" or "rejected"
}

type ScholarshipProviderLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type SuperadminRegisterRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=6"`
	FirstName  string `json:"first_name" binding:"required"`
	LastName   string `json:"last_name" binding:"required"`
	AccessCode string `json:"access_code" binding:"required"`
}

type SuperadminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UpdateProfileAccessRequest struct {
	ProfileAccess map[string]bool `json:"profile_access" binding:"required"`
}

type ClaimRegisterRequest struct {
	CollegeID                uint   `json:"college_id" binding:"required"`
	InstitutionName          string `json:"institution_name" binding:"required"`
	RegistrationNumber       string `json:"registration_number" binding:"required"`
	Email                    string `json:"email" binding:"required,email"`
	ContactNumber            string `json:"contact_number"`
	Province                 string `json:"province"`
	District                 string `json:"district"`
	LocalBody                string `json:"local_body"`
	OrganizationType         string `json:"organization_type"`
	PANNumber                string `json:"pan_number"`
	WebsiteURL               string `json:"website_url"`
	ContactPerson            string `json:"contact_person"`
	ContactPersonDesignation string `json:"contact_person_designation"`
	ContactPersonPhone       string `json:"contact_person_phone"`
}

type RejectClaimRequest struct {
	ClaimID         uint   `json:"claim_id" binding:"required"`
	RejectionReason string `json:"rejection_reason"`
}

type RecordPaymentRequest struct {
	PaymentDate string  `json:"payment_date" binding:"required"`
	PaidForDays int     `json:"paid_for_days" binding:"required"`
	Amount      float64 `json:"amount"`
	Remarks     string  `json:"remarks"`
}

type InstitutionFilter struct {
	Search        string `form:"search"`
	Type          string `form:"type"`
	PaymentStatus string `form:"payment_status"`
	Verification  string `form:"verification"`
	Claim         string `form:"claim"`
	Province      string `form:"province"`
	Level         string `form:"level"`
}

type SuperadminDashboardStats struct {
	TotalStudents       int64 `json:"total_students"`
	TotalInstitutions   int64 `json:"total_institutions"`
	TotalProviders      int64 `json:"total_providers"`
	PendingInstitutions int64 `json:"pending_institutions"`
	PendingProviders    int64 `json:"pending_providers"`
}

// Superadmin CRUD request types for programs, entrances, admission pages
type SuperadminCreateProgramRequest struct {
	InstitutionID uint        `json:"institution_id" binding:"required"`
	Name          string      `json:"name" binding:"required"`
	Description   string      `json:"description"`
	Duration      string      `json:"duration"`
	Fee           string      `json:"fee"`
	Eligibility   string      `json:"eligibility"`
	Capacity      int         `json:"capacity"`
	BannerURL     string      `json:"banner_url"`
	Data          interface{} `json:"data"`
	Status        string      `json:"status"`
}

type SuperadminUpdateProgramRequest struct {
	InstitutionID uint        `json:"institution_id" binding:"required"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	Duration      string      `json:"duration"`
	Fee           string      `json:"fee"`
	Eligibility   string      `json:"eligibility"`
	Capacity      int         `json:"capacity"`
	BannerURL     string      `json:"banner_url"`
	Data          interface{} `json:"data"`
	Status        string      `json:"status"`
}

type SuperadminCreateEntranceRequest struct {
	InstitutionID     uint            `json:"institution_id" binding:"required"`
	Title             string          `json:"title" binding:"required"`
	Description       string          `json:"description"`
	Program           string          `json:"program"`
	Date              string          `json:"date" binding:"required"`
	StartTime         string          `json:"start_time"`
	EndTime           string          `json:"end_time"`
	Duration          int             `json:"duration"`
	TotalMarks        int             `json:"total_marks"`
	PassingMarks      int             `json:"passing_marks"`
	TotalSeats        int             `json:"total_seats"`
	Instructions      string          `json:"instructions"`
	HeroBanner        string          `json:"hero_banner"`
	Questions         interface{}     `json:"questions"`
	Status            string          `json:"status"`
	ApplicationFee    string          `json:"application_fee"`
	OverviewDetails   json.RawMessage `json:"overview_details"`
	ExamDateSchedules json.RawMessage `json:"exam_date_schedules"`
	EligibilityList   json.RawMessage `json:"eligibility_list"`
	ApplicationSteps  json.RawMessage `json:"application_steps"`
	ExamPattern       json.RawMessage `json:"exam_pattern"`
	SubjectMarks      json.RawMessage `json:"subject_marks"`
	ModelSets         json.RawMessage `json:"model_sets"`
	UpcomingDates     json.RawMessage `json:"upcoming_dates"`
	ContactPersons    json.RawMessage `json:"contact_persons"`
	Faqs              json.RawMessage `json:"faqs"`
	ApplicationLink   string          `json:"application_link"`
	NoticeFile        string          `json:"notice_file"`
}

type SuperadminUpdateEntranceRequest struct {
	InstitutionID     uint            `json:"institution_id" binding:"required"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	Program           string          `json:"program"`
	Date              string          `json:"date"`
	StartTime         string          `json:"start_time"`
	EndTime           string          `json:"end_time"`
	Duration          int             `json:"duration"`
	TotalMarks        int             `json:"total_marks"`
	PassingMarks      int             `json:"passing_marks"`
	TotalSeats        int             `json:"total_seats"`
	Instructions      string          `json:"instructions"`
	HeroBanner        string          `json:"hero_banner"`
	Questions         interface{}     `json:"questions"`
	Status            string          `json:"status"`
	ApplicationFee    string          `json:"application_fee"`
	OverviewDetails   json.RawMessage `json:"overview_details"`
	ExamDateSchedules json.RawMessage `json:"exam_date_schedules"`
	EligibilityList   json.RawMessage `json:"eligibility_list"`
	ApplicationSteps  json.RawMessage `json:"application_steps"`
	ExamPattern       json.RawMessage `json:"exam_pattern"`
	SubjectMarks      json.RawMessage `json:"subject_marks"`
	ModelSets         json.RawMessage `json:"model_sets"`
	UpcomingDates     json.RawMessage `json:"upcoming_dates"`
	ContactPersons    json.RawMessage `json:"contact_persons"`
	Faqs              json.RawMessage `json:"faqs"`
	ApplicationLink   string          `json:"application_link"`
	NoticeFile        string          `json:"notice_file"`
}

type SuperadminCreateAdmissionPageRequest struct {
	InstitutionID uint            `json:"institution_id" binding:"required"`
	Data          json.RawMessage `json:"data"`
	Status        string          `json:"status"`
}

type SuperadminUpdateAdmissionPageRequest struct {
	InstitutionID uint            `json:"institution_id" binding:"required"`
	Data          json.RawMessage `json:"data"`
	Status        *string         `json:"status"`
}

type SuperadminDeleteRequest struct {
	InstitutionID uint `json:"institution_id" binding:"required"`
}

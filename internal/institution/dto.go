package institution

type CreateProgramRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Duration    string `json:"duration"`
	Fee         string `json:"fee"`
	Eligibility string `json:"eligibility"`
	Capacity    int    `json:"capacity"`
}

type UpdateProgramRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Duration    string `json:"duration"`
	Fee         string `json:"fee"`
	Eligibility string `json:"eligibility"`
	Capacity    int    `json:"capacity"`
	Status      string `json:"status"`
}

type ProgramResponse struct {
	ID            uint   `json:"id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	InstitutionID uint   `json:"institution_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Duration      string `json:"duration"`
	Fee           string `json:"fee"`
	Eligibility   string `json:"eligibility"`
	Capacity      int    `json:"capacity"`
	Status        string `json:"status"`
}

type CreateMediaRequest struct {
	URL   string `json:"url" binding:"required"`
	Type  string `json:"type" binding:"required"`
	Title string `json:"title"`
}

type MediaResponse struct {
	ID            uint   `json:"id"`
	CreatedAt     string `json:"created_at"`
	InstitutionID uint   `json:"institution_id"`
	URL           string `json:"url"`
	Type          string `json:"type"`
	Title         string `json:"title"`
}

type CounsellingSessionResponse struct {
	ID            uint   `json:"id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	InstitutionID uint   `json:"institution_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	ScheduledAt   string `json:"scheduled_at"`
	Duration      int    `json:"duration"`
	MaxSeats      int    `json:"max_seats"`
	BookedSeats   int    `json:"booked_seats"`
	Status        string `json:"status"`
}

type CounsellingBookingResponse struct {
	ID        uint                        `json:"id"`
	CreatedAt string                      `json:"created_at"`
	UpdatedAt string                      `json:"updated_at"`
	SessionID uint                        `json:"session_id"`
	UserID    uint                        `json:"user_id"`
	Status    string                      `json:"status"`
	Notes     string                      `json:"notes"`
	Session   *CounsellingSessionResponse `json:"session,omitempty"`
}

type UpdateBookingStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type CreateEntranceRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Date        string `json:"date" binding:"required"`
	Duration    int    `json:"duration"`
	TotalSeats  int    `json:"total_seats"`
}

type UpdateEntranceRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Duration    int    `json:"duration"`
	TotalSeats  int    `json:"total_seats"`
	Status      string `json:"status"`
}

type EntranceResponse struct {
	ID            uint   `json:"id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	InstitutionID uint   `json:"institution_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Date          string `json:"date"`
	Duration      int    `json:"duration"`
	TotalSeats    int    `json:"total_seats"`
	FilledSeats   int    `json:"filled_seats"`
	Status        string `json:"status"`
}

type EntranceApplicantResponse struct {
	ID         uint    `json:"id"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	EntranceID uint    `json:"entrance_id"`
	UserID     uint    `json:"user_id"`
	Status     string  `json:"status"`
	Score      float64 `json:"score"`
	Rank       int     `json:"rank"`
}

type CreateEventRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Date        string `json:"date" binding:"required"`
	Location    string `json:"location"`
	Image       string `json:"image"`
}

type UpdateEventRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Location    string `json:"location"`
	Image       string `json:"image"`
	Status      string `json:"status"`
}

type EventResponse struct {
	ID            uint   `json:"id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	InstitutionID uint   `json:"institution_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Date          string `json:"date"`
	Location      string `json:"location"`
	Image         string `json:"image"`
	Status        string `json:"status"`
}

type CreateNewsRequest struct {
	Title    string `json:"title" binding:"required"`
	Content  string `json:"content"`
	Excerpt  string `json:"excerpt"`
	Image    string `json:"image"`
	Category string `json:"category"`
}

type UpdateNewsRequest struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Excerpt  string `json:"excerpt"`
	Image    string `json:"image"`
	Category string `json:"category"`
}

type NewsResponse struct {
	ID            uint   `json:"id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	InstitutionID uint   `json:"institution_id"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	Excerpt       string `json:"excerpt"`
	Image         string `json:"image"`
	Category      string `json:"category"`
	Published     bool   `json:"published"`
}

type CreateQMSRequest struct {
	Title       string  `json:"title" binding:"required"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Score       float64 `json:"score"`
}

type UpdateQMSRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Status      string  `json:"status"`
	Score       float64 `json:"score"`
}

type QMSResponse struct {
	ID            uint    `json:"id"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	InstitutionID uint    `json:"institution_id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Category      string  `json:"category"`
	Status        string  `json:"status"`
	Score         float64 `json:"score"`
	Documents     []byte  `json:"documents"`
}

type CreateMessageRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	Subject string `json:"subject" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type MessageResponse struct {
	ID            uint   `json:"id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	InstitutionID uint   `json:"institution_id"`
	UserID        uint   `json:"user_id"`
	Subject       string `json:"subject"`
	Content       string `json:"content"`
	Read          bool   `json:"read"`
	Direction     string `json:"direction"`
}

type StudentContact struct {
	UserID      uint   `json:"user_id"`
	Name        string `json:"name"`
	LastMessage string `json:"last_message"`
	Unread      int    `json:"unread"`
}

type UpdateSettingsRequest struct {
	EmailNotifs   bool   `json:"email_notifications"`
	Timezone      string `json:"timezone"`
	Language      string `json:"language"`
	PublicProfile bool   `json:"public_profile"`
}

type SettingsResponse struct {
	ID            uint   `json:"id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	InstitutionID uint   `json:"institution_id"`
	EmailNotifs   bool   `json:"email_notifications"`
	Timezone      string `json:"timezone"`
	Language      string `json:"language"`
	PublicProfile bool   `json:"public_profile"`
}

type UpdateProfileRequest struct {
	InstitutionName      string      `json:"institution_name"`
	RegistrationNumber   string      `json:"registration_number"`
	Location             string      `json:"location"`
	Website              string      `json:"website"`
	LogoURL              string      `json:"logo_url"`
	BannerURL            string      `json:"banner_url"`
	About                string      `json:"about"`
	Vision               string      `json:"vision"`
	Mission              string      `json:"mission"`
	Videos               interface{} `json:"videos"`
	OverviewData         interface{} `json:"overview_data"`
	LeadershipData       interface{} `json:"leadership_data"`
	CoursesData          interface{} `json:"courses_data"`
	ProgramsData         interface{} `json:"programs_data"`
	FacilitiesData       interface{} `json:"facilities_data"`
	AlumniData           interface{} `json:"alumni_data"`
	DownloadsData        interface{} `json:"downloads_data"`
	WhatsNewData         interface{} `json:"whats_new_data"`
	EligibilityData      interface{} `json:"eligibility_data"`
	AdmissionProcessData interface{} `json:"admission_process_data"`
	ScholarshipsData     interface{} `json:"scholarships_data"`
	FaqsData             interface{} `json:"faqs_data"`
	ContactPersonsData   interface{} `json:"contact_persons_data"`
	BrochureData         interface{} `json:"brochure_data"`
}

type ProfileResponse struct {
	ID                    uint        `json:"id"`
	InstitutionName       string      `json:"institution_name"`
	Email                 string      `json:"email"`
	RegistrationNumber    string      `json:"registration_number"`
	Role                  string      `json:"role"`
	Location              string      `json:"location,omitempty"`
	Website               string      `json:"website,omitempty"`
	LogoURL               string      `json:"logo_url,omitempty"`
	BannerURL             string      `json:"banner_url,omitempty"`
	About                 string      `json:"about,omitempty"`
	Vision                string      `json:"vision,omitempty"`
	Mission               string      `json:"mission,omitempty"`
	Videos                interface{} `json:"videos,omitempty"`
	OverviewData          interface{} `json:"overview_data,omitempty"`
	LeadershipData        interface{} `json:"leadership_data,omitempty"`
	CoursesData           interface{} `json:"courses_data,omitempty"`
	ProgramsData          interface{} `json:"programs_data,omitempty"`
	FacilitiesData        interface{} `json:"facilities_data,omitempty"`
	AlumniData            interface{} `json:"alumni_data,omitempty"`
	DownloadsData         interface{} `json:"downloads_data,omitempty"`
	WhatsNewData          interface{} `json:"whats_new_data,omitempty"`
	EligibilityData       interface{} `json:"eligibility_data,omitempty"`
	AdmissionProcessData  interface{} `json:"admission_process_data,omitempty"`
	ScholarshipsData      interface{} `json:"scholarships_data,omitempty"`
	FaqsData              interface{} `json:"faqs_data,omitempty"`
	ContactPersonsData    interface{} `json:"contact_persons_data,omitempty"`
	BrochureData          interface{} `json:"brochure_data,omitempty"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

type DashboardResponse struct {
	TotalPrograms   int64 `json:"total_programs"`
	TotalStudents   int64 `json:"total_students"`
	ActiveEntrances int64 `json:"active_entrances"`
	PendingBookings int64 `json:"pending_bookings"`
	UnreadMessages  int64 `json:"unread_messages"`
}

type ProgramStat struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Entrances int64  `json:"entrances"`
}

type AnalyticsResponse struct {
	ProgramStats    []ProgramStat `json:"program_stats"`
	TotalApplicants int64         `json:"total_applicants"`
}

type CreateScholarshipRequest struct {
	Title           string   `json:"title" binding:"required"`
	Provider        string   `json:"provider"`
	Location        string   `json:"location"`
	Value           string   `json:"value"`
	Deadline        string   `json:"deadline"`
	DegreeLevel     string   `json:"degree_level"`
	FundingType     string   `json:"funding_type"`
	ScholarshipType string   `json:"scholarship_type"`
	Description     string   `json:"description"`
	ImageURL        string   `json:"image_url"`
	FieldOfStudy    []string `json:"field_of_study"`
}

type UpdateScholarshipRequest struct {
	Title           string   `json:"title"`
	Provider        string   `json:"provider"`
	Location        string   `json:"location"`
	Value           string   `json:"value"`
	Deadline        string   `json:"deadline"`
	DegreeLevel     string   `json:"degree_level"`
	FundingType     string   `json:"funding_type"`
	ScholarshipType string   `json:"scholarship_type"`
	Description     string   `json:"description"`
	ImageURL        string   `json:"image_url"`
	FieldOfStudy    []string `json:"field_of_study"`
}

type ScholarshipResponse struct {
	ID              uint     `json:"id"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	Title           string   `json:"title"`
	Provider        string   `json:"provider"`
	Location        string   `json:"location"`
	Value           string   `json:"value"`
	Deadline        string   `json:"deadline"`
	DegreeLevel     string   `json:"degree_level"`
	FundingType     string   `json:"funding_type"`
	ScholarshipType string   `json:"scholarship_type"`
	Description     string   `json:"description"`
	ImageURL        string   `json:"image_url"`
	FieldOfStudy    []string `json:"field_of_study"`
}

type AdmissionResponse struct {
	ID                uint        `json:"id"`
	CreatedAt         string      `json:"created_at"`
	UpdatedAt         string      `json:"updated_at"`
	UserID            *uint       `json:"user_id,omitempty"`
	CollegeID         uint        `json:"college_id"`
	ProgramName       string      `json:"program_name"`
	ProgramLevel      string      `json:"program_level"`
	StudentName       string      `json:"student_name"`
	StudentEmail      string      `json:"student_email"`
	StudentPhone      string      `json:"student_phone"`
	DateOfBirth       *string     `json:"date_of_birth,omitempty"`
	Gender            string      `json:"gender,omitempty"`
	Address           string      `json:"address,omitempty"`
	City              string      `json:"city,omitempty"`
	LastQualification string      `json:"last_qualification,omitempty"`
	Institution       string      `json:"institution,omitempty"`
	GPA               string      `json:"gpa,omitempty"`
	EntranceScore     string      `json:"entrance_score,omitempty"`
	Statement         string      `json:"statement,omitempty"`
	Status            string      `json:"status"`
	Notes             string      `json:"notes,omitempty"`
	ReviewedBy        *uint       `json:"reviewed_by,omitempty"`
	ReviewedAt        *string     `json:"reviewed_at,omitempty"`
	College           *CollegeDTO `json:"college,omitempty"`
	User              *UserDTO    `json:"user,omitempty"`
}

type UpdateAdmissionStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending under_review approved rejected waitlisted"`
	Notes  string `json:"notes"`
}

type ScholarshipApplicationResponse struct {
	ID            uint                 `json:"id"`
	CreatedAt     string               `json:"created_at"`
	UpdatedAt     string               `json:"updated_at"`
	ScholarshipID uint                 `json:"scholarship_id"`
	UserID        uint                 `json:"user_id"`
	Status        string               `json:"status"`
	CoverLetter   string               `json:"cover_letter"`
	Documents     []byte               `json:"documents"`
	Scholarship   *ScholarshipResponse `json:"scholarship,omitempty"`
	User          *UserDTO             `json:"user,omitempty"`
}

type UpdateScholarshipApplicationStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending under_review approved rejected shortlisted"`
	Notes  string `json:"notes"`
}

type CollegeDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type UserDTO struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
}

type PublicInstitutionResponse struct {
	ID              uint   `json:"id"`
	InstitutionName string `json:"institution_name"`
	LogoURL         string `json:"logo_url,omitempty"`
	BannerURL       string `json:"banner_url,omitempty"`
	About           string `json:"about,omitempty"`
	District        string `json:"district,omitempty"`
	WebsiteURL      string `json:"website_url,omitempty"`
	Status          string `json:"status"`
}

type PublicInstitutionDetailResponse struct {
	ID                 uint        `json:"id"`
	InstitutionName    string      `json:"institution_name"`
	LogoURL            string      `json:"logo_url,omitempty"`
	BannerURL          string      `json:"banner_url,omitempty"`
	About              string      `json:"about,omitempty"`
	Vision             string      `json:"vision,omitempty"`
	Mission            string      `json:"mission,omitempty"`
	District           string      `json:"district,omitempty"`
	WebsiteURL         string      `json:"website_url,omitempty"`
	Videos             interface{} `json:"videos,omitempty"`
	OverviewData       interface{} `json:"overview_data,omitempty"`
	LeadershipData     interface{} `json:"leadership_data,omitempty"`
	CoursesData        interface{} `json:"courses_data,omitempty"`
	ProgramsData       interface{} `json:"programs_data,omitempty"`
	FacilitiesData     interface{} `json:"facilities_data,omitempty"`
	AlumniData         interface{} `json:"alumni_data,omitempty"`
	GalleryData        interface{} `json:"gallery_data,omitempty"`
	DownloadsData      interface{} `json:"downloads_data,omitempty"`
}

type PaginatedResponse struct {
	Meta PaginationMeta `json:"meta"`
}

type PaginationMeta struct {
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

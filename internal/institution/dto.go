package institution

import "encoding/json"

type CreateProgramRequest struct {
	Name                string      `json:"name" binding:"required"`
	Description         string      `json:"description"`
	Duration            string      `json:"duration"`
	Fee                 string      `json:"fee"`
	Eligibility         string      `json:"eligibility"`
	Capacity            int         `json:"capacity"`
	BannerURL           string      `json:"banner_url"`
	InstitutionName     string      `json:"institution_name"`
	InstitutionLocation string      `json:"institution_location"`
	InstitutionLink     string      `json:"institution_link"`
	Data                interface{} `json:"data"`
	Status              string      `json:"status"`
	GlobalCourseID      *uint       `json:"globalCourseId"`
}

type UpdateProgramRequest struct {
	Name                string      `json:"name"`
	Description         string      `json:"description"`
	Duration            string      `json:"duration"`
	Fee                 string      `json:"fee"`
	Eligibility         string      `json:"eligibility"`
	Capacity            int         `json:"capacity"`
	BannerURL           string      `json:"banner_url"`
	InstitutionName     string      `json:"institution_name"`
	InstitutionLocation string      `json:"institution_location"`
	InstitutionLink     string      `json:"institution_link"`
	Data                interface{} `json:"data"`
	Status              string      `json:"status"`
	GlobalCourseID      *uint       `json:"globalCourseId"`
}

type ProgramResponse struct {
	ID                  uint        `json:"id"`
	CreatedAt           string      `json:"created_at"`
	UpdatedAt           string      `json:"updated_at"`
	InstitutionID       uint        `json:"institution_id"`
	InstitutionName     string      `json:"institution_name"`
	InstitutionLocation string      `json:"institution_location"`
	InstitutionLink     string      `json:"institution_link"`
	Name                string      `json:"name"`
	Description         string      `json:"description"`
	Duration            string      `json:"duration"`
	Fee                 string      `json:"fee"`
	Eligibility         string      `json:"eligibility"`
	Capacity            int         `json:"capacity"`
	BannerURL           string      `json:"banner_url"`
	Data                interface{} `json:"data"`
	Status              string      `json:"status"`
	GlobalCourseID      *uint       `json:"globalCourseId,omitempty"`
	GlobalCourseTitle   string      `json:"globalCourseTitle,omitempty"`
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
	ActualStatus  string `json:"actual_status"`
}

type UpdateCounsellingSessionRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ScheduledAt string `json:"scheduled_at"`
	Duration    int    `json:"duration"`
	MaxSeats    int    `json:"max_seats"`
	Status      string `json:"status"`
}

type CounsellingBookingResponse struct {
	ID               uint                        `json:"id"`
	CreatedAt        string                      `json:"created_at"`
	UpdatedAt        string                      `json:"updated_at"`
	SessionID        uint                        `json:"session_id"`
	UserID           uint                        `json:"user_id"`
	Status           string                      `json:"status"`
	Notes            string                      `json:"notes"`
	StudentName      string                      `json:"student_name"`
	StudentPhone     string                      `json:"student_phone"`
	StudentEmail     string                      `json:"student_email"`
	ProgramLevel     string                      `json:"program_level"`
	InterestedCourse string                      `json:"interested_course"`
	SessionMode      string                      `json:"session_mode"`
	MeetingLink      string                      `json:"meeting_link"`
	MeetingPlatform  string                      `json:"meeting_platform"`
	Session          *CounsellingSessionResponse `json:"session,omitempty"`
}

type UpdateBookingStatusRequest struct {
	Status          string `json:"status" binding:"required"`
	MeetingLink     string `json:"meeting_link"`
	MeetingPlatform string `json:"meeting_platform"`
}

type CreateCounsellingSessionRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ScheduledAt string `json:"scheduled_at"`
	Duration    int    `json:"duration"`
	MaxSeats    int    `json:"max_seats"`
}

type PublicCounsellingBookingRequest struct {
	SessionID        uint   `json:"session_id" binding:"required"`
	ProgramLevel     string `json:"program_level" binding:"required"`
	InterestedCourse string `json:"interested_course" binding:"required"`
	SessionMode      string `json:"session_mode" binding:"required"`
	StudentName      string `json:"student_name" binding:"required"`
	StudentPhone     string `json:"student_phone" binding:"required"`
	StudentEmail     string `json:"student_email" binding:"required,email"`
	StudentNotes     string `json:"student_notes"`
}

type PublicCounsellingSessionResponse struct {
	ID             uint   `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	ScheduledAt    string `json:"scheduled_at"`
	Duration       int    `json:"duration"`
	MaxSeats       int    `json:"max_seats"`
	BookedSeats    int    `json:"booked_seats"`
	AvailableSeats int    `json:"available_seats"`
	Status         string `json:"status"`
}

type CreateEntranceRequest struct {
	Title                  string          `json:"title" binding:"required"`
	Description            string          `json:"description"`
	Program                string          `json:"program"`
	Date                   string          `json:"date" binding:"required"`
	StartTime              string          `json:"start_time"`
	EndTime                string          `json:"end_time"`
	Duration               int             `json:"duration"`
	TotalMarks             int             `json:"total_marks"`
	PassingMarks           int             `json:"passing_marks"`
	TotalSeats             int             `json:"total_seats"`
	Instructions           string          `json:"instructions"`
	HeroBanner             string          `json:"hero_banner"`
	Questions              interface{}     `json:"questions"`
	Status                 string          `json:"status"`
	InstitutionName        string          `json:"institution_name"`
	InstitutionLocation    string          `json:"institution_location"`
	InstitutionLink        string          `json:"institution_link"`
	InstitutionAffiliation string          `json:"institution_affiliation"`
	InstitutionLogo        string          `json:"institution_logo"`
	ApplicationFee         string          `json:"application_fee"`
	OverviewDetails        json.RawMessage `json:"overview_details"`
	ExamDateSchedules      json.RawMessage `json:"exam_date_schedules"`
	EligibilityList        json.RawMessage `json:"eligibility_list"`
	ApplicationSteps       json.RawMessage `json:"application_steps"`
	ExamPattern            json.RawMessage `json:"exam_pattern"`
	SubjectMarks           json.RawMessage `json:"subject_marks"`
	ModelSets              json.RawMessage `json:"model_sets"`
	UpcomingDates          json.RawMessage `json:"upcoming_dates"`
	ContactPersons         json.RawMessage `json:"contact_persons"`
	Faqs                   json.RawMessage `json:"faqs"`
	Email                  string          `json:"email"`
	ContactNumber          string          `json:"contact_number"`
	SocialLinks            json.RawMessage `json:"social_links"`
	ApplicationLink        string          `json:"application_link"`
	NoticeFile             string          `json:"notice_file"`
	EmbeddedMap            string          `json:"embedded_map"`
	RequiredDocuments      json.RawMessage `json:"required_documents"`
	ExaminationSchedule    json.RawMessage `json:"examination_schedule"`
	ProgramsOffered        json.RawMessage `json:"programs_offered"`
}

type UpdateEntranceRequest struct {
	Title                  string          `json:"title"`
	Description            string          `json:"description"`
	Program                string          `json:"program"`
	Date                   string          `json:"date"`
	StartTime              string          `json:"start_time"`
	EndTime                string          `json:"end_time"`
	Duration               int             `json:"duration"`
	TotalMarks             int             `json:"total_marks"`
	PassingMarks           int             `json:"passing_marks"`
	TotalSeats             int             `json:"total_seats"`
	Instructions           string          `json:"instructions"`
	HeroBanner             string          `json:"hero_banner"`
	Questions              interface{}     `json:"questions"`
	Status                 string          `json:"status"`
	InstitutionName        string          `json:"institution_name"`
	InstitutionLocation    string          `json:"institution_location"`
	InstitutionLink        string          `json:"institution_link"`
	InstitutionAffiliation string          `json:"institution_affiliation"`
	InstitutionLogo        string          `json:"institution_logo"`
	ApplicationFee         string          `json:"application_fee"`
	OverviewDetails        json.RawMessage `json:"overview_details"`
	ExamDateSchedules      json.RawMessage `json:"exam_date_schedules"`
	EligibilityList        json.RawMessage `json:"eligibility_list"`
	ApplicationSteps       json.RawMessage `json:"application_steps"`
	ExamPattern            json.RawMessage `json:"exam_pattern"`
	SubjectMarks           json.RawMessage `json:"subject_marks"`
	ModelSets              json.RawMessage `json:"model_sets"`
	UpcomingDates          json.RawMessage `json:"upcoming_dates"`
	ContactPersons         json.RawMessage `json:"contact_persons"`
	Faqs                   json.RawMessage `json:"faqs"`
	Email                  string          `json:"email"`
	ContactNumber          string          `json:"contact_number"`
	SocialLinks            json.RawMessage `json:"social_links"`
	ApplicationLink        string          `json:"application_link"`
	NoticeFile             string          `json:"notice_file"`
	EmbeddedMap            string          `json:"embedded_map"`
	RequiredDocuments      json.RawMessage `json:"required_documents"`
	ExaminationSchedule    json.RawMessage `json:"examination_schedule"`
	ProgramsOffered        json.RawMessage `json:"programs_offered"`
}

type EntranceResponse struct {
	ID                     uint            `json:"id"`
	CreatedAt              string          `json:"created_at"`
	UpdatedAt              string          `json:"updated_at"`
	InstitutionID          uint            `json:"institution_id"`
	InstitutionName        string          `json:"institution_name"`
	InstitutionLocation    string          `json:"institution_location"`
	InstitutionLink        string          `json:"institution_link"`
	InstitutionAffiliation string          `json:"institution_affiliation"`
	InstitutionLogo        string          `json:"institution_logo"`
	Title                  string          `json:"title"`
	Description            string          `json:"description"`
	Program                string          `json:"program"`
	Date                   string          `json:"date"`
	StartTime              string          `json:"start_time"`
	EndTime                string          `json:"end_time"`
	Duration               int             `json:"duration"`
	TotalMarks             int             `json:"total_marks"`
	PassingMarks           int             `json:"passing_marks"`
	TotalSeats             int             `json:"total_seats"`
	FilledSeats            int             `json:"filled_seats"`
	Instructions           string          `json:"instructions"`
	HeroBanner             string          `json:"hero_banner"`
	Questions              interface{}     `json:"questions"`
	Status                 string          `json:"status"`
	ApplicationFee         string          `json:"application_fee"`
	OverviewDetails        json.RawMessage `json:"overview_details"`
	ExamDateSchedules      json.RawMessage `json:"exam_date_schedules"`
	EligibilityList        json.RawMessage `json:"eligibility_list"`
	ApplicationSteps       json.RawMessage `json:"application_steps"`
	ExamPattern            json.RawMessage `json:"exam_pattern"`
	SubjectMarks           json.RawMessage `json:"subject_marks"`
	ModelSets              json.RawMessage `json:"model_sets"`
	UpcomingDates          json.RawMessage `json:"upcoming_dates"`
	ContactPersons         json.RawMessage `json:"contact_persons"`
	Faqs                   json.RawMessage `json:"faqs"`
	Email                  string          `json:"email"`
	ContactNumber          string          `json:"contact_number"`
	SocialLinks            json.RawMessage `json:"social_links"`
	ApplicationLink        string          `json:"application_link"`
	NoticeFile             string          `json:"notice_file"`
	EmbeddedMap            string          `json:"embedded_map"`
	RequiredDocuments      json.RawMessage `json:"required_documents"`
	ExaminationSchedule    json.RawMessage `json:"examination_schedule"`
	ProgramsOffered        json.RawMessage `json:"programs_offered"`
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

type UpdateEventRequest struct {
	Name               string   `json:"name"`
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
	StartDate          string   `json:"start_date"`
	EndDate            string   `json:"end_date"`
	Location           string   `json:"location"`
	Tags               []string `json:"tags"`
	EnableRegistration bool     `json:"enable_registration"`
	Status             string   `json:"status"`
}

type EventResponse struct {
	ID                 uint     `json:"id"`
	Slug               string   `json:"slug"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
	InstitutionID      uint     `json:"institution_id"`
	Name               string   `json:"name"`
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
	StartDate          *string  `json:"start_date"`
	EndDate            *string  `json:"end_date"`
	Location           string   `json:"location"`
	Tags               []string `json:"tags"`
	EnableRegistration bool     `json:"enable_registration"`
	Status             string   `json:"status"`
	Attendees          int      `json:"attendees"`
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

type UpdateNewsRequest struct {
	Title         string   `json:"title"`
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
	ID            uint     `json:"id"`
	Slug          string   `json:"slug"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	InstitutionID uint     `json:"institution_id"`
	Title         string   `json:"title"`
	ShortDesc     string   `json:"short_desc"`
	Content       string   `json:"content"`
	ImageURL      string   `json:"image_url"`
	NewsType      string   `json:"news_type"`
	PublishedBy   string   `json:"published_by"`
	PublishDate   *string  `json:"publish_date"`
	Tags          []string `json:"tags"`
	AllowComments bool     `json:"allow_comments"`
	Status        string   `json:"status"`
	PublishedAt   *string  `json:"published_at"`
}

type CreateBlogRequest struct {
	Title        string `json:"title" binding:"required"`
	Content      string `json:"content"`
	Excerpt      string `json:"excerpt"`
	Image        string `json:"image"`
	Category     string `json:"category"`
	BlogCategory string `json:"blog_category"`
	ReadTime     string `json:"read_time"`
	Tags         string `json:"tags"`
	Status       string `json:"status"`
}

type UpdateBlogRequest struct {
	Title        string `json:"title"`
	Content      string `json:"content"`
	Excerpt      string `json:"excerpt"`
	Image        string `json:"image"`
	Category     string `json:"category"`
	BlogCategory string `json:"blog_category"`
	ReadTime     string `json:"read_time"`
	Tags         string `json:"tags"`
	Status       string `json:"status"`
}

type BlogResponse struct {
	ID            uint    `json:"id"`
	Slug          string  `json:"slug"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	InstitutionID uint    `json:"institution_id"`
	Title         string  `json:"title"`
	Content       string  `json:"content"`
	Excerpt       string  `json:"excerpt"`
	Image         string  `json:"image"`
	Category      string  `json:"category"`
	BlogCategory  string  `json:"blog_category"`
	ReadTime      string  `json:"read_time"`
	Tags          string  `json:"tags"`
	Status        string  `json:"status"`
	PublishedAt   *string `json:"published_at"`
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

type CreateInquiryRequest struct {
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

// --- Admission Profile Sub-Object Types ---

type OverviewData struct {
	OverviewHeading     string `json:"overviewHeading"`
	OverviewDesc        string `json:"overviewDesc"`
	ApplicationFormLink string `json:"applicationFormLink"`
	Level               string `json:"level"`
}

type WhatsNewData struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	BtnText     string `json:"btnText"`
	BtnLink     string `json:"btnLink"`
}

type ProgramDataItem struct {
	Title           string   `json:"title"`
	Level           string   `json:"level"`
	Subtitle        string   `json:"subtitle"`
	AdmissionStatus string   `json:"admissionStatus"`
	ProgramIcon     string   `json:"programIcon"`
	Description     string   `json:"description"`
	Streams         []string `json:"streams"`
	Careers         []string `json:"careers"`
}

type FacilityDataItem struct {
	Heading      string `json:"heading"`
	FacilityIcon string `json:"facilityIcon"`
	Icon         string `json:"icon"`
	Description  string `json:"description"`
	Desc         string `json:"desc"`
}

type CourseDataItem struct {
	CourseName      string `json:"courseName"`
	Name            string `json:"name"`
	Level           string `json:"level"`
	CurriculumLink  string `json:"curriculumLink"`
	FeesText        string `json:"feesText"`
	Fees            string `json:"fees"`
	Duration        string `json:"duration"`
	Eligibility     string `json:"eligibility"`
	Seats           string `json:"seats"`
	SubDescription  string `json:"sub_description"`
	ApplicationDate string `json:"applicationDate"`
	ApplyLink       string `json:"applyLink"`
}

type DownloadDataItem struct {
	Title       string `json:"title"`
	Name        string `json:"name"`
	Description string `json:"description"`
	File        string `json:"file"`
	Size        string `json:"size"`
}

type AlumniDataItem struct {
	Photo    string `json:"photo"`
	Name     string `json:"name"`
	Job      string `json:"job"`
	Batch    string `json:"batch"`
	Linkedin string `json:"linkedin"`
}

type FaqDataItem struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type ContactPersonItem struct {
	Name        string `json:"name"`
	Designation string `json:"designation"`
	Number      string `json:"number"`
	Email       string `json:"email"`
	WhatsApp    string `json:"whatsapp"`
}

type ScholarshipDataItem struct {
	Name        string `json:"name"`
	Level       string `json:"level"`
	Stream      string `json:"stream"`
	Coverage    string `json:"coverage"`
	Eligibility string `json:"eligibility"`
	Seats       string `json:"seats"`
}

type EligibilityCriterion struct {
	Level       string   `json:"level"`
	Stream      string   `json:"stream"`
	Eligibility []string `json:"eligibility"`
	Documents   []string `json:"documents"`
}

type EligibilityData struct {
	Criteria []EligibilityCriterion `json:"criteria"`
}

func (e *EligibilityData) UnmarshalJSON(data []byte) error {
	var obj struct {
		Criteria []EligibilityCriterion `json:"criteria"`
	}
	if err := json.Unmarshal(data, &obj); err == nil && obj.Criteria != nil {
		e.Criteria = obj.Criteria
		return nil
	}
	var arr []EligibilityCriterion
	if err := json.Unmarshal(data, &arr); err == nil {
		e.Criteria = arr
		return nil
	}
	return json.Unmarshal(data, &obj)
}

type AdmissionProcessItem struct {
	StepNumber  string `json:"stepNumber"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type BrochureData struct {
	URL string `json:"url"`
}

// ProfileData mirrors the full JSONB shape stored in the profile_data column.
type ProfileData struct {
	Videos               interface{}            `json:"videos"`
	OverviewData         interface{}            `json:"overview_data"`
	LeadershipData       interface{}            `json:"leadership_data"`
	CoursesData          []CourseDataItem       `json:"courses_data"`
	ProgramsData         []ProgramDataItem      `json:"programs_data"`
	FacilitiesData       []FacilityDataItem     `json:"facilities_data"`
	AlumniData           []AlumniDataItem       `json:"alumni_data"`
	DownloadsData        []DownloadDataItem     `json:"downloads_data"`
	GalleryData          interface{}            `json:"gallery_data"`
	WhatsNewData         *WhatsNewData          `json:"whats_new_data"`
	EligibilityData      *EligibilityData       `json:"eligibility_data"`
	AdmissionProcessData []AdmissionProcessItem `json:"admission_process_data"`
	ScholarshipsData     []ScholarshipDataItem  `json:"scholarships_data"`
	FaqsData             []FaqDataItem          `json:"faqs_data"`
	ContactPersonsData   []ContactPersonItem    `json:"contact_persons_data"`
	BrochureData         *BrochureData          `json:"brochure_data"`
}

type UpdateProfileRequest struct {
	Status               string                 `json:"status"`
	InstitutionName      string                 `json:"institution_name"`
	RegistrationNumber   string                 `json:"registration_number"`
	Location             string                 `json:"location"`
	Website              string                 `json:"website"`
	Level                string                 `json:"level"`
	ContactEmail         string                 `json:"contact_email"`
	ContactPhone         string                 `json:"contact_phone"`
	Affiliation                string                 `json:"affiliation"`
	UniversityAffiliations     interface{}            `json:"university_affiliations"`
	NonUniversityAffiliation   string                 `json:"non_university_affiliation"`
	MapURL                     string                 `json:"map_url"`
	FacebookURL          string                 `json:"facebook_url"`
	InstagramURL         string                 `json:"instagram_url"`
	TiktokURL            string                 `json:"tiktok_url"`
	YoutubeURL           string                 `json:"youtube_url"`
	LinkedinURL          string                 `json:"linkedin_url"`
	LogoURL              string                 `json:"logo_url"`
	BannerURL            string                 `json:"banner_url"`
	CardImageURL         string                 `json:"card_image_url"`
	About                string                 `json:"about"`
	Vision               string                 `json:"vision"`
	Mission              string                 `json:"mission"`
	Videos               interface{}            `json:"videos"`
	OverviewData         interface{}            `json:"overview_data"`
	LeadershipData       interface{}            `json:"leadership_data"`
	CoursesData          []CourseDataItem       `json:"courses_data"`
	ProgramsData         []ProgramDataItem      `json:"programs_data"`
	FacilitiesData       []FacilityDataItem     `json:"facilities_data"`
	AlumniData           []AlumniDataItem       `json:"alumni_data"`
	DownloadsData        []DownloadDataItem     `json:"downloads_data"`
	GalleryData          interface{}            `json:"gallery_data"`
	WhatsNewData         *WhatsNewData          `json:"whats_new_data"`
	EligibilityData      *EligibilityData       `json:"eligibility_data"`
	AdmissionProcessData []AdmissionProcessItem `json:"admission_process_data"`
	ScholarshipsData     []ScholarshipDataItem  `json:"scholarships_data"`
	FaqsData             []FaqDataItem          `json:"faqs_data"`
	ContactPersonsData   []ContactPersonItem    `json:"contact_persons_data"`
	BrochureData         *BrochureData          `json:"brochure_data"`
}

type ProfileResponse struct {
	ID                   uint                   `json:"id"`
	InstitutionName      string                 `json:"institution_name"`
	Email                string                 `json:"email"`
	SubscriptionType     string                 `json:"subscription_type"`
	RegistrationNumber   string                 `json:"registration_number"`
	Role                 string                 `json:"role"`
	ProfileStatus        string                 `json:"profile_status"`
	Location             string                 `json:"location,omitempty"`
	Website              string                 `json:"website,omitempty"`
	ContactEmail         string                 `json:"contact_email,omitempty"`
	ContactPhone         string                 `json:"contact_phone,omitempty"`
	MapURL               string                 `json:"map_url,omitempty"`
	FacebookURL          string                 `json:"facebook_url,omitempty"`
	InstagramURL         string                 `json:"instagram_url,omitempty"`
	TiktokURL            string                 `json:"tiktok_url,omitempty"`
	YoutubeURL           string                 `json:"youtube_url,omitempty"`
	LinkedinURL          string                 `json:"linkedin_url,omitempty"`
	LogoURL              string                 `json:"logo_url,omitempty"`
	BannerURL            string                 `json:"banner_url,omitempty"`
	CardImageURL         string                 `json:"card_image_url,omitempty"`
	About                string                 `json:"about,omitempty"`
	Vision               string                 `json:"vision,omitempty"`
	Mission              string                 `json:"mission,omitempty"`
	Affiliation          string                 `json:"affiliation,omitempty"`
	Level                string                 `json:"level,omitempty"`
	Videos               interface{}            `json:"videos,omitempty"`
	OverviewData         interface{}            `json:"overview_data,omitempty"`
	LeadershipData       interface{}            `json:"leadership_data,omitempty"`
	CoursesData          []CourseDataItem       `json:"courses_data,omitempty"`
	ProgramsData         []ProgramDataItem      `json:"programs_data,omitempty"`
	FacilitiesData       []FacilityDataItem     `json:"facilities_data,omitempty"`
	AlumniData           interface{}            `json:"alumni_data,omitempty"`
	DownloadsData        []DownloadDataItem     `json:"downloads_data,omitempty"`
	GalleryData          interface{}            `json:"gallery_data,omitempty"`
	WhatsNewData         *WhatsNewData          `json:"whats_new_data,omitempty"`
	EligibilityData      *EligibilityData       `json:"eligibility_data,omitempty"`
	AdmissionProcessData []AdmissionProcessItem `json:"admission_process_data,omitempty"`
	ScholarshipsData     []ScholarshipDataItem  `json:"scholarships_data,omitempty"`
	FaqsData             []FaqDataItem          `json:"faqs_data,omitempty"`
	ContactPersonsData   []ContactPersonItem    `json:"contact_persons_data,omitempty"`
	BrochureData         *BrochureData          `json:"brochure_data,omitempty"`
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
	ShortDesc       string   `json:"short_desc"`
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
	Status          string   `json:"status"`
}

type UpdateScholarshipRequest struct {
	Title           string   `json:"title"`
	ShortDesc       string   `json:"short_desc"`
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
	Status          string   `json:"status"`
}

type ScholarshipResponse struct {
	ID              uint     `json:"id"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	InstitutionID   uint     `json:"institution_id"`
	Title           string   `json:"title"`
	ShortDesc       string   `json:"short_desc"`
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
	Status          string   `json:"status"`
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
	Verified        bool   `json:"verified"`
	Claimed         bool   `json:"claimed"`
	Affiliation     string `json:"affiliation,omitempty"`
	UniversityID    *uint  `json:"university_id"`
	IsSponsored     bool   `json:"is_sponsored"`
	LogoURL         string `json:"logo_url,omitempty"`
	BannerURL       string `json:"banner_url,omitempty"`
	CardImageURL    string `json:"card_image_url,omitempty"`
	About           string `json:"about,omitempty"`
	District        string `json:"district,omitempty"`
	WebsiteURL      string `json:"website_url,omitempty"`
	Status          string `json:"status"`
	Featured        bool   `json:"featured"`
	CollegeID       uint   `json:"college_id"`
	Type            string `json:"type"`
}

type PublicInstitutionDetailResponse struct {
	ID                      uint                  `json:"id"`
	InstitutionName         string                `json:"institution_name"`
	Verified                bool                  `json:"verified"`
	Claimed                 bool                  `json:"claimed"`
	Featured                bool                  `json:"featured"`
	ProfileStatus           string                `json:"profile_status"`
	LogoURL                 string                `json:"logo_url,omitempty"`
	BannerURL               string                `json:"banner_url,omitempty"`
	CardImageURL            string                `json:"card_image_url,omitempty"`
	About                   string                `json:"about,omitempty"`
	Vision                  string                `json:"vision,omitempty"`
	Mission                 string                `json:"mission,omitempty"`
	District                string                `json:"district,omitempty"`
	WebsiteURL              string                `json:"website_url,omitempty"`
	Level                   string                `json:"level,omitempty"`
	Videos                  interface{}           `json:"videos,omitempty"`
	OverviewData            interface{}           `json:"overview_data,omitempty"`
	LeadershipData          interface{}           `json:"leadership_data,omitempty"`
	CoursesData             []CourseDataItem      `json:"courses_data,omitempty"`
	ProgramsData            []ProgramDataItem     `json:"programs_data,omitempty"`
	FacilitiesData          []FacilityDataItem    `json:"facilities_data,omitempty"`
	AlumniData              interface{}           `json:"alumni_data,omitempty"`
	GalleryData             interface{}           `json:"gallery_data,omitempty"`
	DownloadsData           []DownloadDataItem    `json:"downloads_data,omitempty"`
	FaqsData                []FaqDataItem         `json:"faqs_data,omitempty"`
	InstitutionPrograms     []ProgramResponse     `json:"institution_programs,omitempty"`
	InstitutionEvents       []EventResponse       `json:"institution_events,omitempty"`
	InstitutionNews         []NewsResponse        `json:"institution_news,omitempty"`
	InstitutionScholarships []ScholarshipResponse `json:"institution_scholarships,omitempty"`
	AdmissionPageID         uint                  `json:"admission_page_id,omitempty"`
	AdmissionPageData       json.RawMessage       `json:"admission_page_data,omitempty"`
	ContactEmail            string                `json:"contact_email,omitempty"`
	ContactPhone            string                `json:"contact_phone,omitempty"`
	MapURL                  string                `json:"map_url,omitempty"`
	FacebookURL             string                `json:"facebook_url,omitempty"`
	InstagramURL            string                `json:"instagram_url,omitempty"`
	TiktokURL               string                `json:"tiktok_url,omitempty"`
	YoutubeURL              string                `json:"youtube_url,omitempty"`
	LinkedinURL             string                `json:"linkedin_url,omitempty"`
	BrochureData            *BrochureData         `json:"brochure_data,omitempty"`
	Type                    string                `json:"type"`
}

// --- Admission Page DTOs ---

type CreateAdmissionPageRequest struct {
	InstitutionName     string          `json:"institution_name"`
	InstitutionLocation string          `json:"institution_location"`
	InstitutionLink     string          `json:"institution_link"`
	Data                json.RawMessage `json:"data"`
	Status              string          `json:"status"` // "draft" | "published"
}

type UpdateAdmissionPageRequest struct {
	InstitutionName     string          `json:"institution_name"`
	InstitutionLocation string          `json:"institution_location"`
	InstitutionLink     string          `json:"institution_link"`
	Data                json.RawMessage `json:"data"`
	Status              *string         `json:"status"`
}

type AdmissionPageResponse struct {
	ID            uint            `json:"id"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	InstitutionID uint            `json:"institution_id"`
	Title         string          `json:"title"`
	Status        string          `json:"status"`
	PublishedAt   *string         `json:"published_at,omitempty"`
	Data          json.RawMessage `json:"data,omitempty"`
}

type AdmissionPageListItem struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	Program     string  `json:"program"`
	Level       string  `json:"level"`
	Status      string  `json:"status"`
	PublishedAt *string `json:"published_at,omitempty"`
	LastEdited  string  `json:"last_edited"`
	Applicants  int     `json:"applicants"`
}

type PaginatedResponse struct {
	Meta PaginationMeta `json:"meta"`
}

// --- Published Admission Institution DTOs (for public listing) ---

type FeaturedProgramItem struct {
	Title           string `json:"title"`
	AdmissionStatus string `json:"admissionStatus"`
}

type PublishedAdmissionInstitutionItem struct {
	ID               uint                  `json:"id"`
	AdmissionPageID  uint                  `json:"admission_page_id"`
	Name             string                `json:"name"`
	ImageURL         string                `json:"image_url,omitempty"`
	Location         string                `json:"location"`
	Type             string                `json:"type"`
	Rating           float64               `json:"rating"`
	Website          string                `json:"website,omitempty"`
	Affiliation      string                `json:"affiliation,omitempty"`
	Verified         bool                  `json:"verified"`
	FeaturedPrograms []FeaturedProgramItem `json:"featured_programs"`
	Programs         int                   `json:"programs"`
	ContactEmail     string                `json:"contact_email,omitempty"`
	ContactPhone     string                `json:"contact_phone,omitempty"`
	HeroBanner       string                `json:"hero_banner,omitempty"`
}

type PublishedAdmissionInstitutionListResponse struct {
	Colleges   []PublishedAdmissionInstitutionItem `json:"colleges"`
	Pagination PaginationMeta                      `json:"pagination"`
}

type PaginationMeta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	PageSize   int   `json:"pageSize"`
	TotalPages int64 `json:"totalPages"`
}

// --- Published Admission Institution Detail DTOs ---

type PublishedAdmissionInstitutionDetailResponse struct {
	Institution *PublishedAdmissionInstitutionItem `json:"institution"`
	Data        json.RawMessage                    `json:"data"`
	CreatedAt   string                             `json:"created_at"`
	UpdatedAt   string                             `json:"updated_at"`
	PublishedAt *string                            `json:"published_at,omitempty"`
}

type PublicInstitutionFilterCountsResponse struct {
	Total           int64            `json:"total"`
	TypeCounts      map[string]int64 `json:"type_counts"`
	TypeCountsByID  map[string]int64 `json:"type_counts_by_id"`
	FacetCountsByID map[string]int64 `json:"facet_counts_by_id"`
	Featured        int64            `json:"featured"`
	Verified        int64            `json:"verified"`
	Popular         int64            `json:"popular"`
}

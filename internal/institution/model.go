package institution

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"studsphere/backend/internal/shared/slug"
)

type InstitutionProgram struct {
	ID                  uint           `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID       uint           `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	InstitutionName     string         `gorm:"default:''" json:"institution_name"`
	InstitutionLocation string         `gorm:"default:''" json:"institution_location"`
	InstitutionLink     string         `gorm:"default:''" json:"institution_link"`
	Name                string         `gorm:"not null" json:"name"`
	Description         string         `gorm:"type:text" json:"description"`
	Duration            string         `json:"duration"`
	Fee                 string         `json:"fee"`
	Eligibility         string         `gorm:"type:text" json:"eligibility"`
	Capacity            int            `json:"capacity"`
	BannerURL           string         `json:"banner_url"`
	Data                *string        `gorm:"type:jsonb;default:'{}'" json:"data"`
	Status              string         `gorm:"default:'active'" json:"status"`
	GlobalCourseID      *uint          `gorm:"default:null" json:"globalCourseId"`
	Overrides           *string        `gorm:"type:jsonb;default:'{}'" json:"overrides"`
	NullifiedFields     *string        `gorm:"type:jsonb;default:'[]'" json:"nullifiedFields"`
}

type InstitutionMedia struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	InstitutionID uint      `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	URL           string    `json:"url"`
	Type          string    `json:"type"`
	Title         string    `json:"title"`
}

type InstitutionCounsellingSession struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	Title         string         `json:"title"`
	Description   string         `gorm:"type:text" json:"description"`
	ScheduledAt   time.Time      `json:"scheduled_at"`
	Duration      int            `json:"duration"`
	MaxSeats      int            `json:"max_seats"`
	BookedSeats   int            `gorm:"default:0" json:"booked_seats"`
	Status        string         `gorm:"default:'scheduled'" json:"status"`
}

type InstitutionCounsellingBooking struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	SessionID        uint           `gorm:"index" json:"session_id"`
	UserID           uint           `gorm:"index" json:"user_id"`
	Status           string         `gorm:"default:'pending'" json:"status"`
	Notes            string         `gorm:"type:text" json:"notes"`
	StudentName      string         `gorm:"default:''" json:"student_name"`
	StudentPhone     string         `gorm:"default:''" json:"student_phone"`
	StudentEmail     string         `gorm:"default:''" json:"student_email"`
	ProgramLevel     string         `gorm:"default:''" json:"program_level"`
	InterestedCourse string         `gorm:"default:''" json:"interested_course"`
	SessionMode      string         `gorm:"type:varchar(20);default:'in_person'" json:"session_mode"`

	Session InstitutionCounsellingSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

type InstitutionEntrance struct {
	ID                     uint           `gorm:"primarykey" json:"id"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	DeletedAt              gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID          uint           `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	InstitutionName        string         `gorm:"default:''" json:"institution_name"`
	InstitutionLocation    string         `gorm:"default:''" json:"institution_location"`
	InstitutionLink        string         `gorm:"default:''" json:"institution_link"`
	InstitutionAffiliation string         `gorm:"default:''" json:"institution_affiliation"`
	Title                  string         `gorm:"not null" json:"title"`
	Description            string         `gorm:"type:text" json:"description"`
	Program                string         `json:"program"`
	Date                   time.Time      `json:"date"`
	StartTime              string         `json:"start_time"`
	EndTime                string         `json:"end_time"`
	Duration               int            `json:"duration"`
	TotalMarks             int            `json:"total_marks"`
	PassingMarks           int            `json:"passing_marks"`
	TotalSeats             int            `json:"total_seats"`
	FilledSeats            int            `gorm:"default:0" json:"filled_seats"`
	Instructions           string         `gorm:"type:text" json:"instructions"`
	HeroBanner             string         `json:"hero_banner"`
	Questions              *string        `gorm:"type:jsonb" json:"questions"`
	Status                 string         `gorm:"default:'upcoming'" json:"status"`
	ApplicationFee         string         `json:"application_fee"`
	OverviewDetails        []byte         `gorm:"type:jsonb;default:'[]'" json:"overview_details"`
	ExamDateSchedules      []byte         `gorm:"type:jsonb;default:'[]'" json:"exam_date_schedules"`
	EligibilityList        []byte         `gorm:"type:jsonb;default:'[]'" json:"eligibility_list"`
	ApplicationSteps       []byte         `gorm:"type:jsonb;default:'[]'" json:"application_steps"`
	ExamPattern            []byte         `gorm:"type:jsonb;default:'[]'" json:"exam_pattern"`
	SubjectMarks           []byte         `gorm:"type:jsonb;default:'[]'" json:"subject_marks"`
	ModelSets              []byte         `gorm:"type:jsonb;default:'[]'" json:"model_sets"`
	UpcomingDates          []byte         `gorm:"type:jsonb;default:'[]'" json:"upcoming_dates"`
	ContactPersons         []byte         `gorm:"type:jsonb;default:'[]'" json:"contact_persons"`
	Faqs                   []byte         `gorm:"type:jsonb;default:'[]'" json:"faqs"`
	Email                  string         `json:"email"`
	ContactNumber          string         `json:"contact_number"`
	SocialLinks            []byte         `gorm:"type:jsonb;default:'[]'" json:"social_links"`
	ApplicationLink        string         `json:"application_link"`
	NoticeFile             string         `json:"notice_file"`
	EmbeddedMap            string         `gorm:"default:''" json:"embedded_map"`
	RequiredDocuments      []byte         `gorm:"type:jsonb;default:'[]'" json:"required_documents"`
	ExaminationSchedule    []byte         `gorm:"type:jsonb;default:'[]'" json:"examination_schedule"`
	ProgramsOffered        []byte         `gorm:"type:jsonb;default:'[]'" json:"programs_offered"`
}

type InstitutionEntranceApplicant struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	EntranceID uint           `gorm:"index" json:"entrance_id"`
	UserID     uint           `gorm:"index" json:"user_id"`
	Status     string         `gorm:"default:'registered'" json:"status"`
	Score      float64        `json:"score"`
	Rank       int            `json:"rank"`
}

type InstitutionEvent struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID      uint           `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	Slug               string         `gorm:"uniqueIndex" json:"slug"`
	Name               string         `gorm:"not null" json:"name"`
	ShortDesc          string         `gorm:"type:text" json:"short_desc"`
	Description        string         `gorm:"type:text" json:"description"`
	ImageURL           string         `json:"image_url"`
	EventType          string         `json:"event_type"`
	Category           string         `json:"category"`
	MaxParticipants    int            `json:"max_participants"`
	OnlineLink         string         `json:"online_link"`
	OrganizedBy        string         `json:"organized_by"`
	ContactPerson      string         `json:"contact_person"`
	ContactEmail       string         `json:"contact_email"`
	StartDate          *time.Time     `json:"start_date"`
	EndDate            *time.Time     `json:"end_date"`
	Location           string         `json:"location"`
	Tags               *string        `gorm:"type:jsonb;default:'[]'" json:"tags"`
	EnableRegistration bool           `gorm:"default:false" json:"enable_registration"`
	Status             string         `gorm:"default:'draft'" json:"status"`
}

func (ie *InstitutionEvent) BeforeCreate(tx *gorm.DB) error {
	if ie.Slug == "" {
		ie.Slug = slug.GenerateUnique("inst-"+ie.Name, func(s string) bool {
			var count int64
			tx.Model(&InstitutionEvent{}).Where("slug = ?", s).Count(&count)
			return count > 0
		})
	}
	return nil
}

type InstitutionNews struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	Slug          string         `gorm:"uniqueIndex" json:"slug"`
	Title         string         `gorm:"not null" json:"title"`
	ShortDesc     string         `gorm:"type:text" json:"short_desc"`
	Content       string         `gorm:"type:text" json:"content"`
	ImageURL      string         `json:"image_url"`
	NewsType      string         `json:"news_type"`
	PublishedBy   string         `json:"published_by"`
	PublishDate   *string        `json:"publish_date"`
	Tags          *string        `gorm:"type:jsonb;default:'[]'" json:"tags"`
	AllowComments bool           `gorm:"default:false" json:"allow_comments"`
	Status        string         `gorm:"default:'draft'" json:"status"`
	PublishedAt   *time.Time     `json:"published_at"`
}

func (in *InstitutionNews) BeforeCreate(tx *gorm.DB) error {
	if in.Slug == "" {
		in.Slug = slug.GenerateUnique("inst-"+in.Title, func(s string) bool {
			var count int64
			tx.Model(&InstitutionNews{}).Where("slug = ?", s).Count(&count)
			return count > 0
		})
	}
	return nil
}

type InstitutionBlog struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	Slug          string         `gorm:"uniqueIndex" json:"slug"`
	Title         string         `gorm:"not null" json:"title"`
	Content       string         `gorm:"type:text" json:"content"`
	Excerpt       string         `json:"excerpt"`
	Image         string         `json:"image"`
	Category      string         `json:"category"`
	BlogCategory  string         `json:"blog_category"`
	ReadTime      string         `json:"read_time"`
	Tags          string         `json:"tags"`
	Status        string         `gorm:"default:'draft'" json:"status"`
	PublishedAt   *time.Time     `json:"published_at"`
}

func (ib *InstitutionBlog) BeforeCreate(tx *gorm.DB) error {
	if ib.Slug == "" {
		ib.Slug = slug.GenerateUnique("inst-"+ib.Title, func(s string) bool {
			var count int64
			tx.Model(&InstitutionBlog{}).Where("slug = ?", s).Count(&count)
			return count > 0
		})
	}
	return nil
}

type InstitutionQMS struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	Title         string         `gorm:"not null" json:"title"`
	Description   string         `gorm:"type:text" json:"description"`
	Category      string         `json:"category"`
	Status        string         `gorm:"default:'pending'" json:"status"`
	Score         float64        `json:"score"`
	Documents     []byte         `gorm:"type:jsonb" json:"documents"`
}

type InstitutionMessage struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID uint           `gorm:"index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	UserID        uint           `gorm:"index" json:"user_id"`
	Subject       string         `json:"subject"`
	Content       string         `gorm:"type:text" json:"content"`
	Read          bool           `gorm:"default:false" json:"read"`
	Direction     string         `json:"direction"`
}

type InstitutionSettings struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	InstitutionID uint      `gorm:"uniqueIndex;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	EmailNotifs   bool      `gorm:"default:true" json:"email_notifications"`
	Timezone      string    `gorm:"default:'UTC'" json:"timezone"`
	Language      string    `gorm:"default:'en'" json:"language"`
	PublicProfile bool      `gorm:"default:true" json:"public_profile"`
}

type InstitutionUser struct {
	ID                 uint           `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionName    string         `gorm:"not null" json:"institution_name" binding:"required"`
	RegistrationNumber string         `gorm:"uniqueIndex;not null" json:"registration_number" binding:"required"`
	Email              string         `gorm:"uniqueIndex;not null" json:"email" binding:"required,email"`
	GoogleID           *string        `gorm:"uniqueIndex;default:null" json:"google_id"`
	Password           *string        `json:"-"`
	Role               string         `gorm:"default:'institution'" json:"role"`
	Status             string         `gorm:"default:'pending'" json:"status"`
	District           string         `gorm:"default:''" json:"district"`
	WebsiteURL         string         `gorm:"default:''" json:"website_url"`
	LogoURL            string         `gorm:"default:''" json:"logo_url"`
	BannerURL          string         `gorm:"default:''" json:"banner_url"`
	About              string         `gorm:"type:text" json:"about"`
	Vision             string         `gorm:"type:text" json:"vision"`
	Mission            string         `gorm:"type:text" json:"mission"`
	Claimed            bool           `gorm:"default:false" json:"claimed"`
	Verified           bool           `gorm:"default:false" json:"verified"`
	Affiliation        string         `gorm:"default:''" json:"affiliation"`
	CollegeID          uint           `gorm:"default:0" json:"college_id"`
	ContactEmail       string         `gorm:"default:''" json:"contact_email"`
	ContactPhone       string         `gorm:"default:''" json:"contact_phone"`
	MapURL             string         `gorm:"default:''" json:"map_url"`
	FacebookURL        string         `gorm:"default:''" json:"facebook_url"`
	InstagramURL       string         `gorm:"default:''" json:"instagram_url"`
	TiktokURL          string         `gorm:"default:''" json:"tiktok_url"`
	YoutubeURL         string         `gorm:"default:''" json:"youtube_url"`
	LinkedinURL        string         `gorm:"default:''" json:"linkedin_url"`
	OrganizationType   string         `gorm:"default:''" json:"organization_type"`
	ProfileData        *string        `gorm:"type:jsonb;default:'{}'" json:"profile_data"`
	Featured           bool           `gorm:"default:false" json:"featured"`
}

func (u *InstitutionUser) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hashedStr := string(hashedPassword)
	u.Password = &hashedStr
	return nil
}

func (u *InstitutionUser) CheckPassword(password string) error {
	if u.Password == nil {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return bcrypt.CompareHashAndPassword([]byte(*u.Password), []byte(password))
}

type College struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	UniversityID uint   `json:"university_id"`
}

type Scholarship struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID   uint           `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	Title           string         `gorm:"not null" json:"title"`
	ShortDesc       string         `gorm:"type:text" json:"short_desc"`
	Provider        string         `json:"provider"`
	Location        string         `json:"location"`
	Value           string         `json:"value"`
	Deadline        time.Time      `json:"deadline"`
	DegreeLevel     string         `json:"degree_level"`
	FundingType     string         `json:"funding_type"`
	ScholarshipType string         `json:"scholarship_type"`
	Description     string         `gorm:"type:text" json:"description"`
	ImageURL        string         `json:"image_url"`
	FieldOfStudy    []byte         `gorm:"type:jsonb" json:"field_of_study"`
	Status          string         `gorm:"default:'draft'" json:"status"`
}

type ScholarshipApplication struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	ScholarshipID uint           `gorm:"index" json:"scholarship_id"`
	UserID        uint           `gorm:"index" json:"user_id"`
	Status        string         `gorm:"default:'pending'" json:"status"`
	CoverLetter   string         `gorm:"type:text" json:"cover_letter"`
	Documents     []byte         `gorm:"type:jsonb" json:"documents"`

	Scholarship Scholarship `gorm:"foreignKey:ScholarshipID" json:"scholarship,omitempty"`
	User        User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

type Admission struct {
	ID                uint           `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
	UserID            *uint          `gorm:"index" json:"user_id,omitempty"`
	CollegeID         uint           `gorm:"index;not null" json:"college_id"`
	ProgramName       string         `gorm:"not null" json:"program_name"`
	ProgramLevel      string         `gorm:"not null" json:"program_level"`
	StudentName       string         `gorm:"not null" json:"student_name"`
	StudentEmail      string         `gorm:"not null" json:"student_email"`
	StudentPhone      string         `gorm:"not null" json:"student_phone"`
	DateOfBirth       *time.Time     `json:"date_of_birth,omitempty"`
	Gender            string         `json:"gender,omitempty"`
	Address           string         `json:"address,omitempty"`
	City              string         `json:"city,omitempty"`
	LastQualification string         `json:"last_qualification,omitempty"`
	Institution       string         `json:"institution,omitempty"`
	GPA               string         `json:"gpa,omitempty"`
	EntranceScore     string         `json:"entrance_score,omitempty"`
	Statement         string         `gorm:"type:text" json:"statement,omitempty"`
	Status            string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	Notes             string         `gorm:"type:text" json:"notes,omitempty"`
	ReviewedBy        *uint          `json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time     `json:"reviewed_at,omitempty"`

	College College `gorm:"foreignKey:CollegeID" json:"college,omitempty"`
	User    *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

type AdmissionPage struct {
	ID                  uint           `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	InstitutionID       uint           `gorm:"index;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"institution_id"`
	InstitutionName     string         `gorm:"default:''" json:"institution_name"`
	InstitutionLocation string         `gorm:"default:''" json:"institution_location"`
	InstitutionLink     string         `gorm:"default:''" json:"institution_link"`
	Title               string         `json:"title"`
	Status              string         `gorm:"default:'draft'" json:"status"`
	PublishedAt         *time.Time     `json:"published_at,omitempty"`
	Data                *string        `gorm:"type:jsonb;default:'{}'" json:"data"`
}

type User struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

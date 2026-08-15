package education

import (
	"time"

	"gorm.io/gorm"

	"studsphere/backend/internal/shared/slug"
)

type Exam struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	Slug         string         `gorm:"uniqueIndex" json:"slug"`
	Title        string         `gorm:"not null" json:"title" binding:"required"`
	Board        string         `json:"board"`
	Badges       []byte         `gorm:"type:jsonb" json:"badges"`
	Level        string         `json:"level"`
	Type         string         `json:"type"`
	ExamDate     string         `json:"examDate"`
	ExamDateAD   time.Time      `json:"examDateAD"`
	FormDeadline string         `json:"formDeadline"`
	Fee          string         `json:"fee"`
	Highlights   []byte         `gorm:"type:jsonb" json:"highlights"`
	Description  string         `gorm:"type:text" json:"description"`
	Status       string         `json:"status"`
	ImageUrl     string         `json:"imageUrl"`
	University   string         `json:"university"`
	Faculty      string         `json:"faculty"`
	NepaliDate   string         `json:"nepaliDate"`
	Overview     string         `gorm:"type:text" json:"overview"`
	Weightage    []byte         `gorm:"type:jsonb" json:"weightage"`
	Timeline     []byte         `gorm:"type:jsonb" json:"timeline"`
	Notices      []byte         `gorm:"type:jsonb" json:"notices"`
	Faqs         []byte         `gorm:"type:jsonb" json:"faqs"`
}

func (Exam) TableName() string {
	return "exams"
}

type Course struct {
	ID                       uint           `gorm:"primarykey" json:"id"`
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at"`
	DeletedAt                gorm.DeletedAt `gorm:"index" json:"-"`
	Title                    string         `gorm:"not null" json:"title" binding:"required"`
	ShortTitle               string         `json:"shortTitle"`
	CollegesCount            int            `json:"collegesCount"`
	AffiliationID            *uint          `gorm:"index" json:"affiliationId"`
	NonUniversityAffiliation string         `json:"nonUniversityAffiliation"`
	Badges                   []byte         `gorm:"type:jsonb" json:"badges"`
	Level                    string         `json:"level"`
	Field                    string         `json:"field"`
	FieldOfStudy             string         `json:"fieldOfStudy"`
	Duration                 string         `json:"duration"`
	EstFee                   string         `json:"estFee"`
	Highlights               []byte         `gorm:"type:jsonb" json:"highlights"`
	CareerPath               string         `json:"careerPath"`
	Description              string         `gorm:"type:text" json:"description"`
	Location                 string         `json:"location"`
	GovtFee                  string         `json:"govtFee"`
	PrivateFee               string         `json:"privateFee"`
	FeeStructure             string         `gorm:"type:text" json:"feeStructure"`
	EligibilityText          string         `gorm:"type:text" json:"eligibilityText"`
	Mode                     string         `json:"mode"`
	DegreeLabel              string         `json:"degreeLabel"`
	About                    []byte         `gorm:"type:jsonb" json:"about"`
	Curriculum               []byte         `gorm:"type:jsonb" json:"curriculum"`
	Downloads                []byte         `gorm:"type:jsonb" json:"downloads"`
	Admissions               []byte         `gorm:"type:jsonb" json:"admissions"`
	Careers                  []byte         `gorm:"type:jsonb" json:"careers"`
	IsGlobal                 bool           `gorm:"default:false" json:"isGlobal"`
	Status                   string         `gorm:"default:'draft'" json:"status"`
	CreatedBy                uint           `gorm:"default:0" json:"createdBy"`
	SourceProgramID          *uint          `gorm:"default:null" json:"sourceProgramId"`

	// New global fields
	WhoShouldChoose  []byte `gorm:"type:jsonb;default:'[]'" json:"whoShouldChoose"`
	Features         []byte `gorm:"type:jsonb;default:'[]'" json:"features"`
	EligibilityRows  []byte `gorm:"type:jsonb;default:'[]'" json:"eligibilityRows"`
	AdmissionSteps   []byte `gorm:"type:jsonb;default:'[]'" json:"admissionSteps"`
	SubjectGroups    []byte `gorm:"type:jsonb;default:'[]'" json:"subjectGroups"`
	FeeItems         []byte `gorm:"type:jsonb;default:'[]'" json:"feeItems"`
	ScholarshipDesc  string `json:"scholarshipDesc"`
	ScholarshipNotes string `json:"scholarshipNotes"`
	Scholarships     []byte `gorm:"type:jsonb;default:'[]'" json:"scholarships"`
	FullTimeCourses  []byte `gorm:"type:jsonb;default:'[]'" json:"fullTimeCourses"`
	FAQs             []byte `gorm:"type:jsonb;default:'[]'" json:"faqs"`
	BannerURL        string `json:"bannerUrl"`
}

func (Course) TableName() string {
	return "courses"
}

type Affiliation struct {
	ID   uint   `gorm:"primarykey" json:"id"`
	Name string `gorm:"not null" json:"name"`
}

func (Affiliation) TableName() string {
	return "affiliations"
}

type PersonaItem struct {
	Icon      string `json:"icon"`
	Title     string `json:"title"`
	ShortDesc string `json:"shortDesc"`
}

type FeatureItem struct {
	Title     string `json:"title"`
	ShortDesc string `json:"shortDesc"`
}

type EligibilityRow struct {
	Level       string   `json:"level"`
	Stream      string   `json:"stream"`
	Eligibility []string `json:"eligibility"`
	Documents   []string `json:"documents"`
}

type AdmissionStep struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type SubjectGroup struct {
	GroupName   string   `json:"groupName"`
	Description string   `json:"description"`
	Subjects    []string `json:"subjects"`
	Careers     []string `json:"careers"`
}

type FeeItem struct {
	Particular string `json:"particular"`
	Amount     string `json:"amount"`
	Frequency  string `json:"frequency"`
	Notes      string `json:"notes"`
}

type ScholarshipItem struct {
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Coverage    string `json:"coverage"`
	Requirement string `json:"requirement"`
}

type FullTimeCourse struct {
	Course    string `json:"course"`
	TotalFees string `json:"totalFees"`
	Seats     string `json:"seats"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type FaqItem struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type CareerItem struct {
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
}

type DownloadItem struct {
	Title string `json:"title"`
	Size  string `json:"size"`
	File  string `json:"file"`
}

type CourseOverrides struct {
	Description *string      `json:"description,omitempty"`
	BannerURL   *string      `json:"bannerUrl,omitempty"`
	Careers     []CareerItem `json:"careers,omitempty"`
	FAQs        []FaqItem    `json:"faqs,omitempty"`
}

type ResolvedCourse struct {
	ID                       uint             `json:"id"`
	Title                    string           `json:"title"`
	Duration                 string           `json:"duration"`
	Level                    string           `json:"level"`
	AffiliationID            *uint            `json:"affiliationId"`
	AffiliationName          string           `json:"affiliationName"`
	NonUniversityAffiliation string           `json:"nonUniversityAffiliation"`
	Description              string           `json:"description"`
	BannerURL                string           `json:"bannerUrl"`
	Careers                  []CareerItem     `json:"careers"`
	FAQs                     []FaqItem        `json:"faqs"`
	EligibilityRows          []EligibilityRow `json:"eligibilityRows"`
	AdmissionSteps           []AdmissionStep  `json:"admissionSteps"`
	SubjectGroups            []SubjectGroup   `json:"subjectGroups"`
	ScholarshipDesc          string           `json:"scholarshipDesc"`
	ScholarshipNotes         string           `json:"scholarshipNotes"`
	Scholarships             []ScholarshipItem `json:"scholarships"`
	InstitutionID            uint             `json:"institutionId"`
	Fee                      string           `json:"fee"`
	Eligibility              string           `json:"eligibility"`
	Capacity                 int              `json:"capacity"`
	WhoShouldChoose          []PersonaItem    `json:"whoShouldChoose"`
	Features                 []FeatureItem    `json:"features"`
	FullTimeCourses          []FullTimeCourse `json:"fullTimeCourses"`
	FeeItems                 []FeeItem        `json:"feeItems"`
	Status                   string           `json:"status"`
}

type CollegeUniversityCourse struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	CollegeID    uint           `gorm:"not null;index;uniqueIndex:idx_college_uni_course" json:"college_id"`
	UniversityID uint           `gorm:"not null;index;uniqueIndex:idx_college_uni_course" json:"university_id"`
	CourseID     uint           `gorm:"not null;index;uniqueIndex:idx_college_uni_course" json:"course_id"`
	Status       string         `json:"status"`
}

func (CollegeUniversityCourse) TableName() string {
	return "college_university_courses"
}

type News struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	Slug         string         `gorm:"uniqueIndex" json:"slug"`
	UniversityID uint           `gorm:"index" json:"university_id"`
	Category     string         `json:"category"`
	Title        string         `gorm:"not null" json:"title" binding:"required"`
	Excerpt      string         `gorm:"type:text" json:"excerpt"`
	Content      string         `gorm:"type:text" json:"content"`
	Image        string         `json:"image"`
	Author       string         `json:"author"`
	Date         string         `json:"date"`
	ReadTime     string         `json:"readTime"`
	Source       string         `json:"source"`
	Tags         []byte         `gorm:"type:jsonb" json:"tags"`
}

func (n *News) BeforeCreate(tx *gorm.DB) error {
	if n.Slug == "" {
		n.Slug = slug.GenerateUnique("edu-"+n.Title, func(s string) bool {
			var count int64
			tx.Model(&News{}).Where("slug = ?", s).Count(&count)
			return count > 0
		})
	}
	return nil
}

func (News) TableName() string {
	return "news"
}

type Event struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	Slug            string         `gorm:"uniqueIndex" json:"slug"`
	UniversityID    uint           `gorm:"index" json:"university_id"`
	Title           string         `gorm:"not null" json:"title" binding:"required"`
	Excerpt         string         `gorm:"type:text" json:"excerpt"`
	Description     string         `gorm:"type:text" json:"description"`
	Category        string         `json:"category"`
	Organizer       string         `json:"organizer"`
	Location        string         `json:"location"`
	Date            string         `json:"date"`
	Time            string         `json:"time"`
	RegistrationFee string         `json:"registration_fee"`
	Image           string         `json:"image"`
	Interested      int            `json:"interested"`
	Trending        bool           `json:"trending"`
	Featured        bool           `gorm:"default:false;index" json:"featured"`
}

func (e *Event) BeforeCreate(tx *gorm.DB) error {
	if e.Slug == "" {
		e.Slug = slug.GenerateUnique("edu-"+e.Title, func(s string) bool {
			var count int64
			tx.Model(&Event{}).Where("slug = ?", s).Count(&count)
			return count > 0
		})
	}
	return nil
}

func (Event) TableName() string {
	return "events"
}

type University struct {
	ID   uint   `gorm:"primarykey" json:"id"`
	Name string `json:"name"`
}

func (University) TableName() string {
	return "universities"
}

type College struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	UniversityID uint           `gorm:"index" json:"university_id"`
	University   University     `gorm:"foreignKey:UniversityID" json:"university,omitempty"`
	Name         string         `gorm:"not null;index" json:"name"`
	Location     string         `gorm:"not null" json:"location"`
	Rating       float64        `json:"rating"`
	Verified     bool           `gorm:"default:false" json:"verified"`
	Email        string         `json:"email,omitempty"`
	Phone        string         `json:"phone,omitempty"`
}

func (College) TableName() string {
	return "colleges"
}

type Blog struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Title     string         `gorm:"not null" json:"title" binding:"required"`
	Slug      string         `gorm:"uniqueIndex" json:"slug"`
	Excerpt   string         `gorm:"type:text" json:"excerpt"`
	Content   string         `gorm:"type:text" json:"content"`
	Image     string         `json:"image"`
	Author    string         `json:"author"`
	Category  string         `json:"category"`
	Tags      []byte         `gorm:"type:jsonb" json:"tags"`
	ReadTime  string         `json:"read_time"`
	Featured  bool           `gorm:"default:false" json:"featured"`
	Published bool           `gorm:"default:true" json:"published"`
	Views     int            `gorm:"default:0" json:"views"`
}

func (Blog) TableName() string {
	return "blogs"
}

type BlogComment struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	BlogID    uint           `gorm:"not null;index" json:"blog_id"`
	Author    string         `gorm:"not null" json:"author"`
	Avatar    string         `json:"avatar"`
	Message   string         `gorm:"type:text;not null" json:"message" binding:"required"`
	Likes     int            `gorm:"default:0" json:"likes"`
}

func (BlogComment) TableName() string {
	return "blog_comments"
}

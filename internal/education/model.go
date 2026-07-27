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
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	Title           string         `gorm:"not null" json:"title" binding:"required"`
	ShortTitle      string         `json:"shortTitle"`
	CollegesCount   int            `json:"collegesCount"`
	Affiliation     string         `json:"affiliation"`
	Badges          []byte         `gorm:"type:jsonb" json:"badges"`
	Level           string         `json:"level"`
	Field           string         `json:"field"`
	Duration        string         `json:"duration"`
	EstFee          string         `json:"estFee"`
	Highlights      []byte         `gorm:"type:jsonb" json:"highlights"`
	CareerPath      string         `json:"careerPath"`
	Description     string         `gorm:"type:text" json:"description"`
	Location        string         `json:"location"`
	GovtFee         string         `json:"govtFee"`
	PrivateFee      string         `json:"privateFee"`
	Mode            string         `json:"mode"`
	DegreeLabel     string         `json:"degreeLabel"`
	About           []byte         `gorm:"type:jsonb" json:"about"`
	Curriculum      []byte         `gorm:"type:jsonb" json:"curriculum"`
	Admissions      []byte         `gorm:"type:jsonb" json:"admissions"`
	Careers         []byte         `gorm:"type:jsonb" json:"careers"`
	IsGlobal        bool           `gorm:"default:false" json:"isGlobal"`
	Status          string         `gorm:"default:'draft'" json:"status"`
	CreatedBy       uint           `gorm:"default:0" json:"createdBy"`
	SourceProgramID *uint          `gorm:"default:null" json:"sourceProgramId"`
}

func (Course) TableName() string {
	return "courses"
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

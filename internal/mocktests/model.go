package mocktests

import (
	"time"

	"gorm.io/gorm"
)

// MockTest is its own domain (not a study_resources row) because every test
// owns an ordered question/option graph with exactly one correct option per
// question.
type MockTest struct {
	ID          uint   `gorm:"primarykey" json:"id"`
	Title       string `gorm:"not null" json:"title"`
	Description string `gorm:"type:text;default:''" json:"description"`
	Course      string `gorm:"default:'';index:idx_mock_tests_course" json:"course"`
	Year        string `gorm:"default:'';index:idx_mock_tests_year" json:"year"`
	// DurationMinutes is the advertised test length. Zero means unspecified.
	DurationMinutes int `gorm:"not null;default:0" json:"duration_minutes"`
	// IsPublished defaults to TRUE so the public endpoints work for freshly
	// created tests; drafts are opt-in.
	IsPublished bool           `gorm:"not null;default:true;index:idx_mock_tests_published" json:"is_published"`
	Views       int            `gorm:"not null;default:0" json:"views"`
	Attempts    int            `gorm:"not null;default:0" json:"attempts"`
	CreatedBy   uint           `gorm:"not null;default:0;index:idx_mock_tests_created_by" json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Questions []MockQuestion `gorm:"foreignKey:MockTestID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"questions,omitempty"`
}

type MockQuestion struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	MockTestID   uint   `gorm:"not null;index:idx_mock_questions_test_id" json:"mock_test_id"`
	QuestionText string `gorm:"type:text;not null" json:"question_text"`
	Explanation  string `gorm:"type:text;default:''" json:"explanation"`
	Order        int    `gorm:"not null;default:0" json:"order"`
	// CorrectOptionID is the answer key: a structural reference to one of this
	// question's own options. It is never serialized — every public and admin
	// payload goes through an explicit DTO — and it is validated on write.
	CorrectOptionID uint      `gorm:"not null;default:0;index:idx_mock_questions_correct_option" json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	Options []MockOption `gorm:"foreignKey:MockQuestionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"options,omitempty"`
}

type MockOption struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	MockQuestionID uint      `gorm:"not null;index:idx_mock_options_question_id" json:"mock_question_id"`
	OptionText     string    `gorm:"type:text;not null" json:"option_text"`
	Order          int       `gorm:"not null;default:0" json:"order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// MockAttempt stores a graded submission so the owner can reload the result
// later. It never contains the answer key.
type MockAttempt struct {
	ID                uint `gorm:"primarykey" json:"id"`
	MockTestID        uint `gorm:"not null;index:idx_mock_attempts_test" json:"mock_test_id"`
	UserID            uint `gorm:"not null;index:idx_mock_attempts_user" json:"user_id"`
	Score             int  `gorm:"not null;default:0" json:"score"`
	TotalQuestions    int  `gorm:"not null;default:0" json:"total_questions"`
	AnsweredQuestions int  `gorm:"not null;default:0" json:"answered_questions"`
	// ResultJSON is the marshalled []QuestionResultDTO of the graded run.
	ResultJSON string    `gorm:"type:text;default:''" json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

package mocktests

// ---------------------------------------------------------------- admin input

// CreateMockTestRequest is the admin payload for POST /admin/mock-tests.
// Option rows are never accepted from the client: the server assigns IDs and
// resolves the answer key from the is_correct flag.
type CreateMockTestRequest struct {
	Title           string          `json:"title" binding:"required"`
	Description     string          `json:"description"`
	Course          string          `json:"course"`
	Year            string          `json:"year"`
	DurationMinutes *int            `json:"duration_minutes"`
	IsPublished     *bool           `json:"is_published"`
	Questions       []QuestionInput `json:"questions" binding:"required"`
}

type QuestionInput struct {
	QuestionText string        `json:"question_text" binding:"required"`
	Explanation  string        `json:"explanation"`
	Options      []OptionInput `json:"options" binding:"required"`
}

type OptionInput struct {
	OptionText string `json:"option_text" binding:"required"`
	IsCorrect  bool   `json:"is_correct"`
}

// UpdateMockTestRequest replaces the whole graph atomically: any provided
// question/option list becomes the new, complete set for the test.
type UpdateMockTestRequest struct {
	Title           *string         `json:"title"`
	Description     *string         `json:"description"`
	Course          *string         `json:"course"`
	Year            *string         `json:"year"`
	DurationMinutes *int            `json:"duration_minutes"`
	IsPublished     *bool           `json:"is_published"`
	Questions       []QuestionInput `json:"questions"`
}

// ---------------------------------------------------------------- public input

// SubmitMockTestRequest accepts answers only. Any score, correctness flag or
// option text sent by the client is ignored: grading is server-side.
type SubmitMockTestRequest struct {
	Answers []AnswerInput `json:"answers" binding:"required"`
}

type AnswerInput struct {
	QuestionID uint `json:"question_id" binding:"required"`
	OptionID   uint `json:"option_id" binding:"required"`
}

// ------------------------------------------------------------------ admin DTO

// AdminMockTestSummaryDTO is the admin list projection. It adds the publish
// state, creator and attempt counters to the public summary, and still carries
// no question graph — so it can never expose an answer key.
type AdminMockTestSummaryDTO struct {
	PublicMockTestSummaryDTO
	IsPublished bool `json:"is_published"`
	CreatedBy   uint `json:"created_by"`
	Attempts    int  `json:"attempts"`
}

// MockTestDTO is the admin view of a test. It is the only payload that may
// carry the answer key.
type MockTestDTO struct {
	ID              uint              `json:"id"`
	Title           string            `json:"title"`
	Description     string            `json:"description"`
	Course          string            `json:"course"`
	Year            string            `json:"year"`
	DurationMinutes int               `json:"duration_minutes"`
	IsPublished     bool              `json:"is_published"`
	Views           int               `json:"views"`
	Attempts        int               `json:"attempts"`
	QuestionCount   int               `json:"question_count"`
	CreatedBy       uint              `json:"created_by"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
	Questions       []MockQuestionDTO `json:"questions,omitempty"`
}

type MockQuestionDTO struct {
	ID              uint            `json:"id"`
	QuestionText    string          `json:"question_text"`
	Explanation     string          `json:"explanation"`
	Order           int             `json:"order"`
	CorrectOptionID uint            `json:"correct_option_id"`
	Options         []MockOptionDTO `json:"options"`
}

type MockOptionDTO struct {
	ID         uint   `json:"id"`
	OptionText string `json:"option_text"`
	Order      int    `json:"order"`
}

// ------------------------------------------------------------------ public DTO

// PublicMockTestSummaryDTO is used by the public list. It has no question
// graph at all, so the answer key cannot be inferred from it.
type PublicMockTestSummaryDTO struct {
	ID              uint   `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Course          string `json:"course"`
	Year            string `json:"year"`
	DurationMinutes int    `json:"duration_minutes"`
	Views           int    `json:"views"`
	QuestionCount   int    `json:"question_count"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// PublicMockTestDetailDTO is the public test view. There is deliberately no
// correct_option_id, no is_correct and no explanation field anywhere in this
// graph: an unauthenticated client can never learn the answer key from it.
type PublicMockTestDetailDTO struct {
	ID              uint                    `json:"id"`
	Title           string                  `json:"title"`
	Description     string                  `json:"description"`
	Course          string                  `json:"course"`
	Year            string                  `json:"year"`
	DurationMinutes int                     `json:"duration_minutes"`
	Views           int                     `json:"views"`
	QuestionCount   int                     `json:"question_count"`
	CreatedAt       string                  `json:"created_at"`
	UpdatedAt       string                  `json:"updated_at"`
	Questions       []PublicMockQuestionDTO `json:"questions"`
}

type PublicMockQuestionDTO struct {
	ID           uint                  `json:"id"`
	QuestionText string                `json:"question_text"`
	Order        int                   `json:"order"`
	Options      []PublicMockOptionDTO `json:"options"`
}

type PublicMockOptionDTO struct {
	ID         uint   `json:"id"`
	OptionText string `json:"option_text"`
	Order      int    `json:"order"`
}

// ---------------------------------------------------------------- submit DTOs

// QuestionResultDTO reports the server's verdict for one answered question. It
// exposes correctness, never the correct option itself.
type QuestionResultDTO struct {
	QuestionID       uint `json:"question_id"`
	SelectedOptionID uint `json:"selected_option_id"`
	IsCorrect        bool `json:"is_correct"`
}

type SubmitResultDTO struct {
	AttemptID         uint                `json:"attempt_id"`
	MockTestID        uint                `json:"mock_test_id"`
	Score             int                 `json:"score"`
	TotalQuestions    int                 `json:"total_questions"`
	AnsweredQuestions int                 `json:"answered_questions"`
	ScorePercent      float64             `json:"score_percent"`
	Results           []QuestionResultDTO `json:"results"`
}

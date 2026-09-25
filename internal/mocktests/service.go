package mocktests

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"studsphere/backend/internal/shared/sanitize"
)

// Domain errors surfaced to the handler as 400/404 responses.
var (
	ErrTestNotFound       = errors.New("mock test not found")
	ErrNoQuestions        = errors.New("a mock test needs at least one question")
	ErrNoCorrectOption    = errors.New("each question needs exactly one correct option")
	ErrTooFewOptions      = errors.New("each question needs at least two options")
	ErrEmptyQuestion      = errors.New("question text is required")
	ErrEmptyOption        = errors.New("option text is required")
	ErrDurationOutOfRange = errors.New("duration_minutes cannot be negative")
	ErrAttemptNotFound    = errors.New("attempt not found")
	ErrAttemptForbidden   = errors.New("attempt does not belong to you")
)

const (
	maxTitleLength        = 255
	maxQuestionTextLen    = 5000
	maxOptionTextLen      = 2000
	maxCourseYearLength   = 100
	maxQuestionsPerTest   = 500
	maxOptionsPerQuestion = 20
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func normalizePageLimit(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

// ---------------------------------------------------------------- read paths

// GetTests returns the public (published-only) list projection.
func (s *Service) GetTests(filters TestFilters, page, limit int) ([]PublicMockTestSummaryDTO, int64, error) {
	page, limit = normalizePageLimit(page, limit)
	tests, total, err := s.repo.FindAll(filters, page, limit)
	if err != nil {
		return nil, 0, err
	}

	ids := make([]uint, 0, len(tests))
	for i := range tests {
		ids = append(ids, tests[i].ID)
	}
	counts, err := s.repo.QuestionCountsByTest(ids)
	if err != nil {
		// A missing count must not break the list; report zero instead.
		counts = map[uint]int{}
	}

	out := make([]PublicMockTestSummaryDTO, 0, len(tests))
	for i := range tests {
		out = append(out, toPublicSummary(&tests[i], counts[tests[i].ID]))
	}
	return out, total, nil
}

// GetAdminTests returns the admin list projection, including drafts.
func (s *Service) GetAdminTests(filters TestFilters, page, limit int) ([]AdminMockTestSummaryDTO, int64, error) {
	page, limit = normalizePageLimit(page, limit)
	tests, total, err := s.repo.FindAll(filters, page, limit)
	if err != nil {
		return nil, 0, err
	}

	ids := make([]uint, 0, len(tests))
	for i := range tests {
		ids = append(ids, tests[i].ID)
	}
	counts, err := s.repo.QuestionCountsByTest(ids)
	if err != nil {
		counts = map[uint]int{}
	}

	out := make([]AdminMockTestSummaryDTO, 0, len(tests))
	for i := range tests {
		out = append(out, AdminMockTestSummaryDTO{
			PublicMockTestSummaryDTO: toPublicSummary(&tests[i], counts[tests[i].ID]),
			IsPublished:              tests[i].IsPublished,
			CreatedBy:                tests[i].CreatedBy,
			Attempts:                 tests[i].Attempts,
		})
	}
	return out, total, nil
}

// GetPublicTest returns the public detail projection without the answer key.
func (s *Service) GetPublicTest(id uint) (*PublicMockTestDetailDTO, error) {
	test, err := s.repo.FindTestByID(id, true)
	if err != nil {
		return nil, ErrTestNotFound
	}
	detail := toPublicDetail(test)
	// View counting is best-effort analytics.
	_ = s.repo.IncrementViews(id)
	return &detail, nil
}

// GetAdminTest returns the full graph including the answer key.
func (s *Service) GetAdminTest(id uint) (*MockTestDTO, error) {
	test, err := s.repo.FindTestByID(id, false)
	if err != nil {
		return nil, ErrTestNotFound
	}
	return toAdminDTO(test), nil
}

// --------------------------------------------------------------- write paths

func (s *Service) CreateTest(req CreateMockTestRequest, createdBy uint) (*MockTestDTO, error) {
	seeds, err := validateGraph(req.Questions)
	if err != nil {
		return nil, err
	}
	title := sanitize.TruncatePlainText(req.Title, maxTitleLength)
	if title == "" {
		return nil, errors.New("title is required")
	}

	test := &MockTest{
		Title:           title,
		Description:     sanitize.RichText(req.Description),
		Course:          sanitize.TruncatePlainText(req.Course, maxCourseYearLength),
		Year:            sanitize.TruncatePlainText(req.Year, maxCourseYearLength),
		DurationMinutes: durationOrZero(req.DurationMinutes),
		IsPublished:     req.IsPublished == nil || *req.IsPublished,
		CreatedBy:       createdBy,
	}
	if err := s.repo.CreateTestWithGraph(test, seeds); err != nil {
		return nil, err
	}
	return s.GetAdminTest(test.ID)
}

// UpdateTest replaces the graph atomically. When req.Questions is present it
// becomes the new complete set; otherwise only the scalars are updated.
func (s *Service) UpdateTest(id uint, req UpdateMockTestRequest) (*MockTestDTO, error) {
	test, err := s.repo.FindTestByIDWithoutGraph(id)
	if err != nil {
		return nil, ErrTestNotFound
	}

	if req.Title != nil {
		title := sanitize.TruncatePlainText(*req.Title, maxTitleLength)
		if title == "" {
			return nil, errors.New("title is required")
		}
		test.Title = title
	}
	if req.Description != nil {
		test.Description = sanitize.RichText(*req.Description)
	}
	if req.Course != nil {
		test.Course = sanitize.TruncatePlainText(*req.Course, maxCourseYearLength)
	}
	if req.Year != nil {
		test.Year = sanitize.TruncatePlainText(*req.Year, maxCourseYearLength)
	}
	if req.DurationMinutes != nil {
		duration, err := validateDuration(*req.DurationMinutes)
		if err != nil {
			return nil, err
		}
		test.DurationMinutes = duration
	}
	if req.IsPublished != nil {
		test.IsPublished = *req.IsPublished
	}

	if req.Questions == nil {
		if err := s.repo.UpdateTestScalars(test); err != nil {
			return nil, err
		}
		return s.GetAdminTest(id)
	}

	seeds, err := validateGraph(req.Questions)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceTestGraph(test, seeds); err != nil {
		return nil, err
	}
	return s.GetAdminTest(id)
}

func (s *Service) DeleteTest(id uint) error {
	if _, err := s.repo.FindTestByIDWithoutGraph(id); err != nil {
		return ErrTestNotFound
	}
	return s.repo.DeleteTest(id)
}

// ------------------------------------------------------------------ grading

// SubmitTest grades an attempt server-side. Client-supplied scores, correctness
// flags or option text are ignored: the request only carries question_id and
// option_id pairs, and every pair is validated against this test's own graph.
func (s *Service) SubmitTest(id, userID uint, answers []AnswerInput) (*SubmitResultDTO, error) {
	test, err := s.repo.FindTestByID(id, true)
	if err != nil {
		return nil, ErrTestNotFound
	}
	if len(answers) == 0 {
		return nil, errors.New("at least one answer is required")
	}
	if len(answers) > len(test.Questions) {
		return nil, errors.New("too many answers for this mock test")
	}

	questionByID := make(map[uint]*MockQuestion, len(test.Questions))
	ownerByOptionID := make(map[uint]uint, len(test.Questions)*4)
	for i := range test.Questions {
		question := &test.Questions[i]
		questionByID[question.ID] = question
		for j := range question.Options {
			ownerByOptionID[question.Options[j].ID] = question.ID
		}
	}

	results := make([]QuestionResultDTO, 0, len(answers))
	answered := make(map[uint]bool, len(answers))
	correct := 0
	for _, answer := range answers {
		question, ok := questionByID[answer.QuestionID]
		if !ok {
			return nil, fmt.Errorf("question %d does not belong to this mock test", answer.QuestionID)
		}
		if answered[answer.QuestionID] {
			return nil, fmt.Errorf("duplicate answer for question %d", answer.QuestionID)
		}
		answered[answer.QuestionID] = true

		owner, ok := ownerByOptionID[answer.OptionID]
		if !ok {
			return nil, fmt.Errorf("option %d does not belong to this mock test", answer.OptionID)
		}
		if owner != answer.QuestionID {
			return nil, fmt.Errorf("option %d does not belong to question %d", answer.OptionID, answer.QuestionID)
		}

		isCorrect := question.CorrectOptionID != 0 && question.CorrectOptionID == answer.OptionID
		if isCorrect {
			correct++
		}
		results = append(results, QuestionResultDTO{
			QuestionID:       question.ID,
			SelectedOptionID: answer.OptionID,
			IsCorrect:        isCorrect,
		})
	}

	result := SubmitResultDTO{
		MockTestID:        test.ID,
		Score:             correct,
		TotalQuestions:    len(test.Questions),
		AnsweredQuestions: len(results),
		Results:           results,
	}
	result.ScorePercent = percentage(correct, len(test.Questions))

	payload, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	attempt := &MockAttempt{
		MockTestID:        test.ID,
		UserID:            userID,
		Score:             result.Score,
		TotalQuestions:    result.TotalQuestions,
		AnsweredQuestions: result.AnsweredQuestions,
		ResultJSON:        string(payload),
	}
	if err := s.repo.CreateAttempt(attempt); err != nil {
		return nil, err
	}
	result.AttemptID = attempt.ID
	return &result, nil
}

// GetAttempt returns a stored graded result, scoped to its owner.
func (s *Service) GetAttempt(attemptID, userID uint) (*SubmitResultDTO, error) {
	attempt, err := s.repo.FindAttemptByID(attemptID)
	if err != nil {
		return nil, ErrAttemptNotFound
	}
	if attempt.UserID != userID {
		return nil, ErrAttemptForbidden
	}

	var results []QuestionResultDTO
	if attempt.ResultJSON != "" {
		if err := json.Unmarshal([]byte(attempt.ResultJSON), &results); err != nil {
			results = nil
		}
	}
	result := SubmitResultDTO{
		AttemptID:         attempt.ID,
		MockTestID:        attempt.MockTestID,
		Score:             attempt.Score,
		TotalQuestions:    attempt.TotalQuestions,
		AnsweredQuestions: attempt.AnsweredQuestions,
		Results:           results,
	}
	result.ScorePercent = percentage(attempt.Score, attempt.TotalQuestions)
	return &result, nil
}

// --------------------------------------------------------------- validation

func validateDuration(minutes int) (int, error) {
	if minutes < 0 {
		return 0, ErrDurationOutOfRange
	}
	if minutes > 24*60 {
		return 0, errors.New("duration_minutes cannot exceed 1440")
	}
	return minutes, nil
}

func durationOrZero(minutes *int) int {
	if minutes == nil || *minutes < 0 {
		return 0
	}
	if *minutes > 24*60 {
		return 24 * 60
	}
	return *minutes
}

// validateGraph enforces: at least one question, at most a few hundred, every
// question needs at least two non-empty options, and exactly one option per
// question is flagged correct. It returns the sanitized seeds.
func validateGraph(questions []QuestionInput) ([]QuestionSeed, error) {
	if len(questions) == 0 {
		return nil, ErrNoQuestions
	}
	if len(questions) > maxQuestionsPerTest {
		return nil, fmt.Errorf("a mock test cannot have more than %d questions", maxQuestionsPerTest)
	}

	seeds := make([]QuestionSeed, 0, len(questions))
	for qi, question := range questions {
		// Question text is PLAIN text, not rich text: HTML-escaping it would
		// turn "2 < 3" into "2 &lt; 3" in storage and break the rendered
		// question. It is only trimmed and length-capped. Descriptions and
		// explanations stay rich-text sanitized.
		text := sanitize.TruncatePlainText(question.QuestionText, maxQuestionTextLen)
		if text == "" {
			return nil, fmt.Errorf("question %d: %w", qi+1, ErrEmptyQuestion)
		}
		if len(question.Options) < 2 {
			return nil, fmt.Errorf("question %d: %w", qi+1, ErrTooFewOptions)
		}
		if len(question.Options) > maxOptionsPerQuestion {
			return nil, fmt.Errorf("question %d: cannot have more than %d options", qi+1, maxOptionsPerQuestion)
		}

		correctCount := 0
		options := make([]OptionSeed, 0, len(question.Options))
		for oi, option := range question.Options {
			optionText := sanitize.TruncatePlainText(option.OptionText, maxOptionTextLen)
			if optionText == "" {
				return nil, fmt.Errorf("question %d option %d: %w", qi+1, oi+1, ErrEmptyOption)
			}
			if option.IsCorrect {
				correctCount++
			}
			options = append(options, OptionSeed{Text: optionText, IsCorrect: option.IsCorrect})
		}
		if correctCount != 1 {
			return nil, fmt.Errorf("question %d: %w", qi+1, ErrNoCorrectOption)
		}

		seeds = append(seeds, QuestionSeed{
			Text:        text,
			Explanation: sanitize.RichText(question.Explanation),
			Options:     options,
		})
	}
	return seeds, nil
}

func percentage(score, total int) float64 {
	if total <= 0 {
		return 0
	}
	value := float64(score) / float64(total) * 100
	return float64(int(value*100+0.5)) / 100
}

// ------------------------------------------------------------ DTO conversion

func toPublicSummary(test *MockTest, questionCount int) PublicMockTestSummaryDTO {
	return PublicMockTestSummaryDTO{
		ID:              test.ID,
		Title:           test.Title,
		Description:     test.Description,
		Course:          test.Course,
		Year:            test.Year,
		DurationMinutes: test.DurationMinutes,
		Views:           test.Views,
		QuestionCount:   questionCount,
		CreatedAt:       formatTime(test.CreatedAt),
		UpdatedAt:       formatTime(test.UpdatedAt),
	}
}

func toPublicDetail(test *MockTest) PublicMockTestDetailDTO {
	detail := PublicMockTestDetailDTO{
		ID:              test.ID,
		Title:           test.Title,
		Description:     test.Description,
		Course:          test.Course,
		Year:            test.Year,
		DurationMinutes: test.DurationMinutes,
		Views:           test.Views,
		QuestionCount:   len(test.Questions),
		CreatedAt:       formatTime(test.CreatedAt),
		UpdatedAt:       formatTime(test.UpdatedAt),
		Questions:       make([]PublicMockQuestionDTO, 0, len(test.Questions)),
	}
	for i := range test.Questions {
		question := &test.Questions[i]
		publicQuestion := PublicMockQuestionDTO{
			ID:           question.ID,
			QuestionText: question.QuestionText,
			Order:        question.Order,
			Options:      make([]PublicMockOptionDTO, 0, len(question.Options)),
		}
		for j := range question.Options {
			publicQuestion.Options = append(publicQuestion.Options, PublicMockOptionDTO{
				ID:         question.Options[j].ID,
				OptionText: question.Options[j].OptionText,
				Order:      question.Options[j].Order,
			})
		}
		detail.Questions = append(detail.Questions, publicQuestion)
	}
	return detail
}

func toAdminDTO(test *MockTest) *MockTestDTO {
	dto := &MockTestDTO{
		ID:              test.ID,
		Title:           test.Title,
		Description:     test.Description,
		Course:          test.Course,
		Year:            test.Year,
		DurationMinutes: test.DurationMinutes,
		IsPublished:     test.IsPublished,
		Views:           test.Views,
		Attempts:        test.Attempts,
		QuestionCount:   len(test.Questions),
		CreatedBy:       test.CreatedBy,
		CreatedAt:       formatTime(test.CreatedAt),
		UpdatedAt:       formatTime(test.UpdatedAt),
		Questions:       make([]MockQuestionDTO, 0, len(test.Questions)),
	}
	for i := range test.Questions {
		question := &test.Questions[i]
		adminQuestion := MockQuestionDTO{
			ID:              question.ID,
			QuestionText:    question.QuestionText,
			Explanation:     question.Explanation,
			Order:           question.Order,
			CorrectOptionID: question.CorrectOptionID,
			Options:         make([]MockOptionDTO, 0, len(question.Options)),
		}
		for j := range question.Options {
			adminQuestion.Options = append(adminQuestion.Options, MockOptionDTO{
				ID:         question.Options[j].ID,
				OptionText: question.Options[j].OptionText,
				Order:      question.Options[j].Order,
			})
		}
		dto.Questions = append(dto.Questions, adminQuestion)
	}
	return dto
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

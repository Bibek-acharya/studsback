package mocktests

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&MockTest{}, &MockQuestion{}, &MockOption{}, &MockAttempt{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func setupRouter(t *testing.T, h *Handler, authed bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if authed {
		r.Use(func(c *gin.Context) {
			c.Set("user_id", uint(7))
			c.Set("user_role", "superadmin")
			c.Next()
		})
	}
	RegisterRoutes(r, nil, nil, h)
	return r
}

func validRequest(title string) CreateMockTestRequest {
	return CreateMockTestRequest{
		Title:           title,
		Course:          "Physics",
		Year:            "2081",
		DurationMinutes: intPtr(45),
		Questions: []QuestionInput{
			{
				QuestionText: "What is 2 + 2?",
				Explanation:  "Basic arithmetic",
				Options: []OptionInput{
					{OptionText: "3"},
					{OptionText: "4", IsCorrect: true},
					{OptionText: "5"},
				},
			},
			{
				QuestionText: "Force is measured in?",
				Options: []OptionInput{
					{OptionText: "Newton", IsCorrect: true},
					{OptionText: "Joule"},
				},
			},
		},
	}
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }
func createTest(t *testing.T, svc *Service, title string) *MockTestDTO {
	t.Helper()
	dto, err := svc.CreateTest(validRequest(title), 1)
	if err != nil {
		t.Fatalf("create test: %v", err)
	}
	return dto
}

// ------------------------------------------------------------- graph writes

func TestCreateTestPersistsGraphAndAnswerKey(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	dto := createTest(t, svc, "Unit test 2081")

	if dto.ID == 0 {
		t.Fatal("test was not persisted")
	}
	if !dto.IsPublished {
		t.Error("new tests must default to published")
	}
	if dto.DurationMinutes != 45 {
		t.Errorf("duration = %d, want 45", dto.DurationMinutes)
	}
	if len(dto.Questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(dto.Questions))
	}

	first := dto.Questions[0]
	if first.Order != 1 {
		t.Errorf("question 1 order = %d, want 1", first.Order)
	}
	if len(first.Options) != 3 {
		t.Fatalf("options = %d, want 3", len(first.Options))
	}
	if first.CorrectOptionID == 0 {
		t.Fatal("correct_option_id was not resolved")
	}
	// The answer key must point at this question's own correct option.
	var matched bool
	for _, option := range first.Options {
		if option.ID == first.CorrectOptionID {
			matched = true
			if option.Order != 2 {
				t.Errorf("correct option order = %d, want 2 (the is_correct option)", option.Order)
			}
		}
	}
	if !matched {
		t.Error("correct_option_id does not reference an option of this question")
	}
	if dto.Questions[1].Order != 2 {
		t.Errorf("question 2 order = %d, want 2", dto.Questions[1].Order)
	}

	// Re-read straight from the database.
	stored, err := svc.GetAdminTest(dto.ID)
	if err != nil {
		t.Fatalf("get admin test: %v", err)
	}
	if stored.QuestionCount != 2 {
		t.Errorf("question count = %d, want 2", stored.QuestionCount)
	}
}

func TestCreateTestValidationRollsBack(t *testing.T) {
	cases := []struct {
		name  string
		build func() CreateMockTestRequest
		want  error
	}{
		{
			name: "no questions",
			build: func() CreateMockTestRequest {
				req := validRequest("x")
				req.Questions = nil
				return req
			},
			want: ErrNoQuestions,
		},
		{
			name: "one option only",
			build: func() CreateMockTestRequest {
				req := validRequest("x")
				req.Questions[0].Options = req.Questions[0].Options[:1]
				return req
			},
			want: ErrTooFewOptions,
		},
		{
			name: "no correct option",
			build: func() CreateMockTestRequest {
				req := validRequest("x")
				req.Questions[0].Options[1].IsCorrect = false
				return req
			},
			want: ErrNoCorrectOption,
		},
		{
			name: "two correct options",
			build: func() CreateMockTestRequest {
				req := validRequest("x")
				req.Questions[0].Options[2].IsCorrect = true
				return req
			},
			want: ErrNoCorrectOption,
		},
		{
			name: "empty option text",
			build: func() CreateMockTestRequest {
				req := validRequest("x")
				req.Questions[0].Options[0].OptionText = "   "
				return req
			},
			want: ErrEmptyOption,
		},
		{
			name: "empty question text",
			build: func() CreateMockTestRequest {
				req := validRequest("x")
				req.Questions[0].QuestionText = ""
				return req
			},
			want: ErrEmptyQuestion,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTestDB(t)
			svc := NewService(NewRepository(db))

			if _, err := svc.CreateTest(tc.build(), 1); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}

			// Nothing may be persisted: the whole insert is one transaction.
			var tests, questions, options int64
			db.Model(&MockTest{}).Count(&tests)
			db.Model(&MockQuestion{}).Count(&questions)
			db.Model(&MockOption{}).Count(&options)
			if tests != 0 || questions != 0 || options != 0 {
				t.Errorf("rollback failed: tests=%d questions=%d options=%d", tests, questions, options)
			}
		})
	}
}

func TestUpdateTestReplacesGraphAtomically(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)
	dto := createTest(t, svc, "Original")
	oldQuestionIDs := make([]uint, 0, len(dto.Questions))
	for _, q := range dto.Questions {
		oldQuestionIDs = append(oldQuestionIDs, q.ID)
	}

	newTitle := "Replaced"
	req := UpdateMockTestRequest{
		Title: &newTitle,
		Questions: []QuestionInput{
			{
				QuestionText: "Only question now?",
				Options: []OptionInput{
					{OptionText: "yes", IsCorrect: true},
					{OptionText: "no"},
				},
			},
		},
	}
	updated, err := svc.UpdateTest(dto.ID, req)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != newTitle {
		t.Errorf("title = %q, want %q", updated.Title, newTitle)
	}
	if len(updated.Questions) != 1 {
		t.Fatalf("questions = %d, want 1", len(updated.Questions))
	}
	if updated.Questions[0].QuestionText != "Only question now?" {
		t.Errorf("question text = %q", updated.Questions[0].QuestionText)
	}
	if updated.Questions[0].CorrectOptionID != updated.Questions[0].Options[0].ID {
		t.Error("answer key was not re-resolved after the replace")
	}

	// The old graph must be gone, not orphaned.
	var remaining int64
	db.Model(&MockQuestion{}).Count(&remaining)
	if remaining != 1 {
		t.Errorf("question rows = %d, want 1 (old graph must be deleted)", remaining)
	}
	db.Model(&MockOption{}).Count(&remaining)
	if remaining != 2 {
		t.Errorf("option rows = %d, want 2 (old options must be deleted)", remaining)
	}
	var orphans int64
	db.Model(&MockQuestion{}).Where("id IN ?", oldQuestionIDs).Count(&orphans)
	if orphans != 0 {
		t.Errorf("old question rows still present: %d", orphans)
	}
}

func TestUpdateTestInvalidGraphLeavesPreviousGraphIntact(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Keep me")

	before, err := svc.GetAdminTest(dto.ID)
	if err != nil {
		t.Fatalf("get admin test: %v", err)
	}

	bad := UpdateMockTestRequest{
		Questions: []QuestionInput{
			{
				QuestionText: "Broken",
				Options: []OptionInput{
					{OptionText: "only one"},
				},
			},
		},
	}
	if _, err := svc.UpdateTest(dto.ID, bad); !errors.Is(err, ErrTooFewOptions) {
		t.Fatalf("error = %v, want %v", err, ErrTooFewOptions)
	}

	after, err := svc.GetAdminTest(dto.ID)
	if err != nil {
		t.Fatalf("get admin test after failure: %v", err)
	}
	if after.Title != before.Title || len(after.Questions) != len(before.Questions) {
		t.Errorf("graph changed after a rejected update: %+v", after)
	}
	for i := range before.Questions {
		if after.Questions[i].ID != before.Questions[i].ID {
			t.Errorf("question %d was replaced by a rejected update", i)
		}
		if after.Questions[i].CorrectOptionID != before.Questions[i].CorrectOptionID {
			t.Errorf("question %d answer key changed by a rejected update", i)
		}
	}

	var questions, options int64
	db.Model(&MockQuestion{}).Count(&questions)
	db.Model(&MockOption{}).Count(&options)
	if questions != 2 || options != 5 {
		t.Errorf("rows after rollback: questions=%d options=%d, want 2/5", questions, options)
	}
}

func TestUpdateTestScalarsOnlyKeepsGraph(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Scalars")

	title := "Renamed"
	draft := false
	updated, err := svc.UpdateTest(dto.ID, UpdateMockTestRequest{Title: &title, IsPublished: &draft})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != title {
		t.Errorf("title = %q", updated.Title)
	}
	if updated.IsPublished {
		t.Error("is_published=false was not persisted")
	}
	if len(updated.Questions) != 2 || updated.QuestionCount != 2 {
		t.Errorf("graph changed by a scalar-only update: %+v", updated)
	}
}

func TestDeleteTestRemovesGraph(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Delete me")

	if err := svc.DeleteTest(dto.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetAdminTest(dto.ID); !errors.Is(err, ErrTestNotFound) {
		t.Errorf("error = %v, want ErrTestNotFound", err)
	}
	var questions, options int64
	db.Model(&MockQuestion{}).Count(&questions)
	db.Model(&MockOption{}).Count(&options)
	if questions != 0 || options != 0 {
		t.Errorf("orphaned rows: questions=%d options=%d", questions, options)
	}
	if err := svc.DeleteTest(999); !errors.Is(err, ErrTestNotFound) {
		t.Errorf("delete missing error = %v, want ErrTestNotFound", err)
	}
}

// -------------------------------------------------------------- public DTOs

func TestPublicDTONeverLeaksAnswerKey(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Public shape")

	detail, err := svc.GetPublicTest(dto.ID)
	if err != nil {
		t.Fatalf("get public test: %v", err)
	}

	payload, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(payload)
	for _, forbidden := range []string{"correct_option_id", "CorrectOptionID", "is_correct", "IsCorrect", "explanation", "Explanation"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("public payload leaks %q:\n%s", forbidden, body)
		}
	}
	// Option IDs are public by design (a submit payload references them), so
	// the guarantee is structural: no field marks which option is correct.
	if strings.Contains(body, "correct") {
		t.Errorf("public payload marks an answer key:\n%s", body)
	}
	if len(detail.Questions) != 2 || len(detail.Questions[0].Options) != 3 {
		t.Errorf("public graph is incomplete: %+v", detail.Questions)
	}

	// The list projection has no graph at all.
	list, _, err := svc.GetTests(TestFilters{PublishedOnly: true}, 1, 20)
	if err != nil {
		t.Fatalf("get tests: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list length = %d, want 1", len(list))
	}
	if list[0].QuestionCount != 2 {
		t.Errorf("question count = %d, want 2", list[0].QuestionCount)
	}
	listPayload, _ := json.Marshal(list)
	if strings.Contains(string(listPayload), "options") || strings.Contains(string(listPayload), "questions") {
		t.Errorf("public list must not carry a graph:\n%s", listPayload)
	}
}

func TestPublicDTOSerializationOfModelCannotLeakKey(t *testing.T) {
	// Defense in depth: even marshalling the raw model (which some other
	// handler might do by mistake) hides the answer key.
	question := MockQuestion{ID: 1, QuestionText: "q", CorrectOptionID: 987654}
	question.Options = []MockOption{{ID: 1, OptionText: "plain option"}}
	payload, err := json.Marshal(question)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(payload), "987654") || strings.Contains(string(payload), "correct") {
		t.Errorf("raw model serialized the answer key:\n%s", payload)
	}
}

func TestPublicListOnlyShowsPublishedTests(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	published := createTest(t, svc, "Published")

	req := validRequest("Draft")
	req.IsPublished = boolPtr(false)
	draft, err := svc.CreateTest(req, 1)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.IsPublished {
		t.Error("draft was stored as published")
	}

	public, total, err := svc.GetTests(TestFilters{PublishedOnly: true}, 1, 20)
	if err != nil {
		t.Fatalf("public list: %v", err)
	}
	if total != 1 || len(public) != 1 || public[0].ID != published.ID {
		t.Fatalf("public list leaked a draft: total=%d %+v", total, public)
	}

	admin, adminTotal, err := svc.GetTests(TestFilters{}, 1, 20)
	if err != nil {
		t.Fatalf("admin list: %v", err)
	}
	if adminTotal != 2 || len(admin) != 2 {
		t.Errorf("admin list total=%d len=%d, want 2", adminTotal, len(admin))
	}

	if _, err := svc.GetPublicTest(draft.ID); !errors.Is(err, ErrTestNotFound) {
		t.Errorf("public detail of a draft = %v, want ErrTestNotFound", err)
	}
	if _, err := svc.GetAdminTest(draft.ID); err != nil {
		t.Errorf("admin detail of a draft failed: %v", err)
	}
}

// Search and course filters compare LOWER(column) against a LIKE pattern, so a
// mixed-case query term must still match.
func TestSearchIsCaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)

	physics := validRequest("Physics Entrance Model")
	physics.Course = "Physics"
	physics.Description = "<p>Newtonian mechanics revision</p>"
	if _, err := svc.CreateTest(physics, 1); err != nil {
		t.Fatalf("create: %v", err)
	}
	maths := validRequest("Mathematics Drill")
	maths.Course = "Mathematics"
	if _, err := svc.CreateTest(maths, 1); err != nil {
		t.Fatalf("create: %v", err)
	}

	for _, search := range []string{"physics", "PHYSICS", "PhYsIcS", "  physics  "} {
		tests, total, err := repo.FindAll(TestFilters{Search: search}, 1, 20)
		if err != nil {
			t.Fatalf("FindAll(%q): %v", search, err)
		}
		if total != 1 || len(tests) != 1 {
			t.Errorf("search %q: total=%d len=%d, want 1", search, total, len(tests))
			continue
		}
		if tests[0].Title != "Physics Entrance Model" {
			t.Errorf("search %q matched %q", search, tests[0].Title)
		}
	}

	// The description is matched case-insensitively too.
	if _, total, err := repo.FindAll(TestFilters{Search: "NEWTONIAN"}, 1, 20); err != nil || total != 1 {
		t.Errorf("description search: total=%d err=%v, want 1", total, err)
	}

	// The course filter must lowercase its term as well.
	for _, course := range []string{"mathematics", "MATHEMATICS", "Math"} {
		tests, total, err := repo.FindAll(TestFilters{Course: course}, 1, 20)
		if err != nil {
			t.Fatalf("FindAll(course=%q): %v", course, err)
		}
		if total != 1 || len(tests) != 1 || tests[0].Title != "Mathematics Drill" {
			t.Errorf("course %q: total=%d %+v", course, total, tests)
		}
	}

	// A term that matches nothing returns nothing.
	if _, total, err := repo.FindAll(TestFilters{Search: "chemistry"}, 1, 20); err != nil || total != 0 {
		t.Errorf("unmatched search: total=%d err=%v, want 0", total, err)
	}
}

func TestAdminListExposesPublishStateWithoutGraph(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	createTest(t, svc, "Published one")
	draftReq := validRequest("Draft one")
	draftReq.IsPublished = boolPtr(false)
	if _, err := svc.CreateTest(draftReq, 42); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	admin, total, err := svc.GetAdminTests(TestFilters{}, 1, 20)
	if err != nil {
		t.Fatalf("admin list: %v", err)
	}
	if total != 2 || len(admin) != 2 {
		t.Fatalf("admin list total=%d len=%d, want 2", total, len(admin))
	}

	published, draft := 0, 0
	for _, item := range admin {
		if item.IsPublished {
			published++
		} else {
			draft++
		}
		if item.QuestionCount != 2 {
			t.Errorf("question count = %d, want 2", item.QuestionCount)
		}
	}
	if published != 1 || draft != 1 {
		t.Errorf("published=%d draft=%d, want 1/1", published, draft)
	}

	payload, err := json.Marshal(admin)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(payload)
	if !strings.Contains(body, `"is_published"`) {
		t.Errorf("admin list lost the publish state:\n%s", body)
	}
	// The admin list must still not carry a graph or an answer key.
	for _, forbidden := range []string{"correct_option_id", "options", "questions"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("admin list must not carry %q:\n%s", forbidden, body)
		}
	}
}

// ----------------------------------------------------------------- grading

func TestSubmitTestGradesServerSide(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Grading")

	answers := make([]AnswerInput, 0, 2)
	for _, question := range dto.Questions {
		answers = append(answers, AnswerInput{
			QuestionID: question.ID,
			OptionID:   question.CorrectOptionID,
		})
	}
	// One deliberately wrong answer: pick an option that is not the key.
	answers[1].OptionID = 0
	for _, option := range dto.Questions[1].Options {
		if option.ID != dto.Questions[1].CorrectOptionID {
			answers[1].OptionID = option.ID
			break
		}
	}
	if answers[1].OptionID == 0 || answers[1].OptionID == dto.Questions[1].CorrectOptionID {
		t.Fatal("test setup: no wrong option available")
	}

	result, err := svc.SubmitTest(dto.ID, 42, answers)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.Score != 1 {
		t.Errorf("score = %d, want 1", result.Score)
	}
	if result.TotalQuestions != 2 || result.AnsweredQuestions != 2 {
		t.Errorf("total/answered = %d/%d, want 2/2", result.TotalQuestions, result.AnsweredQuestions)
	}
	if result.ScorePercent != 50 {
		t.Errorf("score percent = %v, want 50", result.ScorePercent)
	}
	if len(result.Results) != 2 || !result.Results[0].IsCorrect || result.Results[1].IsCorrect {
		t.Errorf("per-question results = %+v", result.Results)
	}
	if result.AttemptID == 0 {
		t.Error("attempt was not persisted")
	}

	// The attempt counter moved, and the graded result is reloadable by owner.
	var reloaded MockTest
	db.First(&reloaded, dto.ID)
	if reloaded.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", reloaded.Attempts)
	}
	stored, err := svc.GetAttempt(result.AttemptID, 42)
	if err != nil {
		t.Fatalf("get attempt: %v", err)
	}
	if stored.Score != 1 || len(stored.Results) != 2 {
		t.Errorf("stored attempt = %+v", stored)
	}
	if _, err := svc.GetAttempt(result.AttemptID, 43); !errors.Is(err, ErrAttemptForbidden) {
		t.Errorf("cross-user attempt read = %v, want ErrAttemptForbidden", err)
	}
	if _, err := svc.GetAttempt(9999, 42); !errors.Is(err, ErrAttemptNotFound) {
		t.Errorf("missing attempt = %v, want ErrAttemptNotFound", err)
	}
}

func TestSubmitTestIgnoresClientSuppliedScoring(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "Anti cheat")

	// A client claiming a perfect score with bogus fields attached.
	raw := `{"answers":[{"question_id":` + itoa(dto.Questions[0].ID) +
		`,"option_id":` + itoa(dto.Questions[0].Options[0].ID) +
		`,"is_correct":true,"score":100,"option_text":"forged"}]}`

	var req SubmitMockTestRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	result, err := svc.SubmitTest(dto.ID, 5, req.Answers)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.Score != 0 {
		t.Errorf("score = %d, want 0: the client's correctness claim was trusted", result.Score)
	}
	if result.Results[0].IsCorrect {
		t.Error("a wrong option was graded correct")
	}
}

func TestSubmitTestRejectsForeignOptions(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	first := createTest(t, svc, "Test one")
	second := createTest(t, svc, "Test two")

	// An option that belongs to the OTHER test.
	foreignOptionID := second.Questions[0].Options[0].ID
	cases := []struct {
		name    string
		answers []AnswerInput
		wantErr string
	}{
		{
			name:    "option from another test",
			answers: []AnswerInput{{QuestionID: first.Questions[0].ID, OptionID: foreignOptionID}},
			wantErr: "does not belong to this mock test",
		},
		{
			name:    "option from another question of the same test",
			answers: []AnswerInput{{QuestionID: first.Questions[0].ID, OptionID: first.Questions[1].Options[0].ID}},
			wantErr: "does not belong to question",
		},
		{
			name:    "question from another test",
			answers: []AnswerInput{{QuestionID: second.Questions[0].ID, OptionID: second.Questions[0].Options[0].ID}},
			wantErr: "does not belong to this mock test",
		},
		{
			name:    "unknown question",
			answers: []AnswerInput{{QuestionID: 99999, OptionID: first.Questions[0].Options[0].ID}},
			wantErr: "does not belong to this mock test",
		},
		{
			name:    "unknown option",
			answers: []AnswerInput{{QuestionID: first.Questions[0].ID, OptionID: 99999}},
			wantErr: "does not belong to this mock test",
		},
		{
			name: "duplicate answer",
			answers: []AnswerInput{
				{QuestionID: first.Questions[0].ID, OptionID: first.Questions[0].Options[0].ID},
				{QuestionID: first.Questions[0].ID, OptionID: first.Questions[0].Options[1].ID},
			},
			wantErr: "duplicate answer",
		},
		{
			name:    "no answers",
			answers: nil,
			wantErr: "at least one answer",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SubmitTest(first.ID, 9, tc.answers)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
			// A rejected submission must not create an attempt.
			var attempts int64
			db.Model(&MockAttempt{}).Count(&attempts)
			if attempts != 0 {
				t.Errorf("attempts = %d, want 0 for a rejected submission", attempts)
			}
		})
	}
}

func TestSubmitTestRequiresPublishedTest(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	req := validRequest("Draft")
	req.IsPublished = boolPtr(false)
	draft, err := svc.CreateTest(req, 1)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	_, err = svc.SubmitTest(draft.ID, 3, []AnswerInput{{
		QuestionID: draft.Questions[0].ID,
		OptionID:   draft.Questions[0].CorrectOptionID,
	}})
	if !errors.Is(err, ErrTestNotFound) {
		t.Errorf("submit to a draft = %v, want ErrTestNotFound", err)
	}
}

// ------------------------------------------------------------- sanitization

// Descriptions and explanations are rich text and must be sanitized.
func TestDescriptionsAreSanitized(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	req := validRequest("Sanitize me")
	req.Description = `<p>Intro</p><script>alert(1)</script><p onclick="x()">Body</p>`
	req.Questions[0].Explanation = `<p>Because</p><script>alert(2)</script>`
	dto, err := svc.CreateTest(req, 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if strings.Contains(dto.Description, "<script") || strings.Contains(dto.Description, "onclick") {
		t.Errorf("description not sanitized: %q", dto.Description)
	}
	if !strings.Contains(dto.Description, "Intro") || !strings.Contains(dto.Description, "Body") {
		t.Errorf("description lost content: %q", dto.Description)
	}
	if strings.Contains(dto.Questions[0].Explanation, "<script") || strings.Contains(dto.Questions[0].Explanation, "alert(2)") {
		t.Errorf("explanation not sanitized: %q", dto.Questions[0].Explanation)
	}
	if !strings.Contains(dto.Questions[0].Explanation, "Because") {
		t.Errorf("explanation lost content: %q", dto.Questions[0].Explanation)
	}
}

// Question text is PLAIN text: it must not be HTML-escaped, so mathematical
// comparisons such as "2 < 3" survive verbatim and render correctly.
func TestQuestionTextIsPlainText(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	plainValues := []string{
		"Which is larger: 2 < 3 or 3 < 2?",
		"a < b && b > c",
		`He said "energy" & left`,
		"5 > 3, 2 < 4",
	}

	var lastID uint
	for i, value := range plainValues {
		req := validRequest(fmt.Sprintf("Plain question %d", i+1))
		req.Questions[0].QuestionText = value
		created, err := svc.CreateTest(req, 1)
		if err != nil {
			t.Fatalf("create %q: %v", value, err)
		}
		if got := created.Questions[0].QuestionText; got != value {
			t.Errorf("question text = %q, want %q (plain text must not be escaped)", got, value)
		}
		lastID = created.ID
	}

	// The stored value must also round-trip through JSON unchanged.
	public, err := svc.GetPublicTest(lastID)
	if err != nil {
		t.Fatalf("get public: %v", err)
	}
	payload, err := json.Marshal(public)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded PublicMockTestDetailDTO
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := decoded.Questions[0].QuestionText; got != plainValues[len(plainValues)-1] {
		t.Errorf("JSON round trip = %q, want %q", got, plainValues[len(plainValues)-1])
	}

	// Option text follows the same plain-text rule.
	optReq := validRequest("Plain options")
	optReq.Questions[0].Options[0].OptionText = "x < y"
	optDTO, err := svc.CreateTest(optReq, 1)
	if err != nil {
		t.Fatalf("create options: %v", err)
	}
	if got := optDTO.Questions[0].Options[0].OptionText; got != "x < y" {
		t.Errorf("option text = %q, want %q", got, "x < y")
	}

	// Trimming still applies, and blank text is still rejected.
	blankReq := validRequest("Blank question")
	blankReq.Questions[0].QuestionText = "   "
	if _, err := svc.CreateTest(blankReq, 1); !errors.Is(err, ErrEmptyQuestion) {
		t.Errorf("blank question error = %v, want ErrEmptyQuestion", err)
	}
	trimReq := validRequest("Trimmed question")
	trimReq.Questions[0].QuestionText = "  spaced out  "
	trimDTO, err := svc.CreateTest(trimReq, 1)
	if err != nil {
		t.Fatalf("create trimmed: %v", err)
	}
	if got := trimDTO.Questions[0].QuestionText; got != "spaced out" {
		t.Errorf("trimmed question = %q, want %q", got, "spaced out")
	}

	// Over-long text is capped, not rejected.
	longReq := validRequest("Long question")
	longReq.Questions[0].QuestionText = strings.Repeat("a", maxQuestionTextLen+500)
	longDTO, err := svc.CreateTest(longReq, 1)
	if err != nil {
		t.Fatalf("create long: %v", err)
	}
	if len(longDTO.Questions[0].QuestionText) != maxQuestionTextLen {
		t.Errorf("capped length = %d, want %d", len(longDTO.Questions[0].QuestionText), maxQuestionTextLen)
	}
}

// ------------------------------------------------------------------- routes

func TestSubmitRequiresAuthentication(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(NewService(NewRepository(db)))

	// An unauthenticated router: the auth middleware rejects everything.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rejected := false
	authMW := func(c *gin.Context) {
		rejected = true
		c.AbortWithStatus(http.StatusUnauthorized)
	}
	RegisterRoutes(r, authMW, nil, h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/mock-tests/1/submit", strings.NewReader(`{"answers":[]}`)))
	if !rejected {
		t.Fatal("submit route was reachable without authentication")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}

	// Browsing stays public.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/mock-tests", nil))
	if w.Code != http.StatusOK {
		t.Errorf("public list status = %d, want 200 without auth", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/mock-tests/1", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("public detail status = %d, want 404 for a missing test", w.Code)
	}
}

func TestAdminRoutesAreSuperadminGuarded(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(NewService(NewRepository(db)))
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", "student")
		c.Next()
	})
	RegisterRoutes(r, nil, func(c *gin.Context) {
		c.AbortWithStatus(http.StatusForbidden)
	}, h)

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/mock-tests", ""},
		{http.MethodGet, "/api/v1/admin/mock-tests/1", ""},
		{http.MethodPost, "/api/v1/admin/mock-tests", `{}`},
		{http.MethodPut, "/api/v1/admin/mock-tests/1", `{}`},
		{http.MethodDelete, "/api/v1/admin/mock-tests/1", ""},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want 403", tc.method, tc.path, w.Code)
		}
	}
}

func TestRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, nil, nil, NewHandler(NewService(NewRepository(setupTestDB(t)))))

	registered := map[string]bool{}
	for _, route := range r.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /api/v1/mock-tests",
		"GET /api/v1/mock-tests/:id",
		"POST /api/v1/mock-tests/:id/submit",
		"GET /api/v1/mock-tests/:id/attempts/:attempt_id",
		"GET /api/v1/admin/mock-tests",
		"GET /api/v1/admin/mock-tests/:id",
		"POST /api/v1/admin/mock-tests",
		"PUT /api/v1/admin/mock-tests/:id",
		"DELETE /api/v1/admin/mock-tests/:id",
	} {
		if !registered[want] {
			t.Errorf("missing route %s", want)
		}
	}
}

func TestPublicHandlerHidesAnswerKeyOverHTTP(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "HTTP shape")
	h := NewHandler(svc)
	r := setupRouter(t, h, false)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/mock-tests/"+itoa(dto.ID), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, forbidden := range []string{"correct_option_id", "is_correct", "explanation"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("HTTP payload leaks %q:\n%s", forbidden, body)
		}
	}
	if !strings.Contains(body, "What is 2 + 2?") {
		t.Errorf("payload lost the question text:\n%s", body)
	}
}

func TestSubmitHandlerGradesOverHTTP(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))
	dto := createTest(t, svc, "HTTP grading")
	h := NewHandler(svc)
	r := setupRouter(t, h, true)

	payload := `{"answers":[{"question_id":` + itoa(dto.Questions[0].ID) +
		`,"option_id":` + itoa(dto.Questions[0].CorrectOptionID) + `}]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/mock-tests/"+itoa(dto.ID)+"/submit", strings.NewReader(payload)))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"score":1`) {
		t.Errorf("expected a score of 1:\n%s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "correct_option_id") {
		t.Errorf("submit response leaked the answer key:\n%s", w.Body.String())
	}

	// The attempt result route is owner scoped and works for the owner.
	attemptID := extractAttemptID(t, w.Body.String())
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/api/v1/mock-tests/"+itoa(dto.ID)+"/attempts/"+itoa(attemptID), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("attempt status = %d, body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"score":1`) {
		t.Errorf("attempt result lost the score:\n%s", w.Body.String())
	}
}

func TestCreateHandlerRejectsInvalidGraph(t *testing.T) {
	db := setupTestDB(t)
	h := NewHandler(NewService(NewRepository(db)))
	r := setupRouter(t, h, true)

	body := `{"title":"Bad","questions":[{"question_text":"q","options":[{"option_text":"a","is_correct":true}]}]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/mock-tests", strings.NewReader(body)))
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "at least two options") {
		t.Errorf("unexpected error message: %s", w.Body.String())
	}
}

func itoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func extractAttemptID(t *testing.T, body string) uint {
	t.Helper()
	var envelope struct {
		Data struct {
			AttemptID uint `json:"attempt_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Data.AttemptID == 0 {
		t.Fatalf("no attempt_id in response: %s", body)
	}
	return envelope.Data.AttemptID
}

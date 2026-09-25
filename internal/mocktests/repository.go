package mocktests

import (
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// OptionSeed is one option as supplied by an admin. IDs are assigned by the
// database, so the answer key is resolved structurally after the insert.
type OptionSeed struct {
	Text      string
	IsCorrect bool
}

// QuestionSeed is one question with its ordered options.
type QuestionSeed struct {
	Text        string
	Explanation string
	Options     []OptionSeed
}

type TestFilters struct {
	Search        string
	Course        string
	Year          string
	PublishedOnly bool
}

func (r *Repository) buildQuery(filters TestFilters) *gorm.DB {
	q := r.db.Model(&MockTest{})
	if filters.PublishedOnly {
		q = q.Where("is_published = ?", true)
	}
	if search := strings.TrimSpace(filters.Search); search != "" {
		// The columns are compared through LOWER(), so the pattern must be
		// lowercased too — otherwise a mixed-case query never matches.
		like := "%" + strings.ToLower(search) + "%"
		q = q.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR LOWER(course) LIKE ?", like, like, like)
	}
	if course := strings.TrimSpace(filters.Course); course != "" {
		q = q.Where("LOWER(course) LIKE ?", "%"+strings.ToLower(course)+"%")
	}
	if filters.Year != "" {
		q = q.Where("year = ?", filters.Year)
	}
	return q
}

func (r *Repository) FindAll(filters TestFilters, page, limit int) ([]MockTest, int64, error) {
	var total int64
	if err := r.buildQuery(filters).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tests []MockTest
	err := r.buildQuery(filters).
		Order("created_at DESC, id DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&tests).Error
	if err != nil {
		return nil, 0, err
	}
	return tests, total, nil
}

// FindTestByID loads a test with its ordered questions and options. When
// publishedOnly is true an unpublished test is reported as not found.
func (r *Repository) FindTestByID(id uint, publishedOnly bool) (*MockTest, error) {
	var test MockTest
	q := r.db.
		Preload("Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC, id ASC")
		}).
		Preload("Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC, id ASC")
		})
	if publishedOnly {
		q = q.Where("is_published = ?", true)
	}
	if err := q.First(&test, id).Error; err != nil {
		return nil, err
	}
	return &test, nil
}

// FindTestByIDWithoutGraph loads only the test scalars.
func (r *Repository) FindTestByIDWithoutGraph(id uint) (*MockTest, error) {
	var test MockTest
	if err := r.db.First(&test, id).Error; err != nil {
		return nil, err
	}
	return &test, nil
}

// QuestionCountsByTest returns the question count per test ID for the given
// page of tests.
func (r *Repository) QuestionCountsByTest(ids []uint) (map[uint]int, error) {
	counts := make(map[uint]int, len(ids))
	if len(ids) == 0 {
		return counts, nil
	}
	type row struct {
		MockTestID uint
		Total      int
	}
	var rows []row
	err := r.db.Model(&MockQuestion{}).
		Select("mock_test_id, COUNT(*) as total").
		Where("mock_test_id IN ?", ids).
		Group("mock_test_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, rw := range rows {
		counts[rw.MockTestID] = rw.Total
	}
	return counts, nil
}

// CreateTestWithGraph inserts the test and its full question/option graph in a
// single transaction. Option IDs are assigned by the database and the answer
// key is written as the concrete option ID of the is_correct option. Any
// failure rolls the whole graph back.
func (r *Repository) CreateTestWithGraph(test *MockTest, seeds []QuestionSeed) error {
	// IsPublished has a DEFAULT TRUE tag, and GORM both omits zero values with
	// a database default and reads that default back into the struct. The
	// requested value is therefore captured first and an explicit draft is
	// written inside the same transaction.
	requested := test.IsPublished
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Questions", "Questions.Options").Create(test).Error; err != nil {
			return err
		}
		if !requested {
			if err := tx.Model(&MockTest{}).
				Where("id = ?", test.ID).
				UpdateColumn("is_published", false).Error; err != nil {
				return err
			}
		}
		test.IsPublished = requested
		return insertGraph(tx, test.ID, seeds)
	})
}

// ReplaceTestGraph swaps the question/option graph of an existing test
// atomically: old options and questions are removed, the parent scalars are
// saved, and the new graph is inserted. A failure at any point leaves the
// previous graph untouched.
func (r *Repository) ReplaceTestGraph(test *MockTest, seeds []QuestionSeed) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var questionIDs []uint
		if err := tx.Model(&MockQuestion{}).Where("mock_test_id = ?", test.ID).Pluck("id", &questionIDs).Error; err != nil {
			return err
		}
		if len(questionIDs) > 0 {
			if err := tx.Where("mock_question_id IN ?", questionIDs).Delete(&MockOption{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("mock_test_id = ?", test.ID).Delete(&MockQuestion{}).Error; err != nil {
			return err
		}
		if err := tx.Omit("Questions", "Questions.Options").Save(test).Error; err != nil {
			return err
		}
		return insertGraph(tx, test.ID, seeds)
	})
}

// insertGraph creates the questions and options of one test and links the
// answer key. It must be called inside a transaction.
func insertGraph(tx *gorm.DB, testID uint, seeds []QuestionSeed) error {
	for qi, seed := range seeds {
		question := MockQuestion{
			MockTestID:   testID,
			QuestionText: seed.Text,
			Explanation:  seed.Explanation,
			Order:        qi + 1,
		}
		// Insert the question first so options can reference it.
		if err := tx.Omit("Options").Create(&question).Error; err != nil {
			return err
		}

		correctOptionID := uint(0)
		options := make([]MockOption, 0, len(seed.Options))
		for oi, optionSeed := range seed.Options {
			option := MockOption{
				MockQuestionID: question.ID,
				OptionText:     optionSeed.Text,
				Order:          oi + 1,
			}
			if err := tx.Create(&option).Error; err != nil {
				return err
			}
			if optionSeed.IsCorrect {
				correctOptionID = option.ID
			}
			options = append(options, option)
		}
		if correctOptionID == 0 {
			return ErrNoCorrectOption
		}

		if err := tx.Model(&MockQuestion{}).
			Where("id = ?", question.ID).
			UpdateColumn("correct_option_id", correctOptionID).Error; err != nil {
			return err
		}

		question.CorrectOptionID = correctOptionID
		question.Options = options
	}
	return nil
}

// UpdateTestScalars saves only the parent row, leaving the graph alone.
func (r *Repository) UpdateTestScalars(test *MockTest) error {
	return r.db.Omit("Questions", "Questions.Options").Save(test).Error
}

// DeleteTest soft-deletes the test and hard-deletes its graph so a soft-delete
// can never leave orphaned options behind.
func (r *Repository) DeleteTest(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var questionIDs []uint
		if err := tx.Model(&MockQuestion{}).Where("mock_test_id = ?", id).Pluck("id", &questionIDs).Error; err != nil {
			return err
		}
		if len(questionIDs) > 0 {
			if err := tx.Where("mock_question_id IN ?", questionIDs).Delete(&MockOption{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("mock_test_id = ?", id).Delete(&MockQuestion{}).Error; err != nil {
			return err
		}
		return tx.Delete(&MockTest{}, id).Error
	})
}

func (r *Repository) IncrementViews(id uint) error {
	return r.db.Model(&MockTest{}).
		Where("id = ?", id).
		UpdateColumn("views", gorm.Expr("views + 1")).Error
}

// CreateAttempt stores a graded attempt and bumps the test attempt counter in
// one transaction.
func (r *Repository) CreateAttempt(attempt *MockAttempt) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(attempt).Error; err != nil {
			return err
		}
		return tx.Model(&MockTest{}).
			Where("id = ?", attempt.MockTestID).
			UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
	})
}

func (r *Repository) FindAttemptByID(id uint) (*MockAttempt, error) {
	var attempt MockAttempt
	if err := r.db.First(&attempt, id).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

package review

import (
	"errors"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(review *Review) error {
	return r.db.Create(review).Error
}

func (r *Repository) FindByID(id uint) (*Review, error) {
	var review Review
	err := r.db.Preload("User").First(&review, id).Error
	if err != nil {
		return nil, err
	}
	return &review, nil
}

func (r *Repository) FindByIDAndUser(id, userID uint) (*Review, error) {
	var review Review
	err := r.db.Preload("User").Where("id = ? AND user_id = ?", id, userID).First(&review).Error
	if err != nil {
		return nil, err
	}
	return &review, nil
}

func (r *Repository) FindByUser(userID uint, page, limit int) ([]Review, int64, error) {
	var reviews []Review
	var total int64

	if err := r.db.Model(&Review{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Preload("User").Where("user_id = ?", userID).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&reviews).Error
	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

func (r *Repository) FindByUniversity(universityID uint, page, limit int) ([]Review, int64, error) {
	var reviews []Review
	var total int64

	if err := r.db.Model(&Review{}).Where("university_id = ? AND is_published = ?", universityID, true).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Preload("User").Where("university_id = ? AND is_published = ?", universityID, true).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&reviews).Error
	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

func (r *Repository) FindAllByUniversity(universityID uint) ([]Review, error) {
	var reviews []Review
	err := r.db.Preload("User").Where("university_id = ? AND is_published = ?", universityID, true).
		Find(&reviews).Error
	return reviews, err
}

func (r *Repository) FindByUserAndUniversity(userID, universityID uint) (*Review, error) {
	var review Review
	err := r.db.Preload("User").Where("user_id = ? AND university_id = ?", userID, universityID).First(&review).Error
	if err != nil {
		return nil, err
	}
	return &review, nil
}

func (r *Repository) FindByCollege(collegeID uint, page, limit int) ([]Review, int64, error) {
	var reviews []Review
	var total int64

	if err := r.db.Model(&Review{}).Where("college_id = ? AND is_published = ?", collegeID, true).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Preload("User").Where("college_id = ? AND is_published = ?", collegeID, true).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&reviews).Error
	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

func (r *Repository) FindByInstitution(instID uint, page, limit int) ([]Review, int64, error) {
	var reviews []Review
	var total int64

	if err := r.db.Model(&Review{}).Where("institution_id = ? AND is_published = ?", instID, true).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Preload("User").Where("institution_id = ? AND is_published = ?", instID, true).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&reviews).Error
	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

func (r *Repository) FindAllByCollege(collegeID uint) ([]Review, error) {
	var reviews []Review
	err := r.db.Where("college_id = ? AND is_published = ?", collegeID, true).
		Find(&reviews).Error
	return reviews, err
}

func (r *Repository) FindAllByInstitution(instID uint) ([]Review, error) {
	var reviews []Review
	err := r.db.Where("institution_id = ? AND is_published = ?", instID, true).
		Find(&reviews).Error
	return reviews, err
}

func (r *Repository) Save(review *Review) error {
	return r.db.Save(review).Error
}

func (r *Repository) UpdateUniversityFields(review *Review, updates map[string]interface{}) error {
	result := r.db.Model(&Review{}).
		Where("id = ? AND user_id = ? AND university_id = ?", review.ID, review.UserID, review.UniversityID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Delete(id, userID uint) error {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&Review{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// VoteCountsForReviews returns per-review [upvotes, downvotes] derived from the votes table.
func (r *Repository) VoteCountsForReviews(reviewIDs []uint) (map[uint][2]int, error) {
	result := make(map[uint][2]int)
	if len(reviewIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		ReviewID uint
		Vote     string
		Count    int64
	}
	err := r.db.Model(&ReviewHelpful{}).
		Select("review_id, vote, COUNT(*) as count").
		Where("review_id IN ?", reviewIDs).
		Group("review_id, vote").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts := result[row.ReviewID]
		if row.Vote == "down" {
			counts[1] = int(row.Count)
		} else {
			counts[0] = int(row.Count)
		}
		result[row.ReviewID] = counts
	}
	return result, nil
}

func (r *Repository) UserVotesForReviews(userID uint, reviewIDs []uint) (map[uint]string, error) {
	result := make(map[uint]string)
	if len(reviewIDs) == 0 {
		return result, nil
	}
	var rows []ReviewHelpful
	err := r.db.Where("user_id = ? AND review_id IN ?", userID, reviewIDs).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ReviewID] = row.Vote
	}
	return result, nil
}

// ToggleVote records the user's vote; voting the same way again removes it.
// Returns the resulting vote ("" when removed).
func (r *Repository) ToggleVote(reviewID, userID uint, vote string) (string, error) {
	var existing ReviewHelpful
	err := r.db.Where("review_id = ? AND user_id = ?", reviewID, userID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return vote, r.db.Create(&ReviewHelpful{ReviewID: reviewID, UserID: userID, Vote: vote}).Error
		}
		return "", err
	}
	if existing.Vote == vote {
		return "", r.db.Delete(&existing).Error
	}
	return vote, r.db.Model(&existing).Update("vote", vote).Error
}

func (r *Repository) CreateReport(report *ReviewReport) error {
	return r.db.Create(report).Error
}

func (r *Repository) UpdateUniversityRating(universityID uint, avgRating float64, reviewCount int) error {
	return r.db.Table("universities").
		Where("id = ?", universityID).
		Updates(map[string]interface{}{
			"rating":       avgRating,
			"review_count": reviewCount,
		}).Error
}

// Admin methods for managing reviews
func (r *Repository) AdminDeleteReview(id uint) error {
	result := r.db.Delete(&Review{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) AdminGetAllReviews(page, limit int) ([]Review, int64, error) {
	var reviews []Review
	var total int64

	if err := r.db.Model(&Review{}).Where("university_id > 0").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Preload("User").Where("university_id > 0").
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&reviews).Error
	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

// Date report methods
func (r *Repository) CreateDateReport(report *DateReport) error {
	return r.db.Create(report).Error
}

func (r *Repository) FindDateReportByID(id uint) (*DateReport, error) {
	var report DateReport
	err := r.db.First(&report, id).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *Repository) GetAllDateReports(page, limit int) ([]DateReport, int64, error) {
	var reports []DateReport
	var total int64

	if err := r.db.Model(&DateReport{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&reports).Error
	if err != nil {
		return nil, 0, err
	}

	return reports, total, nil
}

func (r *Repository) UpdateDateReportStatus(id uint, status string) error {
	result := r.db.Model(&DateReport{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) DeleteDateReport(id uint) error {
	result := r.db.Delete(&DateReport{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) GetUniversityByID(id uint) (string, error) {
	var name string
	err := r.db.Table("universities").Where("id = ?", id).Pluck("name", &name).Error
	return name, err
}

func (r *Repository) UpdateCollegeRating(collegeID uint) error {
	if collegeID == 0 {
		return nil
	}
	var result struct {
		AvgRating  float64
		TotalCount int64
	}
	err := r.db.Table("reviews").
		Select("COALESCE(AVG((ratings->>'Overall Experience')::numeric), 0) as avg_rating, COUNT(*) as total_count").
		Where("college_id = ? AND is_published = ?", collegeID, true).
		Scan(&result).Error
	if err != nil {
		return err
	}
	return r.db.Table("colleges").
		Where("id = ?", collegeID).
		Updates(map[string]interface{}{
			"rating":  result.AvgRating,
			"reviews": result.TotalCount,
		}).Error
}

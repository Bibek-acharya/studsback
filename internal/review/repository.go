package review

import (
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
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&review).Error
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
	err := r.db.Where("user_id = ?", userID).
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
	err := r.db.Where("college_id = ? AND is_published = ?", collegeID, true).
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

func (r *Repository) Save(review *Review) error {
	return r.db.Save(review).Error
}

func (r *Repository) SaveByUserAndUniversity(review *Review) error {
	result := r.db.Where("id = ? AND user_id = ? AND university_id = ?", review.ID, review.UserID, review.UniversityID).Save(review)
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

func (r *Repository) HasMarkedHelpful(reviewID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&ReviewHelpful{}).
		Where("review_id = ? AND user_id = ?", reviewID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) MarkHelpful(reviewID, userID uint) error {
	entry := &ReviewHelpful{
		ReviewID: reviewID,
		UserID:   userID,
	}
	return r.db.Create(entry).Error
}

func (r *Repository) IncrementHelpfulCount(reviewID uint) error {
	return r.db.Model(&Review{}).Where("id = ?", reviewID).
		UpdateColumn("helpful_count", gorm.Expr("helpful_count + 1")).Error
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

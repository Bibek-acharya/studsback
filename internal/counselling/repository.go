package counselling

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByUserID(userID uint) ([]CounsellingBooking, error) {
	var bookings []CounsellingBooking
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&bookings).Error
	return bookings, err
}

func (r *Repository) Create(booking *CounsellingBooking) error {
	return r.db.Create(booking).Error
}

func (r *Repository) CheckDuplicateBooking(userID uint, sessionDate, sessionTime string) bool {
	var count int64
	r.db.Model(&CounsellingBooking{}).Where(
		"user_id = ? AND session_date = ? AND session_time = ?",
		userID, sessionDate, sessionTime,
	).Count(&count)
	return count > 0
}

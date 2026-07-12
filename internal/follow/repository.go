package follow

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

func (r *Repository) Follow(userID, institutionID uint) error {
	err := r.db.Create(&UserFollow{UserID: userID, InstitutionID: institutionID}).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil // already following, idempotent
		}
		return err
	}
	return nil
}

func (r *Repository) Unfollow(userID, institutionID uint) error {
	return r.db.Where("user_id = ? AND institution_id = ?", userID, institutionID).
		Delete(&UserFollow{}).Error
}

func (r *Repository) IsFollowing(userID, institutionID uint) (bool, error) {
	var count int64
	err := r.db.Model(&UserFollow{}).
		Where("user_id = ? AND institution_id = ?", userID, institutionID).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) GetFollowedInstitutionIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&UserFollow{}).Where("user_id = ?", userID).
		Pluck("institution_id", &ids).Error
	return ids, err
}

func (r *Repository) GetFollowerCount(institutionID uint) (int64, error) {
	var count int64
	err := r.db.Model(&UserFollow{}).Where("institution_id = ?", institutionID).
		Count(&count).Error
	return count, err
}

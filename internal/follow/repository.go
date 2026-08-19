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

func (r *Repository) Follow(userID, targetID uint, targetType string) error {
	if targetType == "" {
		targetType = "institution"
	}
	err := r.db.Create(&UserFollow{UserID: userID, TargetID: targetID, TargetType: targetType}).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return err
	}
	return nil
}

func (r *Repository) Unfollow(userID, targetID uint, targetType string) error {
	if targetType == "" {
		targetType = "institution"
	}
	return r.db.Where("user_id = ? AND target_id = ? AND target_type = ?", userID, targetID, targetType).
		Delete(&UserFollow{}).Error
}

func (r *Repository) IsFollowing(userID, targetID uint, targetType string) (bool, error) {
	if targetType == "" {
		targetType = "institution"
	}
	var count int64
	err := r.db.Model(&UserFollow{}).
		Where("user_id = ? AND target_id = ? AND target_type = ?", userID, targetID, targetType).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) GetFollowedTargetIDs(userID uint, targetType string) ([]uint, error) {
	if targetType == "" {
		targetType = "institution"
	}
	var ids []uint
	err := r.db.Model(&UserFollow{}).Where("user_id = ? AND target_type = ?", userID, targetType).
		Pluck("target_id", &ids).Error
	return ids, err
}

func (r *Repository) GetFollowerCount(targetID uint, targetType string) (int64, error) {
	if targetType == "" {
		targetType = "institution"
	}
	var count int64
	err := r.db.Model(&UserFollow{}).Where("target_id = ? AND target_type = ?", targetID, targetType).
		Count(&count).Error
	return count, err
}

package follow

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Follow creates the follow row. Returns created=false when the user already
// follows the target (duplicate insert is not an error).
func (r *Repository) Follow(userID, targetID uint, targetType string) (bool, error) {
	if targetType == "" {
		targetType = "institution"
	}
	err := r.db.Create(&UserFollow{UserID: userID, TargetID: targetID, TargetType: targetType}).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || isDuplicateFollowError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isDuplicateFollowError(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "duplicate key") ||
		strings.Contains(lower, "unique constraint") ||
		strings.Contains(lower, "unique violation")
}

// ApprovedInstitutionUserID resolves the institution account that claimed the
// college (D-Q13: silent when none exists). institution_users is owned by the
// institution module; accessed via raw SQL to avoid an import cycle.
func (r *Repository) ApprovedInstitutionUserID(collegeID uint) (uint, error) {
	var id uint
	err := r.db.Raw(
		`SELECT id FROM institution_users WHERE college_id = ? AND status = 'approved' AND deleted_at IS NULL LIMIT 1`,
		collegeID,
	).Scan(&id).Error
	return id, err
}

// UserNameByID resolves a display name from the users table (owned by the
// auth module; accessed via raw SQL to avoid an import cycle).
func (r *Repository) UserNameByID(userID uint) (string, error) {
	var row struct{ FirstName, LastName string }
	err := r.db.Raw(
		`SELECT first_name, last_name FROM users WHERE id = ? AND deleted_at IS NULL LIMIT 1`,
		userID,
	).Scan(&row).Error
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.Join([]string{row.FirstName, row.LastName}, " ")), nil
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

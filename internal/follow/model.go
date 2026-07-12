package follow

import "time"

type UserFollow struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index:idx_follow_user_institution,unique;not null" json:"user_id"`
	InstitutionID uint      `gorm:"index:idx_follow_user_institution,unique;not null" json:"institution_id"`
	CreatedAt     time.Time `json:"created_at"`
}

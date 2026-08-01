package follow

import "time"

type UserFollow struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index:idx_follow_user_target,unique;not null" json:"user_id"`
	TargetID      uint      `gorm:"index:idx_follow_user_target,unique;not null" json:"target_id"`
	TargetType    string    `gorm:"index:idx_follow_user_target,unique;not null;default:'institution'" json:"target_type"`
	CreatedAt     time.Time `json:"created_at"`
}

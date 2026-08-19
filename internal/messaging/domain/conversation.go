package domain

import "time"

type Conversation struct {
    ID                 uint       `json:"id" gorm:"primaryKey"`
    CreatedAt          time.Time  `json:"created_at"`
    UpdatedAt          time.Time  `json:"updated_at"`
    StudentID          uint       `json:"student_id" gorm:"index"`
    InstitutionID      uint       `json:"institution_id" gorm:"index"`
    LastMessageID      *uint      `json:"last_message_id"`
    LastMessageAt      *time.Time `json:"last_message_at" gorm:"index"`
    LastMessagePreview string     `json:"last_message_preview" gorm:"size:255"`
}

func (Conversation) TableName() string {
    return "conversations"
}

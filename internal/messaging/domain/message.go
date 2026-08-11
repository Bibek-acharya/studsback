package domain

import "time"

type Message struct {
    ID               uint       `json:"id" gorm:"primaryKey"`
    CreatedAt        time.Time  `json:"created_at"`
    ConversationID   uint       `json:"conversation_id" gorm:"index;index:idx_messages_conversation"`
    SenderType       string     `json:"sender_type" gorm:"size:20;not null"`
    SenderID         uint       `json:"sender_id" gorm:"not null;index:idx_messages_sender"`
    ClientMessageID  string     `json:"client_message_id" gorm:"size:36;not null"`
    Content          string     `json:"content" gorm:"type:text"`
    EditedAt         *time.Time `json:"edited_at"`
    DeletedAt        *time.Time `json:"deleted_at"`
    Attachments      []Attachment `json:"attachments" gorm:"-"`
}

func (Message) TableName() string {
    return "messages"
}

type SenderType string

const (
    SenderTypeStudent     SenderType = "student"
    SenderTypeInstitution SenderType = "institution"
)

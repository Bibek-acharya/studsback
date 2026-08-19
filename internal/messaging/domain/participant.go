package domain

import "time"

type Participant struct {
    ID                 uint       `json:"id" gorm:"primaryKey"`
    ConversationID     uint       `json:"conversation_id" gorm:"index"`
    ParticipantType    string     `json:"participant_type" gorm:"size:20;not null"`
    ParticipantID      uint       `json:"participant_id" gorm:"not null"`
    LastReadMessageID  *uint      `json:"last_read_message_id"`
    LastReadAt         *time.Time `json:"last_read_at"`
    UnreadCount        int        `json:"unread_count" gorm:"default:0"`
    IsTyping           bool       `json:"is_typing" gorm:"default:false"`
    TypingAt           *time.Time `json:"typing_at"`
}

func (Participant) TableName() string {
    return "conversation_participants"
}

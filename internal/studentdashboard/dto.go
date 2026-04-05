package studentdashboard

import "time"

type MessageRequest struct {
	ReceiverID uint   `json:"receiver_id" binding:"required"`
	Subject    string `json:"subject" binding:"required"`
	Content    string `json:"content" binding:"required"`
}

type MessageResponse struct {
	ID         uint      `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	SenderID   uint      `json:"sender_id"`
	ReceiverID uint      `json:"receiver_id"`
	Subject    string    `json:"subject"`
	Content    string    `json:"content"`
	Read       bool      `json:"read"`
	Direction  string    `json:"direction"`
}

type MessageReplyRequest struct {
	Content string `json:"content" binding:"required"`
}

type ContactResponse struct {
	UserID      uint   `json:"user_id"`
	Name        string `json:"name"`
	LastMessage string `json:"last_message"`
	Unread      int    `json:"unread"`
}

type CalendarEventRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	StartDate   string `json:"start_date" binding:"required"`
	EndDate     string `json:"end_date"`
	Location    string `json:"location"`
	Link        string `json:"link"`
	Color       string `json:"color"`
	Reminder    bool   `json:"reminder"`
}

type CalendarEventUpdateRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Location    string `json:"location"`
	Link        string `json:"link"`
	Color       string `json:"color"`
	Reminder    *bool  `json:"reminder"`
}

type CalendarEventResponse struct {
	ID          uint      `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UserID      uint      `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	Location    string    `json:"location"`
	Link        string    `json:"link"`
	Color       string    `json:"color"`
	Reminder    bool      `json:"reminder"`
}

type BookmarkRequest struct {
	ItemID   uint   `json:"item_id" binding:"required"`
	ItemType string `json:"type" binding:"required"`
}

type BookmarkResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UserID    uint      `json:"user_id"`
	ItemID    uint      `json:"item_id"`
	ItemType  string    `json:"type"`
}

type NotificationResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    uint      `json:"user_id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Type      string    `json:"type"`
	Read      bool      `json:"read"`
	Link      string    `json:"link"`
}

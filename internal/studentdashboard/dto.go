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

type DashboardStats struct {
	ApplicationsSubmitted int `json:"applications_submitted"`
	SavedColleges         int `json:"saved_colleges"`
	SavedScholarships     int `json:"saved_scholarships"`
	ScholarshipsApplied   int `json:"scholarships_applied"`
	ActiveInvites         int `json:"active_invites"`
	UnreadMessages        int `json:"unread_messages"`
	UpcomingDeadlines     int `json:"upcoming_deadlines"`
	ProfileCompletion     int `json:"profile_completion"`
}

type RecentApplication struct {
	ID          uint   `json:"id"`
	Institution string `json:"institution"`
	Program     string `json:"program"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	UpdatedAt   string `json:"updated_at"`
}

type MyApplication struct {
	ID          uint   `json:"id"`
	Institution string `json:"institution"`
	Program     string `json:"program"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	AppliedDate string `json:"applied_date"`
	Deadline    string `json:"deadline"`
	Location    string `json:"location"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
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
	Type        string `json:"type"`
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
	Type        string `json:"type"`
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
	Type        string    `json:"type"`
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

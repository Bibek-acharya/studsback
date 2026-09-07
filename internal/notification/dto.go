// internal/notification/dto.go
package notification

type NotificationItem struct {
	ID         uint    `json:"id"`
	EventKey   string  `json:"event_key"`
	Category   string  `json:"category"`
	Priority   string  `json:"priority"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	Link       string  `json:"link"`
	Data       any     `json:"data"`
	ReadAt     *string `json:"read_at"`
	ArchivedAt *string `json:"archived_at"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	// Transition envelope (doc 05 §2.2) — legacy fields synthesized:
	Type       string `json:"type"`
	Read       bool   `json:"read"`
	UserID     uint   `json:"user_id,omitempty"`
	ProviderID uint   `json:"provider_id,omitempty"`
	Message    string `json:"message"` // = body
}

type InboxMeta struct {
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

type InboxResponse struct {
	Notifications []NotificationItem `json:"notifications"`
	UnreadCount   int                `json:"unread_count"`
	Meta          InboxMeta          `json:"meta"`
}

type UnreadResponse struct {
	UnreadCount int64 `json:"unread_count"`
}

type BulkReadResponse struct {
	Updated int64 `json:"updated"`
}

type ProviderNotificationItem struct {
	ID         uint   `json:"id"`
	ProviderID uint   `json:"provider_id"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	Type       string `json:"type"`
	Read       bool   `json:"read"`
	Link       string `json:"link"`
	CreatedAt  string `json:"created_at"`
}

type ProviderListResponse struct {
	Notifications []ProviderNotificationItem `json:"notifications"`
	UnreadCount   int                        `json:"unread_count"`
	Meta          InboxMeta                  `json:"meta"`
}

// PublicNotificationResponse is the public banner shape. The system module
// aliases this type so the guest GET keeps serving the identical JSON.
type PublicNotificationResponse struct {
	ID        uint   `json:"id"`
	CreatedAt string `json:"created_at"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	Link      string `json:"link"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	BgColor   string `json:"bg_color"`
}

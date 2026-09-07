// internal/notification/model.go
package notification

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AccountNotification struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	AccountType string `gorm:"size:20;not null" json:"account_type"` // user | institution | provider
	AccountID   uint   `gorm:"not null" json:"account_id"`

	EventKey string `gorm:"size:100;not null" json:"event_key"`
	Category string `gorm:"size:40;not null" json:"category"`
	Priority string `gorm:"size:10;not null;default:'normal'" json:"priority"`

	Title string         `gorm:"size:200;not null" json:"title"`
	Body  string         `gorm:"type:text" json:"body"`
	Link  string         `gorm:"size:500" json:"link"`
	Data  datatypes.JSON `json:"data"`

	ReadAt     *time.Time `json:"read_at"`
	ArchivedAt *time.Time `json:"archived_at"`

	OccurrenceKey string `gorm:"size:200" json:"occurrence_key,omitempty"`
	BroadcastID   *uint  `json:"broadcast_id,omitempty"`
	ActorType     string `gorm:"size:20" json:"actor_type,omitempty"`
	ActorID       uint   `json:"actor_id,omitempty"`
	CorrelationID string `gorm:"size:64" json:"correlation_id,omitempty"`

	LegacySource string `gorm:"size:20" json:"legacy_source,omitempty"`
	LegacyID     *uint  `json:"legacy_id,omitempty"`
}

func (AccountNotification) TableName() string { return "account_notifications" }

type NotificationOutbox struct {
	ID             uint `gorm:"primarykey"`
	CreatedAt      time.Time
	AvailableAt    time.Time
	ClaimedAt      *time.Time
	ClaimToken     string         `gorm:"size:64"`
	LeaseExpiresAt *time.Time     `gorm:"index"`
	Kind           string         `gorm:"size:20;not null"` // dispatch | fanout | anonymous_email
	Payload        datatypes.JSON `gorm:"type:jsonb;not null"`
	OccurrenceKey  string         `gorm:"size:200"`
	Attempts       int            `gorm:"default:0"`
	LastError      string         `gorm:"type:text"`
	Done           bool           `gorm:"index"`
}

func (NotificationOutbox) TableName() string { return "notification_outbox" }

type NotificationBroadcast struct {
	ID               uint `gorm:"primarykey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Title            string         `gorm:"size:200;not null"`
	Body             string         `gorm:"type:text;not null"`
	Link             string         `gorm:"size:500"`
	Priority         string         `gorm:"size:10;not null;default:'critical'"`
	Audience         datatypes.JSON `gorm:"type:jsonb;not null"`
	AudienceCriteria datatypes.JSON `gorm:"type:jsonb"`
	Status           string         `gorm:"size:20;not null;default:'sending'"` // sending|completed|failed|cancelled
	TotalCount       int            `gorm:"default:0"`
	SentCount        int            `gorm:"default:0"`
	FailedCount      int            `gorm:"default:0"`
	CreatedBy        uint           `gorm:"not null"`
	IdempotencyKey   string         `gorm:"size:100"`
}

type NotificationDedupeLease struct {
	ID              uint `gorm:"primarykey"`
	CreatedAt       time.Time
	AccountType     string `gorm:"size:20;not null"`
	AccountID       uint   `gorm:"not null"`
	DedupeKey       string `gorm:"size:200;not null"`
	NotificationID  *uint
	ExpiresAt       time.Time `gorm:"index"`
	SupersededCount int       `gorm:"default:0"`
}

// PublicNotification is the public banner (system notifications) model.
// Moved here from internal/system (Task 13); system references it via a type
// alias. No TableName method — GORM's default pluralization keeps the
// historical public_notifications table.
type PublicNotification struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Title     string         `gorm:"not null" json:"title"`
	Message   string         `gorm:"type:text" json:"message"`
	Type      string         `gorm:"default:'info'" json:"type"`
	Link      string         `json:"link"`
	Active    bool           `gorm:"default:true;index" json:"active"`
	Icon      string         `json:"icon"`
	Color     string         `json:"color"`
	BgColor   string         `json:"bg_color"`
}

package domain

import "time"

type Attachment struct {
    ID           uint      `json:"id" gorm:"primaryKey"`
    CreatedAt    time.Time `json:"created_at"`
    MessageID    uint      `json:"message_id" gorm:"index"`
    UploaderType string    `json:"uploader_type" gorm:"size:20;not null"`
    UploaderID   uint      `json:"uploader_id" gorm:"not null"`
    FileName     string    `json:"file_name" gorm:"size:255;not null"`
    FileSize     int64     `json:"file_size" gorm:"not null"`
    FileType     string    `json:"file_type" gorm:"size:50;not null"`
    StorageKey   string    `json:"storage_key" gorm:"size:500;not null"`
    ThumbnailKey string    `json:"thumbnail_key" gorm:"size:500"`
}

func (Attachment) TableName() string {
    return "message_attachments"
}

type PendingUpload struct {
    ID           uint      `json:"id" gorm:"primaryKey"`
    CreatedAt    time.Time `json:"created_at"`
    UploaderType string    `json:"uploader_type" gorm:"size:20;not null"`
    UploaderID   uint      `json:"uploader_id" gorm:"not null"`
    FileName     string    `json:"file_name" gorm:"size:255;not null"`
    FileSize     int64     `json:"file_size" gorm:"not null"`
    FileType     string    `json:"file_type" gorm:"size:50;not null"`
    StorageKey   string    `json:"storage_key" gorm:"size:500;not null"`
    ThumbnailKey string    `json:"thumbnail_key" gorm:"size:500"`
    MessageID    *uint     `json:"message_id"`
    ExpiresAt    time.Time `json:"expires_at" gorm:"not null"`
}

func (PendingUpload) TableName() string {
    return "pending_uploads"
}

type OutboxEvent struct {
    ID            uint       `json:"id" gorm:"primaryKey"`
    CreatedAt     time.Time  `json:"created_at"`
    AggregateType string     `json:"aggregate_type" gorm:"size:50;not null"`
    AggregateID   uint       `json:"aggregate_id" gorm:"not null"`
    EventType     string     `json:"event_type" gorm:"size:100;not null"`
    Payload       string     `json:"payload" gorm:"type:jsonb;not null"`
    Published     bool       `json:"published" gorm:"default:false"`
    PublishedAt   *time.Time `json:"published_at"`
    RetryCount    int        `json:"retry_count" gorm:"default:0"`
    MaxRetries    int        `json:"max_retries" gorm:"default:5"`
}

func (OutboxEvent) TableName() string {
    return "outbox_events"
}

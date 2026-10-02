package studyresources

import (
	"time"

	"gorm.io/gorm"
)

type StudyResource struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	Title        string `gorm:"not null" json:"title"`
	Description  string `gorm:"default:''" json:"description"`
	ResourceType string `gorm:"not null;default:'';index:idx_study_resources_resource_type" json:"resource_type"`
	Course       string `gorm:"default:'';index:idx_study_resources_course" json:"course"`
	Year         string `gorm:"default:'';index:idx_study_resources_year" json:"year"`
	FileName     string `gorm:"not null" json:"file_name"`
	FilePath     string `gorm:"not null" json:"file_path"`
	FileURL      string `gorm:"not null" json:"file_url"`
	FileSize     int64  `gorm:"not null;default:0" json:"file_size"`
	MimeType     string `gorm:"default:''" json:"mime_type"`
	Downloads    int    `gorm:"not null;default:0" json:"downloads"`
	// Views counts video-lecture stream requests. Documents keep using
	// Downloads; the counter is incremented by the stream endpoint only.
	Views int `gorm:"not null;default:0" json:"views"`
	// IsPublished defaults to TRUE so every pre-existing row stays publicly
	// visible after the column is added. Public list filters published rows
	// only; the admin list sees everything.
	IsPublished bool `gorm:"not null;default:true;index:idx_study_resources_published" json:"is_published"`
	// DurationSeconds is the optional video length used by the player
	// before metadata is loaded. Zero means "unknown".
	DurationSeconds int  `gorm:"not null;default:0" json:"duration_seconds"`
	UploadedBy      uint `gorm:"not null;default:0" json:"uploaded_by"`
	// ApprovalStatus is the moderation state: pending_review, approved or
	// rejected. See approval.go — the rule that matters is that coins move on
	// publication and never on upload.
	//
	// It DEFAULTS to 'approved', deliberately, and that is not a claim that new
	// uploads skip moderation. It is so every PRE-EXISTING row — uploaded
	// through the long-standing admin route, which has no queue — keeps its
	// published state and does not become invisible to the public list the moment
	// this column is added. A student submission goes through
	// ApprovalService.Submit, which sets pending_review explicitly.
	//
	// is_published and this column are deliberately NOT the same thing. An admin
	// may unpublish an approved resource to take it down; that does not make it
	// pending again or un-approve it, and it does not refund anything.
	ApprovalStatus string `gorm:"not null;default:'approved';index:idx_study_resources_approval_status" json:"approval_status"`
	// ReviewedBy is the MODERATOR who decided, which is not the uploader
	// (UploadedBy). 04 §5.3 requires reviewer identity so "who approved this" is
	// answerable after the fact. NULL for a row that was never moderated.
	ReviewedBy *uint `gorm:"index" json:"reviewed_by,omitempty"`
	// ReviewedAt is when the decision was made.
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	// RejectReason is set only on a rejection, and is required there: it is the
	// only thing the student can act on. See ErrRejectReasonRequired.
	RejectReason string         `gorm:"type:text" json:"reject_reason,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

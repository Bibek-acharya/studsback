package downloadcenter

import "time"

// CreateDownloadItemInput is the multipart form input for creating a download
// item. The file is required; the remaining fields come via c.PostForm.
type CreateDownloadItemInput struct {
	Title       string `form:"title" binding:"required"`
	Description string `form:"description"`
	Category    string `form:"category"`
	IsPublished bool   `form:"is_published"`
	PublishedAt *time.Time `form:"published_at" time_format:"2006-01-02T15:04:05Z07:00"`
}

// UpdateDownloadItemInput supports partial metadata updates via JSON, plus an
// optional replacement file via multipart (metadata fields use form binding).
type UpdateDownloadItemInput struct {
	Title       *string    `json:"title" form:"title"`
	Description *string    `json:"description" form:"description"`
	Category    *string    `json:"category" form:"category"`
	IsPublished *bool      `json:"is_published" form:"is_published"`
	PublishedAt *time.Time `json:"published_at" form:"published_at" time_format:"2006-01-02T15:04:05Z07:00"`
}

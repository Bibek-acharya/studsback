package pressmedia

import "time"

type CreatePressMediaItemInput struct {
	Title       string     `json:"title" binding:"required"`
	Slug        string     `json:"slug"`
	Category    string     `json:"category" binding:"required"`
	Summary     string     `json:"summary"`
	Content     string     `json:"content"`
	ImageURL    string     `json:"image_url"`
	FileURL     string     `json:"file_url"`
	ExternalURL string     `json:"external_url"`
	PublishedAt *time.Time `json:"published_at"`
	IsPublished bool       `json:"is_published"`
}

type UpdatePressMediaItemInput struct {
	Title       *string    `json:"title"`
	Slug        *string    `json:"slug"`
	Category    *string    `json:"category"`
	Summary     *string    `json:"summary"`
	Content     *string    `json:"content"`
	ImageURL    *string    `json:"image_url"`
	FileURL     *string    `json:"file_url"`
	ExternalURL *string    `json:"external_url"`
	PublishedAt *time.Time `json:"published_at"`
	IsPublished *bool      `json:"is_published"`
}

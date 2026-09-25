package studyresources

type CreateResourceRequest struct {
	Title        string `form:"title" binding:"required"`
	ResourceType string `form:"type" binding:"required"`
	Course       string `form:"course"`
	Year         string `form:"year"`
	Description  string `form:"description"`
	// DurationSeconds is optional and only meaningful for video lectures.
	DurationSeconds *int  `form:"duration_seconds"`
	IsPublished     *bool `form:"is_published"`
}

type UpdateResourceRequest struct {
	Title           *string `json:"title"`
	ResourceType    *string `json:"resource_type"`
	Course          *string `json:"course"`
	Year            *string `json:"year"`
	Description     *string `json:"description"`
	DurationSeconds *int    `json:"duration_seconds"`
	IsPublished     *bool   `json:"is_published"`
}

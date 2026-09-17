package studyresources

type CreateResourceRequest struct {
	Title        string `form:"title" binding:"required"`
	ResourceType string `form:"type" binding:"required"`
	Course       string `form:"course"`
	Year         string `form:"year"`
	Description  string `form:"description"`
}

type UpdateResourceRequest struct {
	Title        *string `json:"title"`
	ResourceType *string `json:"resource_type"`
	Course       *string `json:"course"`
	Year         *string `json:"year"`
	Description  *string `json:"description"`
}

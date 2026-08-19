package feedback

type CreateFeedbackRequest struct {
	Rating      int    `json:"rating" binding:"required,min=1,max=5"`
	Experience  string `json:"experience" binding:"required"`
	Designation string `json:"designation"`
	Email       string `json:"email"`
}

type CreateTestimonialRequest struct {
	Name        string `json:"name"`
	Designation string `json:"designation" binding:"required"`
	Rating      int    `json:"rating" binding:"required,min=1,max=5"`
	Review      string `json:"review" binding:"required"`
}

type FeedbackResponse struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"user_id"`
	UserName   string `json:"user_name"`
	Role       string `json:"role"`
	ImageURL   string `json:"image_url"`
	Rating     int    `json:"rating"`
	Experience string `json:"experience"`
	Email      string `json:"email"`
	CreatedAt  string `json:"created_at"`
}

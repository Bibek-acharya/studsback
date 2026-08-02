package review

type CreateReviewRequest struct {
	CollegeID         uint               `json:"college_id" binding:"required"`
	CollegeName       string             `json:"college_name"`
	StudentType       string             `json:"student_type" binding:"required,oneof=current alumni"`
	Course            string             `json:"course" binding:"required"`
	Level             string             `json:"level" binding:"required"`
	BatchYear         int                `json:"batch_year" binding:"required"`
	Ratings           map[string]float64 `json:"ratings" binding:"required"`
	Pros              string             `json:"pros" binding:"required,min=10"`
	Cons              string             `json:"cons" binding:"required,min=10"`
	SummaryTitle      string             `json:"summary_title" binding:"required,min=5"`
	YearlyFee         *float64           `json:"yearly_fee"`
	Scholarship       *bool              `json:"scholarship"`
	InternshipOutcome *string            `json:"internship_outcome"`
	Email             string             `json:"email" binding:"required,email"`
}

type UpdateReviewRequest struct {
	Pros         *string             `json:"pros"`
	Cons         *string             `json:"cons"`
	SummaryTitle *string             `json:"summary_title"`
	Ratings      *map[string]float64 `json:"ratings"`
}

type ReportReviewRequest struct {
	Reason string `json:"reason" binding:"required,min=10"`
}

type ReviewResponse struct {
	ID                uint               `json:"id"`
	CollegeID         uint               `json:"college_id"`
	UniversityID      uint               `json:"university_id"`
	CollegeName       string             `json:"college_name"`
	UserID            uint               `json:"user_id"`
	UserName          string             `json:"user_name"`
	UserInitials      string             `json:"user_initials"`
	StudentType       string             `json:"student_type"`
	Course            string             `json:"course"`
	Level             string             `json:"level"`
	BatchYear         int                `json:"batch_year"`
	Ratings           map[string]float64 `json:"ratings"`
	Pros              string             `json:"pros"`
	Cons              string             `json:"cons"`
	SummaryTitle      string             `json:"summary_title"`
	YearlyFee         *float64           `json:"yearly_fee"`
	Scholarship       *bool              `json:"scholarship"`
	InternshipOutcome *string            `json:"internship_outcome"`
	Email             string             `json:"email"`
	IsVerified        bool               `json:"is_verified"`
	IsPublished       bool               `json:"is_published"`
	HelpfulCount      int                `json:"helpful_count"`
	CreatedAt         string             `json:"created_at"`
	UpdatedAt         string             `json:"updated_at"`
}

type PaginatedReviewsResponse struct {
	Reviews []ReviewResponse `json:"reviews"`
	Meta    Meta             `json:"meta"`
}

type CollegeReviewsResponse struct {
	Reviews          []ReviewResponse   `json:"reviews"`
	OverallRating    float64            `json:"overall_rating"`
	ReviewCount      int                `json:"review_count"`
	CategoryAverages map[string]float64 `json:"category_averages"`
	Meta             Meta               `json:"meta"`
}

type CreateUniversityReviewRequest struct {
	UniversityID uint    `json:"university_id" binding:"required" validate:"required"`
	Rating       float64 `json:"rating" binding:"required,min=1,max=5" validate:"required,min=1,max=5"`
	Pros         string  `json:"pros" binding:"required,min=10" validate:"required,min=10"`
	Cons         string  `json:"cons" binding:"required,min=10" validate:"required,min=10"`
}

type UpdateUniversityReviewRequest struct {
	Rating *float64 `json:"rating" binding:"omitempty,min=1,max=5" validate:"omitempty,min=1,max=5"`
	Pros   *string  `json:"pros" binding:"omitempty,min=10" validate:"omitempty,min=10"`
	Cons   *string  `json:"cons" binding:"omitempty,min=10" validate:"omitempty,min=10"`
}

type UniversityReviewsResponse struct {
	Reviews       []ReviewResponse `json:"reviews"`
	OverallRating float64          `json:"overall_rating"`
	ReviewCount   int              `json:"review_count"`
	Distribution  map[int]int      `json:"distribution"`
	Meta          Meta             `json:"meta"`
}

type Meta struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"total_pages"`
}

type CreateDateReportRequest struct {
	UniversityID uint   `form:"university_id" binding:"required"`
	Contact      string `form:"contact" binding:"required,len=10"`
	Feedback     string `form:"feedback" binding:"required,min=20"`
}

type UpdateDateReportRequest struct {
	Status string `json:"status" binding:"required,oneof=resolved dismissed"`
}

type DateReportResponse struct {
	ID             uint   `json:"id"`
	UniversityID   uint   `json:"university_id"`
	UniversityName string `json:"university_name"`
	Contact        string `json:"contact"`
	Feedback       string `json:"feedback"`
	FileURL        string `json:"file_url"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
}

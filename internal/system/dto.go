package system

import "studsphere/backend/internal/notification"

type ContactInquiryRequest struct {
	Name          string `json:"name" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
	Phone         string `json:"phone"`
	Subject       string `json:"subject" binding:"required"`
	Message       string `json:"message" binding:"required"`
	Type          string `json:"type"`
	InstitutionID *uint  `json:"institution_id"`
}

type ContactInquiryStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type AdRequest struct {
	Title       string `json:"title" binding:"required"`
	ImageURL    string `json:"image_url"`
	LinkURL     string `json:"link_url" binding:"required"`
	Location    string `json:"location"`
	Page        string `json:"page" binding:"required"`
	Position    string `json:"position"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Active      *bool  `json:"active"`
	Priority    int    `json:"priority"`
	CollegeID   *uint  `json:"college_id"`
	CourseID    *uint  `json:"course_id"`
	Description string `json:"description"`
	Accent      string `json:"accent"`
}

type CarouselSlideRequest struct {
	Page        string `json:"page"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	LinkURL     string `json:"link_url"`
	ButtonText  string `json:"button_text"`
	Order       int    `json:"order"`
	Active      *bool  `json:"active"`
}

type CarouselReorderItem struct {
	ID    uint `json:"id" binding:"required"`
	Order int  `json:"order" binding:"required"`
}

type CarouselReorderRequest struct {
	Slides []CarouselReorderItem `json:"slides" binding:"required"`
}

type ContactInquiryResponse struct {
	ID            uint   `json:"id"`
	InstitutionID *uint  `json:"institution_id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Subject       string `json:"subject"`
	Message       string `json:"message"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type AdResponse struct {
	ID                uint    `json:"id"`
	Title             string  `json:"title"`
	ImageURL          string  `json:"image_url"`
	LinkURL           string  `json:"link_url"`
	Location          string  `json:"location"`
	Page              string  `json:"page"`
	Position          string  `json:"position"`
	StartDate         string  `json:"start_date"`
	EndDate           string  `json:"end_date"`
	Active            bool    `json:"active"`
	Clicks            int     `json:"clicks"`
	Impressions       int     `json:"impressions"`
	Priority          int     `json:"priority"`
	CollegeID         *uint   `json:"college_id"`
	CourseID          *uint   `json:"course_id"`
	Description       string  `json:"description"`
	Accent            string  `json:"accent"`
	CollegeName       string  `json:"college_name,omitempty"`
	CollegeImage      string  `json:"college_image,omitempty"`
	CollegeRating     float64 `json:"college_rating,omitempty"`
	CollegeLocation   string  `json:"college_location,omitempty"`
	CollegeWebsite    string  `json:"college_website,omitempty"`
	CourseTitle       string  `json:"course_title,omitempty"`
	CourseLevel       string  `json:"course_level,omitempty"`
	CourseDuration    string  `json:"course_duration,omitempty"`
	CourseField       string  `json:"course_field,omitempty"`
	CourseBannerURL   string  `json:"course_banner_url,omitempty"`
	CourseEstFee      string  `json:"course_est_fee,omitempty"`
	CourseAffiliation string  `json:"course_affiliation,omitempty"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

type CarouselSlideResponse struct {
	ID          uint   `json:"id"`
	Page        string `json:"page"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	LinkURL     string `json:"link_url"`
	ButtonText  string `json:"button_text"`
	Order       int    `json:"order"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// PublicNotificationResponse moved to internal/notification (Task 13); the
// alias keeps the guest GET's response shape byte-identical.
type PublicNotificationResponse = notification.PublicNotificationResponse

// Landing Course DTOs
type LandingCourseFieldResponse struct {
	ID           uint   `json:"id"`
	FieldOfStudy string `json:"field_of_study"`
	DisplayOrder int    `json:"display_order"`
	IsActive     bool   `json:"is_active"`
}

type LandingCourseInstitutionResponse struct {
	ID              uint   `json:"id"`
	FieldID         uint   `json:"field_id"`
	InstitutionID   uint   `json:"institution_id"`
	InstitutionType string `json:"institution_type"`
	InstitutionName string `json:"institution_name"`
	InstitutionLogo string `json:"institution_logo"`
	Slug            string `json:"slug"`
	OrderIndex      int    `json:"order_index"`
}

type LandingCoursePublicResponse struct {
	FieldOfStudy string                             `json:"field_of_study"`
	Institutions []LandingCourseInstitutionResponse `json:"institutions"`
}

// Admin variant: adds the field's own columns for full admin editing.
type LandingCourseAdminFieldResponse struct {
	ID           uint                               `json:"id"`
	FieldOfStudy string                             `json:"field_of_study"`
	DisplayOrder int                                `json:"display_order"`
	IsActive     bool                               `json:"is_active"`
	Institutions []LandingCourseInstitutionResponse `json:"institutions"`
}

type LinkInstitutionRequest struct {
	FieldID         uint   `json:"field_id" binding:"required"`
	InstitutionID   uint   `json:"institution_id" binding:"required"`
	InstitutionType string `json:"institution_type"`
	InstitutionName string `json:"institution_name"`
	InstitutionLogo string `json:"institution_logo"`
	Slug            string `json:"slug"`
}

type ReorderFieldsRequest struct {
	Items []FieldReorderItem `json:"items" binding:"required"`
}

type FieldReorderItem struct {
	ID           uint `json:"id" binding:"required"`
	DisplayOrder int  `json:"display_order" binding:"required"`
}

type UpdateFieldRequest struct {
	FieldOfStudy *string `json:"field_of_study"`
	IsActive     *bool   `json:"is_active"`
	DisplayOrder *int    `json:"display_order"`
}

type ReorderInstitutionsRequest struct {
	Items []InstitutionReorderItem `json:"items" binding:"required"`
}

type InstitutionReorderItem struct {
	ID         uint `json:"id" binding:"required"`
	OrderIndex int  `json:"order_index" binding:"required"`
}

type InstitutionSearchResult struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	LogoURL  string `json:"logo_url"`
	Type     string `json:"type"`
	Slug     string `json:"slug"`
	Location string `json:"location"`
}

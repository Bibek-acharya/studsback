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

// CarouselSlideRequest is the create/update payload for a carousel slide.
//
// The pointer fields are optional but clearable: on update a nil pointer means
// the key was omitted (leave the stored column alone), while a non-nil pointer
// is applied verbatim — including an empty string, which clears the column.
// The JSON wire format is unchanged; only the Go shape differs.
type CarouselSlideRequest struct {
	Page        string  `json:"page"`
	Title       string  `json:"title"`
	Subtitle    *string `json:"subtitle"`
	Description *string `json:"description"`
	ImageURL    string  `json:"image_url"`
	LinkURL     *string `json:"link_url"`
	ButtonText  *string `json:"button_text"`
	Order       int     `json:"order"`
	Active      *bool   `json:"active"`
}

// derefString resolves an optional request field for create. A nil pointer
// (key omitted) and a pointer to an empty string both store the zero value,
// which is the pre-existing create behavior.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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

// Course-finder ad card DTOs

type CourseAdMouInput struct {
	Name       string `json:"name"`
	LogoURL    string `json:"logo_url"`
	CompanyURL string `json:"company_url"`
}

type CourseAdRequest struct {
	Position       string             `json:"position" binding:"required"`
	CourseID       uint               `json:"course_id" binding:"required"`
	InstitutionID  *uint              `json:"institution_id"`
	Subtitle       string             `json:"subtitle"`
	InstitutionIDs []uint             `json:"institution_ids"`
	MouCompanies   []CourseAdMouInput `json:"mou_companies"`
	Active         *bool              `json:"active"`
	Priority       *int               `json:"priority"`
}

type CourseAdCourseResponse struct {
	ID           uint   `json:"id"`
	Title        string `json:"title"`
	Level        string `json:"level"`
	Duration     string `json:"duration"`
	FieldOfStudy string `json:"field_of_study"`
	Affiliation  string `json:"affiliation"`
	EstFee       string `json:"est_fee"`
	BannerURL    string `json:"banner_url"`
	Location     string `json:"location"`
	Description  string `json:"description"`
}

type CourseAdInstitutionResponse struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	ImageURL string  `json:"image_url"`
	Rating   float64 `json:"rating"`
	Location string  `json:"location"`
	Website  string  `json:"website"`
	Slug     string  `json:"slug"`
}

type CourseAdMouResponse struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	LogoURL    string `json:"logo_url"`
	CompanyURL string `json:"company_url"`
}

type CourseAdCardResponse struct {
	ID           uint                          `json:"id"`
	Position     string                        `json:"position"`
	Course       *CourseAdCourseResponse       `json:"course"`
	Subtitle     string                        `json:"subtitle"`
	Institutions []CourseAdInstitutionResponse `json:"institutions"`
	MouCompanies []CourseAdMouResponse         `json:"mou_companies"`
	Active       bool                          `json:"active"`
	Priority     int                           `json:"priority"`
	Clicks       int                           `json:"clicks"`
}

type CourseUpdateLogoRequest struct {
	LogoURL string `json:"logo_url" binding:"required"`
}

// College-finder page ad DTOs

type CollegeAdTrendingRequest struct {
	Kind      string `json:"kind" binding:"required"`
	CollegeID uint   `json:"college_id" binding:"required"`
	Headline  string `json:"headline"`
	Priority  *int   `json:"priority"`
	Active    *bool  `json:"active"`
}

// CollegeAdTrendingUpdateRequest is partial-update friendly: every field is optional.
type CollegeAdTrendingUpdateRequest struct {
	Kind      string `json:"kind"`
	CollegeID *uint  `json:"college_id"`
	Headline  string `json:"headline"`
	Priority  *int   `json:"priority"`
	Active    *bool  `json:"active"`
}

type CollegeAdCollegeResponse struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	ImageURL    string  `json:"image_url"`
	Rating      float64 `json:"rating"`
	Location    string  `json:"location"`
	Type        string  `json:"type"`
	CollegeID   uint    `json:"college_id"`
	Website     string  `json:"website"`
	ReviewCount int     `json:"review_count"`
}

type CollegeAdTrendingItemResponse struct {
	ID       uint                      `json:"id"`
	Kind     string                    `json:"kind"`
	Headline string                    `json:"headline"`
	Priority int                       `json:"priority"`
	Active   bool                      `json:"active"`
	College  *CollegeAdCollegeResponse `json:"college"`
}

type CollegeAdTrendingGroupedResponse struct {
	Spotlight    []CollegeAdTrendingItemResponse `json:"spotlight"`
	MostSearched []CollegeAdTrendingItemResponse `json:"most_searched"`
}

type CollegeAdFeedbackRequest struct {
	Helpful bool     `json:"helpful"`
	Rating  int      `json:"rating"`
	Reasons []string `json:"reasons"`
	Comment string   `json:"comment"`
}

type CollegeAdFeedbackItemResponse struct {
	ID        uint   `json:"id"`
	Helpful   bool   `json:"helpful"`
	Rating    int    `json:"rating"`
	Reasons   string `json:"reasons"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
}

type CollegeAdFeedbackStatsResponse struct {
	Total           int64 `json:"total"`
	HelpfulCount    int64 `json:"helpful_count"`
	NotHelpfulCount int64 `json:"not_helpful_count"`
}

type CollegeAdFeedbackResponse struct {
	Items []CollegeAdFeedbackItemResponse `json:"items"`
	Stats CollegeAdFeedbackStatsResponse  `json:"stats"`
}

// Find-college ad card settings DTOs

// UpdateCollegeAdCardSettingsRequest is partial-update friendly: every key is
// optional; only provided keys are applied. *bool also makes gin's JSON
// binding reject non-boolean values with 400.
type UpdateCollegeAdCardSettingsRequest struct {
	Trending *bool `json:"trending"`
	ByType   *bool `json:"by_type"`
	Rating   *bool `json:"rating"`
}

type CollegeAdCardSettingsResponse struct {
	Trending bool `json:"trending"`
	ByType   bool `json:"by_type"`
	Rating   bool `json:"rating"`
}

// CollegeTypeCountResponse is one group of the ByType card counts. Type is
// the raw college_type DB value; the frontend owns label mapping.
type CollegeTypeCountResponse struct {
	Type  string `json:"type" gorm:"column:type"`
	Count int64  `json:"count" gorm:"column:count"`
}

// Advertise request DTOs

type AdvertiseRequestRequest struct {
	Name         string `json:"name" binding:"required"`
	Designation  string `json:"designation"`
	Contact      string `json:"contact"`
	Email        string `json:"email"`
	AdvertiseFor string `json:"advertise_for" binding:"required"`
	Note         string `json:"note"`
}

type AdvertiseStatusRequest struct {
	Status string `json:"status" binding:"required"`
	Note   string `json:"note"`
}

type AdvertiseRequestResponse struct {
	ID            uint   `json:"id"`
	InstitutionID uint   `json:"institution_id"`
	Name          string `json:"name"`
	Designation   string `json:"designation"`
	Contact       string `json:"contact"`
	Email         string `json:"email"`
	AdvertiseFor  string `json:"advertise_for"`
	Status        string `json:"status"`
	Note          string `json:"note"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

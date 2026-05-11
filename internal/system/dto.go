package system

type ContactInquiryRequest struct {
	Name    string `json:"name" binding:"required"`
	Email   string `json:"email" binding:"required,email"`
	Phone   string `json:"phone"`
	Subject string `json:"subject" binding:"required"`
	Message string `json:"message" binding:"required"`
	Type    string `json:"type"`
}

type ContactInquiryStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type AdRequest struct {
	Title     string `json:"title" binding:"required"`
	ImageURL  string `json:"image_url"`
	LinkURL   string `json:"link_url"`
	Location  string `json:"location"`
	Page      string `json:"page" binding:"required"`
	Position  string `json:"position"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Active    *bool  `json:"active"`
	Priority  int    `json:"priority"`
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
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type AdResponse struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	ImageURL    string `json:"image_url"`
	LinkURL     string `json:"link_url"`
	Location    string `json:"location"`
	Page        string `json:"page"`
	Position    string `json:"position"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Active      bool   `json:"active"`
	Clicks      int    `json:"clicks"`
	Impressions int    `json:"impressions"`
	Priority    int    `json:"priority"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
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

type PublicNotificationResponse struct {
	ID        uint   `json:"id"`
	CreatedAt string `json:"created_at"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	Link      string `json:"link"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	BgColor   string `json:"bg_color"`
}

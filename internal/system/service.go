package system

import (
	"errors"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func parseTime(s string) (time.Time, error) {
	formats := []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, nil
}

func (s *Service) SubmitContactInquiry(req ContactInquiryRequest) (*ContactInquiry, error) {
	inquiryType := req.Type
	if inquiryType == "" {
		inquiryType = "general"
	}

	inquiry := &ContactInquiry{
		Name:    req.Name,
		Email:   req.Email,
		Phone:   req.Phone,
		Subject: req.Subject,
		Message: req.Message,
		Type:    inquiryType,
		Status:  "new",
	}

	if err := s.repo.CreateContactInquiry(inquiry); err != nil {
		return nil, errors.New("failed to submit inquiry")
	}

	return inquiry, nil
}

func (s *Service) GetContactInquiries(page, limit int, status, inquiryType string) ([]ContactInquiry, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	return s.repo.FindContactInquiries(page, limit, status, inquiryType)
}

func (s *Service) GetContactInquiryByID(id uint) (*ContactInquiry, error) {
	return s.repo.FindContactInquiryByID(id)
}

func (s *Service) UpdateContactInquiryStatus(id uint, status string) (*ContactInquiry, error) {
	validStatuses := map[string]bool{
		"new": true, "read": true, "in_progress": true, "resolved": true, "closed": true,
	}
	if !validStatuses[status] {
		return nil, errors.New("invalid status")
	}

	return s.repo.UpdateContactInquiryStatus(id, status)
}

func (s *Service) DeleteContactInquiry(id uint) error {
	return s.repo.DeleteContactInquiry(id)
}

func (s *Service) GetAds(page, limit int, pageFilter string, active *bool) ([]Ad, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	return s.repo.FindAds(page, limit, pageFilter, active)
}

func (s *Service) GetActiveAds(page string) ([]Ad, error) {
	return s.repo.FindActiveAds(page)
}

func (s *Service) GetAdByID(id uint) (*Ad, error) {
	return s.repo.FindAdByID(id)
}

func (s *Service) CreateAd(req AdRequest) (*Ad, error) {
	ad := &Ad{
		Title:    req.Title,
		ImageURL: req.ImageURL,
		LinkURL:  req.LinkURL,
		Page:     req.Page,
		Position: req.Position,
		Active:   true,
		Priority: req.Priority,
	}

	if req.StartDate != "" {
		if t, err := parseTime(req.StartDate); err == nil && !t.IsZero() {
			ad.StartDate = t
		}
	}
	if req.EndDate != "" {
		if t, err := parseTime(req.EndDate); err == nil && !t.IsZero() {
			ad.EndDate = t
		}
	}
	if req.Active != nil {
		ad.Active = *req.Active
	}

	if err := s.repo.CreateAd(ad); err != nil {
		return nil, errors.New("failed to create ad")
	}

	return ad, nil
}

func (s *Service) UpdateAd(id uint, req AdRequest) (*Ad, error) {
	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}
	if req.LinkURL != "" {
		updates["link_url"] = req.LinkURL
	}
	if req.Page != "" {
		updates["page"] = req.Page
	}
	if req.Position != "" {
		updates["position"] = req.Position
	}
	if req.StartDate != "" {
		if t, err := parseTime(req.StartDate); err == nil && !t.IsZero() {
			updates["start_date"] = t
		}
	}
	if req.EndDate != "" {
		if t, err := parseTime(req.EndDate); err == nil && !t.IsZero() {
			updates["end_date"] = t
		}
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if req.Priority != 0 {
		updates["priority"] = req.Priority
	}

	return s.repo.UpdateAd(id, updates)
}

func (s *Service) DeleteAd(id uint) error {
	return s.repo.DeleteAd(id)
}

func (s *Service) TrackAdClick(id uint) (*Ad, error) {
	return s.repo.TrackAdClick(id)
}

func (s *Service) GetCarousels(page string, active *bool) ([]CarouselSlide, error) {
	return s.repo.FindCarouselSlides(page, active)
}

func (s *Service) GetCarouselSlideByID(id uint) (*CarouselSlide, error) {
	return s.repo.FindCarouselSlideByID(id)
}

func (s *Service) CreateCarouselSlide(req CarouselSlideRequest) (*CarouselSlide, error) {
	page := req.Page
	if page == "" {
		page = "landing"
	}

	slide := &CarouselSlide{
		Page:        page,
		Title:       req.Title,
		Subtitle:    req.Subtitle,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		LinkURL:     req.LinkURL,
		ButtonText:  req.ButtonText,
		Order:       req.Order,
		Active:      true,
	}

	if req.Active != nil {
		slide.Active = *req.Active
	}

	if err := s.repo.CreateCarouselSlide(slide); err != nil {
		return nil, errors.New("failed to create slide")
	}

	return slide, nil
}

func (s *Service) UpdateCarouselSlide(id uint, req CarouselSlideRequest) (*CarouselSlide, error) {
	updates := map[string]interface{}{}
	if req.Page != "" {
		updates["page"] = req.Page
	}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Subtitle != "" {
		updates["subtitle"] = req.Subtitle
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}
	if req.LinkURL != "" {
		updates["link_url"] = req.LinkURL
	}
	if req.ButtonText != "" {
		updates["button_text"] = req.ButtonText
	}
	if req.Order != 0 {
		updates["order"] = req.Order
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}

	return s.repo.UpdateCarouselSlide(id, updates)
}

func (s *Service) DeleteCarouselSlide(id uint) error {
	return s.repo.DeleteCarouselSlide(id)
}

func (s *Service) ReorderCarouselSlides(items []struct {
	ID    uint
	Order int
}) error {
	return s.repo.ReorderCarouselSlides(items)
}

func (s *Service) GetActivePublicNotifications() ([]PublicNotification, error) {
	return s.repo.FindActivePublicNotifications()
}

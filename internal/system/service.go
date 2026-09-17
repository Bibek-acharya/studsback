package system

import (
	"context"
	"errors"
	"time"

	"studsphere/backend/internal/notification"
)

type Service struct {
	repo     *Repository
	notifier notification.Notifier
}

func NewService(repo *Repository, notifier notification.Notifier) *Service {
	return &Service{repo: repo, notifier: notifier}
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
		Name:          req.Name,
		Email:         req.Email,
		Phone:         req.Phone,
		Subject:       req.Subject,
		Message:       req.Message,
		Type:          inquiryType,
		Status:        "new",
		InstitutionID: req.InstitutionID,
	}

	if err := s.repo.CreateContactInquiry(inquiry); err != nil {
		return nil, errors.New("failed to submit inquiry")
	}

	audience, _ := s.notifier.ForRoles(context.Background(), "superadmin", "admin")
	if len(audience) > 0 {
		_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventSystemInquiryReceived,
			Recipients: audience,
			Data:       map[string]any{"name": req.Name, "email": req.Email, "subject": req.Subject},
		})
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
		"new": true, "New": true, "read": true, "in_progress": true, "resolved": true, "closed": true, "Closed": true,
		"In Contact": true, "Follow Up": true, "Admitted": true,
	}
	if !validStatuses[status] {
		return nil, errors.New("invalid status")
	}

	inquiry, err := s.repo.UpdateContactInquiryStatus(id, status)
	if err != nil {
		return nil, err
	}

	// Notify the inquirer when they have a registered account (doc 07:
	// "if registered account; else email" — guest email copy is a P3 seam,
	// silently skipped here until that path exists).
	if s.notifier != nil && inquiry.Email != "" {
		if userID, err := s.repo.UserIDByEmail(inquiry.Email); err == nil && userID != 0 {
			_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventSystemInquiryReplied,
				Recipients: []notification.Ref{{Type: "user", ID: userID}},
				Data:       map[string]any{"subject": inquiry.Subject},
			})
		}
	}

	return inquiry, nil
}

func (s *Service) DeleteContactInquiry(id uint) error {
	return s.repo.DeleteContactInquiry(id)
}

func (s *Service) GetInstitutionInquiries(institutionID uint, page, limit int, status, inquiryType, search string) ([]ContactInquiry, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	return s.repo.FindInstitutionInquiries(institutionID, page, limit, status, inquiryType, search)
}

func (s *Service) GetAds(page, limit int, pageFilter, positionFilter string, active *bool) ([]Ad, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	return s.repo.FindAds(page, limit, pageFilter, positionFilter, active)
}

func (s *Service) GetActiveAds(page, position string) ([]Ad, error) {
	return s.repo.FindActiveAds(page, position)
}

func (s *Service) GetAdByID(id uint) (*Ad, error) {
	return s.repo.FindAdByID(id)
}

func (s *Service) CreateAd(req AdRequest) (*Ad, error) {
	ad := &Ad{
		Title:       req.Title,
		ImageURL:    req.ImageURL,
		LinkURL:     req.LinkURL,
		Location:    req.Location,
		Page:        req.Page,
		Position:    req.Position,
		Active:      true,
		Priority:    req.Priority,
		CollegeID:   req.CollegeID,
		CourseID:    req.CourseID,
		Description: req.Description,
		Accent:      req.Accent,
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
	if req.Location != "" {
		updates["location"] = req.Location
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
	if req.CollegeID != nil {
		updates["college_id"] = *req.CollegeID
	}
	if req.CourseID != nil {
		updates["course_id"] = *req.CourseID
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Accent != "" {
		updates["accent"] = req.Accent
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

	// New slides with no explicit order go last by priority within the page.
	if req.Order == 0 {
		maxOrder, err := s.repo.MaxCarouselSlideOrder(page)
		if err != nil {
			return nil, errors.New("failed to determine slide order")
		}
		slide.Order = maxOrder + 1
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

func (s *Service) CreatePublicNotification(title, message, notifType, link, icon, color, bgColor string) (*PublicNotification, error) {
	n := &PublicNotification{
		Title:   title,
		Message: message,
		Type:    notifType,
		Link:    link,
		Active:  true,
		Icon:    icon,
		Color:   color,
		BgColor: bgColor,
	}
	if err := s.repo.CreatePublicNotification(n); err != nil {
		return nil, err
	}
	return n, nil
}

// Landing Course methods

func (s *Service) GetPublicLandingCourses() ([]LandingCoursePublicResponse, error) {
	fields, err := s.repo.FindActiveLandingFields()
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return []LandingCoursePublicResponse{}, nil
	}

	fieldIDs := make([]uint, len(fields))
	for i, f := range fields {
		fieldIDs[i] = f.ID
	}

	institutionsMap, err := s.repo.FindInstitutionsByFieldIDs(fieldIDs)
	if err != nil {
		return nil, err
	}

	var result []LandingCoursePublicResponse
	for _, field := range fields {
		institutions := institutionsMap[field.ID]
		if institutions == nil {
			institutions = []LandingCourseInstitution{}
		}
		instResponses := make([]LandingCourseInstitutionResponse, len(institutions))
		for i, inst := range institutions {
			instResponses[i] = LandingCourseInstitutionResponse{
				ID:              inst.ID,
				FieldID:         inst.FieldID,
				InstitutionID:   inst.InstitutionID,
				InstitutionType: inst.InstitutionType,
				InstitutionName: inst.InstitutionName,
				InstitutionLogo: inst.InstitutionLogo,
				Slug:            inst.Slug,
				OrderIndex:      inst.OrderIndex,
			}
		}
		result = append(result, LandingCoursePublicResponse{
			FieldOfStudy: field.FieldOfStudy,
			Institutions: instResponses,
		})
	}
	return result, nil
}

func (s *Service) GetAdminLandingCourses() ([]LandingCourseAdminFieldResponse, error) {
	fields, err := s.repo.FindAllLandingFields()
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return []LandingCourseAdminFieldResponse{}, nil
	}

	fieldIDs := make([]uint, len(fields))
	for i, f := range fields {
		fieldIDs[i] = f.ID
	}

	institutionsMap, err := s.repo.FindInstitutionsByFieldIDs(fieldIDs)
	if err != nil {
		return nil, err
	}

	var result []LandingCourseAdminFieldResponse
	for _, field := range fields {
		institutions := institutionsMap[field.ID]
		if institutions == nil {
			institutions = []LandingCourseInstitution{}
		}
		instResponses := make([]LandingCourseInstitutionResponse, len(institutions))
		for i, inst := range institutions {
			instResponses[i] = LandingCourseInstitutionResponse{
				ID:              inst.ID,
				FieldID:         inst.FieldID,
				InstitutionID:   inst.InstitutionID,
				InstitutionType: inst.InstitutionType,
				InstitutionName: inst.InstitutionName,
				InstitutionLogo: inst.InstitutionLogo,
				Slug:            inst.Slug,
				OrderIndex:      inst.OrderIndex,
			}
		}
		result = append(result, LandingCourseAdminFieldResponse{
			ID:           field.ID,
			FieldOfStudy: field.FieldOfStudy,
			DisplayOrder: field.DisplayOrder,
			IsActive:     field.IsActive,
			Institutions: instResponses,
		})
	}
	return result, nil
}

func (s *Service) UpdateLandingField(id uint, req UpdateFieldRequest) error {
	updates := map[string]interface{}{}
	if req.FieldOfStudy != nil && *req.FieldOfStudy != "" {
		updates["field_of_study"] = *req.FieldOfStudy
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.DisplayOrder != nil {
		updates["display_order"] = *req.DisplayOrder
	}
	if len(updates) == 0 {
		return nil
	}
	return s.repo.UpdateLandingField(id, updates)
}

func (s *Service) ReorderLandingFields(req ReorderFieldsRequest) error {
	return s.repo.ReorderLandingFields(req.Items)
}

func (s *Service) LinkInstitution(req LinkInstitutionRequest) (*LandingCourseInstitution, error) {
	// Check field exists
	_, err := s.repo.FindLandingFieldByID(req.FieldID)
	if err != nil {
		return nil, errors.New("field not found")
	}

	// Check max 5 per field
	count, err := s.repo.CountInstitutionsByField(req.FieldID)
	if err != nil {
		return nil, err
	}
	if count >= 5 {
		return nil, errors.New("maximum 5 institutions per field")
	}

	instType := req.InstitutionType
	if instType == "" {
		instType = "institution"
	}

	// Check duplicate (type-aware: a college and an institution sharing a
	// numeric id are distinct links)
	exists, err := s.repo.InstitutionLinkExists(req.FieldID, req.InstitutionID, instType)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("institution already linked to this field")
	}

	inst := &LandingCourseInstitution{
		FieldID:         req.FieldID,
		InstitutionID:   req.InstitutionID,
		InstitutionType: instType,
		InstitutionName: req.InstitutionName,
		InstitutionLogo: req.InstitutionLogo,
		Slug:            req.Slug,
		OrderIndex:      int(count),
	}

	if err := s.repo.CreateLandingInstitution(inst); err != nil {
		return nil, errors.New("failed to link institution")
	}
	return inst, nil
}

func (s *Service) UnlinkInstitution(id uint) error {
	return s.repo.DeleteLandingInstitution(id)
}

func (s *Service) ReorderLandingInstitutions(req ReorderInstitutionsRequest) error {
	return s.repo.ReorderLandingInstitutions(req.Items)
}

func (s *Service) SearchInstitutionsForLanding(query string) ([]InstitutionSearchResult, error) {
	return s.repo.SearchInstitutions(query)
}

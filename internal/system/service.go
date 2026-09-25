package system

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

// GetCarousels returns the slides of a single page. The page filter is
// optional: a blank page falls back to the landing hero default (the historic
// behavior) instead of matching every page, so callers can ask for a specific
// page such as "study-resources" without mixing in landing slides.
func (s *Service) GetCarousels(page string, active *bool) ([]CarouselSlide, error) {
	if strings.TrimSpace(page) == "" {
		page = CarouselPageLanding
	}
	return s.repo.FindCarouselSlides(page, active)
}

func (s *Service) GetCarouselSlideByID(id uint) (*CarouselSlide, error) {
	return s.repo.FindCarouselSlideByID(id)
}

func (s *Service) CreateCarouselSlide(req CarouselSlideRequest) (*CarouselSlide, error) {
	page := req.Page
	if page == "" {
		page = CarouselPageLanding
	}

	slide := &CarouselSlide{
		Page:        page,
		Title:       req.Title,
		Subtitle:    derefString(req.Subtitle),
		Description: derefString(req.Description),
		ImageURL:    req.ImageURL,
		LinkURL:     derefString(req.LinkURL),
		ButtonText:  derefString(req.ButtonText),
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
	// Optional-but-clearable fields: a non-nil pointer is applied even when it
	// is empty so the admin can clear the column; nil leaves it untouched.
	if req.Subtitle != nil {
		updates["subtitle"] = *req.Subtitle
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}
	if req.LinkURL != nil {
		updates["link_url"] = *req.LinkURL
	}
	if req.ButtonText != nil {
		updates["button_text"] = *req.ButtonText
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

// Course-finder ad cards

// validationError marks user-input mistakes the handler maps to 400.
type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

func newValidationError(msg string) error { return &validationError{msg: msg} }

const (
	courseAdPositionMulti  = "multi_college"
	courseAdPositionSingle = "single_college"
)

func validCourseAdPosition(p string) bool {
	return p == courseAdPositionMulti || p == courseAdPositionSingle
}

func (s *Service) validateCourseAdChildLimits(position string, institutionIDs []uint, mous []CourseAdMouInput) error {
	if position == courseAdPositionMulti {
		if len(institutionIDs) == 0 {
			return newValidationError("at least one institution is required for multi_college")
		}
		if len(institutionIDs) > 7 {
			return newValidationError("maximum 7 institutions per multi_college card")
		}
	} else {
		if len(institutionIDs) != 1 {
			return newValidationError("single_college requires exactly one institution")
		}
	}
	if len(mous) > 12 {
		return newValidationError("maximum 12 MOU companies per card")
	}
	for _, m := range mous {
		if m.Name == "" {
			return newValidationError("MOU company name is required")
		}
	}
	return nil
}

func (s *Service) GetCourseAdCards(position string, activeOnly bool) ([]CourseAdCardResponse, error) {
	if !validCourseAdPosition(position) {
		return nil, newValidationError("position must be multi_college or single_college")
	}
	limit := 0
	if activeOnly {
		limit = 10
	}
	cards, err := s.repo.FindCourseAdCards(position, activeOnly, limit)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ResolveCourseAdEntities(cards); err != nil {
		return nil, err
	}
	responses := make([]CourseAdCardResponse, len(cards))
	for i := range cards {
		responses[i] = toCourseAdCardResponse(&cards[i])
	}
	return responses, nil
}

func (s *Service) GetCourseAdCard(id uint) (*CourseAdCardResponse, error) {
	card, err := s.repo.FindCourseAdCardByID(id)
	if err != nil {
		return nil, err
	}
	cards := []CourseAdCard{*card}
	if err := s.repo.ResolveCourseAdEntities(cards); err != nil {
		return nil, err
	}
	resp := toCourseAdCardResponse(&cards[0])
	return &resp, nil
}

func (s *Service) CreateCourseAdCard(req CourseAdRequest) (*CourseAdCardResponse, error) {
	if !validCourseAdPosition(req.Position) {
		return nil, newValidationError("position must be multi_college or single_college")
	}
	if err := s.validateCourseAdChildLimits(req.Position, req.InstitutionIDs, req.MouCompanies); err != nil {
		return nil, err
	}

	count, err := s.repo.CountCourseAdCards(req.Position)
	if err != nil {
		return nil, err
	}
	if count >= 10 {
		return nil, newValidationError("maximum 10 cards per position")
	}

	card := &CourseAdCard{
		Position: req.Position,
		CourseID: req.CourseID,
		Subtitle: req.Subtitle,
		Active:   true,
	}
	if req.Active != nil {
		card.Active = *req.Active
	}
	if req.Priority != nil {
		card.Priority = *req.Priority
	}
	if req.Position == courseAdPositionSingle && req.InstitutionID != nil {
		card.InstitutionID = req.InstitutionID
	}

	institutions := make([]CourseAdCardInstitution, len(req.InstitutionIDs))
	for i, id := range req.InstitutionIDs {
		institutions[i] = CourseAdCardInstitution{InstitutionID: id, OrderIndex: i}
	}
	mous := make([]CourseAdCardMouCompany, len(req.MouCompanies))
	for i, m := range req.MouCompanies {
		mous[i] = CourseAdCardMouCompany{Name: m.Name, LogoURL: m.LogoURL, CompanyURL: m.CompanyURL}
	}

	if err := s.repo.CreateCourseAdCard(card, institutions, mous); err != nil {
		return nil, err
	}
	return s.GetCourseAdCard(card.ID)
}

func (s *Service) UpdateCourseAdCard(id uint, req CourseAdRequest) (*CourseAdCardResponse, error) {
	existing, err := s.repo.FindCourseAdCardByID(id)
	if err != nil {
		if err.Error() == "record not found" {
			return nil, errors.New("record not found")
		}
		return nil, err
	}

	position := req.Position
	if position == "" {
		position = existing.Position
	}
	if !validCourseAdPosition(position) {
		return nil, newValidationError("position must be multi_college or single_college")
	}
	institutionIDs := req.InstitutionIDs
	if len(institutionIDs) == 0 && position == existing.Position {
		for _, l := range existing.Institutions {
			institutionIDs = append(institutionIDs, l.InstitutionID)
		}
	}
	mous := req.MouCompanies
	if err := s.validateCourseAdChildLimits(position, institutionIDs, mous); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Position != "" {
		updates["position"] = req.Position
	}
	if req.CourseID != 0 {
		updates["course_id"] = req.CourseID
	}
	if req.Subtitle != "" {
		updates["subtitle"] = req.Subtitle
	}
	if req.InstitutionID != nil {
		updates["institution_id"] = *req.InstitutionID
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}

	var replaceInstitutions *[]CourseAdCardInstitution
	if len(req.InstitutionIDs) > 0 {
		inst := make([]CourseAdCardInstitution, len(req.InstitutionIDs))
		for i, id := range req.InstitutionIDs {
			inst[i] = CourseAdCardInstitution{InstitutionID: id, OrderIndex: i}
		}
		replaceInstitutions = &inst
	}
	var replaceMous *[]CourseAdCardMouCompany
	if len(req.MouCompanies) > 0 {
		mr := make([]CourseAdCardMouCompany, len(req.MouCompanies))
		for i, m := range req.MouCompanies {
			// Preserve the logo when the company is unchanged and only the
			// URL/name rows are sent — PUT payloads commonly omit logo_url.
			logo := m.LogoURL
			if logo == "" && i < len(existing.MouCompanies) && existing.MouCompanies[i].Name == m.Name {
				logo = existing.MouCompanies[i].LogoURL
			}
			mr[i] = CourseAdCardMouCompany{Name: m.Name, LogoURL: logo, CompanyURL: m.CompanyURL}
		}
		replaceMous = &mr
	}

	card, err := s.repo.UpdateCourseAdCard(id, updates, replaceInstitutions, replaceMous)
	if err != nil {
		return nil, err
	}
	cards := []CourseAdCard{*card}
	if err := s.repo.ResolveCourseAdEntities(cards); err != nil {
		return nil, err
	}
	resp := toCourseAdCardResponse(&cards[0])
	return &resp, nil
}

func (s *Service) DeleteCourseAdCard(id uint) error {
	return s.repo.DeleteCourseAdCard(id)
}

func (s *Service) TrackCourseAdCardClick(id uint) error {
	return s.repo.TrackCourseAdCardClick(id)
}

// Advertise requests

// allowedAdvertiseFor values are the canonical advertisements shared
// with the frontend ADVERTISE_FOR_OPTIONS constants.
var allowedAdvertiseFor = map[string]bool{
	"course-finder:multi_college":  true,
	"course-finder:single_college": true,
	"landing-popup":                true,
	"hero-banner":                  true,
	"showcase-banner":              true,
	"landing-courses":              true,
	"university-affiliation":       true,
}

func (s *Service) SubmitAdvertiseRequest(institutionID uint, req AdvertiseRequestRequest) (*AdvertiseRequestResponse, error) {
	if !allowedAdvertiseFor[req.AdvertiseFor] {
		return nil, newValidationError("advertise_for must be one of: course-finder:multi_college, course-finder:single_college, landing-popup, hero-banner, showcase-banner, landing-courses, university-affiliation")
	}
	if req.Email == "" {
		email, err := s.repo.InstitutionUserEmail(institutionID)
		if err != nil || email == "" {
			return nil, newValidationError("email is required")
		}
		req.Email = email
	}

	request := &AdvertiseRequest{
		InstitutionID: institutionID,
		Name:          req.Name,
		Designation:   req.Designation,
		Contact:       req.Contact,
		Email:         req.Email,
		AdvertiseFor:  req.AdvertiseFor,
		Status:        "pending",
		Note:          req.Note,
	}
	if err := s.repo.CreateAdvertiseRequest(request); err != nil {
		return nil, errors.New("failed to submit advertise request")
	}
	resp := toAdvertiseRequestResponse(request)
	return &resp, nil
}

func (s *Service) GetInstitutionAdvertiseRequests(institutionID uint) ([]AdvertiseRequestResponse, error) {
	requests, err := s.repo.FindAdvertiseRequestsByInstitution(institutionID)
	if err != nil {
		return nil, err
	}
	responses := make([]AdvertiseRequestResponse, len(requests))
	for i := range requests {
		responses[i] = toAdvertiseRequestResponse(&requests[i])
	}
	return responses, nil
}

func (s *Service) GetAdvertiseRequests(page, limit int, status string) ([]AdvertiseRequestResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	requests, total, err := s.repo.FindAdvertiseRequests(page, limit, status)
	if err != nil {
		return nil, 0, err
	}
	responses := make([]AdvertiseRequestResponse, len(requests))
	for i := range requests {
		responses[i] = toAdvertiseRequestResponse(&requests[i])
	}
	return responses, total, nil
}

func (s *Service) UpdateAdvertiseRequestStatus(id uint, req AdvertiseStatusRequest) (*AdvertiseRequestResponse, error) {
	if req.Status != "approved" && req.Status != "declined" {
		return nil, newValidationError("status must be approved or declined")
	}
	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	updated, err := s.repo.UpdateAdvertiseRequestStatus(id, req.Status, note)
	if err != nil {
		if err.Error() == "record not found" {
			return nil, err
		}
		return nil, err
	}
	resp := toAdvertiseRequestResponse(updated)
	return &resp, nil
}

func toAdvertiseRequestResponse(req *AdvertiseRequest) AdvertiseRequestResponse {
	return AdvertiseRequestResponse{
		ID:            req.ID,
		InstitutionID: req.InstitutionID,
		Name:          req.Name,
		Designation:   req.Designation,
		Contact:       req.Contact,
		Email:         req.Email,
		AdvertiseFor:  req.AdvertiseFor,
		Status:        req.Status,
		Note:          req.Note,
		CreatedAt:     req.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     req.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// College-finder page ads

const (
	collegeAdKindSpotlight    = "spotlight"
	collegeAdKindMostSearched = "most_searched"

	maxCollegeAdTrendingPerKind    = 10
	maxCollegeAdFeedbackCommentLen = 1000
	maxCollegeAdFeedbackRows       = 500
)

func validCollegeAdKind(k string) bool {
	return k == collegeAdKindSpotlight || k == collegeAdKindMostSearched
}

// GetCollegeAdTrending lists trending items, active-only when requested.
// kind filters to one kind when non-empty.
func (s *Service) GetCollegeAdTrending(kind string, activeOnly bool) ([]CollegeAdTrendingItemResponse, error) {
	if kind != "" && !validCollegeAdKind(kind) {
		return nil, newValidationError("kind must be spotlight or most_searched")
	}
	items, err := s.repo.FindCollegeAdTrendingItems(kind, activeOnly)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ResolveCollegeAdTrending(items); err != nil {
		return nil, err
	}
	responses := make([]CollegeAdTrendingItemResponse, len(items))
	for i := range items {
		responses[i] = toCollegeAdTrendingResponse(&items[i])
	}
	return responses, nil
}

// GetPublicCollegeAdTrending returns active items grouped by kind.
func (s *Service) GetPublicCollegeAdTrending() (CollegeAdTrendingGroupedResponse, error) {
	items, err := s.repo.FindCollegeAdTrendingItems("", true)
	if err != nil {
		return CollegeAdTrendingGroupedResponse{}, err
	}
	if err := s.repo.ResolveCollegeAdTrending(items); err != nil {
		return CollegeAdTrendingGroupedResponse{}, err
	}
	grouped := CollegeAdTrendingGroupedResponse{
		Spotlight:    []CollegeAdTrendingItemResponse{},
		MostSearched: []CollegeAdTrendingItemResponse{},
	}
	for i := range items {
		resp := toCollegeAdTrendingResponse(&items[i])
		if items[i].Kind == collegeAdKindSpotlight {
			grouped.Spotlight = append(grouped.Spotlight, resp)
		} else {
			grouped.MostSearched = append(grouped.MostSearched, resp)
		}
	}
	return grouped, nil
}

func (s *Service) GetCollegeAdTrendingItem(id uint) (*CollegeAdTrendingItemResponse, error) {
	item, err := s.repo.FindCollegeAdTrendingItemByID(id)
	if err != nil {
		return nil, err
	}
	items := []CollegeAdTrendingItem{*item}
	if err := s.repo.ResolveCollegeAdTrending(items); err != nil {
		return nil, err
	}
	resp := toCollegeAdTrendingResponse(&items[0])
	return &resp, nil
}

func (s *Service) CreateCollegeAdTrending(req CollegeAdTrendingRequest) (*CollegeAdTrendingItemResponse, error) {
	if !validCollegeAdKind(req.Kind) {
		return nil, newValidationError("kind must be spotlight or most_searched")
	}
	if req.CollegeID == 0 {
		return nil, newValidationError("college_id is required")
	}

	count, err := s.repo.CountCollegeAdTrending(req.Kind)
	if err != nil {
		return nil, err
	}
	if count >= maxCollegeAdTrendingPerKind {
		return nil, newValidationError("maximum 10 items per kind")
	}

	item := &CollegeAdTrendingItem{
		Kind:      req.Kind,
		CollegeID: req.CollegeID,
		Headline:  req.Headline,
		Active:    true,
	}
	if req.Active != nil {
		item.Active = *req.Active
	}
	if req.Priority != nil {
		item.Priority = *req.Priority
	}

	if err := s.repo.CreateCollegeAdTrendingItem(item); err != nil {
		return nil, err
	}
	return s.GetCollegeAdTrendingItem(item.ID)
}

func (s *Service) UpdateCollegeAdTrending(id uint, req CollegeAdTrendingUpdateRequest) (*CollegeAdTrendingItemResponse, error) {
	if _, err := s.repo.FindCollegeAdTrendingItemByID(id); err != nil {
		if err.Error() == "record not found" {
			return nil, errors.New("record not found")
		}
		return nil, err
	}
	if req.Kind != "" && !validCollegeAdKind(req.Kind) {
		return nil, newValidationError("kind must be spotlight or most_searched")
	}

	updates := map[string]interface{}{}
	if req.Kind != "" {
		updates["kind"] = req.Kind
	}
	if req.CollegeID != nil && *req.CollegeID != 0 {
		updates["college_id"] = *req.CollegeID
	}
	if req.Headline != "" {
		updates["headline"] = req.Headline
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}

	if len(updates) > 0 {
		if _, err := s.repo.UpdateCollegeAdTrendingItem(id, updates); err != nil {
			return nil, err
		}
	}
	return s.GetCollegeAdTrendingItem(id)
}

func (s *Service) DeleteCollegeAdTrending(id uint) error {
	return s.repo.DeleteCollegeAdTrendingItem(id)
}

// SubmitCollegeAdFeedback persists recommendation feedback. The comment is
// capped server-side and reasons are stored as a comma-separated string.
func (s *Service) SubmitCollegeAdFeedback(req CollegeAdFeedbackRequest) error {
	comment := strings.TrimSpace(req.Comment)
	if runes := []rune(comment); len(runes) > maxCollegeAdFeedbackCommentLen {
		comment = string(runes[:maxCollegeAdFeedbackCommentLen])
	}
	fb := &CollegeRecommendationFeedback{
		Helpful: req.Helpful,
		Rating:  req.Rating,
		Reasons: strings.Join(req.Reasons, ","),
		Comment: comment,
	}
	if err := s.repo.CreateCollegeRecommendationFeedback(fb); err != nil {
		return errors.New("failed to submit feedback")
	}
	return nil
}

func (s *Service) GetCollegeAdFeedback() (CollegeAdFeedbackResponse, error) {
	feedback, err := s.repo.FindCollegeRecommendationFeedback(maxCollegeAdFeedbackRows)
	if err != nil {
		return CollegeAdFeedbackResponse{}, err
	}
	total, helpful, notHelpful, err := s.repo.CountCollegeRecommendationFeedback()
	if err != nil {
		return CollegeAdFeedbackResponse{}, err
	}

	items := make([]CollegeAdFeedbackItemResponse, len(feedback))
	for i := range feedback {
		items[i] = CollegeAdFeedbackItemResponse{
			ID:        feedback[i].ID,
			Helpful:   feedback[i].Helpful,
			Rating:    feedback[i].Rating,
			Reasons:   feedback[i].Reasons,
			Comment:   feedback[i].Comment,
			CreatedAt: feedback[i].CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return CollegeAdFeedbackResponse{
		Items: items,
		Stats: CollegeAdFeedbackStatsResponse{
			Total:           total,
			HelpfulCount:    helpful,
			NotHelpfulCount: notHelpful,
		},
	}, nil
}

// Find-college ad card settings

// collegeAdCardSettingsKey is the single SystemSetting key holding the three
// card toggles as a JSON object.
const collegeAdCardSettingsKey = "find_college_ad_cards"

// defaultCollegeAdCardSettings is the contract when nothing is stored yet:
// every card starts enabled.
func defaultCollegeAdCardSettings() CollegeAdCardSettingsResponse {
	return CollegeAdCardSettingsResponse{Trending: true, ByType: true, Rating: true}
}

func (s *Service) GetCollegeAdCardSettings() (CollegeAdCardSettingsResponse, error) {
	value, found, err := s.repo.GetSystemSetting(collegeAdCardSettingsKey)
	if err != nil {
		return CollegeAdCardSettingsResponse{}, err
	}
	if !found || value == "" {
		return defaultCollegeAdCardSettings(), nil
	}
	// Unmarshal onto the defaults so a partial stored object still yields
	// true for any key it omits.
	settings := defaultCollegeAdCardSettings()
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return CollegeAdCardSettingsResponse{}, err
	}
	return settings, nil
}

// UpdateCollegeAdCardSettings applies only the provided keys on top of the
// current settings and persists the merged object.
func (s *Service) UpdateCollegeAdCardSettings(req UpdateCollegeAdCardSettingsRequest) (CollegeAdCardSettingsResponse, error) {
	settings, err := s.GetCollegeAdCardSettings()
	if err != nil {
		return CollegeAdCardSettingsResponse{}, err
	}
	if req.Trending != nil {
		settings.Trending = *req.Trending
	}
	if req.ByType != nil {
		settings.ByType = *req.ByType
	}
	if req.Rating != nil {
		settings.Rating = *req.Rating
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return CollegeAdCardSettingsResponse{}, err
	}
	if err := s.repo.SetSystemSetting(collegeAdCardSettingsKey, string(data)); err != nil {
		return CollegeAdCardSettingsResponse{}, err
	}
	return settings, nil
}

// GetCollegeTypeCounts returns per-type college counts for the ByType card.
func (s *Service) GetCollegeTypeCounts() ([]CollegeTypeCountResponse, error) {
	return s.repo.CollegeTypeCounts()
}

func toCollegeAdTrendingResponse(item *CollegeAdTrendingItem) CollegeAdTrendingItemResponse {
	resp := CollegeAdTrendingItemResponse{
		ID:       item.ID,
		Kind:     item.Kind,
		Headline: item.Headline,
		Priority: item.Priority,
		Active:   item.Active,
		College:  nil,
	}
	if item.College != nil {
		resp.College = &CollegeAdCollegeResponse{
			ID:          item.College.ID,
			Name:        item.College.Name,
			ImageURL:    item.College.ImageURL,
			Rating:      item.College.Rating,
			Location:    item.College.Location,
			Type:        item.College.Type,
			CollegeID:   item.College.CollegeID,
			Website:     item.College.Website,
			ReviewCount: item.College.ReviewCount,
		}
	}
	return resp
}

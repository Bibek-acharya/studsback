package scholarshipprovider

import (
	"encoding/json"
	"errors"
	"time"

	publicscholarship "studsphere/backend/internal/scholarship"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetDashboard(providerID uint) (*DashboardResponse, error) {
	totalScholarships, totalApplications, pendingApplications, totalInterviews, unreadMessages, err := s.repo.GetDashboardCounts(providerID)
	if err != nil {
		return nil, err
	}

	return &DashboardResponse{
		TotalScholarships:   totalScholarships,
		TotalApplications:   totalApplications,
		PendingApplications: pendingApplications,
		TotalInterviews:     totalInterviews,
		UnreadMessages:      unreadMessages,
	}, nil
}

func (s *Service) GetAnalytics(providerID uint) (*AnalyticsResponse, error) {
	applications, scholarships, err := s.repo.GetAnalytics(providerID)
	if err != nil {
		return nil, err
	}

	statusCounts := map[string]int{}
	for _, app := range applications {
		statusCounts[app.Status]++
	}

	scholarshipStats := []ScholarshipStat{}
	for _, sch := range scholarships {
		appCount, err := s.repo.GetApplicationCountByScholarship(sch.ID)
		if err != nil {
			return nil, err
		}
		scholarshipStats = append(scholarshipStats, ScholarshipStat{
			ID:           sch.ID,
			Title:        sch.Title,
			Applications: appCount,
			Status:       sch.Status,
		})
	}

	return &AnalyticsResponse{
		StatusBreakdown:   statusCounts,
		TotalApplications: len(applications),
		ScholarshipStats:  scholarshipStats,
	}, nil
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

func toJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (s *Service) CreateScholarship(providerID uint, req CreateScholarshipRequest) (*ProviderScholarship, error) {
	status := "draft"
	if req.Status != "" {
		status = req.Status
	}

	var imageURL *string
	if req.ImageURL != "" {
		imageURL = &req.ImageURL
	}

	fieldOfStudy, _ := json.Marshal(req.FieldOfStudy)

	scholarship := &ProviderScholarship{
		ProviderID:      providerID,
		Title:           req.Title,
		Description:     req.Description,
		ImageURL:        imageURL,
		Location:        req.Location,
		Value:           req.Value,
		DegreeLevel:     req.DegreeLevel,
		FundingType:     req.FundingType,
		ScholarshipType: req.ScholarshipType,
		FieldOfStudy:    fieldOfStudy,
		Status:          status,
	}

	if req.Deadline != "" {
		if deadline, err := parseTime(req.Deadline); err == nil {
			scholarship.Deadline = deadline
		}
	}

	if err := s.repo.CreateScholarship(scholarship); err != nil {
		return nil, err
	}

	if err := s.syncPublicScholarship(providerID, req, fieldOfStudy, scholarship.Deadline, false); err != nil {
		return nil, err
	}

	return scholarship, nil
}

func (s *Service) syncPublicScholarship(providerID uint, req CreateScholarshipRequest, fieldOfStudy []byte, deadline time.Time, isUpdate bool) error {
	if req.Status != "active" {
		if isUpdate {
			// If deactivating, remove from public table
			provider, err := s.repo.GetProviderProfile(providerID)
			if err != nil {
				return err
			}
			return s.repo.DeletePublicScholarship(req.Title, provider.ProviderName)
		}
		return nil
	}

	provider, err := s.repo.GetProviderProfile(providerID)
	if err != nil {
		return err
	}

	publicScholarship := &publicscholarship.Scholarship{
		Title:               req.Title,
		Provider:            provider.ProviderName,
		Location:            req.Location,
		Value:               req.Value,
		Deadline:            deadline,
		DegreeLevel:         req.DegreeLevel,
		FundingType:         req.FundingType,
		ScholarshipType:     req.ScholarshipType,
		Description:         req.Description,
		ImageURL:            req.ImageURL,
		FieldOfStudy:        fieldOfStudy,
		EligibilityCriteria: nil,
		RequiredDocuments:   nil,
	}

	if isUpdate {
		existing, err := s.repo.FindPublicScholarship(req.Title, provider.ProviderName)
		if err == nil && existing != nil {
			updates := map[string]interface{}{
				"title":            req.Title,
				"provider":         provider.ProviderName,
				"location":         req.Location,
				"value":            req.Value,
				"deadline":         deadline,
				"degree_level":     req.DegreeLevel,
				"funding_type":     req.FundingType,
				"scholarship_type": req.ScholarshipType,
				"description":      req.Description,
				"image_url":        req.ImageURL,
				"field_of_study":   fieldOfStudy,
			}
			return s.repo.UpdatePublicScholarship(existing.ID, updates)
		}
	}

	return s.repo.CreatePublicScholarship(publicScholarship)
}

func (s *Service) GetScholarships(providerID uint, page, limit int) ([]ProviderScholarship, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	return s.repo.GetScholarshipsByProvider(providerID, page, limit)
}

func (s *Service) GetScholarshipByID(providerID, id uint) (*ProviderScholarship, error) {
	return s.repo.GetScholarshipByIDAndProvider(id, providerID)
}

func (s *Service) UpdateScholarship(providerID, id uint, req CreateScholarshipRequest) (*ProviderScholarship, error) {
	scholarship, err := s.repo.GetScholarshipByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	fieldOfStudy := toJSON(req.FieldOfStudy)
	updates := map[string]interface{}{
		"title":            req.Title,
		"description":      req.Description,
		"location":         req.Location,
		"value":            req.Value,
		"degree_level":     req.DegreeLevel,
		"funding_type":     req.FundingType,
		"scholarship_type": req.ScholarshipType,
		"field_of_study":   fieldOfStudy,
	}

	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	var deadline time.Time
	if req.Deadline != "" {
		if parsed, err := parseTime(req.Deadline); err == nil {
			deadline = parsed
			updates["deadline"] = deadline
		}
	}

	if err := s.repo.UpdateScholarship(scholarship, updates); err != nil {
		return nil, err
	}

	if err := s.syncPublicScholarship(providerID, req, fieldOfStudy, deadline, true); err != nil {
		return nil, err
	}

	return scholarship, nil
}

func (s *Service) DeleteScholarship(providerID, id uint) error {
	scholarship, err := s.repo.GetScholarshipByIDAndProvider(id, providerID)
	if err != nil {
		return err
	}

	provider, err := s.repo.GetProviderProfile(providerID)
	if err != nil {
		return err
	}

	_ = s.repo.DeletePublicScholarship(scholarship.Title, provider.ProviderName)
	return s.repo.DeleteScholarship(id, providerID)
}

func (s *Service) GetApplications(providerID uint, page, limit int, status, scholarshipID string) ([]ProviderApplication, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	return s.repo.GetApplicationsByProvider(providerID, page, limit, status, scholarshipID)
}

func (s *Service) GetApplicationByID(providerID, id uint) (*ProviderApplication, error) {
	return s.repo.GetApplicationByIDAndProvider(id, providerID)
}

func (s *Service) EvaluateApplication(providerID, id uint, req EvaluateApplicationRequest) (*ProviderApplication, error) {
	application, err := s.repo.GetApplicationByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.EvaluateApplication(application, req.Notes); err != nil {
		return nil, err
	}

	return application, nil
}

func (s *Service) UpdateApplicationStatus(providerID, id uint, req UpdateApplicationStatusRequest) (*ProviderApplication, error) {
	application, err := s.repo.GetApplicationByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	validStatuses := map[string]bool{
		"pending": true, "under_review": true, "approved": true,
		"rejected": true, "shortlisted": true,
	}
	if !validStatuses[req.Status] {
		return nil, errors.New("invalid status")
	}

	if err := s.repo.UpdateApplicationStatus(application, req.Status); err != nil {
		return nil, err
	}

	return application, nil
}

func (s *Service) GetInterviews(providerID uint) ([]ProviderInterview, error) {
	return s.repo.GetInterviewsByProvider(providerID)
}

func (s *Service) CreateInterview(providerID uint, req CreateInterviewRequest) (*ProviderInterview, error) {
	scheduledAt, err := parseTime(req.ScheduledAt)
	if err != nil {
		return nil, errors.New("invalid scheduled_at format")
	}

	interview := &ProviderInterview{
		ProviderID:    providerID,
		ApplicationID: req.ApplicationID,
		ScheduledAt:   scheduledAt,
		Duration:      req.Duration,
		Type:          req.Type,
		Location:      req.Location,
		Link:          req.Link,
		Notes:         req.Notes,
		Status:        "scheduled",
	}

	if err := s.repo.CreateInterview(interview); err != nil {
		return nil, err
	}

	return interview, nil
}

func (s *Service) UpdateInterview(providerID, id uint, req UpdateInterviewRequest) (*ProviderInterview, error) {
	interview, err := s.repo.GetInterviewByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.ScheduledAt != "" {
		if t, err := parseTime(req.ScheduledAt); err == nil {
			updates["scheduled_at"] = t
		}
	}
	if req.Duration > 0 {
		updates["duration"] = req.Duration
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.Link != "" {
		updates["link"] = req.Link
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}

	if err := s.repo.UpdateInterview(interview, updates); err != nil {
		return nil, err
	}

	return interview, nil
}

func (s *Service) GetMessages(providerID uint, page, limit int) ([]ProviderMessage, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	return s.repo.GetMessagesByProvider(providerID, page, limit)
}

func (s *Service) CreateMessage(providerID uint, req CreateMessageRequest) (*ProviderMessage, error) {
	message := &ProviderMessage{
		ProviderID: providerID,
		UserID:     req.UserID,
		Subject:    req.Subject,
		Content:    req.Content,
		Direction:  "outbound",
	}

	if err := s.repo.CreateMessage(message); err != nil {
		return nil, err
	}

	return message, nil
}

func (s *Service) GetMessageByID(providerID, id uint) (*ProviderMessage, error) {
	message, err := s.repo.GetMessageByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	if !message.Read {
		if err := s.repo.MarkMessageRead(message); err != nil {
			return nil, err
		}
	}

	return message, nil
}

func (s *Service) GetProviderProfile(providerID uint) (*ScholarshipProviderUser, error) {
	return s.repo.GetProviderProfile(providerID)
}

func (s *Service) UpdateProviderProfile(providerID uint, req UpdateProfileRequest) (*ScholarshipProviderUser, error) {
	provider, err := s.repo.GetProviderProfile(providerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.ProviderName != "" {
		updates["provider_name"] = req.ProviderName
	}
	if req.RegistrationNumber != "" {
		updates["registration_number"] = req.RegistrationNumber
	}

	if err := s.repo.UpdateProviderProfile(provider, updates); err != nil {
		return nil, err
	}

	return provider, nil
}

func (s *Service) GetProviderSettings(providerID uint) (*ProviderSettings, error) {
	return s.repo.GetProviderSettings(providerID)
}

func (s *Service) UpdateProviderSettings(providerID uint, req UpdateSettingsRequest) (*ProviderSettings, error) {
	settings, err := s.repo.GetProviderSettings(providerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"email_notifs": req.EmailNotifs,
		"sms_notifs":   req.SmsNotifs,
		"auto_reject":  req.AutoReject,
		"timezone":     req.Timezone,
		"language":     req.Language,
	}

	if err := s.repo.UpdateProviderSettings(settings, updates); err != nil {
		return nil, err
	}

	return settings, nil
}

func (s *Service) GetNotifications(providerID uint, page, limit int) ([]ProviderNotification, int64, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	return s.repo.GetNotificationsByProvider(providerID, page, limit)
}

func (s *Service) MarkNotificationRead(providerID, id uint) error {
	notification, err := s.repo.GetNotificationByIDAndProvider(id, providerID)
	if err != nil {
		return err
	}

	return s.repo.MarkNotificationRead(notification)
}

func (s *Service) MarkAllNotificationsRead(providerID uint) error {
	return s.repo.MarkAllNotificationsRead(providerID)
}

func (s *Service) CreateNotification(providerID uint, title, message, notifType, link string) error {
	notification := &ProviderNotification{
		ProviderID: providerID,
		Title:      title,
		Message:    message,
		Type:       notifType,
		Link:       link,
	}

	return s.repo.CreateNotification(notification)
}

func (s *Service) CreateNews(providerID uint, req CreateNewsRequest) (*ProviderNews, error) {
	status := "draft"
	if req.Status != "" {
		status = req.Status
	}

	var imageURL *string
	if req.Image != "" {
		imageURL = &req.Image
	}

	news := &ProviderNews{
		ProviderID: providerID,
		Title:      req.Title,
		Content:    req.Content,
		ImageURL:   imageURL,
		Status:     status,
	}

	if status == "published" {
		now := time.Now()
		news.PublishedAt = &now
	}

	if err := s.repo.CreateNews(news); err != nil {
		return nil, err
	}

	return news, nil
}

func (s *Service) GetNews(providerID uint, page, limit int) ([]ProviderNews, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	return s.repo.GetNewsByProvider(providerID, page, limit)
}

func (s *Service) GetNewsByID(providerID, id uint) (*ProviderNews, error) {
	return s.repo.GetNewsByIDAndProvider(id, providerID)
}

func (s *Service) UpdateNews(providerID, id uint, req CreateNewsRequest) (*ProviderNews, error) {
	news, err := s.repo.GetNewsByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"title":   req.Title,
		"content": req.Content,
	}

	if req.Image != "" {
		updates["image_url"] = req.Image
	}
	if req.Status != "" {
		updates["status"] = req.Status
		if req.Status == "published" && news.PublishedAt == nil {
			now := time.Now()
			updates["published_at"] = now
		}
	}

	if err := s.repo.UpdateNews(news, updates); err != nil {
		return nil, err
	}

	return news, nil
}

func (s *Service) DeleteNews(providerID, id uint) error {
	return s.repo.DeleteNews(id, providerID)
}

func (s *Service) CreateEvent(providerID uint, req CreateEventRequest) (*ProviderEvent, error) {
	startDate, err := parseTime(req.StartDate)
	if err != nil {
		return nil, errors.New("invalid start_date format")
	}

	endDate := startDate
	if req.EndDate != "" {
		if t, err := parseTime(req.EndDate); err == nil {
			endDate = t
		}
	}

	status := "upcoming"
	if req.Status != "" {
		status = req.Status
	}

	var imageURL *string
	if req.Image != "" {
		imageURL = &req.Image
	}

	event := &ProviderEvent{
		ProviderID:  providerID,
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    imageURL,
		EventType:   req.EventType,
		StartDate:   startDate,
		EndDate:     endDate,
		Location:    req.Location,
		Status:      status,
	}

	if err := s.repo.CreateEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *Service) GetEvents(providerID uint, page, limit int) ([]ProviderEvent, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	return s.repo.GetEventsByProvider(providerID, page, limit)
}

func (s *Service) GetEventByID(providerID, id uint) (*ProviderEvent, error) {
	return s.repo.GetEventByIDAndProvider(id, providerID)
}

func (s *Service) UpdateEvent(providerID, id uint, req CreateEventRequest) (*ProviderEvent, error) {
	event, err := s.repo.GetEventByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
		"event_type":  req.EventType,
		"location":    req.Location,
	}

	if req.Image != "" {
		updates["image_url"] = req.Image
	}
	if req.StartDate != "" {
		if t, err := parseTime(req.StartDate); err == nil {
			updates["start_date"] = t
		}
	}
	if req.EndDate != "" {
		if t, err := parseTime(req.EndDate); err == nil {
			updates["end_date"] = t
		}
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	if err := s.repo.UpdateEvent(event, updates); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *Service) DeleteEvent(providerID, id uint) error {
	return s.repo.DeleteEvent(id, providerID)
}

func (s *Service) CreateBlog(providerID uint, req CreateBlogRequest) (*ProviderBlog, error) {
	status := "draft"
	if req.Status != "" {
		status = req.Status
	}

	var imageURL *string
	if req.Image != "" {
		imageURL = &req.Image
	}

	blog := &ProviderBlog{
		ProviderID: providerID,
		Title:      req.Title,
		Content:    req.Content,
		ImageURL:   imageURL,
		Author:     req.Author,
		Status:     status,
	}

	if status == "published" {
		now := time.Now()
		blog.PublishedAt = &now
	}

	if err := s.repo.CreateBlog(blog); err != nil {
		return nil, err
	}

	return blog, nil
}

func (s *Service) GetBlogs(providerID uint, page, limit int) ([]ProviderBlog, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	return s.repo.GetBlogsByProvider(providerID, page, limit)
}

func (s *Service) GetBlogByID(providerID, id uint) (*ProviderBlog, error) {
	return s.repo.GetBlogByIDAndProvider(id, providerID)
}

func (s *Service) UpdateBlog(providerID, id uint, req CreateBlogRequest) (*ProviderBlog, error) {
	blog, err := s.repo.GetBlogByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"title":   req.Title,
		"content": req.Content,
		"author":  req.Author,
	}

	if req.Image != "" {
		updates["image_url"] = req.Image
	}
	if req.Status != "" {
		updates["status"] = req.Status
		if req.Status == "published" && blog.PublishedAt == nil {
			now := time.Now()
			updates["published_at"] = now
		}
	}

	if err := s.repo.UpdateBlog(blog, updates); err != nil {
		return nil, err
	}

	return blog, nil
}

func (s *Service) DeleteBlog(providerID, id uint) error {
	return s.repo.DeleteBlog(id, providerID)
}

func (s *Service) CreateCalendarEvent(providerID uint, req CreateCalendarEventRequest) (*ProviderCalendarEvent, error) {
	startDate, err := parseTime(req.StartDate)
	if err != nil {
		return nil, errors.New("invalid start_date format")
	}

	endDate := startDate.Add(time.Hour)
	if req.EndDate != "" {
		if t, err := parseTime(req.EndDate); err == nil {
			endDate = t
		}
	}

	event := &ProviderCalendarEvent{
		ProviderID:  providerID,
		Title:       req.Title,
		Description: req.Description,
		StartDate:   startDate,
		EndDate:     endDate,
		Color:       req.Color,
		IsAllDay:    req.IsAllDay,
	}

	if err := s.repo.CreateCalendarEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *Service) GetCalendarEvents(providerID uint) ([]ProviderCalendarEvent, error) {
	return s.repo.GetCalendarEventsByProvider(providerID)
}

func (s *Service) GetCalendarEventByID(providerID, id uint) (*ProviderCalendarEvent, error) {
	return s.repo.GetCalendarEventByIDAndProvider(id, providerID)
}

func (s *Service) UpdateCalendarEvent(providerID, id uint, req CreateCalendarEventRequest) (*ProviderCalendarEvent, error) {
	event, err := s.repo.GetCalendarEventByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"title":       req.Title,
		"description": req.Description,
		"color":       req.Color,
		"is_all_day":  req.IsAllDay,
	}

	if req.StartDate != "" {
		if t, err := parseTime(req.StartDate); err == nil {
			updates["start_date"] = t
		}
	}
	if req.EndDate != "" {
		if t, err := parseTime(req.EndDate); err == nil {
			updates["end_date"] = t
		}
	}

	if err := s.repo.UpdateCalendarEvent(event, updates); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *Service) DeleteCalendarEvent(providerID, id uint) error {
	return s.repo.DeleteCalendarEvent(id, providerID)
}

func (s *Service) CreateResult(providerID uint, req CreateResultRequest) (*ProviderResult, error) {
	status := "draft"
	if req.Status != "" {
		status = req.Status
	}

	result := &ProviderResult{
		ProviderID:    providerID,
		ScholarshipID: req.ScholarshipID,
		Title:         req.Title,
		Status:        status,
		Results:       req.Results,
	}

	if status == "published" {
		now := time.Now()
		result.PublishedAt = &now
	}

	if err := s.repo.CreateResult(result); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) GetResults(providerID uint, page, limit int) ([]ProviderResult, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	return s.repo.GetResultsByProvider(providerID, page, limit)
}

func (s *Service) GetResultByID(providerID, id uint) (*ProviderResult, error) {
	return s.repo.GetResultByIDAndProvider(id, providerID)
}

func (s *Service) UpdateResult(providerID, id uint, req CreateResultRequest) (*ProviderResult, error) {
	result, err := s.repo.GetResultByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"title":          req.Title,
		"scholarship_id": req.ScholarshipID,
		"results":        req.Results,
	}

	if req.Status != "" {
		updates["status"] = req.Status
		if req.Status == "published" && result.PublishedAt == nil {
			now := time.Now()
			updates["published_at"] = now
		}
	}

	if err := s.repo.UpdateResult(result, updates); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) DeleteResult(providerID, id uint) error {
	return s.repo.DeleteResult(id, providerID)
}

func (s *Service) CreateAccess(providerID uint, req CreateAccessRequest) (*ProviderAccess, error) {
	role := "viewer"
	if req.Role != "" {
		role = req.Role
	}

	access := &ProviderAccess{
		ProviderID: providerID,
		Email:      req.Email,
		Role:       role,
		Status:     "pending",
	}

	if err := s.repo.CreateAccess(access); err != nil {
		return nil, err
	}

	return access, nil
}

func (s *Service) GetAccess(providerID uint, page, limit int) ([]ProviderAccess, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	return s.repo.GetAccessByProvider(providerID, page, limit)
}

func (s *Service) GetAccessByID(providerID, id uint) (*ProviderAccess, error) {
	return s.repo.GetAccessByIDAndProvider(id, providerID)
}

func (s *Service) UpdateAccess(providerID, id uint, req CreateAccessRequest) (*ProviderAccess, error) {
	access, err := s.repo.GetAccessByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"email": req.Email,
	}

	if req.Role != "" {
		updates["role"] = req.Role
	}

	if err := s.repo.UpdateAccess(access, updates); err != nil {
		return nil, err
	}

	return access, nil
}

func (s *Service) DeleteAccess(providerID, id uint) error {
	return s.repo.DeleteAccess(id, providerID)
}

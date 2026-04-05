package scholarshipprovider

import (
	"encoding/json"
	"errors"
	"time"
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
		FieldOfStudy:    toJSON(req.FieldOfStudy),
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

	return scholarship, nil
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

	updates := map[string]interface{}{
		"title":            req.Title,
		"description":      req.Description,
		"location":         req.Location,
		"value":            req.Value,
		"degree_level":     req.DegreeLevel,
		"funding_type":     req.FundingType,
		"scholarship_type": req.ScholarshipType,
		"field_of_study":   toJSON(req.FieldOfStudy),
	}

	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Deadline != "" {
		if deadline, err := parseTime(req.Deadline); err == nil {
			updates["deadline"] = deadline
		}
	}

	if err := s.repo.UpdateScholarship(scholarship, updates); err != nil {
		return nil, err
	}

	return scholarship, nil
}

func (s *Service) DeleteScholarship(providerID, id uint) error {
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

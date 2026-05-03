package scholarshipprovider

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	publicscholarship "studsphere/backend/internal/scholarship"

	"golang.org/x/crypto/bcrypt"
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
	formats := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid time format")
}

func toJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func normalizeScholarshipStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "published", "active":
		return "published"
	case "draft", "", "pending":
		return "draft"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func parseOptionalTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := parseTime(value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func (s *Service) CreateScholarship(providerID uint, req CreateScholarshipRequest) (*ProviderScholarship, error) {
	status := normalizeScholarshipStatus(req.Status)

	deadlineValue := req.Deadline
	if deadlineValue == "" {
		deadlineValue = req.ApplicationEndDate
	}

	scholarship := &ProviderScholarship{
		ProviderID:               providerID,
		Title:                    req.Title,
		Provider:                 req.Provider,
		Description:              req.Description,
		ProviderName:             req.ProviderName,
		FundingTypeOther:         req.FundingTypeOther,
		ScholarshipTypeOther:     req.ScholarshipTypeOther,
		EducationLevel:           req.EducationLevel,
		EducationLevelOther:      req.EducationLevelOther,
		Location:                 req.Location,
		Value:                    req.Value,
		DegreeLevel:              req.DegreeLevel,
		FundingType:              req.FundingType,
		ScholarshipType:          req.ScholarshipType,
		FieldOfStudy:             toJSON(req.FieldOfStudy),
		Status:                   status,
		ApplyLink:                req.ApplyLink,
		BannerBackgroundImageURL: req.BannerBackgroundImageURL,
		CoverageArea:             req.CoverageArea,
		ContactEmail:             req.ContactEmail,
		PrimaryPhone:             req.PrimaryPhone,
		SecondaryPhone:           req.SecondaryPhone,
		WebsiteUrl:               req.WebsiteUrl,
		OfficeAddress:            req.OfficeAddress,
		MapUrl:                   req.MapUrl,
		AboutParagraph1:          req.AboutParagraph1,
		ScholarshipDescription2:  req.ScholarshipDescription2,
		VideoTutorials:           toJSON(req.VideoTutorials),
		JourneyTimeline:          toJSON(req.JourneyTimeline),
		Timeline:                 toJSON(req.Timeline),
		ScholarshipSectionTitle:  req.ScholarshipSectionTitle,
		ScholarshipSubtitle:      req.ScholarshipSubtitle,
		ScholarshipDescription1:  req.ScholarshipDescription1,
		ScholarshipTypes:         toJSON(req.ScholarshipTypes),
		ScholarshipTypesNew:      toJSON(req.ScholarshipTypesNew),
		SelectionRubric:          toJSON(req.SelectionRubric),
		SelectionRubricNew:       toJSON(req.SelectionRubricNew),
		EligibilitySectionTitle:  req.EligibilitySectionTitle,
		EligibilitySubtitle:      req.EligibilitySubtitle,
		BasicEligibilityCriteria: toJSON(req.BasicEligibilityCriteria),
		FullyFundedCriteria:      toJSON(req.FullyFundedCriteria),
		PartiallyFundedCriteria:  toJSON(req.PartiallyFundedCriteria),
		SelectionProcessSteps:    toJSON(req.SelectionProcessSteps),
		RequiredDocuments:        toJSON(req.RequiredDocuments),
		FAQs:                     toJSON(req.FAQs),
		FAQsNew:                  toJSON(req.FAQsNew),
		GalleryImages:            toJSON(req.GalleryImages),
		GalleryImagesNew:         toJSON(req.GalleryImagesNew),
		PartnerGroups:            toJSON(req.PartnerGroups),
		ExamCenters:              toJSON(req.ExamCenters),
		ExamCentersNew:           toJSON(req.ExamCentersNew),
		Downloads:                toJSON(req.Downloads),
		PaymentConfig:            toJSON(req.PaymentConfig),
	}

	if req.ApplicationStartDate != "" {
		if parsed, ok := parseOptionalTime(req.ApplicationStartDate); ok {
			scholarship.ApplicationStartDate = parsed
		} else {
			return nil, errors.New("invalid application start date")
		}
	}
	if deadlineValue != "" {
		if parsed, ok := parseOptionalTime(deadlineValue); ok {
			scholarship.Deadline = parsed
			scholarship.ApplicationEndDate = parsed
		} else {
			return nil, errors.New("invalid application end date")
		}
	}

	if err := s.repo.CreateScholarship(scholarship); err != nil {
		return nil, err
	}

	if err := s.syncPublicScholarship(providerID, scholarship, status, false); err != nil {
		return nil, err
	}

	return scholarship, nil
}

func (s *Service) syncPublicScholarship(providerID uint, scholarship *ProviderScholarship, status string, isUpdate bool) error {
	if normalizeScholarshipStatus(status) != "published" {
		if isUpdate {
			return s.repo.DeletePublicScholarshipByProviderScholarshipID(scholarship.ID)
		}
		return nil
	}

	provider, err := s.repo.GetProviderProfile(providerID)
	if err != nil {
		return err
	}

	publicScholarship := &publicscholarship.Scholarship{
		Title:                    scholarship.Title,
		Provider:                 provider.ProviderName,
		Location:                 scholarship.Location,
		Value:                    scholarship.Value,
		Deadline:                 scholarship.Deadline,
		DegreeLevel:              scholarship.DegreeLevel,
		FundingType:              scholarship.FundingType,
		ScholarshipType:          scholarship.ScholarshipType,
		Description:              scholarship.Description,
		BannerBackgroundImageURL: scholarship.BannerBackgroundImageURL,
		FieldOfStudy:             scholarship.FieldOfStudy,
		EligibilityCriteria:      scholarship.BasicEligibilityCriteria,
		RequiredDocuments:        scholarship.RequiredDocuments,
		PaymentConfig:            scholarship.PaymentConfig,
		ProviderScholarshipID:    &scholarship.ID,
	}

	existing, err := s.repo.FindPublicScholarshipByProviderScholarshipID(scholarship.ID)
	if err == nil && existing != nil {
		updates := map[string]interface{}{
			"title":                       publicScholarship.Title,
			"provider":                    publicScholarship.Provider,
			"location":                    publicScholarship.Location,
			"value":                       publicScholarship.Value,
			"deadline":                    publicScholarship.Deadline,
			"application_start_date":      publicScholarship.ApplicationStartDate,
			"degree_level":                publicScholarship.DegreeLevel,
			"funding_type":                publicScholarship.FundingType,
			"scholarship_type":            publicScholarship.ScholarshipType,
			"description":                 publicScholarship.Description,
			"banner_background_image_url": publicScholarship.BannerBackgroundImageURL,
			"field_of_study":              publicScholarship.FieldOfStudy,
			"eligibility_criteria":        publicScholarship.EligibilityCriteria,
			"required_documents":          publicScholarship.RequiredDocuments,
			"payment_config":              publicScholarship.PaymentConfig,
			"provider_scholarship_id":     scholarship.ID,
		}
		return s.repo.UpdatePublicScholarship(existing.ID, updates)
	}

	return s.repo.CreatePublicScholarship(publicScholarship, scholarship.ID)
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

	log.Printf("scholarshipprovider: Service.UpdateScholarship - requestedProviderID=%d, scholarship.ProviderID=%d, scholarshipID=%d", providerID, scholarship.ProviderID, scholarship.ID)

	updates := make(map[string]interface{})
	deadlineValue := req.Deadline
	if deadlineValue == "" {
		deadlineValue = req.ApplicationEndDate
	}

	updates["title"] = req.Title
	updates["provider"] = req.Provider
	updates["description"] = req.Description
	updates["provider_name"] = req.ProviderName
	updates["funding_type_other"] = req.FundingTypeOther
	updates["scholarship_type_other"] = req.ScholarshipTypeOther
	updates["education_level"] = req.EducationLevel
	updates["education_level_other"] = req.EducationLevelOther
	updates["location"] = req.Location
	updates["value"] = req.Value
	updates["degree_level"] = req.DegreeLevel
	updates["funding_type"] = req.FundingType
	updates["scholarship_type"] = req.ScholarshipType
	updates["field_of_study"] = toJSON(req.FieldOfStudy)
	updates["status"] = normalizeScholarshipStatus(req.Status)
	updates["apply_link"] = req.ApplyLink
	updates["banner_background_image_url"] = req.BannerBackgroundImageURL
	updates["coverage_area"] = req.CoverageArea
	updates["contact_email"] = req.ContactEmail
	updates["primary_phone"] = req.PrimaryPhone
	updates["secondary_phone"] = req.SecondaryPhone
	updates["website_url"] = req.WebsiteUrl
	updates["office_address"] = req.OfficeAddress
	updates["map_url"] = req.MapUrl
	updates["about_paragraph_1"] = req.AboutParagraph1
	updates["scholarship_description_2"] = req.ScholarshipDescription2
	updates["video_tutorials"] = toJSON(req.VideoTutorials)
	updates["journey_timeline"] = toJSON(req.JourneyTimeline)
	updates["timeline"] = toJSON(req.Timeline)
	updates["scholarship_section_title"] = req.ScholarshipSectionTitle
	updates["scholarship_subtitle"] = req.ScholarshipSubtitle
	updates["scholarship_description_1"] = req.ScholarshipDescription1
	updates["scholarship_types"] = toJSON(req.ScholarshipTypes)
	updates["scholarship_types_new"] = toJSON(req.ScholarshipTypesNew)
	updates["selection_rubric"] = toJSON(req.SelectionRubric)
	updates["selection_rubric_new"] = toJSON(req.SelectionRubricNew)
	updates["eligibility_section_title"] = req.EligibilitySectionTitle
	updates["eligibility_subtitle"] = req.EligibilitySubtitle
	updates["basic_eligibility_criteria"] = toJSON(req.BasicEligibilityCriteria)
	updates["fully_funded_criteria"] = toJSON(req.FullyFundedCriteria)
	updates["partially_funded_criteria"] = toJSON(req.PartiallyFundedCriteria)
	updates["selection_process_steps"] = toJSON(req.SelectionProcessSteps)
	updates["required_documents"] = toJSON(req.RequiredDocuments)
	updates["faqs"] = toJSON(req.FAQs)
	updates["faqs_new"] = toJSON(req.FAQsNew)
	updates["gallery_images"] = toJSON(req.GalleryImages)
	updates["gallery_images_new"] = toJSON(req.GalleryImagesNew)
	updates["partner_groups"] = toJSON(req.PartnerGroups)
	updates["exam_centers"] = toJSON(req.ExamCenters)
	updates["exam_centers_new"] = toJSON(req.ExamCentersNew)
	updates["downloads"] = toJSON(req.Downloads)
	updates["payment_config"] = toJSON(req.PaymentConfig)

	if req.ApplicationStartDate != "" {
		if parsed, ok := parseOptionalTime(req.ApplicationStartDate); ok {
			updates["application_start_date"] = parsed
		} else {
			return nil, errors.New("invalid application start date")
		}
	} else {
		updates["application_start_date"] = time.Time{}
	}

	if deadlineValue != "" {
		if parsed, ok := parseOptionalTime(deadlineValue); ok {
			updates["deadline"] = parsed
			updates["application_end_date"] = parsed
		} else {
			return nil, errors.New("invalid application end date")
		}
	} else {
		updates["deadline"] = time.Time{}
		updates["application_end_date"] = time.Time{}
	}

	if len(updates) == 0 {
		return scholarship, nil
	}

	if err := s.repo.UpdateScholarship(scholarship, updates); err != nil {
		log.Printf("scholarshipprovider: UpdateScholarship repo.UpdateScholarship error: %v", err)
		return nil, err
	}

	resolved := *scholarship
	resolved.Title = req.Title
	resolved.Provider = req.Provider
	resolved.Description = req.Description
	resolved.ProviderName = req.ProviderName
	resolved.FundingTypeOther = req.FundingTypeOther
	resolved.ScholarshipTypeOther = req.ScholarshipTypeOther
	resolved.EducationLevel = req.EducationLevel
	resolved.EducationLevelOther = req.EducationLevelOther
	resolved.Location = req.Location
	resolved.Value = req.Value
	resolved.DegreeLevel = req.DegreeLevel
	resolved.FundingType = req.FundingType
	resolved.ScholarshipType = req.ScholarshipType
	resolved.FieldOfStudy = toJSON(req.FieldOfStudy)
	resolved.Status = normalizeScholarshipStatus(req.Status)
	resolved.ApplyLink = req.ApplyLink
	resolved.BannerBackgroundImageURL = req.BannerBackgroundImageURL
	resolved.CoverageArea = req.CoverageArea
	resolved.ContactEmail = req.ContactEmail
	resolved.PrimaryPhone = req.PrimaryPhone
	resolved.SecondaryPhone = req.SecondaryPhone
	resolved.WebsiteUrl = req.WebsiteUrl
	resolved.OfficeAddress = req.OfficeAddress
	resolved.MapUrl = req.MapUrl
	resolved.AboutParagraph1 = req.AboutParagraph1
	resolved.ScholarshipDescription2 = req.ScholarshipDescription2
	resolved.VideoTutorials = toJSON(req.VideoTutorials)
	resolved.JourneyTimeline = toJSON(req.JourneyTimeline)
	resolved.Timeline = toJSON(req.Timeline)
	resolved.ScholarshipSectionTitle = req.ScholarshipSectionTitle
	resolved.ScholarshipSubtitle = req.ScholarshipSubtitle
	resolved.ScholarshipDescription1 = req.ScholarshipDescription1
	resolved.ScholarshipTypes = toJSON(req.ScholarshipTypes)
	resolved.ScholarshipTypesNew = toJSON(req.ScholarshipTypesNew)
	resolved.SelectionRubric = toJSON(req.SelectionRubric)
	resolved.SelectionRubricNew = toJSON(req.SelectionRubricNew)
	resolved.EligibilitySectionTitle = req.EligibilitySectionTitle
	resolved.EligibilitySubtitle = req.EligibilitySubtitle
	resolved.BasicEligibilityCriteria = toJSON(req.BasicEligibilityCriteria)
	resolved.FullyFundedCriteria = toJSON(req.FullyFundedCriteria)
	resolved.PartiallyFundedCriteria = toJSON(req.PartiallyFundedCriteria)
	resolved.SelectionProcessSteps = toJSON(req.SelectionProcessSteps)
	resolved.RequiredDocuments = toJSON(req.RequiredDocuments)
	resolved.FAQs = toJSON(req.FAQs)
	resolved.FAQsNew = toJSON(req.FAQsNew)
	resolved.GalleryImages = toJSON(req.GalleryImages)
	resolved.GalleryImagesNew = toJSON(req.GalleryImagesNew)
	resolved.PartnerGroups = toJSON(req.PartnerGroups)
	resolved.ExamCenters = toJSON(req.ExamCenters)
	resolved.ExamCentersNew = toJSON(req.ExamCentersNew)
	resolved.Downloads = toJSON(req.Downloads)
	resolved.PaymentConfig = toJSON(req.PaymentConfig)
	if req.ApplicationStartDate != "" {
		if parsed, ok := parseOptionalTime(req.ApplicationStartDate); ok {
			resolved.ApplicationStartDate = parsed
		} else {
			return nil, errors.New("invalid application start date")
		}
	} else {
		resolved.ApplicationStartDate = time.Time{}
	}
	if deadlineValue != "" {
		if parsed, ok := parseOptionalTime(deadlineValue); ok {
			resolved.Deadline = parsed
			resolved.ApplicationEndDate = parsed
		} else {
			return nil, errors.New("invalid application end date")
		}
	} else {
		resolved.Deadline = time.Time{}
		resolved.ApplicationEndDate = time.Time{}
	}

	statusToSync := normalizeScholarshipStatus(scholarship.Status)
	if req.Status != "" {
		statusToSync = normalizeScholarshipStatus(req.Status)
	}
	if err := s.syncPublicScholarship(providerID, &resolved, statusToSync, true); err != nil {
		log.Printf("scholarshipprovider: UpdateScholarship syncPublicScholarship error: %v", err)
	}

	return scholarship, nil
}

func (s *Service) DeleteScholarship(providerID, id uint) error {
	scholarship, err := s.repo.GetScholarshipByIDAndProvider(id, providerID)
	if err != nil {
		return err
	}

	_ = s.repo.DeletePublicScholarshipByProviderScholarshipID(scholarship.ID)
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

	if err := s.repo.EvaluateApplication(application, req.Score, req.Passing, req.Notes); err != nil {
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
	if req.ContactNumber != "" {
		updates["contact_number"] = req.ContactNumber
	}
	if req.PANNumber != "" {
		updates["pan_number"] = req.PANNumber
	}
	if req.WebsiteURL != "" {
		updates["website_url"] = req.WebsiteURL
	}

	if err := s.repo.UpdateProviderProfile(provider, updates); err != nil {
		return nil, err
	}

	return provider, nil
}

func (s *Service) ChangePassword(providerID uint, req ChangePasswordRequest) error {
	provider, err := s.repo.GetProviderProfile(providerID)
	if err != nil {
		return err
	}

	if provider.Password == nil || *provider.Password == "" {
		return errors.New("password not set for this account")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*provider.Password), []byte(req.CurrentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"password": string(hashedPassword),
	}

	if err := s.repo.UpdateProviderProfile(provider, updates); err != nil {
		return err
	}

	return nil
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
	if req.ImageURL != "" {
		imageURL = &req.ImageURL
	}

	tagsBytes, _ := json.Marshal(req.Tags)

	news := &ProviderNews{
		ProviderID:    providerID,
		Title:         req.Title,
		ShortDesc:     req.ShortDesc,
		Content:       req.Content,
		ImageURL:      imageURL,
		NewsType:      req.NewsType,
		PublishedBy:   req.PublishedBy,
		Tags:          tagsBytes,
		AllowComments: req.AllowComments,
		Status:        status,
	}

	if req.PublishDate != "" {
		if t, err := time.Parse("2006-01-02", req.PublishDate); err == nil {
			news.PublishDate = &t
		}
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
		"title":          req.Title,
		"short_desc":     req.ShortDesc,
		"content":        req.Content,
		"news_type":      req.NewsType,
		"published_by":   req.PublishedBy,
		"allow_comments": req.AllowComments,
	}

	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}

	if req.Tags != nil {
		tagsBytes, _ := json.Marshal(req.Tags)
		updates["tags"] = tagsBytes
	}

	if req.PublishDate != "" {
		if t, err := time.Parse("2006-01-02", req.PublishDate); err == nil {
			updates["publish_date"] = t
		}
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
	if req.ImageURL != "" {
		imageURL = &req.ImageURL
	}

	tagsBytes, _ := json.Marshal(req.Tags)

	event := &ProviderEvent{
		ProviderID:         providerID,
		Name:               req.Name,
		ShortDesc:          req.ShortDesc,
		Description:        req.Description,
		ImageURL:           imageURL,
		EventType:          req.EventType,
		Category:           req.Category,
		MaxParticipants:    req.MaxParticipants,
		OnlineLink:         req.OnlineLink,
		OrganizedBy:        req.OrganizedBy,
		ContactPerson:      req.ContactPerson,
		ContactEmail:       req.ContactEmail,
		StartDate:          startDate,
		EndDate:            endDate,
		Location:           req.Location,
		Tags:               tagsBytes,
		EnableRegistration: req.EnableRegistration,
		Status:             status,
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
		"name":                req.Name,
		"short_desc":          req.ShortDesc,
		"description":         req.Description,
		"event_type":          req.EventType,
		"category":            req.Category,
		"max_participants":    req.MaxParticipants,
		"online_link":         req.OnlineLink,
		"organized_by":        req.OrganizedBy,
		"contact_person":      req.ContactPerson,
		"contact_email":       req.ContactEmail,
		"location":            req.Location,
		"enable_registration": req.EnableRegistration,
	}

	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}

	if req.Tags != nil {
		tagsBytes, _ := json.Marshal(req.Tags)
		updates["tags"] = tagsBytes
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

func (s *Service) GetPublishedNews(page, limit int) ([]ProviderNews, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 12
	}
	return s.repo.GetPublishedNews(page, limit)
}

func (s *Service) GetPublishedNewsByID(id uint) (*ProviderNews, error) {
	return s.repo.GetPublishedNewsByID(id)
}

func (s *Service) GetPublishedEvents(page, limit int) ([]ProviderEvent, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 12
	}
	return s.repo.GetPublishedEvents(page, limit)
}

func (s *Service) GetPublishedEventByID(id uint) (*ProviderEvent, error) {
	return s.repo.GetPublishedEventByID(id)
}

func toAccessUserResponse(user *ProviderAccessUser) *AccessUserResponse {
	var perms []string
	json.Unmarshal(user.Permissions, &perms)
	return &AccessUserResponse{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		ProviderID:  user.ProviderID,
		Name:        user.Name,
		Email:       user.Email,
		Role:        user.Role,
		RoleLabel:   user.RoleLabel,
		Status:      user.Status,
		LastActive:  user.LastActive,
		Avatar:      user.Avatar,
		Permissions: perms,
	}
}

func (s *Service) CreateAccessUser(req CreateAccessUserRequest, providerID uint) (*AccessUserResponse, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	role := "user"
	if req.Role != "" {
		role = req.Role
	}
	roleLabel := "User"
	if req.RoleLabel != "" {
		roleLabel = req.RoleLabel
	}

	var perms []string
	if len(req.Permissions) > 0 {
		perms = req.Permissions
	}

	user := &ProviderAccessUser{
		ProviderID:  providerID,
		Name:        req.Name,
		Email:       req.Email,
		Password:    string(hashedPassword),
		Role:        role,
		RoleLabel:   roleLabel,
		Status:      "Active",
		LastActive:  time.Now(),
		Avatar:      fmt.Sprintf("https://i.pravatar.cc/150?u=%d", time.Now().UnixNano()),
		Permissions: toJSON(perms),
	}

	if err := s.repo.CreateAccessUser(user); err != nil {
		return nil, err
	}

	return toAccessUserResponse(user), nil
}

func (s *Service) GetAccessUsers(providerID uint, page, limit int) (*AccessUserListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	users, total, err := s.repo.GetAccessUsers(providerID, page, limit)
	if err != nil {
		return nil, err
	}

	var userResponses []AccessUserResponse
	for _, user := range users {
		userResponses = append(userResponses, *toAccessUserResponse(&user))
	}

	return &AccessUserListResponse{
		Users: userResponses,
		Meta: PaginationMeta{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	}, nil
}

func (s *Service) GetAccessUser(id uint) (*AccessUserResponse, error) {
	user, err := s.repo.GetAccessUserByID(id)
	if err != nil {
		return nil, err
	}
	return toAccessUserResponse(user), nil
}

func (s *Service) UpdateAccessUser(id uint, req UpdateAccessUserRequest) (*AccessUserResponse, error) {
	user, err := s.repo.GetAccessUserByID(id)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
		user.Name = req.Name
	}
	if req.Email != "" {
		updates["email"] = req.Email
		user.Email = req.Email
	}
	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		updates["password"] = string(hashedPassword)
		user.Password = string(hashedPassword)
	}
	if req.Role != "" {
		updates["role"] = req.Role
		user.Role = req.Role
	}
	if req.RoleLabel != "" {
		updates["role_label"] = req.RoleLabel
		user.RoleLabel = req.RoleLabel
	}
	if req.Status != "" {
		updates["status"] = req.Status
		user.Status = req.Status
	}
	if len(req.Permissions) > 0 {
		updates["permissions"] = toJSON(req.Permissions)
		user.Permissions = toJSON(req.Permissions)
	}

	if err := s.repo.UpdateAccessUser(user); err != nil {
		return nil, err
	}

	return toAccessUserResponse(user), nil
}

func (s *Service) DeleteAccessUser(id uint) error {
	return s.repo.DeleteAccessUser(id)
}

func (s *Service) UpdatePermissions(id uint, permissions []string) error {
	return s.repo.UpdateAccessUserPermissions(id, toJSON(permissions))
}

func (s *Service) LoginAccessUser(email, password string, providerID uint) (*AccessUserResponse, error) {
	log.Printf("[DEBUG] LoginAccessUser: email=%s, providerID=%d", email, providerID)
	
	users, _, err := s.repo.GetAccessUsers(providerID, 1, 1000)
	if err != nil || len(users) == 0 {
		log.Printf("[DEBUG] No users found with providerID=%d, trying 1", providerID)
		users, _, err = s.repo.GetAccessUsers(1, 1, 1000)
	}
	if err != nil || len(users) == 0 {
		log.Printf("[DEBUG] No users found at all")
		return nil, errors.New("invalid credentials")
	}

	log.Printf("[DEBUG] Found %d users", len(users))

	var user *ProviderAccessUser
	for _, u := range users {
		log.Printf("[DEBUG] Checking user: email=%s, providerID=%d", u.Email, u.ProviderID)
		if u.Email == email && (providerID == 0 || u.ProviderID == providerID) {
			user = &u
			break
		}
	}
	if user == nil {
		log.Printf("[DEBUG] User not found")
		return nil, errors.New("invalid credentials")
	}

	log.Printf("[DEBUG] Found user: %s, stored password len: %d", user.Name, len(user.Password))

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Printf("[DEBUG] Password mismatch: %v", err)
		return nil, errors.New("invalid credentials")
	}

	user.LastActive = time.Now()
	if err := s.repo.UpdateAccessUser(user); err != nil {
		return nil, err
	}

	return toAccessUserResponse(user), nil
}

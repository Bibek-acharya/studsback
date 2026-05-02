package scholarshipprovider

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
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

	var imageURL *string
	if req.ImageURL != "" {
		imageURL = &req.ImageURL
	}

	var bannerBG *string
	if req.BannerBackgroundImageURL != "" {
		bannerBG = &req.BannerBackgroundImageURL
	}

	scholarship := &ProviderScholarship{
		ProviderID:               providerID,
		Title:                    req.Title,
		Description:              req.Description,
		ImageURL:                 imageURL,
		Location:                 req.Location,
		Value:                    req.Value,
		TotalSeats:               req.TotalSeats,
		AmountPerStudent:         req.AmountPerStudent,
		DegreeLevel:              req.DegreeLevel,
		FundingType:              req.FundingType,
		ScholarshipType:          req.ScholarshipType,
		FieldOfStudy:             toJSON(req.FieldOfStudy),
		Status:                   status,
		BannerBackgroundImageURL: bannerBG,
		PaymentConfig:            toJSON(req.PaymentConfig),
		ApplicationStartDate:     time.Time{},
		ResultPublicationDate:    time.Time{},
		AboutParagraph1:          req.AboutParagraph1,
		AboutParagraph2:          req.AboutParagraph2,
		VideoTutorials:           toJSON(req.VideoTutorials),
		JourneyTimeline:          toJSON(req.JourneyTimeline),
		ScholarshipSectionTitle:  req.ScholarshipSectionTitle,
		ScholarshipSubtitle:      req.ScholarshipSubtitle,
		ScholarshipDescription1:  req.ScholarshipDescription1,
		ScholarshipDescription2:  req.ScholarshipDescription2,
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

		// New fields from prototype
		ProviderName:          req.ProviderName,
		FundingTypeOther:     req.FundingTypeOther,
		ScholarshipTypeOther: req.ScholarshipTypeOther,
		EducationLevel:       req.EducationLevel,
		EducationLevelOther:  req.EducationLevelOther,
		ApplyLink:            req.ApplyLink,
		CoverageArea:         req.CoverageArea,
		ContactEmail:         req.ContactEmail,
		PrimaryPhone:         req.PrimaryPhone,
		SecondaryPhone:       req.SecondaryPhone,
		WebsiteUrl:           req.WebsiteUrl,
		OfficeAddress:        req.OfficeAddress,
		MapUrl:               req.MapUrl,
		PaymentConfig:        toJSON(req.PaymentConfig),
	}

	if req.ApplicationStartDate != "" {
		if parsed, ok := parseOptionalTime(req.ApplicationStartDate); ok {
			scholarship.ApplicationStartDate = parsed
		} else {
			return nil, errors.New("invalid application start date")
		}
	}
	if req.ResultPublicationDate != "" {
		if parsed, ok := parseOptionalTime(req.ResultPublicationDate); ok {
			scholarship.ResultPublicationDate = parsed
		} else {
			return nil, errors.New("invalid result publication date")
		}
	}
	if req.Deadline != "" {
		if deadline, err := parseTime(req.Deadline); err == nil {
			scholarship.Deadline = deadline
		} else {
			return nil, errors.New("invalid application end date")
		}
	} else if parsed, ok := parseOptionalTime(req.ApplicationEndDate); ok {
		scholarship.Deadline = parsed
	} else if req.ApplicationEndDate != "" {
		return nil, errors.New("invalid application end date")
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
		Title:                 scholarship.Title,
		Provider:              provider.ProviderName,
		Location:              scholarship.Location,
		Value:                 scholarship.Value,
		Deadline:              scholarship.Deadline,
		TotalSeats:            scholarship.TotalSeats,
		AmountPerStudent:      scholarship.AmountPerStudent,
		ApplicationStartDate:  scholarship.ApplicationStartDate,
		ResultPublicationDate: scholarship.ResultPublicationDate,
		DegreeLevel:           scholarship.DegreeLevel,
		FundingType:           scholarship.FundingType,
		ScholarshipType:       scholarship.ScholarshipType,
		Description:           scholarship.Description,
		ImageURL:              "",
		FieldOfStudy:          scholarship.FieldOfStudy,
		EligibilityCriteria:   scholarship.EligibilityCriteria,
		RequiredDocuments:     scholarship.RequiredDocuments,
		PaymentConfig:         scholarship.PaymentConfig,
		ProviderScholarshipID: &scholarship.ID,
	}
	if scholarship.ImageURL != nil {
		publicScholarship.ImageURL = *scholarship.ImageURL
	}

	existing, err := s.repo.FindPublicScholarshipByProviderScholarshipID(scholarship.ID)
	if err == nil && existing != nil {
		updates := map[string]interface{}{
			"title":                   publicScholarship.Title,
			"provider":                publicScholarship.Provider,
			"location":                publicScholarship.Location,
			"value":                   publicScholarship.Value,
			"deadline":                publicScholarship.Deadline,
			"total_seats":             publicScholarship.TotalSeats,
			"amount_per_student":      publicScholarship.AmountPerStudent,
			"application_start_date":  publicScholarship.ApplicationStartDate,
			"result_publication_date": publicScholarship.ResultPublicationDate,
			"degree_level":            publicScholarship.DegreeLevel,
			"funding_type":            publicScholarship.FundingType,
			"scholarship_type":        publicScholarship.ScholarshipType,
			"description":             publicScholarship.Description,
			"image_url":               publicScholarship.ImageURL,
			"field_of_study":          publicScholarship.FieldOfStudy,
			"payment_config":          publicScholarship.PaymentConfig,
			"provider_scholarship_id": scholarship.ID,
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

	// Only update fields that are provided (non-empty)
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.Value != "" {
		updates["value"] = req.Value
	}
	if req.DegreeLevel != "" {
		updates["degree_level"] = req.DegreeLevel
	}
	if req.FundingType != "" {
		updates["funding_type"] = req.FundingType
	}
	if req.ScholarshipType != "" {
		updates["scholarship_type"] = req.ScholarshipType
	}
	if req.TotalSeats != 0 {
		updates["total_seats"] = req.TotalSeats
	}
	if req.AmountPerStudent != 0 {
		updates["amount_per_student"] = req.AmountPerStudent
	}
	if len(req.PaymentConfig) > 0 {
		updates["payment_config"] = req.PaymentConfig
	}
	if len(req.FieldOfStudy) > 0 {
		updates["field_of_study"] = toJSON(req.FieldOfStudy)
	}
	if req.Status != "" {
		updates["status"] = normalizeScholarshipStatus(req.Status)
	}
	if req.AboutParagraph1 != "" {
		updates["about_paragraph1"] = req.AboutParagraph1
	}
	if req.AboutParagraph2 != "" {
		updates["about_paragraph2"] = req.AboutParagraph2
	}
	if len(req.VideoTutorials) > 0 {
		updates["video_tutorials"] = toJSON(req.VideoTutorials)
	}
	if len(req.JourneyTimeline) > 0 {
		updates["journey_timeline"] = toJSON(req.JourneyTimeline)
	}
	if req.ScholarshipSectionTitle != "" {
		updates["scholarship_section_title"] = req.ScholarshipSectionTitle
	}
	if req.ScholarshipSubtitle != "" {
		updates["scholarship_subtitle"] = req.ScholarshipSubtitle
	}
	if req.ScholarshipDescription1 != "" {
		updates["scholarship_description1"] = req.ScholarshipDescription1
	}
	if req.ScholarshipDescription2 != "" {
		updates["scholarship_description2"] = req.ScholarshipDescription2
	}
	if len(req.ScholarshipTypes) > 0 {
		updates["scholarship_types"] = toJSON(req.ScholarshipTypes)
	}
	if len(req.SelectionRubric) > 0 {
		updates["selection_rubric"] = toJSON(req.SelectionRubric)
	}
	if req.EligibilitySectionTitle != "" {
		updates["eligibility_section_title"] = req.EligibilitySectionTitle
	}
	if req.EligibilitySubtitle != "" {
		updates["eligibility_subtitle"] = req.EligibilitySubtitle
	}
	if len(req.BasicEligibilityCriteria) > 0 {
		updates["basic_eligibility_criteria"] = toJSON(req.BasicEligibilityCriteria)
	}
	if len(req.FullyFundedCriteria) > 0 {
		updates["fully_funded_criteria"] = toJSON(req.FullyFundedCriteria)
	}
	if len(req.PartiallyFundedCriteria) > 0 {
		updates["partially_funded_criteria"] = toJSON(req.PartiallyFundedCriteria)
	}
	if len(req.SelectionProcessSteps) > 0 {
		updates["selection_process_steps"] = toJSON(req.SelectionProcessSteps)
	}
	if len(req.RequiredDocuments) > 0 {
		updates["required_documents"] = toJSON(req.RequiredDocuments)
	}
	if len(req.FAQs) > 0 {
		updates["faqs"] = toJSON(req.FAQs)
	}
	if len(req.GalleryImages) > 0 {
		updates["gallery_images"] = toJSON(req.GalleryImages)
	}
	if len(req.PartnerGroups) > 0 {
		updates["partner_groups"] = toJSON(req.PartnerGroups)
	}
	if len(req.ExamCenters) > 0 {
		updates["exam_centers"] = toJSON(req.ExamCenters)
	}
	if len(req.Downloads) > 0 {
		updates["downloads"] = toJSON(req.Downloads)
	}

	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}
	if req.BannerBackgroundImageURL != "" {
		updates["banner_background_image_url"] = req.BannerBackgroundImageURL
	}
	if req.ApplicationStartDate != "" {
		if parsed, ok := parseOptionalTime(req.ApplicationStartDate); ok {
			updates["application_start_date"] = parsed
		} else {
			return nil, errors.New("invalid application start date")
		}
	}
	if req.ResultPublicationDate != "" {
		if parsed, ok := parseOptionalTime(req.ResultPublicationDate); ok {
			updates["result_publication_date"] = parsed
		} else {
			return nil, errors.New("invalid result publication date")
		}
	}

	if req.Deadline != "" {
		if parsed, err := parseTime(req.Deadline); err == nil {
			updates["deadline"] = parsed
		} else {
			return nil, errors.New("invalid application end date")
		}
	} else if parsed, ok := parseOptionalTime(req.ApplicationEndDate); ok {
		updates["deadline"] = parsed
	} else if req.ApplicationEndDate != "" {
		return nil, errors.New("invalid application end date")
	}

	// New fields from prototype
	if req.ProviderName != "" {
		updates["provider_name"] = req.ProviderName
	}
	if req.FundingTypeOther != "" {
		updates["funding_type_other"] = req.FundingTypeOther
	}
	if req.ScholarshipTypeOther != "" {
		updates["scholarship_type_other"] = req.ScholarshipTypeOther
	}
	if req.EducationLevel != "" {
		updates["education_level"] = req.EducationLevel
	}
	if req.EducationLevelOther != "" {
		updates["education_level_other"] = req.EducationLevelOther
	}
	if req.ApplyLink != "" {
		updates["apply_link"] = req.ApplyLink
	}
	if req.CoverageArea != "" {
		updates["coverage_area"] = req.CoverageArea
	}
	if req.ContactEmail != "" {
		updates["contact_email"] = req.ContactEmail
	}
	if req.PrimaryPhone != "" {
		updates["primary_phone"] = req.PrimaryPhone
	}
	if req.SecondaryPhone != "" {
		updates["secondary_phone"] = req.SecondaryPhone
	}
	if req.WebsiteUrl != "" {
		updates["website_url"] = req.WebsiteUrl
	}
	if req.OfficeAddress != "" {
		updates["office_address"] = req.OfficeAddress
	}
	if req.MapUrl != "" {
		updates["map_url"] = req.MapUrl
	}
	if req.PaymentConfig != nil {
		updates["payment_config"] = toJSON(req.PaymentConfig)
	}

	// JSON fields
	if len(req.ScholarshipTypesNew) > 0 {
		updates["scholarship_types_new"] = toJSON(req.ScholarshipTypesNew)
	}
	if len(req.SelectionRubricNew) > 0 {
		updates["selection_rubric_new"] = toJSON(req.SelectionRubricNew)
	}
	if len(req.FAQsNew) > 0 {
		updates["faqs_new"] = toJSON(req.FAQsNew)
	}
	if len(req.GalleryImagesNew) > 0 {
		updates["gallery_images_new"] = toJSON(req.GalleryImagesNew)
	}
	if len(req.ExamCentersNew) > 0 {
		updates["exam_centers_new"] = toJSON(req.ExamCentersNew)
	}

	if len(updates) == 0 {
		return scholarship, nil
	}

	if err := s.repo.UpdateScholarship(scholarship, updates); err != nil {
		log.Printf("scholarshipprovider: UpdateScholarship repo.UpdateScholarship error: %v", err)
		return nil, err
	}

	resolved := *scholarship
	if req.Title != "" {
		resolved.Title = req.Title
	}
	if req.Description != "" {
		resolved.Description = req.Description
	}
	if req.Location != "" {
		resolved.Location = req.Location
	}
	if req.Value != "" {
		resolved.Value = req.Value
	}
	if req.DegreeLevel != "" {
		resolved.DegreeLevel = req.DegreeLevel
	}
	if req.FundingType != "" {
		resolved.FundingType = req.FundingType
	}
	if req.ScholarshipType != "" {
		resolved.ScholarshipType = req.ScholarshipType
	}
	if req.TotalSeats != 0 {
		resolved.TotalSeats = req.TotalSeats
	}
	if req.AmountPerStudent != 0 {
		resolved.AmountPerStudent = req.AmountPerStudent
	}
	if len(req.PaymentConfig) > 0 {
		resolved.PaymentConfig = req.PaymentConfig
	}
	if len(req.FieldOfStudy) > 0 {
		resolved.FieldOfStudy = toJSON(req.FieldOfStudy)
	}
	if req.AboutParagraph1 != "" {
		resolved.AboutParagraph1 = req.AboutParagraph1
	}
	if req.AboutParagraph2 != "" {
		resolved.AboutParagraph2 = req.AboutParagraph2
	}
	if len(req.VideoTutorials) > 0 {
		resolved.VideoTutorials = toJSON(req.VideoTutorials)
	}
	if len(req.JourneyTimeline) > 0 {
		resolved.JourneyTimeline = toJSON(req.JourneyTimeline)
	}
	if req.ScholarshipSectionTitle != "" {
		resolved.ScholarshipSectionTitle = req.ScholarshipSectionTitle
	}
	if req.ScholarshipSubtitle != "" {
		resolved.ScholarshipSubtitle = req.ScholarshipSubtitle
	}
	if req.ScholarshipDescription1 != "" {
		resolved.ScholarshipDescription1 = req.ScholarshipDescription1
	}
	if req.ScholarshipDescription2 != "" {
		resolved.ScholarshipDescription2 = req.ScholarshipDescription2
	}
	if len(req.ScholarshipTypes) > 0 {
		resolved.ScholarshipTypes = toJSON(req.ScholarshipTypes)
	}
	if len(req.SelectionRubric) > 0 {
		resolved.SelectionRubric = toJSON(req.SelectionRubric)
	}
	if req.EligibilitySectionTitle != "" {
		resolved.EligibilitySectionTitle = req.EligibilitySectionTitle
	}
	if req.EligibilitySubtitle != "" {
		resolved.EligibilitySubtitle = req.EligibilitySubtitle
	}
	if len(req.BasicEligibilityCriteria) > 0 {
		resolved.BasicEligibilityCriteria = toJSON(req.BasicEligibilityCriteria)
	}
	if len(req.FullyFundedCriteria) > 0 {
		resolved.FullyFundedCriteria = toJSON(req.FullyFundedCriteria)
	}
	if len(req.PartiallyFundedCriteria) > 0 {
		resolved.PartiallyFundedCriteria = toJSON(req.PartiallyFundedCriteria)
	}
	if len(req.SelectionProcessSteps) > 0 {
		resolved.SelectionProcessSteps = toJSON(req.SelectionProcessSteps)
	}
	if len(req.RequiredDocuments) > 0 {
		resolved.RequiredDocuments = toJSON(req.RequiredDocuments)
	}
	if len(req.FAQs) > 0 {
		resolved.FAQs = toJSON(req.FAQs)
	}
	if len(req.GalleryImages) > 0 {
		resolved.GalleryImages = toJSON(req.GalleryImages)
	}
	if len(req.PartnerGroups) > 0 {
		resolved.PartnerGroups = toJSON(req.PartnerGroups)
	}
	if len(req.ExamCenters) > 0 {
		resolved.ExamCenters = toJSON(req.ExamCenters)
	}
	if len(req.Downloads) > 0 {
		resolved.Downloads = toJSON(req.Downloads)
	}
	if req.ImageURL != "" {
		resolved.ImageURL = &req.ImageURL
	}
	if req.BannerBackgroundImageURL != "" {
		resolved.BannerBackgroundImageURL = &req.BannerBackgroundImageURL
	}
	if req.ApplicationStartDate != "" {
		if parsed, ok := parseOptionalTime(req.ApplicationStartDate); ok {
			resolved.ApplicationStartDate = parsed
		} else {
			return nil, errors.New("invalid application start date")
		}
	}
	if req.ResultPublicationDate != "" {
		if parsed, ok := parseOptionalTime(req.ResultPublicationDate); ok {
			resolved.ResultPublicationDate = parsed
		} else {
			return nil, errors.New("invalid result publication date")
		}
	}
	if req.Deadline != "" {
		if parsed, err := parseTime(req.Deadline); err == nil {
			resolved.Deadline = parsed
		} else {
			return nil, errors.New("invalid application end date")
		}
	} else if parsed, ok := parseOptionalTime(req.ApplicationEndDate); ok {
		resolved.Deadline = parsed
	} else if req.ApplicationEndDate != "" {
		return nil, errors.New("invalid application end date")
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

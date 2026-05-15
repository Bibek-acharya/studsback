package scholarshipprovider

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"studsphere/backend/internal/emailqueue"
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

	details, err := s.repo.GetDashboardDetails(providerID)
	if err != nil {
		return nil, err
	}

	// Proactive profile completeness check
	profile, err := s.repo.GetProviderProfile(providerID)
	if err == nil {
		isIncomplete := profile.ContactNumber == "" || profile.PANNumber == "" || profile.WebsiteURL == ""
		if isIncomplete {
			// Check if notification already exists
			exists, _ := s.repo.CheckNotificationExists(providerID, "Profile Incomplete")
			if !exists {
				s.repo.CreateNotification(&ProviderNotification{
					ProviderID: providerID,
					Title:      "Profile Incomplete",
					Message:    "Your profile is incomplete—please complete it to continue.",
					Type:       "system",
					Link:       "org-profile",
				})
			}
		}
	}

	return &DashboardResponse{
		TotalScholarships:   totalScholarships,
		TotalApplications:   totalApplications,
		PendingApplications: pendingApplications,
		TotalInterviews:     totalInterviews,
		UnreadMessages:      unreadMessages,
		ScholarshipStats:    details,
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

func (s *Service) GetDetailedAnalytics(providerID uint, filters DetailedAnalyticsFilters) (*DetailedAnalyticsResponse, error) {
	apps, err := s.repo.GetFilteredApplications(providerID, filters)
	if err != nil {
		return nil, err
	}

	res := &DetailedAnalyticsResponse{
		TotalApplicants:    len(apps),
		Gender:             []MetricCount{},
		Ethnicity:          []MetricCount{},
		GPABreakdown:       []MetricCount{},
		SchoolType:         []MetricCount{},
		Stream:             []MetricCount{},
		Province:           []MetricCount{},
		District:           []MetricCount{},
		Status:             []MetricCount{},
		PaymentMethods:     []MetricCount{},
		GPABySchoolType:    []MetricCount{},
		GenderByProvince:   []CrossMetric{},
		StreamByProvince:   []CrossMetric{},
		SchoolTypeByProvince: []CrossMetric{},
		ExamCenters:        []ExamCenterMetric{},
	}

	genderMap := make(map[string]int)
	ethnicityMap := make(map[string]int)
	schoolTypeMap := make(map[string]int)
	streamMap := make(map[string]int)
	provinceMap := make(map[string]int)
	districtMap := make(map[string]int)
	statusMap := make(map[string]int)
	districtSet := make(map[string]bool)

	gpaBins := map[string]int{
		"1.6 - 2.0": 0,
		"2.0 - 2.4": 0,
		"2.4 - 2.8": 0,
		"2.8 - 3.2": 0,
		"3.2 - 3.6": 0,
		"3.6 - 4.0": 0,
	}

	gpaBySchoolTypeSum := make(map[string]float64)
	gpaBySchoolTypeCount := make(map[string]int)

	genderByProvince := make(map[string]map[string]int)
	streamByProvince := make(map[string]map[string]int)
	schoolTypeByProvince := make(map[string]map[string]int)
	examCenterMap := make(map[string]*ExamCenterMetric)

	for _, app := range apps {
		if app.Gender != "" {
			genderMap[app.Gender]++
		}
		if app.Ethnicity != "" {
			ethnicityMap[app.Ethnicity]++
		}
		if app.SchoolType != "" {
			schoolTypeMap[app.SchoolType]++
		}
		if app.Stream != "" {
			streamMap[app.Stream]++
		}
		if app.Province != "" {
			provinceMap[app.Province]++
		}
		if app.District != "" {
			districtMap[app.District]++
			districtSet[app.District] = true
		}
		if app.Status != "" {
			statusMap[app.Status]++
		}

		if app.GPA > 0 {
			switch {
			case app.GPA <= 2.0:
				gpaBins["1.6 - 2.0"]++
			case app.GPA <= 2.4:
				gpaBins["2.0 - 2.4"]++
			case app.GPA <= 2.8:
				gpaBins["2.4 - 2.8"]++
			case app.GPA <= 3.2:
				gpaBins["2.8 - 3.2"]++
			case app.GPA <= 3.6:
				gpaBins["3.2 - 3.6"]++
			default:
				gpaBins["3.6 - 4.0"]++
			}
		}

		if app.SchoolType != "" {
			gpaBySchoolTypeSum[app.SchoolType] += app.GPA
			gpaBySchoolTypeCount[app.SchoolType]++
		}

		prov := app.Province
		if prov == "" {
			prov = "Unknown"
		}

		if app.Gender != "" {
			if genderByProvince[prov] == nil {
				genderByProvince[prov] = make(map[string]int)
			}
			genderByProvince[prov][app.Gender]++
		}

		if app.Stream != "" {
			if streamByProvince[prov] == nil {
				streamByProvince[prov] = make(map[string]int)
			}
			streamByProvince[prov][app.Stream]++
		}

		if app.SchoolType != "" {
			if schoolTypeByProvince[prov] == nil {
				schoolTypeByProvince[prov] = make(map[string]int)
			}
			schoolTypeByProvince[prov][app.SchoolType]++
		}

		if app.ExamCenter != "" {
			stream := app.Stream
			if existing, ok := examCenterMap[app.ExamCenter]; ok {
				if stream == "Science" || stream == "science" {
					existing.Science++
				} else {
					existing.Management++
				}
			} else {
				m := 0
				s := 0
				if stream == "Science" || stream == "science" {
					s = 1
				} else {
					m = 1
				}
				examCenterMap[app.ExamCenter] = &ExamCenterMetric{
					Name:       app.ExamCenter,
					Management: m,
					Science:    s,
				}
			}
		}
	}

	res.Gender = mapToMetricCount(genderMap)
	res.Ethnicity = mapToMetricCount(ethnicityMap)
	res.SchoolType = mapToMetricCount(schoolTypeMap)
	res.Stream = mapToMetricCount(streamMap)
	res.Province = mapToMetricCount(provinceMap)
	res.District = mapToMetricCount(districtMap)
	res.Status = mapToMetricCount(statusMap)
	res.DistrictCount = len(districtSet)

	res.GPABreakdown = []MetricCount{
		{Label: "1.6 - 2.0", Count: gpaBins["1.6 - 2.0"]},
		{Label: "2.0 - 2.4", Count: gpaBins["2.0 - 2.4"]},
		{Label: "2.4 - 2.8", Count: gpaBins["2.4 - 2.8"]},
		{Label: "2.8 - 3.2", Count: gpaBins["2.8 - 3.2"]},
		{Label: "3.2 - 3.6", Count: gpaBins["3.2 - 3.6"]},
		{Label: "3.6 - 4.0", Count: gpaBins["3.6 - 4.0"]},
	}

	for schoolType, sum := range gpaBySchoolTypeSum {
		if count := gpaBySchoolTypeCount[schoolType]; count > 0 {
			avg := sum / float64(count)
			res.GPABySchoolType = append(res.GPABySchoolType, MetricCount{
				Label: schoolType,
				Count: int(avg * 100),
			})
		}
	}
	if len(res.GPABySchoolType) == 0 {
		res.GPABySchoolType = []MetricCount{}
	}

	for province, genderCounts := range genderByProvince {
		cm := CrossMetric{Label: province, Values: []MetricCount{}}
		for gender, count := range genderCounts {
			cm.Values = append(cm.Values, MetricCount{Label: gender, Count: count})
		}
		res.GenderByProvince = append(res.GenderByProvince, cm)
	}
	if len(res.GenderByProvince) == 0 {
		res.GenderByProvince = []CrossMetric{}
	}

	for province, streamCounts := range streamByProvince {
		cm := CrossMetric{Label: province, Values: []MetricCount{}}
		for stream, count := range streamCounts {
			cm.Values = append(cm.Values, MetricCount{Label: stream, Count: count})
		}
		res.StreamByProvince = append(res.StreamByProvince, cm)
	}
	if len(res.StreamByProvince) == 0 {
		res.StreamByProvince = []CrossMetric{}
	}

	for province, stCounts := range schoolTypeByProvince {
		cm := CrossMetric{Label: province, Values: []MetricCount{}}
		for st, count := range stCounts {
			cm.Values = append(cm.Values, MetricCount{Label: st, Count: count})
		}
		res.SchoolTypeByProvince = append(res.SchoolTypeByProvince, cm)
	}
	if len(res.SchoolTypeByProvince) == 0 {
		res.SchoolTypeByProvince = []CrossMetric{}
	}

	for _, ec := range examCenterMap {
		res.ExamCenters = append(res.ExamCenters, *ec)
	}
	if len(res.ExamCenters) == 0 {
		res.ExamCenters = []ExamCenterMetric{}
	}

	// Get payment data for admit cards and payment methods
	scholarshipAppIDs := make([]uint, 0)
	for _, app := range apps {
		if app.ScholarshipApplicationID != nil {
			scholarshipAppIDs = append(scholarshipAppIDs, *app.ScholarshipApplicationID)
		}
	}

	if len(scholarshipAppIDs) > 0 {
		payments, err := s.repo.GetPaymentsByApplicationIDs(scholarshipAppIDs)
		if err == nil {
			paymentMethods := make(map[string]int)
			for _, p := range payments {
				if p.Method != "" {
					paymentMethods[p.Method]++
				}
				if p.Status == "completed" {
					res.AdmitCardsSent++
				} else {
					res.AdmitCardsPending++
				}
			}
			res.PaymentMethods = mapToMetricCount(paymentMethods)
		}
	}

	return res, nil
}

func mapToMetricCount(m map[string]int) []MetricCount {
	counts := make([]MetricCount, 0, len(m))
	for label, count := range m {
		if label == "" {
			label = "Unknown"
		}
		counts = append(counts, MetricCount{Label: label, Count: count})
	}
	return counts
}

func parseTime(s string) (time.Time, error) {
	formats := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05", "2006-01-02"}
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

	var applicationStartDate time.Time
	if parsed, ok := parseOptionalTime(req.ApplicationStartDate); ok {
		applicationStartDate = parsed
	}
	var deadlineTime time.Time
	if parsed, ok := parseOptionalTime(deadlineValue); ok {
		deadlineTime = parsed
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
		Deadline:                 deadlineTime,
		ApplicationStartDate:     applicationStartDate,
		ApplicationEndDate:       deadlineTime,
		DegreeLevel:              req.DegreeLevel,
		FundingType:              req.FundingType,
		ScholarshipType:          req.ScholarshipType,
		FieldOfStudy:             toJSON(req.FieldOfStudy),
		Status:                   status,
		ApplyLink:                req.ApplyLink,
		ImageURL:                 req.BannerBackgroundImageURL,
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
		PartnerMessages:          toJSON(req.PartnerMessages),
		ExamCenters:              toJSON(req.ExamCenters),
		ExamCentersNew:           toJSON(req.ExamCentersNew),
		Downloads:                toJSON(req.Downloads),
		PaymentConfig:            toJSON(req.PaymentConfig),
		ExamDate:                 req.ExamDate,
		ExamTime:                 req.ExamTime,
	}

	if err := s.repo.CreateScholarship(scholarship); err != nil {
		return nil, err
	}

	if err := s.syncPublicScholarship(scholarship, status, false); err != nil {
		return nil, err
	}

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "Scholarship Created",
		Message:    "Your scholarship has been created successfully.",
		Type:       "scholarship",
		Link:       "manage-scholarships",
	})

	return scholarship, nil
}

func (s *Service) syncPublicScholarship(scholarship *ProviderScholarship, status string, isUpdate bool) error {
	if normalizeScholarshipStatus(status) != "published" {
		if isUpdate {
			return s.repo.DeletePublicScholarshipByProviderScholarshipID(scholarship.ID)
		}
		return nil
	}

	publicScholarship := &publicscholarship.Scholarship{
		Title:                    scholarship.Title,
		Provider:                 scholarship.Provider,
		Location:                 scholarship.Location,
		Value:                    scholarship.Value,
		Deadline:                 scholarship.Deadline,
		ApplicationStartDate:     scholarship.ApplicationStartDate,
		DegreeLevel:              scholarship.DegreeLevel,
		FundingType:              scholarship.FundingType,
		ScholarshipType:          scholarship.ScholarshipType,
		Description:              scholarship.Description,
		ImageURL:                 scholarship.BannerBackgroundImageURL,
		BannerBackgroundImageURL: scholarship.BannerBackgroundImageURL,
		FieldOfStudy:             scholarship.FieldOfStudy,
		EligibilityCriteria:      scholarship.BasicEligibilityCriteria,
		RequiredDocuments:        scholarship.RequiredDocuments,
		PaymentConfig:            scholarship.PaymentConfig,
		ProviderScholarshipID:    &scholarship.ID,
		ProviderName:             scholarship.ProviderName,
		FundingTypeOther:         scholarship.FundingTypeOther,
		ScholarshipTypeOther:     scholarship.ScholarshipTypeOther,
		EducationLevel:           scholarship.EducationLevel,
		EducationLevelOther:      scholarship.EducationLevelOther,
		ApplyLink:                scholarship.ApplyLink,
		CoverageArea:             scholarship.CoverageArea,
		ContactEmail:             scholarship.ContactEmail,
		PrimaryPhone:             scholarship.PrimaryPhone,
		SecondaryPhone:           scholarship.SecondaryPhone,
		WebsiteUrl:               scholarship.WebsiteUrl,
		OfficeAddress:            scholarship.OfficeAddress,
		MapUrl:                   scholarship.MapUrl,
		AboutParagraph1:          scholarship.AboutParagraph1,
		VideoTutorials:           scholarship.VideoTutorials,
		JourneyTimeline:          scholarship.JourneyTimeline,
		Timeline:                 scholarship.Timeline,
		ScholarshipSectionTitle:  scholarship.ScholarshipSectionTitle,
		ScholarshipSubtitle:      scholarship.ScholarshipSubtitle,
		ScholarshipDescription1:  scholarship.ScholarshipDescription1,
		ScholarshipDescription2:  scholarship.ScholarshipDescription2,
		ScholarshipTypes:         scholarship.ScholarshipTypes,
		ScholarshipTypesNew:      scholarship.ScholarshipTypesNew,
		SelectionRubric:          scholarship.SelectionRubric,
		SelectionRubricNew:       scholarship.SelectionRubricNew,
		EligibilitySectionTitle:  scholarship.EligibilitySectionTitle,
		EligibilitySubtitle:      scholarship.EligibilitySubtitle,
		BasicEligibilityCriteria: scholarship.BasicEligibilityCriteria,
		FullyFundedCriteria:      scholarship.FullyFundedCriteria,
		PartiallyFundedCriteria:  scholarship.PartiallyFundedCriteria,
		SelectionProcessSteps:    scholarship.SelectionProcessSteps,
		FAQsNew:                  scholarship.FAQsNew,
		GalleryImages:            scholarship.GalleryImages,
		GalleryImagesNew:         scholarship.GalleryImagesNew,
		PartnerGroups:            scholarship.PartnerGroups,
		PartnerMessages:          scholarship.PartnerMessages,
		ExamCenters:              scholarship.ExamCenters,
		ExamCentersNew:           scholarship.ExamCentersNew,
		Downloads:                scholarship.Downloads,
		ExamDate:                 scholarship.ExamDate,
		ExamTime:                 scholarship.ExamTime,
	}

	log.Printf("scholarshipprovider: syncPublicScholarship - syncing providerScholarshipID=%d", scholarship.ID)
	existing, err := s.repo.FindPublicScholarshipByProviderScholarshipID(scholarship.ID)
	if err == nil && existing != nil {
		log.Printf("scholarshipprovider: syncPublicScholarship - updating existing public scholarship ID=%d", existing.ID)
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
			"image_url":                   publicScholarship.ImageURL,
			"banner_background_image_url": publicScholarship.BannerBackgroundImageURL,
			"field_of_study":              publicScholarship.FieldOfStudy,
			"eligibility_criteria":        publicScholarship.EligibilityCriteria,
			"required_documents":          publicScholarship.RequiredDocuments,
			"payment_config":              publicScholarship.PaymentConfig,
			"provider_scholarship_id":     scholarship.ID,
			"provider_name":               publicScholarship.ProviderName,
			"funding_type_other":         publicScholarship.FundingTypeOther,
			"scholarship_type_other":     publicScholarship.ScholarshipTypeOther,
			"education_level":           publicScholarship.EducationLevel,
			"education_level_other":      publicScholarship.EducationLevelOther,
			"apply_link":                publicScholarship.ApplyLink,
			"coverage_area":             publicScholarship.CoverageArea,
			"contact_email":             publicScholarship.ContactEmail,
			"primary_phone":             publicScholarship.PrimaryPhone,
			"secondary_phone":           publicScholarship.SecondaryPhone,
			"website_url":               publicScholarship.WebsiteUrl,
			"office_address":            publicScholarship.OfficeAddress,
			"map_url":                   publicScholarship.MapUrl,
			"about_paragraph_1":          publicScholarship.AboutParagraph1,
			"video_tutorials":           publicScholarship.VideoTutorials,
			"journey_timeline":          publicScholarship.JourneyTimeline,
			"timeline":                 publicScholarship.Timeline,
			"scholarship_section_title": publicScholarship.ScholarshipSectionTitle,
			"scholarship_subtitle":      publicScholarship.ScholarshipSubtitle,
			"scholarship_description_1": publicScholarship.ScholarshipDescription1,
			"scholarship_description_2": publicScholarship.ScholarshipDescription2,
			"scholarship_types":         publicScholarship.ScholarshipTypes,
			"scholarship_types_new":     publicScholarship.ScholarshipTypesNew,
			"selection_rubric":          publicScholarship.SelectionRubric,
			"selection_rubric_new":       publicScholarship.SelectionRubricNew,
			"eligibility_section_title": publicScholarship.EligibilitySectionTitle,
			"eligibility_subtitle":      publicScholarship.EligibilitySubtitle,
			"basic_eligibility_criteria": publicScholarship.BasicEligibilityCriteria,
			"fully_funded_criteria":      publicScholarship.FullyFundedCriteria,
			"partially_funded_criteria":  publicScholarship.PartiallyFundedCriteria,
			"selection_process_steps":    publicScholarship.SelectionProcessSteps,
			"faqs_new":                  publicScholarship.FAQsNew,
			"gallery_images":            publicScholarship.GalleryImages,
			"gallery_images_new":         publicScholarship.GalleryImagesNew,
			"partner_groups":            publicScholarship.PartnerGroups,
			"partner_messages":          publicScholarship.PartnerMessages,
			"exam_centers":              publicScholarship.ExamCenters,
			"exam_centers_new":           publicScholarship.ExamCentersNew,
			"downloads":                publicScholarship.Downloads,
			"exam_date":                publicScholarship.ExamDate,
			"exam_time":                publicScholarship.ExamTime,
		}
		return s.repo.UpdatePublicScholarship(existing.ID, updates)
	}

	log.Printf("scholarshipprovider: syncPublicScholarship - creating new public scholarship")
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
	updates["image_url"] = req.BannerBackgroundImageURL
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
	updates["partner_messages"] = toJSON(req.PartnerMessages)
	updates["exam_centers"] = toJSON(req.ExamCenters)
	updates["exam_centers_new"] = toJSON(req.ExamCentersNew)
	updates["downloads"] = toJSON(req.Downloads)
	updates["payment_config"] = toJSON(req.PaymentConfig)
	updates["exam_date"] = req.ExamDate
	updates["exam_time"] = req.ExamTime

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
		parsed, ok := parseOptionalTime(deadlineValue)
		if ok {
			if !parsed.IsZero() {
				updates["deadline"] = parsed
				updates["application_end_date"] = parsed
			} else {
				updates["deadline"] = time.Time{}
				updates["application_end_date"] = time.Time{}
			}
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
	resolved.ImageURL = req.BannerBackgroundImageURL
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
	resolved.PartnerMessages = toJSON(req.PartnerMessages)
	resolved.ExamCenters = toJSON(req.ExamCenters)
	resolved.ExamCentersNew = toJSON(req.ExamCentersNew)
	resolved.Downloads = toJSON(req.Downloads)
	resolved.PaymentConfig = toJSON(req.PaymentConfig)
	resolved.ExamDate = req.ExamDate
	resolved.ExamTime = req.ExamTime
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
		parsed, ok := parseOptionalTime(deadlineValue)
		if ok {
			if !parsed.IsZero() {
				resolved.Deadline = parsed
				resolved.ApplicationEndDate = parsed
			} else {
				resolved.Deadline = time.Time{}
				resolved.ApplicationEndDate = time.Time{}
			}
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
	if err := s.syncPublicScholarship(&resolved, statusToSync, true); err != nil {
		log.Printf("scholarshipprovider: UpdateScholarship syncPublicScholarship error: %v", err)
	}

	message := "Your scholarship draft has been updated."
	title := "Scholarship Updated"
	if statusToSync == "published" || statusToSync == "active" {
		message = "Your scholarship is now live and visible in the directory."
		title = "Scholarship Published"
	}

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      title,
		Message:    message,
		Type:       "scholarship",
		Link:       "manage-scholarships",
	})

	return &resolved, nil
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

	applications, total, err := s.repo.GetApplicationsByProvider(providerID, page, limit, status, scholarshipID)
	if err != nil {
		return nil, 0, err
	}

	// Auto-approve pending eSewa payments and send admit cards
	for i, app := range applications {
		if app.Payment != nil && app.Payment.Method == "esewa" && app.Payment.Status == "pending" && app.ScholarshipApplicationID != nil {
			payment, lookupErr := s.repo.FindPaymentByApplicationID(*app.ScholarshipApplicationID)
			if lookupErr != nil {
				continue
			}
			payment.Status = "completed"
			now := time.Now()
			payment.PaidAt = &now
			if updateErr := s.repo.UpdatePayment(payment); updateErr != nil {
				continue
			}
			applications[i].Payment.Status = "completed"
			applications[i].Payment.PaidAt = &now
			go s.sendAdmitCard(&app, payment)
		}
	}

	return applications, total, nil
}

func (s *Service) GetApplicationByID(providerID, id uint) (*ProviderApplication, error) {
	app, err := s.repo.GetApplicationByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}

	// Auto-approve eSewa payments — eSewa only reports successful transactions
	if app.Payment != nil && app.Payment.Method == "esewa" && app.Payment.Status == "pending" {
		payment, lookupErr := s.repo.FindPaymentByApplicationID(*app.ScholarshipApplicationID)
		if lookupErr == nil {
			payment.Status = "completed"
			now := time.Now()
			payment.PaidAt = &now
			if updateErr := s.repo.UpdatePayment(payment); updateErr == nil {
				app.Payment.Status = "completed"
				app.Payment.PaidAt = &now
				s.sendAdmitCard(app, payment)
			}
		}
	}

	return app, nil
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

func (s *Service) ApproveApplicationPayment(providerID uint, applicationID uint, approve bool, reason string) (*ProviderApplication, error) {
	application, err := s.repo.GetApplicationByIDAndProvider(applicationID, providerID)
	if err != nil {
		return nil, errors.New("application not found")
	}

	if application.ScholarshipApplicationID == nil {
		return nil, errors.New("no linked scholarship application")
	}
	payment, err := s.repo.FindPaymentByApplicationID(*application.ScholarshipApplicationID)
	if err != nil {
		return nil, errors.New("payment not found")
	}

	if approve {
		payment.Status = "completed"
		now := time.Now()
		payment.PaidAt = &now
		payment.ApprovedBy = providerID

		application.Status = "pending"

		if err := s.repo.UpdatePayment(payment); err != nil {
			return nil, errors.New("failed to update payment")
		}

		if _, err := s.repo.UpdateApplicationStatusOnly(application.ID, "pending"); err != nil {
			return nil, errors.New("failed to update application status")
		}

		// Send admit card
		go s.sendAdmitCard(application, payment)
	} else {
		payment.Status = "failed"
		payment.RejectionReason = reason

		if err := s.repo.UpdatePayment(payment); err != nil {
			return nil, errors.New("failed to update payment")
		}

		if application.Email != "" {
			reasonText := reason
			if reasonText == "" {
				reasonText = "No specific reason provided."
			}
			subject := "Payment Status Update - StudSphere"
			html := fmt.Sprintf(`
<p>Dear %s,</p>
<p>Your payment for the application has been <strong>rejected</strong>.</p>
<div style="background:#fef2f2;border:1px solid #fecaca;border-radius:8px;padding:16px;margin:16px 0;">
    <p style="margin:0 0 4px 0;font-weight:600;color:#991b1b;">Reason:</p>
    <p style="margin:0;color:#b91c1c;">%s</p>
</div>
<p>Please contact support if you have any questions.</p>
<div class="signature">
    <p>Best Regards,</p>
    <p>Team Studsphere</p>
</div>`, application.FullName, reasonText)
			go emailqueue.EnqueueGenericEmail(application.Email, subject, html)
		}
	}

	return application, nil
}

func (s *Service) sendAdmitCard(application *ProviderApplication, payment *publicscholarship.Payment) {
	scholarship, err := s.repo.FindScholarshipByID(payment.ScholarshipID)
	if err != nil {
		log.Printf("sendAdmitCard: scholarship not found: %v", err)
		return
	}

	dobStr := ""
	if !application.DateOfBirthAD.IsZero() {
		dobStr = application.DateOfBirthAD.Format("02-Jan-2006")
	} else if application.DateOfBirthBS != "" {
		dobStr = application.DateOfBirthBS
	}

	cardData := publicscholarship.AdmitCardData{
		CandidateName:    application.FullName,
		DateOfBirth:      dobStr,
		Gender:           application.Gender,
		RollNumber:       application.RollNumber,
		ExamCentre:       application.ExamCenter,
		Stream:           application.Stream,
		PhotoURL:         publicscholarship.PhotoToBase64(application.PhotoURL),
		ScholarshipTitle: scholarship.Title,
		Provider:         scholarship.Provider,
		ExamDate:         scholarship.ExamDate,
		ExamTime:         scholarship.ExamTime,
		SubjectName:      application.Stream,
	}

	pdfBytes, err := publicscholarship.GenerateAdmitCardPDF(cardData, func() string {
		if seq, err := s.repo.GetNextRollNumber(); err == nil {
			rn := fmt.Sprintf("PS-%05d", seq)
			s.repo.UpdateRollNumber(application.ID, rn)
			application.RollNumber = rn
			return rn
		}
		return ""
	})
	if err != nil {
		_ = emailqueue.SendAdmitCardEmail(application.Email, application.FullName, scholarship.Title, nil)
		return
	}

	_ = emailqueue.SendAdmitCardEmail(application.Email, application.FullName, scholarship.Title, pdfBytes)
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

	if req.Status == "rejected" {
		reason := req.Reason
		if len(reason) > 250 {
			reason = reason[:250]
		}
		application.RejectionReason = reason
	}

	if err := s.repo.UpdateApplicationStatus(application, req.Status); err != nil {
		return nil, err
	}

	message := fmt.Sprintf("You have %s an application.", req.Status)
	switch req.Status {
	case "shortlisted":
		message = "You have shortlisted an application."
	case "rejected":
		message = "You have rejected an application."
	}

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "Application Status Updated",
		Message:    message,
		Type:       "application",
		Link:       "applications",
	})

	if req.Status == "rejected" && application.Email != "" {
		reason := application.RejectionReason
		if reason == "" {
			reason = "No specific reason provided."
		}
		subject := "Application Status Update - StudSphere"
		html := fmt.Sprintf(`
<p>Dear %s,</p>
<p>We regret to inform you that your application has been <strong>rejected</strong>.</p>
<div style="background:#fef2f2;border:1px solid #fecaca;border-radius:8px;padding:16px;margin:16px 0;">
    <p style="margin:0 0 4px 0;font-weight:600;color:#991b1b;">Reason:</p>
    <p style="margin:0;color:#b91c1c;">%s</p>
</div>
<p>We appreciate your interest and encourage you to apply for future opportunities.</p>
<div class="signature">
    <p>Best Regards,</p>
    <p>Team Studsphere</p>
</div>`, application.FullName, reason)
		go emailqueue.EnqueueGenericEmail(application.Email, subject, html)
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

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "Interview Scheduled",
		Message:    "An interview has been scheduled.",
		Type:       "interview",
		Link:       "interviews",
	})

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

	messages, total, err := s.repo.GetMessagesByProvider(providerID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	userIDs := make([]uint, 0, len(messages))
	userIDSet := make(map[uint]bool)
	for _, m := range messages {
		if m.UserID > 0 && !userIDSet[m.UserID] {
			userIDs = append(userIDs, m.UserID)
			userIDSet[m.UserID] = true
		}
	}

	if len(userIDs) > 0 {
		type userInfo struct {
			ID        uint
			FirstName string
			LastName  string
			Email     string
		}
		var users []userInfo
		s.repo.GetDB().Table("users").Select("id, first_name, last_name, email").Where("id IN ?", userIDs).Find(&users)
		userMap := make(map[uint]userInfo)
		for _, u := range users {
			userMap[u.ID] = u
		}
		for i, m := range messages {
			if u, ok := userMap[m.UserID]; ok {
				messages[i].UserName = u.FirstName + " " + u.LastName
				messages[i].UserEmail = u.Email
			}
		}
	}

	return messages, total, nil
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

func (s *Service) CreateMessageFromUser(userID uint, req CreateMessageFromUserRequest) (*ProviderMessage, error) {
	message := &ProviderMessage{
		ProviderID: req.ProviderID,
		UserID:     userID,
		Subject:    req.Subject,
		Content:    req.Content,
		Direction:  "incoming",
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

func (s *Service) MarkMessageRead(providerID, id uint) error {
	message, err := s.repo.GetMessageByIDAndProvider(id, providerID)
	if err != nil {
		return err
	}
	return s.repo.MarkMessageRead(message)
}

func (s *Service) GetProviderName(providerID uint) string {
	var provider ScholarshipProviderUser
	if err := s.repo.db.First(&provider, providerID).Error; err != nil {
		return "Organization"
	}
	return provider.ProviderName
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
	if req.LogoURL != "" {
		updates["logo_url"] = req.LogoURL
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	if req.AboutText != "" {
		updates["about_text"] = req.AboutText
	}
	if req.Mission != "" {
		updates["mission"] = req.Mission
	}
	if req.Values != "" {
		updates["values"] = req.Values
	}
	if req.FounderName != "" {
		updates["founder_name"] = req.FounderName
	}
	if req.FounderRole != "" {
		updates["founder_role"] = req.FounderRole
	}
	if req.FounderMessage != "" {
		updates["founder_message"] = req.FounderMessage
	}
	if req.FounderImageURL != "" {
		updates["founder_image_url"] = req.FounderImageURL
	}
	if req.FacebookURL != "" {
		updates["facebook_url"] = req.FacebookURL
	}
	if req.InstagramURL != "" {
		updates["instagram_url"] = req.InstagramURL
	}
	if req.YoutubeURL != "" {
		updates["youtube_url"] = req.YoutubeURL
	}
	if req.LinkedInURL != "" {
		updates["linkedin_url"] = req.LinkedInURL
	}
	if req.MapURL != "" {
		updates["map_url"] = req.MapURL
	}
	if req.BrochureURL != "" {
		updates["brochure_url"] = req.BrochureURL
	}
	if req.BannerURL != "" {
		updates["banner_url"] = req.BannerURL
	}

	if err := s.repo.UpdateProviderProfile(provider, updates); err != nil {
		return nil, err
	}

	provider, err = s.repo.GetProviderProfile(providerID)
	if err != nil {
		return nil, err
	}

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "Profile Updated",
		Message:    "Your profile has been updated successfully.",
		Type:       "system",
		Link:       "org-profile",
	})

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

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "Password Changed",
		Message:    "Your password has been changed successfully.",
		Type:       "system",
		Link:       "settings",
	})

	return nil
}

func (s *Service) ChangeEmail(userID uint, isSubUser bool, req ChangeEmailRequest) error {
	var currentPassword string

	if isSubUser {
		user, err := s.repo.GetAccessUserByID(userID)
		if err != nil {
			return err
		}
		currentPassword = user.Password
	} else {
		provider, err := s.repo.GetProviderProfile(userID)
		if err != nil {
			return err
		}
		if provider.Password == nil {
			return errors.New("password not set for this account")
		}
		currentPassword = *provider.Password
	}

	if currentPassword == "" {
		return errors.New("password not set for this account")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(currentPassword), []byte(req.Password)); err != nil {
		return errors.New("invalid password")
	}

	// Check if new email is taken
	taken, err := s.repo.IsEmailTaken(req.NewEmail)
	if err != nil {
		return err
	}
	if taken {
		return errors.New("email already in use")
	}

	if isSubUser {
		err = s.repo.UpdateAccessUserEmail(userID, req.NewEmail)
	} else {
		err = s.repo.UpdateProviderEmail(userID, req.NewEmail)
	}

	if err == nil {
		s.repo.CreateNotification(&ProviderNotification{
			ProviderID: userID,
			Title:      "Email Updated",
			Message:    "Your email address has been updated successfully.",
			Type:       "system",
			Link:       "settings",
		})
	}
	return err
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

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "News Created",
		Message:    "A new news item has been created.",
		Type:       "news",
		Link:       "news-directory",
	})

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

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "Event Created",
		Message:    "A new event has been created.",
		Type:       "event",
		Link:       "events-directory",
	})

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

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "Blog Created",
		Message:    "A new blog post has been created.",
		Type:       "blog",
		Link:       "blog-directory",
	})

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

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "Calendar Task Added",
		Message:    "A new task has been added to your calendar.",
		Type:       "calendar",
		Link:       "calendar",
	})

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

func (s *Service) CreateWrittenExam(providerID uint, req CreateWrittenExamRequest) (*WrittenExam, error) {
	status := "draft"
	if req.Status != "" {
		status = req.Status
	}
	exam := &WrittenExam{
		ProviderID:    providerID,
		ScholarshipID: req.ScholarshipID,
		Title:         req.Title,
		ExamDate:      req.ExamDate,
		Duration:      req.Duration,
		Location:      req.Location,
		TotalMarks:    req.TotalMarks,
		PassingMarks:  req.PassingMarks,
		Status:        status,
	}
	if err := s.repo.CreateWrittenExam(exam); err != nil {
		return nil, err
	}
	return exam, nil
}

func (s *Service) GetWrittenExams(providerID uint, page, limit int) ([]WrittenExam, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	return s.repo.GetWrittenExamsByProvider(providerID, page, limit)
}

func (s *Service) GetWrittenExamsByScholarship(providerID, scholarshipID uint) ([]WrittenExam, error) {
	return s.repo.GetWrittenExamsByScholarship(providerID, scholarshipID)
}

func (s *Service) GetWrittenExamByID(providerID, id uint) (*WrittenExam, error) {
	exam, err := s.repo.GetWrittenExamByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}
	results, err := s.repo.GetWrittenExamResults(exam.ID)
	if err == nil {
		exam.Results = results
	}
	return exam, nil
}

func (s *Service) UpdateWrittenExam(providerID, id uint, req UpdateWrittenExamRequest) (*WrittenExam, error) {
	exam, err := s.repo.GetWrittenExamByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.ExamDate != "" {
		updates["exam_date"] = req.ExamDate
	}
	if req.Duration > 0 {
		updates["duration"] = req.Duration
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.TotalMarks > 0 {
		updates["total_marks"] = req.TotalMarks
	}
	if req.PassingMarks > 0 {
		updates["passing_marks"] = req.PassingMarks
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if len(updates) > 0 {
		if err := s.repo.UpdateWrittenExam(exam, updates); err != nil {
			return nil, err
		}
	}
	return exam, nil
}

func (s *Service) DeleteWrittenExam(providerID, id uint) error {
	return s.repo.DeleteWrittenExam(id, providerID)
}

func (s *Service) AddWrittenExamResult(examID, providerID uint, req AddWrittenExamResultRequest) (*WrittenExam, error) {
	exam, err := s.repo.GetWrittenExamByIDAndProvider(examID, providerID)
	if err != nil {
		return nil, err
	}
	result := &WrittenExamResult{
		WrittenExamID: examID,
		ApplicationID: req.ApplicationID,
		MarksObtained: req.MarksObtained,
		Remarks:       req.Remarks,
	}
	if err := s.repo.CreateWrittenExamResult(result); err != nil {
		return nil, err
	}
	results, _ := s.repo.GetWrittenExamResults(examID)
	exam.Results = results
	return exam, nil
}

func (s *Service) UpdateWrittenExamResult(examID, resultID, providerID uint, req UpdateWrittenExamResultRequest) (*WrittenExam, error) {
	if _, err := s.repo.GetWrittenExamByIDAndProvider(examID, providerID); err != nil {
		return nil, err
	}
	result, err := s.repo.GetWrittenExamResultByID(resultID, examID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	updates["marks_obtained"] = req.MarksObtained
	if req.Remarks != "" {
		updates["remarks"] = req.Remarks
	}
	if err := s.repo.UpdateWrittenExamResult(result, updates); err != nil {
		return nil, err
	}
	exam, _ := s.repo.GetWrittenExamByIDAndProvider(examID, providerID)
	results, _ := s.repo.GetWrittenExamResults(examID)
	exam.Results = results
	return exam, nil
}

func (s *Service) DeleteWrittenExamResult(examID, resultID, providerID uint) error {
	if _, err := s.repo.GetWrittenExamByIDAndProvider(examID, providerID); err != nil {
		return err
	}
	return s.repo.DeleteWrittenExamResult(resultID, examID)
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

func (s *Service) GetPublishedBlogs(page, limit int) ([]ProviderBlog, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 12
	}
	return s.repo.GetPublishedBlogs(page, limit)
}

func (s *Service) GetPublishedBlogByID(id uint) (*ProviderBlog, error) {
	return s.repo.GetPublishedBlogByID(id)
}

// ─── Public Provider Profile ────────────────────────────────────
func (s *Service) GetPublicProviderProfile(id uint) (*PublicProviderProfileResponse, error) {
	provider, err := s.repo.GetProviderByID(id)
	if err != nil {
		return nil, err
	}

	logoURL := ""
	if provider.LogoURL != nil {
		logoURL = *provider.LogoURL
	}

	services, _ := s.repo.GetServicesByProvider(id)
	sectors, _ := s.repo.GetSectorsByProvider(id)
	projects, _ := s.repo.GetProjectsByProvider(id)
	gallery, _ := s.repo.GetGalleryImagesByProvider(id)
	reviews, _ := s.repo.GetPublishedReviews(id)
	schCount, newsCount, eventCount, blogCount, _ := s.repo.CountPublishedProviderContent(id)

	serviceResponses := make([]ServiceResponse, len(services))
	for i, svc := range services {
		serviceResponses[i] = ServiceResponse{
			ID: svc.ID, Icon: svc.Icon, Title: svc.Title,
			Description: svc.Description, ExternalLink: svc.ExternalLink, SortOrder: svc.SortOrder,
		}
	}

	sectorResponses := make([]SectorResponse, len(sectors))
	for i, sec := range sectors {
		sectorResponses[i] = SectorResponse{
			ID: sec.ID, Name: sec.Name, Description: sec.Description,
			Color: sec.Color, ImageURL: sec.ImageURL, Icon: sec.Icon, ExternalLink: sec.ExternalLink, SortOrder: sec.SortOrder,
		}
	}

	projectResponses := make([]ProjectResponse, len(projects))
	for i, proj := range projects {
		dateStr := ""
		if !proj.Date.IsZero() {
			dateStr = proj.Date.Format("2006-01-02")
		}
		projectResponses[i] = ProjectResponse{
			ID: proj.ID, Title: proj.Title, Description: proj.Description,
			ImageURL: proj.ImageURL, Category: proj.Category, ExternalLink: proj.ExternalLink, Date: dateStr, SortOrder: proj.SortOrder,
		}
	}

	galleryResponses := make([]GalleryImageResponse, len(gallery))
	for i, img := range gallery {
		galleryResponses[i] = GalleryImageResponse{
			ID: img.ID, Folder: img.Folder, ImageURL: img.ImageURL, Caption: img.Caption, SortOrder: img.SortOrder,
		}
	}

	reviewResponses := make([]ReviewResponse, len(reviews))
	for i, rev := range reviews {
		reviewResponses[i] = ReviewResponse{
			ID: rev.ID, AuthorName: rev.AuthorName, AvatarURL: rev.AvatarURL,
			Rating: rev.Rating, Title: rev.Title, Content: rev.Content,
			Pros: rev.Pros, Cons: rev.Cons, CreatedAt: rev.CreatedAt.Format(time.RFC3339),
		}
	}

	schList, _, _ := s.repo.GetPublishedScholarshipsByProvider(id, 1, 100)
	newsList, _, _ := s.repo.GetPublishedNewsByProvider(id, 1, 100)

	scholarshipResponses := make([]ScholarshipResponse, len(schList))
	for i, sch := range schList {
		scholarshipResponses[i] = toScholarshipResponse(&sch)
	}

	newsResponses := make([]NewsResponse, len(newsList))
	for i, n := range newsList {
		newsResponses[i] = toNewsResponse(&n)
	}

	return &PublicProviderProfileResponse{
		ID:               provider.ID,
		ProviderName:     provider.ProviderName,
		Email:            provider.Email,
		ContactNumber:    provider.ContactNumber,
		WebsiteURL:       provider.WebsiteURL,
		LogoURL:          logoURL,
		Address:          provider.Address,
		AboutText:        provider.AboutText,
		Mission:          provider.Mission,
		Values:           provider.Values,
		FounderName:      provider.FounderName,
		FounderRole:      provider.FounderRole,
		FounderMessage:   provider.FounderMessage,
		FounderImageURL:  provider.FounderImageURL,
		FacebookURL:      provider.FacebookURL,
		InstagramURL:     provider.InstagramURL,
		YoutubeURL:       provider.YoutubeURL,
		LinkedInURL:      provider.LinkedInURL,
		MapURL:           provider.MapURL,
		BrochureURL:      provider.BrochureURL,
		BannerURL:        provider.BannerURL,

		Services:         serviceResponses,
		Sectors:          sectorResponses,
		Projects:         projectResponses,
		Gallery:          galleryResponses,
		Reviews:          reviewResponses,
		Scholarships:     scholarshipResponses,
		News:             newsResponses,
		ScholarshipCount: schCount,
		NewsCount:        newsCount,
		EventCount:       eventCount,
		BlogCount:        blogCount,
	}, nil
}

// ─── Services CRUD ────────────────────────────────────────────
func (s *Service) CreateService(providerID uint, req CreateServiceRequest) (*ProviderService, error) {
	item := &ProviderService{
		ProviderID:  providerID,
		Icon:        req.Icon,
		Title:        req.Title,
		Description:  req.Description,
		ExternalLink: req.ExternalLink,
		SortOrder:    req.SortOrder,
	}
	if err := s.repo.CreateService(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) GetServices(providerID uint) ([]ProviderService, error) {
	return s.repo.GetServicesByProvider(providerID)
}

func (s *Service) GetServiceByID(providerID, id uint) (*ProviderService, error) {
	return s.repo.GetServiceByIDAndProvider(id, providerID)
}

func (s *Service) UpdateService(providerID, id uint, req CreateServiceRequest) (*ProviderService, error) {
	item, err := s.repo.GetServiceByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"icon": req.Icon, "title": req.Title,
		"description": req.Description, "external_link": req.ExternalLink,
		"sort_order": req.SortOrder,
	}
	if err := s.repo.UpdateService(item, updates); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteService(providerID, id uint) error {
	return s.repo.DeleteService(id, providerID)
}

// ─── Sectors CRUD ─────────────────────────────────────────────
func (s *Service) CreateSector(providerID uint, req CreateSectorRequest) (*ProviderSector, error) {
	date := time.Time{}
	item := &ProviderSector{
		ProviderID:  providerID,
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		ImageURL:     req.ImageURL,
		Icon:         req.Icon,
		ExternalLink: req.ExternalLink,
		SortOrder:    req.SortOrder,
	}
	if err := s.repo.CreateSector(item); err != nil {
		return nil, err
	}
	_ = date
	return item, nil
}

func (s *Service) GetSectors(providerID uint) ([]ProviderSector, error) {
	return s.repo.GetSectorsByProvider(providerID)
}

func (s *Service) GetSectorByID(providerID, id uint) (*ProviderSector, error) {
	return s.repo.GetSectorByIDAndProvider(id, providerID)
}

func (s *Service) UpdateSector(providerID, id uint, req CreateSectorRequest) (*ProviderSector, error) {
	item, err := s.repo.GetSectorByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"name": req.Name, "description": req.Description,
		"color": req.Color, "image_url": req.ImageURL,
		"icon": req.Icon, "external_link": req.ExternalLink,
		"sort_order": req.SortOrder,
	}
	if err := s.repo.UpdateSector(item, updates); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteSector(providerID, id uint) error {
	return s.repo.DeleteSector(id, providerID)
}

// ─── Projects CRUD ────────────────────────────────────────────
func (s *Service) CreateProject(providerID uint, req CreateProjectRequest) (*ProviderProject, error) {
	date := time.Time{}
	if req.Date != "" {
		if t, err := time.Parse("2006-01-02", req.Date); err == nil {
			date = t
		}
	}
	item := &ProviderProject{
		ProviderID:  providerID,
		Title:       req.Title,
		Description: req.Description,
		ImageURL:     req.ImageURL,
		Category:     req.Category,
		ExternalLink: req.ExternalLink,
		Date:         date,
		SortOrder:    req.SortOrder,
	}
	if err := s.repo.CreateProject(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) GetProjects(providerID uint) ([]ProviderProject, error) {
	return s.repo.GetProjectsByProvider(providerID)
}

func (s *Service) GetProjectByID(providerID, id uint) (*ProviderProject, error) {
	return s.repo.GetProjectByIDAndProvider(id, providerID)
}

func (s *Service) UpdateProject(providerID, id uint, req CreateProjectRequest) (*ProviderProject, error) {
	item, err := s.repo.GetProjectByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"title": req.Title, "description": req.Description,
		"image_url": req.ImageURL, "category": req.Category,
		"external_link": req.ExternalLink,
		"sort_order": req.SortOrder,
	}
	if req.Date != "" {
		if t, err := time.Parse("2006-01-02", req.Date); err == nil {
			updates["date"] = t
		}
	}
	if err := s.repo.UpdateProject(item, updates); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteProject(providerID, id uint) error {
	return s.repo.DeleteProject(id, providerID)
}

// ─── Gallery Images CRUD ──────────────────────────────────────
func (s *Service) CreateGalleryImage(providerID uint, req CreateGalleryImageRequest) (*ProviderGalleryImage, error) {
	item := &ProviderGalleryImage{
		ProviderID: providerID,
		Folder:     req.Folder,
		ImageURL:   req.ImageURL,
		Caption:    req.Caption,
		SortOrder:  req.SortOrder,
	}
	if err := s.repo.CreateGalleryImage(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) GetGalleryImages(providerID uint) ([]ProviderGalleryImage, error) {
	return s.repo.GetGalleryImagesByProvider(providerID)
}

func (s *Service) GetGalleryImageByID(providerID, id uint) (*ProviderGalleryImage, error) {
	return s.repo.GetGalleryImageByIDAndProvider(id, providerID)
}

func (s *Service) UpdateGalleryImage(providerID, id uint, req CreateGalleryImageRequest) (*ProviderGalleryImage, error) {
	item, err := s.repo.GetGalleryImageByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"folder": req.Folder, "image_url": req.ImageURL, "caption": req.Caption,
		"sort_order": req.SortOrder,
	}
	if err := s.repo.UpdateGalleryImage(item, updates); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteGalleryImage(providerID, id uint) error {
	return s.repo.DeleteGalleryImage(id, providerID)
}

// ─── Reviews CRUD ─────────────────────────────────────────────
func (s *Service) CreateReview(providerID uint, req CreateReviewRequest) (*ProviderReview, error) {
	status := req.Status
	if status == "" {
		status = "published"
	}
	item := &ProviderReview{
		ProviderID: providerID,
		AuthorName: req.AuthorName,
		AvatarURL:  req.AvatarURL,
		Rating:     req.Rating,
		Title:      req.Title,
		Content:    req.Content,
		Pros:       req.Pros,
		Cons:       req.Cons,
		Status:     status,
	}
	if err := s.repo.CreateReview(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) GetReviews(providerID uint) ([]ProviderReview, error) {
	return s.repo.GetReviewsByProvider(providerID)
}

func (s *Service) GetReviewByID(providerID, id uint) (*ProviderReview, error) {
	return s.repo.GetReviewByIDAndProvider(id, providerID)
}

func (s *Service) UpdateReview(providerID, id uint, req CreateReviewRequest) (*ProviderReview, error) {
	item, err := s.repo.GetReviewByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"author_name": req.AuthorName, "avatar_url": req.AvatarURL,
		"rating": req.Rating, "title": req.Title,
		"content": req.Content, "pros": req.Pros,
		"cons": req.Cons, "status": req.Status,
	}
	if err := s.repo.UpdateReview(item, updates); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteReview(providerID, id uint) error {
	return s.repo.DeleteReview(id, providerID)
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
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return nil, errors.New("a user with this email already exists")
		}
		return nil, err
	}

	s.repo.CreateNotification(&ProviderNotification{
		ProviderID: providerID,
		Title:      "Access Granted",
		Message:    fmt.Sprintf("Access has been granted to %s.", user.Email),
		Type:       "system",
		Link:       "assign-access",
	})

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

func (s *Service) DeleteAccessUser(id uint, providerID uint) error {
	user, err := s.repo.GetAccessUserByID(id)
	if err == nil {
		s.repo.CreateNotification(&ProviderNotification{
			ProviderID: providerID,
			Title:      "Access Removed",
			Message:    fmt.Sprintf("Access has been removed from %s.", user.Email),
			Type:       "system",
			Link:       "assign-access",
		})
	}
	return s.repo.DeleteAccessUser(id)
}

func (s *Service) UpdatePermissions(id uint, permissions []string) error {
	return s.repo.UpdateAccessUserPermissions(id, toJSON(permissions))
}

func (s *Service) LoginAccessUser(email, password string, providerID uint) (*AccessUserResponse, error) {
	log.Printf("[DEBUG] LoginAccessUser: email=%s, providerID=%d", email, providerID)

	var user *ProviderAccessUser
	var err error

	if providerID > 0 {
		user, err = s.repo.GetAccessUserByEmail(email, providerID)
	} else {
		user, err = s.repo.GetAccessUserByEmailOnly(email)
	}

	if err != nil {
		log.Printf("[DEBUG] User not found: %v", err)
		return nil, errors.New("invalid credentials")
	}

	log.Printf("[DEBUG] Found user: %s, stored password len: %d", user.Name, len(user.Password))

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Printf("[DEBUG] Password mismatch: %v", err)
		return nil, errors.New("invalid credentials")
	}

	user.LastActive = time.Now()
	// Clear password to avoid re-hashing in UpdateAccessUser if not careful
	// But UpdateAccessUser in repo handles it based on empty string
	if err := s.repo.UpdateAccessUser(user); err != nil {
		return nil, err
	}

	return toAccessUserResponse(user), nil
}

func (s *Service) GetUserByID(userID uint) (*UserResponse, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &UserResponse{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Phone:     user.Phone,
		Gender:    user.Gender,
		Address:   user.Address,
		Bio:       user.Bio,
		Role:      user.Role,
	}, nil
}

func (s *Service) ResetAccessUserPassword(userID uint, newPassword string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdateAccessUserField(userID, "password", string(hashed))
}

// ─── Volunteer Service ──────────────────────────────────────────────

func (s *Service) CreateVolunteer(providerID uint, req *CreateVolunteerRequest) (*ProviderVolunteer, error) {
	specificDates, _ := json.Marshal(req.SpecificDates)
	districts, _ := json.Marshal(req.Districts)

	v := &ProviderVolunteer{
		ProviderID:          providerID,
		Title:               req.Title,
		BannerImage:         req.BannerImage,
		Description:         req.Description,
		VolunteerType:       req.VolunteerType,
		VolunteerPayment:    req.VolunteerPayment,
		DateMode:            req.DateMode,
		RangeStart:          req.RangeStart,
		RangeEnd:            req.RangeEnd,
		SpecificDates:       specificDates,
		ApplicationDeadline: req.ApplicationDeadline,
		Districts:           districts,
		Active:              req.Active,
		Location:            req.Location,
	}
	if err := s.repo.CreateVolunteer(v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *Service) GetProviderVolunteers(providerID uint, page, limit int) (*VolunteerListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	volunteers, total, err := s.repo.GetVolunteersByProvider(providerID, page, limit)
	if err != nil {
		return nil, err
	}
	return &VolunteerListResponse{
		Volunteers: toVolunteerResponses(volunteers, ""),
		Meta:       PaginationMeta{Total: total, Page: page, Limit: limit},
	}, nil
}

func (s *Service) GetProviderVolunteerByID(id, providerID uint) (*VolunteerResponse, error) {
	_, err := s.repo.GetVolunteerByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}
	full, err := s.repo.GetVolunteerByID(id)
	if err != nil {
		return nil, err
	}
	resp := toVolunteerResponse(full, "")
	return &resp, nil
}

func (s *Service) UpdateVolunteer(id, providerID uint, req *CreateVolunteerRequest) (*ProviderVolunteer, error) {
	existing, err := s.repo.GetVolunteerByIDAndProvider(id, providerID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	updates["title"] = req.Title
	updates["banner_image"] = req.BannerImage
	updates["description"] = req.Description
	updates["volunteer_type"] = req.VolunteerType
	updates["volunteer_payment"] = req.VolunteerPayment
	updates["location"] = req.Location
	updates["date_mode"] = req.DateMode
	updates["range_start"] = req.RangeStart
	updates["range_end"] = req.RangeEnd
	updates["application_deadline"] = req.ApplicationDeadline
	if req.SpecificDates != nil {
		d, _ := json.Marshal(req.SpecificDates)
		updates["specific_dates"] = d
	}
	if req.Districts != nil {
		d, _ := json.Marshal(req.Districts)
		updates["districts"] = d
	}

	if err := s.repo.UpdateVolunteer(existing, updates); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *Service) DeleteVolunteer(id, providerID uint) error {
	return s.repo.DeleteVolunteer(id, providerID)
}

func (s *Service) ToggleVolunteerActive(id, providerID uint) (*ProviderVolunteer, error) {
	return s.repo.ToggleVolunteerActive(id, providerID)
}

func (s *Service) GetPublicVolunteers(page, limit int, search, volunteerType string) (*VolunteerListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	volunteers, total, err := s.repo.GetPublicVolunteers(page, limit, search, volunteerType)
	if err != nil {
		return nil, err
	}
	return &VolunteerListResponse{
		Volunteers: toVolunteerResponses(volunteers, ""),
		Meta:       PaginationMeta{Total: total, Page: page, Limit: limit},
	}, nil
}

func (s *Service) GetPublicVolunteerByID(id uint) (*VolunteerResponse, error) {
	v, err := s.repo.GetVolunteerByID(id)
	if err != nil {
		return nil, err
	}
	resp := toVolunteerResponse(v, "")
	return &resp, nil
}

func (s *Service) GetPublicVolunteerBySlug(slugStr string) (*VolunteerResponse, error) {
	v, err := s.repo.GetVolunteerBySlug(slugStr)
	if err != nil {
		return nil, err
	}
	resp := toVolunteerResponse(v, "")
	return &resp, nil
}

func (s *Service) ApplyVolunteer(volunteerID uint, req *ApplyVolunteerRequest, cvPath string) (*VolunteerApplication, error) {
	v, err := s.repo.GetVolunteerByID(volunteerID)
	if err != nil {
		return nil, errors.New("volunteer opportunity not found")
	}
	if !v.Active {
		return nil, errors.New("volunteer opportunity is not currently active")
	}
	if v.ApplicationDeadline != "" {
		deadline, err := time.Parse("2006-01-02", v.ApplicationDeadline)
		if err == nil && deadline.Before(time.Now().Truncate(24*time.Hour)) {
			return nil, errors.New("volunteer opportunity deadline has passed")
		}
	}

	availableDays, _ := json.Marshal(req.AvailableDays)
	app := &VolunteerApplication{
		VolunteerID:        volunteerID,
		FullName:           req.FullName,
		Gender:             req.Gender,
		Phone:              req.Phone,
		Email:              req.Email,
		Designation:        req.Designation,
		OtherDesignation:   req.OtherDesignation,
		Province:           req.Province,
		District:           req.District,
		Municipality:       req.Municipality,
		Ward:               req.Ward,
		Tole:               req.Tole,
		ParticipateDistrict: req.ParticipateDistrict,
		AvailableDays:      availableDays,
		VolunteeredBefore:  req.VolunteeredBefore,
		VolunteerDetails:   req.VolunteerDetails,
		CVPath:             cvPath,
		Status:             "pending",
	}
	if err := s.repo.CreateVolunteerApplication(app); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *Service) GetVolunteerApplications(providerID uint, volunteerID *uint, page, limit int, status *string) (*VolunteerApplicationListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	apps, total, err := s.repo.GetVolunteerApplicationsByProvider(providerID, volunteerID, page, limit, status)
	if err != nil {
		return nil, err
	}
	return &VolunteerApplicationListResponse{
		Applications: toVolunteerApplicationResponses(apps),
		Meta:         PaginationMeta{Total: total, Page: page, Limit: limit},
	}, nil
}

func (s *Service) ShortlistVolunteerApplication(id, providerID uint) error {
	app, err := s.repo.GetVolunteerApplicationByID(id)
	if err != nil {
		return err
	}
	_, err = s.repo.GetVolunteerByIDAndProvider(app.VolunteerID, providerID)
	if err != nil {
		return errors.New("application not found or access denied")
	}
	if app.Status != "pending" {
		return errors.New("can only shortlist pending applications")
	}
	return s.repo.UpdateVolunteerApplicationStatus(id, "shortlisted")
}

func (s *Service) UnshortlistVolunteerApplication(id, providerID uint) error {
	app, err := s.repo.GetVolunteerApplicationByID(id)
	if err != nil {
		return err
	}
	_, err = s.repo.GetVolunteerByIDAndProvider(app.VolunteerID, providerID)
	if err != nil {
		return errors.New("application not found or access denied")
	}
	if app.Status != "shortlisted" {
		return errors.New("can only unshortlist shortlisted applications")
	}
	return s.repo.UpdateVolunteerApplicationStatus(id, "pending")
}

func (s *Service) RejectVolunteerApplication(id, providerID uint) error {
	app, err := s.repo.GetVolunteerApplicationByID(id)
	if err != nil {
		return err
	}
	_, err = s.repo.GetVolunteerByIDAndProvider(app.VolunteerID, providerID)
	if err != nil {
		return errors.New("application not found or access denied")
	}
	if app.Status != "pending" && app.Status != "shortlisted" {
		return errors.New("can only reject pending or shortlisted applications")
	}
	return s.repo.UpdateVolunteerApplicationStatus(id, "rejected")
}

// ─── Volunteer Helpers ──────────────────────────────────────────────

func volunteerSlug(v *ProviderVolunteer) string {
	if v.Slug != "" {
		return v.Slug
	}
	generated := strings.ToLower(strings.TrimSpace(v.Title))
	re := regexp.MustCompile(`[^a-z0-9\s-]`)
	generated = re.ReplaceAllString(generated, "")
	re = regexp.MustCompile(`\s+`)
	generated = re.ReplaceAllString(generated, "-")
	return strings.Trim(generated, "-")
}

func toVolunteerResponse(v *ProviderVolunteer, organizer string) VolunteerResponse {
	var specificDates, districts []string
	json.Unmarshal(v.SpecificDates, &specificDates)
	json.Unmarshal(v.Districts, &districts)

	loc := v.Location
	if loc == "" && len(districts) > 0 {
		loc = districts[0]
		if len(districts) > 1 {
			loc += " +" + fmt.Sprintf("%d", len(districts)-1)
		}
	}

	return VolunteerResponse{
		ID:                  v.ID,
		Slug:                volunteerSlug(v),
		CreatedAt:           v.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           v.UpdatedAt.Format(time.RFC3339),
		ProviderID:          v.ProviderID,
		Title:               v.Title,
		BannerImage:         v.BannerImage,
		Description:         v.Description,
		VolunteerType:       v.VolunteerType,
		VolunteerPayment:    v.VolunteerPayment,
		DateMode:            v.DateMode,
		RangeStart:          v.RangeStart,
		RangeEnd:            v.RangeEnd,
		SpecificDates:       specificDates,
		ApplicationDeadline: v.ApplicationDeadline,
		Districts:           districts,
		Active:              v.Active,
		ApplicantCount:      v.ApplicantCount,
		Organizer:           organizer,
		Location:            loc,
	}
}

func toVolunteerResponses(volunteers []ProviderVolunteer, organizer string) []VolunteerResponse {
	res := make([]VolunteerResponse, len(volunteers))
	for i, v := range volunteers {
		res[i] = toVolunteerResponse(&v, organizer)
	}
	return res
}

func toVolunteerApplicationResponse(a *VolunteerApplication) VolunteerApplicationResponse {
	var availableDays []string
	json.Unmarshal(a.AvailableDays, &availableDays)

	return VolunteerApplicationResponse{
		ID:                 a.ID,
		CreatedAt:          a.CreatedAt.Format(time.RFC3339),
		VolunteerID:        a.VolunteerID,
		FullName:           a.FullName,
		Gender:             a.Gender,
		Phone:              a.Phone,
		Email:              a.Email,
		Designation:        a.Designation,
		OtherDesignation:   a.OtherDesignation,
		Province:           a.Province,
		District:           a.District,
		Municipality:       a.Municipality,
		Ward:               a.Ward,
		Tole:               a.Tole,
		ParticipateDistrict: a.ParticipateDistrict,
		AvailableDays:      availableDays,
		VolunteeredBefore:  a.VolunteeredBefore,
		VolunteerDetails:   a.VolunteerDetails,
		CVPath:             a.CVPath,
		Status:             a.Status,
	}
}

func toVolunteerApplicationResponses(apps []VolunteerApplication) []VolunteerApplicationResponse {
	res := make([]VolunteerApplicationResponse, len(apps))
	for i, a := range apps {
		res[i] = toVolunteerApplicationResponse(&a)
	}
	return res
}

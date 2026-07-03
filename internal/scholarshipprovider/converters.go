package scholarshipprovider

import (
	"encoding/json"
	"time"
)

func decodeJSONB[T any](data []byte) T {
	var target T
	if len(data) > 0 {
		json.Unmarshal(data, &target)
	}
	return target
}

func formatTimeOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatTimePtrOrEmpty(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func toScholarshipResponse(s *ProviderScholarship) ScholarshipResponse {
	return ScholarshipResponse{
		ID:                       s.ID,
		CreatedAt:                s.CreatedAt,
		UpdatedAt:                s.UpdatedAt,
		ProviderID:               s.ProviderID,
		Title:                    s.Title,
		Provider:                 s.Provider,
		Description:              s.Description,
		ProviderName:             s.ProviderName,
		FundingTypeOther:         s.FundingTypeOther,
		ScholarshipTypeOther:     s.ScholarshipTypeOther,
		EducationLevel:           s.EducationLevel,
		EducationLevelOther:      s.EducationLevelOther,
		Location:                 s.Location,
		Value:                    s.Value,
		Deadline:                 formatTimeOrEmpty(s.Deadline),
		DegreeLevel:              s.DegreeLevel,
		FundingType:              s.FundingType,
		ScholarshipType:          s.ScholarshipType,
		FieldOfStudy:             decodeJSONB[[]string](s.FieldOfStudy),
		Status:                   s.Status,
		ApplicationStartDate:     formatTimeOrEmpty(s.ApplicationStartDate),
		ApplicationEndDate:       formatTimeOrEmpty(s.ApplicationEndDate),
		ApplyLink:                s.ApplyLink,
		BannerBackgroundImageURL: s.BannerBackgroundImageURL,
		CoverageArea:             s.CoverageArea,
		ContactEmail:             s.ContactEmail,
		PrimaryPhone:             s.PrimaryPhone,
		SecondaryPhone:           s.SecondaryPhone,
		WebsiteUrl:               s.WebsiteUrl,
		OfficeAddress:            s.OfficeAddress,
		MapUrl:                   s.MapUrl,
		AboutParagraph1:          s.AboutParagraph1,
		VideoTutorials:           decodeJSONB[[]VideoTutorial](s.VideoTutorials),
		JourneyTimeline:          decodeJSONB[[]JourneyTimelineItem](s.JourneyTimeline),
		Timeline:                 decodeJSONB[[]TimelineItem](s.Timeline),
		ScholarshipSectionTitle:  s.ScholarshipSectionTitle,
		ScholarshipSubtitle:      s.ScholarshipSubtitle,
		ScholarshipDescription1:  s.ScholarshipDescription1,
		ScholarshipDescription2:  s.ScholarshipDescription2,
		ScholarshipTypes:         decodeJSONB[[]ScholarshipTypeItem](s.ScholarshipTypes),
		ScholarshipTypesNew:      decodeJSONB[[]ScholarshipTypeItem](s.ScholarshipTypesNew),
		SelectionRubric:          decodeJSONB[[]SelectionRubricItem](s.SelectionRubric),
		SelectionRubricNew:       decodeJSONB[[]SelectionRubricItem](s.SelectionRubricNew),
		EligibilitySectionTitle:  s.EligibilitySectionTitle,
		EligibilitySubtitle:      s.EligibilitySubtitle,
		BasicEligibilityCriteria: decodeJSONB[[]string](s.BasicEligibilityCriteria),
		FullyFundedCriteria:      decodeJSONB[[]string](s.FullyFundedCriteria),
		PartiallyFundedCriteria:  decodeJSONB[[]string](s.PartiallyFundedCriteria),
		SelectionProcessSteps:    decodeJSONB[[]SelectionProcessStepItem](s.SelectionProcessSteps),
		RequiredDocuments:        decodeJSONB[[]string](s.RequiredDocuments),
		FAQs:                     decodeJSONB[[]FAQItem](s.FAQs),
		FAQsNew:                  decodeJSONB[[]FAQItem](s.FAQsNew),
		GalleryImages:            decodeJSONB[[]GalleryImageItem](s.GalleryImages),
		GalleryImagesNew:         decodeJSONB[[]GalleryImageItem](s.GalleryImagesNew),
		PartnerGroups:            decodeJSONB[[]PartnerGroup](s.PartnerGroups),
		PartnerMessages:          decodeJSONB[[]PartnerMessage](s.PartnerMessages),
		ExamCenters:              decodeJSONB[[]ExamCenterItem](s.ExamCenters),
		ExamCentersNew:           decodeJSONB[[]ExamCenterItem](s.ExamCentersNew),
		Downloads:                decodeJSONB[[]DownloadItem](s.Downloads),
		PaymentConfig:            decodeJSONB[*PaymentConfig](s.PaymentConfig),
		Image:                    s.BannerBackgroundImageURL,
		ExamDate:                 s.ExamDate,
		ExamTime:                 s.ExamTime,
	}
}

func toNewsResponse(n *ProviderNews) NewsResponse {
	return NewsResponse{
		ID:            n.ID,
		Slug:          n.Slug,
		CreatedAt:     n.CreatedAt,
		UpdatedAt:     n.UpdatedAt,
		ProviderID:    n.ProviderID,
		Title:         n.Title,
		ShortDesc:     n.ShortDesc,
		Content:       n.Content,
		ImageURL:      n.ImageURL,
		NewsType:      n.NewsType,
		PublishedBy:   n.PublishedBy,
		PublishDate:   n.PublishDate,
		Tags:          decodeJSONB[interface{}](n.Tags),
		AllowComments: n.AllowComments,
		Status:        n.Status,
		PublishedAt:   n.PublishedAt,
	}
}

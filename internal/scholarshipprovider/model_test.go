package scholarshipprovider

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func strPtr(s string) *string {
	return &s
}

func TestProviderScholarship_JSON(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name  string
		input ProviderScholarship
	}{
		{
			name: "all fields populated",
			input: ProviderScholarship{
				ID:                         1,
				ProviderID:                 2,
				Title:                      "Test Scholarship",
				Description:                "Test Desc",
				ImageURL:                   strPtr("https://example.com/img.png"),
				Location:                   "Kathmandu",
				Value:                      "5000 USD",
				Deadline:                   now.Add(60 * 24 * time.Hour),
				DegreeLevel:                "Bachelor",
				FundingType:                "Full",
				ScholarshipType:            "Merit-based",
				FieldOfStudy:               []byte(`["CS"]`),
				EligibilityCriteria:        []byte(`["GPA > 3"]`),
				RequiredDocuments:          []byte(`["Transcript"]`),
				Status:                     "published",
				ApplicationsCount:          5,
				BannerBackgroundImageURL:   strPtr("https://example.com/banner.png"),
				AboutParagraph1:            "About 1",
				AboutParagraph2:            "About 2",
				VideoTutorials:             []byte(`[{"url": "vid1"}]`),
				JourneyTimeline:            []byte(`[{"step": 1}]`),
				ScholarshipSectionTitle:    "Scholarship Section",
				ScholarshipSubtitle:        "Subtitle",
				ScholarshipDescription1:    "Desc 1",
				ScholarshipDescription2:    "Desc 2",
				ScholarshipTypes:           []byte(`["Merit"]`),
				SelectionRubric:            []byte(`[{"criteria": "GPA"}]`),
				EligibilitySectionTitle:    "Eligibility",
				EligibilitySubtitle:        "Eligibility Subtitle",
				BasicEligibilityCriteria:   []byte(`["Age > 18"]`),
				FullyFundedCriteria:        []byte(`["Need-based"]`),
				PartiallyFundedCriteria:    []byte(`["Partial"]`),
				SelectionProcessSteps:      []byte(`[{"step": "Apply"}]`),
				FAQs:                       []byte(`[{"q": "What?"}]`),
				GalleryImages:              []byte(`[{"img": "1.png"}]`),
				PartnerGroups:              []byte(`[{"group": "A"}]`),
				ExamCenters:                []byte(`[{"center": "C1"}]`),
				Downloads:                  []byte(`[{"file": "doc.pdf"}]`),
				TotalSeats:                 100,
				AmountPerStudent:           5000.00,
				DisbursementType:           "semester-wise",
				ApplicationStartDate:       now,
				ResultPublicationDate:      now.Add(30 * 24 * time.Hour),
				MinGPA:                     3.0,
				EligibleProvinces:          []byte(`["Province 1"]`),
				SelectionCriteria:          []byte(`[{"criteria": "GPA"}]`),
				InterviewRounds:            2,
				Timeline:                   []byte(`[{"phase": "Application"}]`),
				Achievements:               []byte(`[{"title": "Top 2025"}]`),
				SocialLinks:                []byte(`[{"platform": "Facebook"}]`),
				MapEmbedURL:                "https://maps.google.com/test",
				GuidelinesURL:              "https://example.com/guidelines",
				CreatedAt:                  now.Add(-24 * time.Hour),
				UpdatedAt:                  now,
			},
		},
		{
			name: "nil pointer fields",
			input: ProviderScholarship{
				Title:                     "Nil Ptr Test",
				ImageURL:                  nil,
				BannerBackgroundImageURL:  nil,
			},
		},
		{
			name: "empty byte slices",
			input: ProviderScholarship{
				Title:                     "Empty Byte Slices",
				EligibleProvinces:         []byte{},
				SelectionCriteria:         []byte{},
				Timeline:                  []byte{},
				Achievements:              []byte{},
				SocialLinks:               []byte{},
				FieldOfStudy:              []byte{},
				EligibilityCriteria:       []byte{},
				RequiredDocuments:         []byte{},
				VideoTutorials:            []byte{},
				JourneyTimeline:           []byte{},
				ScholarshipTypes:          []byte{},
				SelectionRubric:           []byte{},
				BasicEligibilityCriteria:  []byte{},
				FullyFundedCriteria:       []byte{},
				PartiallyFundedCriteria:   []byte{},
				SelectionProcessSteps:     []byte{},
				FAQs:                      []byte{},
				GalleryImages:             []byte{},
				PartnerGroups:             []byte{},
				ExamCenters:               []byte{},
				Downloads:                 []byte{},
			},
		},
		{
			name: "edge values",
			input: ProviderScholarship{
				Title:               "Edge Values",
				MinGPA:              0.0,
				TotalSeats:          -5,
				InterviewRounds:     -1,
				AmountPerStudent:    -1000.00,
				ApplicationsCount:   -10,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.input)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var output ProviderScholarship
			err = json.Unmarshal(data, &output)
			if err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}

			// Compare all fields
			if tc.input.ID != output.ID {
				t.Errorf("ID mismatch: got %d, want %d", output.ID, tc.input.ID)
			}
			if tc.input.ProviderID != output.ProviderID {
				t.Errorf("ProviderID mismatch: got %d, want %d", output.ProviderID, tc.input.ProviderID)
			}
			if tc.input.Title != output.Title {
				t.Errorf("Title mismatch: got %s, want %s", output.Title, tc.input.Title)
			}
			if tc.input.Description != output.Description {
				t.Errorf("Description mismatch: got %s, want %s", output.Description, tc.input.Description)
			}
			// ImageURL
			if (tc.input.ImageURL == nil) != (output.ImageURL == nil) {
				t.Errorf("ImageURL nil mismatch: input %v, output %v", tc.input.ImageURL, output.ImageURL)
			} else if tc.input.ImageURL != nil && *tc.input.ImageURL != *output.ImageURL {
				t.Errorf("ImageURL value mismatch: got %s, want %s", *output.ImageURL, *tc.input.ImageURL)
			}
			if tc.input.Location != output.Location {
				t.Errorf("Location mismatch: got %s, want %s", output.Location, tc.input.Location)
			}
			if tc.input.Value != output.Value {
				t.Errorf("Value mismatch: got %s, want %s", output.Value, tc.input.Value)
			}
			if !tc.input.Deadline.Equal(output.Deadline) {
				t.Errorf("Deadline mismatch: got %v, want %v", output.Deadline, tc.input.Deadline)
			}
			if tc.input.DegreeLevel != output.DegreeLevel {
				t.Errorf("DegreeLevel mismatch: got %s, want %s", output.DegreeLevel, tc.input.DegreeLevel)
			}
			if tc.input.FundingType != output.FundingType {
				t.Errorf("FundingType mismatch: got %s, want %s", output.FundingType, tc.input.FundingType)
			}
			if tc.input.ScholarshipType != output.ScholarshipType {
				t.Errorf("ScholarshipType mismatch: got %s, want %s", output.ScholarshipType, tc.input.ScholarshipType)
			}
			if !bytes.Equal(tc.input.FieldOfStudy, output.FieldOfStudy) {
				t.Errorf("FieldOfStudy mismatch: got %s, want %s", output.FieldOfStudy, tc.input.FieldOfStudy)
			}
			if !bytes.Equal(tc.input.EligibilityCriteria, output.EligibilityCriteria) {
				t.Errorf("EligibilityCriteria mismatch: got %s, want %s", output.EligibilityCriteria, tc.input.EligibilityCriteria)
			}
			if !bytes.Equal(tc.input.RequiredDocuments, output.RequiredDocuments) {
				t.Errorf("RequiredDocuments mismatch: got %s, want %s", output.RequiredDocuments, tc.input.RequiredDocuments)
			}
			if tc.input.Status != output.Status {
				t.Errorf("Status mismatch: got %s, want %s", output.Status, tc.input.Status)
			}
			if tc.input.ApplicationsCount != output.ApplicationsCount {
				t.Errorf("ApplicationsCount mismatch: got %d, want %d", output.ApplicationsCount, tc.input.ApplicationsCount)
			}
			// BannerBackgroundImageURL
			if (tc.input.BannerBackgroundImageURL == nil) != (output.BannerBackgroundImageURL == nil) {
				t.Errorf("BannerBackgroundImageURL nil mismatch: input %v, output %v", tc.input.BannerBackgroundImageURL, output.BannerBackgroundImageURL)
			} else if tc.input.BannerBackgroundImageURL != nil && *tc.input.BannerBackgroundImageURL != *output.BannerBackgroundImageURL {
				t.Errorf("BannerBackgroundImageURL value mismatch: got %s, want %s", *output.BannerBackgroundImageURL, *tc.input.BannerBackgroundImageURL)
			}
			if tc.input.AboutParagraph1 != output.AboutParagraph1 {
				t.Errorf("AboutParagraph1 mismatch: got %s, want %s", output.AboutParagraph1, tc.input.AboutParagraph1)
			}
			if tc.input.AboutParagraph2 != output.AboutParagraph2 {
				t.Errorf("AboutParagraph2 mismatch: got %s, want %s", output.AboutParagraph2, tc.input.AboutParagraph2)
			}
			if !bytes.Equal(tc.input.VideoTutorials, output.VideoTutorials) {
				t.Errorf("VideoTutorials mismatch: got %s, want %s", output.VideoTutorials, tc.input.VideoTutorials)
			}
			if !bytes.Equal(tc.input.JourneyTimeline, output.JourneyTimeline) {
				t.Errorf("JourneyTimeline mismatch: got %s, want %s", output.JourneyTimeline, tc.input.JourneyTimeline)
			}
			if tc.input.ScholarshipSectionTitle != output.ScholarshipSectionTitle {
				t.Errorf("ScholarshipSectionTitle mismatch: got %s, want %s", output.ScholarshipSectionTitle, tc.input.ScholarshipSectionTitle)
			}
			if tc.input.ScholarshipSubtitle != output.ScholarshipSubtitle {
				t.Errorf("ScholarshipSubtitle mismatch: got %s, want %s", output.ScholarshipSubtitle, tc.input.ScholarshipSubtitle)
			}
			if tc.input.ScholarshipDescription1 != output.ScholarshipDescription1 {
				t.Errorf("ScholarshipDescription1 mismatch: got %s, want %s", output.ScholarshipDescription1, tc.input.ScholarshipDescription1)
			}
			if tc.input.ScholarshipDescription2 != output.ScholarshipDescription2 {
				t.Errorf("ScholarshipDescription2 mismatch: got %s, want %s", output.ScholarshipDescription2, tc.input.ScholarshipDescription2)
			}
			if !bytes.Equal(tc.input.ScholarshipTypes, output.ScholarshipTypes) {
				t.Errorf("ScholarshipTypes mismatch: got %s, want %s", output.ScholarshipTypes, tc.input.ScholarshipTypes)
			}
			if !bytes.Equal(tc.input.SelectionRubric, output.SelectionRubric) {
				t.Errorf("SelectionRubric mismatch: got %s, want %s", output.SelectionRubric, tc.input.SelectionRubric)
			}
			if tc.input.EligibilitySectionTitle != output.EligibilitySectionTitle {
				t.Errorf("EligibilitySectionTitle mismatch: got %s, want %s", output.EligibilitySectionTitle, tc.input.EligibilitySectionTitle)
			}
			if tc.input.EligibilitySubtitle != output.EligibilitySubtitle {
				t.Errorf("EligibilitySubtitle mismatch: got %s, want %s", output.EligibilitySubtitle, tc.input.EligibilitySubtitle)
			}
			if !bytes.Equal(tc.input.BasicEligibilityCriteria, output.BasicEligibilityCriteria) {
				t.Errorf("BasicEligibilityCriteria mismatch: got %s, want %s", output.BasicEligibilityCriteria, tc.input.BasicEligibilityCriteria)
			}
			if !bytes.Equal(tc.input.FullyFundedCriteria, output.FullyFundedCriteria) {
				t.Errorf("FullyFundedCriteria mismatch: got %s, want %s", output.FullyFundedCriteria, tc.input.FullyFundedCriteria)
			}
			if !bytes.Equal(tc.input.PartiallyFundedCriteria, output.PartiallyFundedCriteria) {
				t.Errorf("PartiallyFundedCriteria mismatch: got %s, want %s", output.PartiallyFundedCriteria, tc.input.PartiallyFundedCriteria)
			}
			if !bytes.Equal(tc.input.SelectionProcessSteps, output.SelectionProcessSteps) {
				t.Errorf("SelectionProcessSteps mismatch: got %s, want %s", output.SelectionProcessSteps, tc.input.SelectionProcessSteps)
			}
			if !bytes.Equal(tc.input.FAQs, output.FAQs) {
				t.Errorf("FAQs mismatch: got %s, want %s", output.FAQs, tc.input.FAQs)
			}
			if !bytes.Equal(tc.input.GalleryImages, output.GalleryImages) {
				t.Errorf("GalleryImages mismatch: got %s, want %s", output.GalleryImages, tc.input.GalleryImages)
			}
			if !bytes.Equal(tc.input.PartnerGroups, output.PartnerGroups) {
				t.Errorf("PartnerGroups mismatch: got %s, want %s", output.PartnerGroups, tc.input.PartnerGroups)
			}
			if !bytes.Equal(tc.input.ExamCenters, output.ExamCenters) {
				t.Errorf("ExamCenters mismatch: got %s, want %s", output.ExamCenters, tc.input.ExamCenters)
			}
			if !bytes.Equal(tc.input.Downloads, output.Downloads) {
				t.Errorf("Downloads mismatch: got %s, want %s", output.Downloads, tc.input.Downloads)
			}
			if tc.input.TotalSeats != output.TotalSeats {
				t.Errorf("TotalSeats mismatch: got %d, want %d", output.TotalSeats, tc.input.TotalSeats)
			}
			if tc.input.AmountPerStudent != output.AmountPerStudent {
				t.Errorf("AmountPerStudent mismatch: got %f, want %f", output.AmountPerStudent, tc.input.AmountPerStudent)
			}
			if tc.input.DisbursementType != output.DisbursementType {
				t.Errorf("DisbursementType mismatch: got %s, want %s", output.DisbursementType, tc.input.DisbursementType)
			}
			if !tc.input.ApplicationStartDate.Equal(output.ApplicationStartDate) {
				t.Errorf("ApplicationStartDate mismatch: got %v, want %v", output.ApplicationStartDate, tc.input.ApplicationStartDate)
			}
			if !tc.input.ResultPublicationDate.Equal(output.ResultPublicationDate) {
				t.Errorf("ResultPublicationDate mismatch: got %v, want %v", output.ResultPublicationDate, tc.input.ResultPublicationDate)
			}
			if tc.input.MinGPA != output.MinGPA {
				t.Errorf("MinGPA mismatch: got %f, want %f", output.MinGPA, tc.input.MinGPA)
			}
			if !bytes.Equal(tc.input.EligibleProvinces, output.EligibleProvinces) {
				t.Errorf("EligibleProvinces mismatch: got %s, want %s", output.EligibleProvinces, tc.input.EligibleProvinces)
			}
			if !bytes.Equal(tc.input.SelectionCriteria, output.SelectionCriteria) {
				t.Errorf("SelectionCriteria mismatch: got %s, want %s", output.SelectionCriteria, tc.input.SelectionCriteria)
			}
			if tc.input.InterviewRounds != output.InterviewRounds {
				t.Errorf("InterviewRounds mismatch: got %d, want %d", output.InterviewRounds, tc.input.InterviewRounds)
			}
			if !bytes.Equal(tc.input.Timeline, output.Timeline) {
				t.Errorf("Timeline mismatch: got %s, want %s", output.Timeline, tc.input.Timeline)
			}
			if !bytes.Equal(tc.input.Achievements, output.Achievements) {
				t.Errorf("Achievements mismatch: got %s, want %s", output.Achievements, tc.input.Achievements)
			}
			if !bytes.Equal(tc.input.SocialLinks, output.SocialLinks) {
				t.Errorf("SocialLinks mismatch: got %s, want %s", output.SocialLinks, tc.input.SocialLinks)
			}
			if tc.input.MapEmbedURL != output.MapEmbedURL {
				t.Errorf("MapEmbedURL mismatch: got %s, want %s", output.MapEmbedURL, tc.input.MapEmbedURL)
			}
			if tc.input.GuidelinesURL != output.GuidelinesURL {
				t.Errorf("GuidelinesURL mismatch: got %s, want %s", output.GuidelinesURL, tc.input.GuidelinesURL)
			}
			if !tc.input.CreatedAt.Equal(output.CreatedAt) {
				t.Errorf("CreatedAt mismatch: got %v, want %v", output.CreatedAt, tc.input.CreatedAt)
			}
			if !tc.input.UpdatedAt.Equal(output.UpdatedAt) {
				t.Errorf("UpdatedAt mismatch: got %v, want %v", output.UpdatedAt, tc.input.UpdatedAt)
			}
			// DeletedAt should not be marshaled
			if output.DeletedAt.Valid {
				t.Errorf("DeletedAt should not be present in JSON, got %v", output.DeletedAt)
			}
		})
	}
}

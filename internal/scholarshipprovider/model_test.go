package scholarshipprovider

import (
	"bytes"
	"testing"
	"time"
)

func TestProviderScholarship_NewFields(t *testing.T) {
	now := time.Now()
	scholarship := ProviderScholarship{
		Title:                 "Test Scholarship",
		TotalSeats:            100,
		AmountPerStudent:      5000.00,
		DisbursementType:      "semester-wise",
		ApplicationStartDate:   now,
		ResultPublicationDate: now.Add(30 * 24 * time.Hour),
		MinGPA:                3.0,
		EligibleProvinces:     []byte(`["Province 1"]`),
		SelectionCriteria:     []byte(`[{"criteria": "GPA"}]`),
		InterviewRounds:       2,
		Timeline:              []byte(`[{"phase": "Application"}]`),
		Achievements:          []byte(`[{"title": "Top 2025"}]`),
		SocialLinks:           []byte(`[{"platform": "Facebook"}]`),
		MapEmbedURL:           "https://maps.google.com/test",
		GuidelinesURL:         "https://example.com/guidelines",
	}

	if scholarship.TotalSeats != 100 {
		t.Errorf("expected TotalSeats 100, got %d", scholarship.TotalSeats)
	}
	if scholarship.AmountPerStudent != 5000.00 {
		t.Errorf("expected AmountPerStudent 5000.00, got %f", scholarship.AmountPerStudent)
	}

	if scholarship.DisbursementType != "semester-wise" {
		t.Errorf("expected DisbursementType 'semester-wise', got %s", scholarship.DisbursementType)
	}

	if !scholarship.ApplicationStartDate.Equal(now) {
		t.Errorf("expected ApplicationStartDate %v, got %v", now, scholarship.ApplicationStartDate)
	}

	expectedResultDate := now.Add(30 * 24 * time.Hour)
	if !scholarship.ResultPublicationDate.Equal(expectedResultDate) {
		t.Errorf("expected ResultPublicationDate %v, got %v", expectedResultDate, scholarship.ResultPublicationDate)
	}

	if scholarship.MinGPA != 3.0 {
		t.Errorf("expected MinGPA 3.0, got %f", scholarship.MinGPA)
	}

	expectedEligibleProvinces := []byte(`["Province 1"]`)
	if !bytes.Equal(scholarship.EligibleProvinces, expectedEligibleProvinces) {
		t.Errorf("expected EligibleProvinces %s, got %s", expectedEligibleProvinces, scholarship.EligibleProvinces)
	}

	expectedSelectionCriteria := []byte(`[{"criteria": "GPA"}]`)
	if !bytes.Equal(scholarship.SelectionCriteria, expectedSelectionCriteria) {
		t.Errorf("expected SelectionCriteria %s, got %s", expectedSelectionCriteria, scholarship.SelectionCriteria)
	}

	if scholarship.InterviewRounds != 2 {
		t.Errorf("expected InterviewRounds 2, got %d", scholarship.InterviewRounds)
	}

	expectedTimeline := []byte(`[{"phase": "Application"}]`)
	if !bytes.Equal(scholarship.Timeline, expectedTimeline) {
		t.Errorf("expected Timeline %s, got %s", expectedTimeline, scholarship.Timeline)
	}

	expectedAchievements := []byte(`[{"title": "Top 2025"}]`)
	if !bytes.Equal(scholarship.Achievements, expectedAchievements) {
		t.Errorf("expected Achievements %s, got %s", expectedAchievements, scholarship.Achievements)
	}

	expectedSocialLinks := []byte(`[{"platform": "Facebook"}]`)
	if !bytes.Equal(scholarship.SocialLinks, expectedSocialLinks) {
		t.Errorf("expected SocialLinks %s, got %s", expectedSocialLinks, scholarship.SocialLinks)
	}

	if scholarship.MapEmbedURL != "https://maps.google.com/test" {
		t.Errorf("expected MapEmbedURL 'https://maps.google.com/test', got %s", scholarship.MapEmbedURL)
	}

	if scholarship.GuidelinesURL != "https://example.com/guidelines" {
		t.Errorf("expected GuidelinesURL 'https://example.com/guidelines', got %s", scholarship.GuidelinesURL)
	}
}

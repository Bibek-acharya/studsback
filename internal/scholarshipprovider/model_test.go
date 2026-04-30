package scholarshipprovider

import (
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
}

package scholarshipprovider

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestProviderScholarship_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, time.May, 3, 12, 30, 0, 0, time.UTC)
	input := ProviderScholarship{
		ID:              1,
		CreatedAt:       now,
		UpdatedAt:       now.Add(2 * time.Hour),
		ProviderID:      42,
		Title:           "Scholarship Title",
		Provider:        "Provider Name",
		Description:     "Main description",
		ProviderName:    "Provider Name",
		Location:        "Kathmandu",
		Value:           "NPR 50000",
		Deadline:        now.Add(24 * time.Hour),
		DegreeLevel:     "Bachelor",
		FundingType:     "Full",
		ScholarshipType: "Merit",
		FieldOfStudy: []byte(`[
			"Computer Science"
		]`),
		Status:                   "published",
		ApplicationStartDate:     now,
		ApplicationEndDate:       now.Add(24 * time.Hour),
		ApplyLink:                "https://example.com/apply",
		BannerBackgroundImageURL: "https://example.com/banner.png",
		CoverageArea:             "Nationwide",
		ContactEmail:             "info@example.com",
		PrimaryPhone:             "9876543210",
		SecondaryPhone:           "9876543211",
		WebsiteUrl:               "https://example.com",
		OfficeAddress:            "Main Street",
		MapUrl:                   "https://maps.example.com",
		AboutParagraph1:          "About the scholarship",
		VideoTutorials:           []byte(`[{"url":"https://youtube.com/embed/abc","title":"Intro","description":"Overview"}]`),
		JourneyTimeline:          []byte(`[{"year":"2024","title":"Started","description":"Program launched"}]`),
		Timeline:                 []byte(`[{"title":"Apply","date":"2026-05-01","description":"Submit the form"}]`),
		ScholarshipSectionTitle:  "Program Details",
		ScholarshipSubtitle:      "Fully funded support",
		ScholarshipDescription1:  "Program overview",
		ScholarshipTypesNew:      []byte(`[{"type":"Full","seats":"10","coverage":"Tuition","eligibility":"SEE graduates"}]`),
		SelectionRubricNew:       []byte(`[{"criteria":"Exam","description":"Written test","weight":"60%"}]`),
		EligibilitySectionTitle:  "Eligibility",
		EligibilitySubtitle:      "Who can apply",
		BasicEligibilityCriteria: []byte(`[
			"Must be a citizen"
		]`),
		FullyFundedCriteria: []byte(`[
			"Income below threshold"
		]`),
		PartiallyFundedCriteria: []byte(`[
			"High academic standing"
		]`),
		SelectionProcessSteps: []byte(`[{"step":1,"title":"Apply","description":"Submit application"}]`),
		RequiredDocuments: []byte(`[
			"Transcript"
		]`),
		FAQsNew:          []byte(`[{"question":"Q?","answer":"A."}]`),
		GalleryImagesNew: []byte(`[{"title":"Campus","url":"https://example.com/image.png"}]`),
		PartnerGroups:    []byte(`[{"groupHeading":"Academic Partners","name":"Partner One","website":"https://partner.example.com","logo":"https://partner.example.com/logo.png"}]`),
		ExamCentersNew:   []byte(`[{"province":"Bagmati","headerColor":"#3B82F6","info":"Info","centerName":"Center One","contactPerson":"Contact","phoneNumber":"9842012345","mapCoordinates":"https://maps.example.com"}]`),
		Downloads:        []byte(`[{"title":"Brochure","description":"Program brochure","url":"https://example.com/brochure.pdf"}]`),
		PaymentConfig:    []byte(`{"enabled":true,"fee_amount":500,"methods":["bank"],"bank_details":{"bank_name":"NMB","account_name":"Scholarship","account_number":"123456","branch":"Kathmandu"},"qr_code":"https://example.com/qr.png"}`),
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var output ProviderScholarship
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(input, output) {
		t.Fatalf("round-trip mismatch:\ninput:  %#v\noutput: %#v", input, output)
	}
}

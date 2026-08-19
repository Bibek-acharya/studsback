package institution

import (
	"encoding/json"
	"testing"
	"time"

	"studsphere/backend/internal/education"
)

func TestInstitutionProgramJSON(t *testing.T) {
	p := InstitutionProgram{
		ID:                  1,
		InstitutionID:       10,
		InstitutionName:     "Test College",
		InstitutionLocation: "Kathmandu",
		InstitutionLink:     "https://test.edu",
		GlobalCourseID:      5,
		Fee:                 "50000",
		Eligibility:         "12th pass",
		Capacity:            100,
		Status:              "active",
		WhoShouldChoose:     []byte(`[{"icon":"🎯","title":"Science Lovers","shortDesc":"For science enthusiasts"}]`),
		Features:            []byte(`[{"title":"Lab Access","shortDesc":"24/7 lab access"}]`),
		FullTimeCourses:     []byte(`[{"course":"BSc","totalFees":"200000","seats":"60","startDate":"2026-01","endDate":"2030-01"}]`),
		FeeItems:            []byte(`[{"particular":"Tuition","amount":"50000","frequency":"yearly","notes":""}]`),
		Overrides:           []byte(`{"fee":"45000"}`),
		NullifiedFields:     []string{"scholarships"},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed InstitutionProgram
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if parsed.GlobalCourseID != 5 {
		t.Errorf("GlobalCourseID = %d, want 5", parsed.GlobalCourseID)
	}
	if parsed.Fee != "50000" {
		t.Errorf("Fee = %q, want %q", parsed.Fee, "50000")
	}
	if parsed.Status != "active" {
		t.Errorf("Status = %q, want %q", parsed.Status, "active")
	}
	if len(parsed.WhoShouldChoose) == 0 {
		t.Error("WhoShouldChoose should not be empty")
	}
	if len(parsed.Features) == 0 {
		t.Error("Features should not be empty")
	}
	if len(parsed.FullTimeCourses) == 0 {
		t.Error("FullTimeCourses should not be empty")
	}
	if len(parsed.FeeItems) == 0 {
		t.Error("FeeItems should not be empty")
	}
	if len(parsed.Overrides) == 0 {
		t.Error("Overrides should not be empty")
	}
	if len(parsed.NullifiedFields) != 1 || parsed.NullifiedFields[0] != "scholarships" {
		t.Errorf("NullifiedFields = %v, want [scholarships]", parsed.NullifiedFields)
	}
}

func TestInstitutionProgramDefaults(t *testing.T) {
	p := InstitutionProgram{}

	if p.GlobalCourseID != 0 {
		t.Errorf("default GlobalCourseID = %d, want 0", p.GlobalCourseID)
	}
	if p.Status != "" {
		t.Errorf("default Status = %q, want empty", p.Status)
	}
	if p.WhoShouldChoose != nil {
		t.Errorf("default WhoShouldChoose = %v, want nil", p.WhoShouldChoose)
	}
	if p.Overrides != nil {
		t.Errorf("default Overrides = %v, want nil", p.Overrides)
	}
}

func TestInstitutionProgramGlobalCourseIDZeroMeansUnlinked(t *testing.T) {
	p := InstitutionProgram{GlobalCourseID: 0}
	if p.GlobalCourseID != 0 {
		t.Error("GlobalCourseID 0 should mean unlinked")
	}

	p2 := InstitutionProgram{GlobalCourseID: 42}
	if p2.GlobalCourseID == 0 {
		t.Error("GlobalCourseID 42 should mean linked")
	}
}

func TestInstitutionProgramOverridesJSON(t *testing.T) {
	overrides := map[string]interface{}{
		"fee": "45000",
	}
	data, _ := json.Marshal(overrides)

	p := InstitutionProgram{
		Overrides: data,
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(p.Overrides, &parsed); err != nil {
		t.Fatalf("Unmarshal overrides failed: %v", err)
	}
	if parsed["fee"] != "45000" {
		t.Errorf("override fee = %v, want 45000", parsed["fee"])
	}
}

func TestInstitutionProgramInstitutionSpecificArrays(t *testing.T) {
	whoData := []byte(`[{"icon":"🎯","title":"Science Lovers","shortDesc":"For science enthusiasts"}]`)
	featData := []byte(`[{"title":"Lab Access","shortDesc":"24/7 lab access"}]`)
	ftcData := []byte(`[{"course":"BSc","totalFees":"200000","seats":"60","startDate":"2026-01","endDate":"2030-01"}]`)
	feeData := []byte(`[{"particular":"Tuition","amount":"50000","frequency":"yearly","notes":""}]`)

	p := InstitutionProgram{
		WhoShouldChoose: whoData,
		Features:        featData,
		FullTimeCourses: ftcData,
		FeeItems:        feeData,
	}

	var parsedWho []map[string]interface{}
	if err := json.Unmarshal(p.WhoShouldChoose, &parsedWho); err != nil {
		t.Fatalf("Unmarshal WhoShouldChoose failed: %v", err)
	}
	if len(parsedWho) != 1 || parsedWho[0]["title"] != "Science Lovers" {
		t.Errorf("WhoShouldChoose = %v, want [{Science Lovers}]", parsedWho)
	}

	var parsedFeat []map[string]interface{}
	if err := json.Unmarshal(p.Features, &parsedFeat); err != nil {
		t.Fatalf("Unmarshal Features failed: %v", err)
	}
	if len(parsedFeat) != 1 || parsedFeat[0]["title"] != "Lab Access" {
		t.Errorf("Features = %v, want [{Lab Access}]", parsedFeat)
	}

	var parsedFTC []map[string]interface{}
	if err := json.Unmarshal(p.FullTimeCourses, &parsedFTC); err != nil {
		t.Fatalf("Unmarshal FullTimeCourses failed: %v", err)
	}
	if len(parsedFTC) != 1 || parsedFTC[0]["course"] != "BSc" {
		t.Errorf("FullTimeCourses = %v, want [{BSc}]", parsedFTC)
	}

	var parsedFee []map[string]interface{}
	if err := json.Unmarshal(p.FeeItems, &parsedFee); err != nil {
		t.Fatalf("Unmarshal FeeItems failed: %v", err)
	}
	if len(parsedFee) != 1 || parsedFee[0]["particular"] != "Tuition" {
		t.Errorf("FeeItems = %v, want [{Tuition}]", parsedFee)
	}
}

func TestCreateProgramRequestJSON(t *testing.T) {
	input := `{
		"fee": "50000",
		"eligibility": "12th pass",
		"capacity": 100,
		"institution_name": "Test College",
		"institution_location": "Kathmandu",
		"institution_link": "https://test.edu",
		"status": "active",
		"globalCourseId": 5,
		"whoShouldChoose": [{"icon":"🎯","title":"Science Lovers","shortDesc":"For science enthusiasts"}],
		"features": [{"title":"Lab Access","shortDesc":"24/7 lab access"}],
		"fullTimeCourses": [{"course":"BSc","totalFees":"200000","seats":"60","startDate":"2026-01","endDate":"2030-01"}],
		"feeItems": [{"particular":"Tuition","amount":"50000","frequency":"yearly","notes":""}],
		"overrides": {"description":"Custom desc"},
		"nullifiedFields": ["scholarships"]
	}`

	var req CreateProgramRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Unmarshal CreateProgramRequest failed: %v", err)
	}
	if req.Fee != "50000" {
		t.Errorf("Fee = %q, want %q", req.Fee, "50000")
	}
	if req.GlobalCourseID != 5 {
		t.Errorf("GlobalCourseID = %d, want 5", req.GlobalCourseID)
	}
	if req.Status != "active" {
		t.Errorf("Status = %q, want %q", req.Status, "active")
	}
	if len(req.WhoShouldChoose) != 1 || req.WhoShouldChoose[0].Title != "Science Lovers" {
		t.Errorf("WhoShouldChoose = %v, want [{Science Lovers}]", req.WhoShouldChoose)
	}
	if len(req.Features) != 1 || req.Features[0].Title != "Lab Access" {
		t.Errorf("Features = %v, want [{Lab Access}]", req.Features)
	}
	if len(req.FullTimeCourses) != 1 || req.FullTimeCourses[0].Course != "BSc" {
		t.Errorf("FullTimeCourses = %v, want [{BSc}]", req.FullTimeCourses)
	}
	if len(req.FeeItems) != 1 || req.FeeItems[0].Particular != "Tuition" {
		t.Errorf("FeeItems = %v, want [{Tuition}]", req.FeeItems)
	}
	if req.Overrides.Description == nil || *req.Overrides.Description != "Custom desc" {
		t.Errorf("Overrides.Description = %v, want Custom desc", req.Overrides.Description)
	}
	if len(req.NullifiedFields) != 1 || req.NullifiedFields[0] != "scholarships" {
		t.Errorf("NullifiedFields = %v, want [scholarships]", req.NullifiedFields)
	}
}

func TestUpdateProgramRequestJSON(t *testing.T) {
	input := `{
		"fee": "60000",
		"capacity": 120,
		"globalCourseId": 10,
		"whoShouldChoose": [{"icon":"🎯","title":"Test","shortDesc":"desc"}],
		"features": [{"title":"Feature1","shortDesc":"desc1"}],
		"fullTimeCourses": [{"course":"BSc","totalFees":"200000","seats":"60","startDate":"2026-01","endDate":"2030-01"}],
		"feeItems": [{"particular":"Tuition","amount":"50000","frequency":"yearly","notes":""}]
	}`

	var req UpdateProgramRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Unmarshal UpdateProgramRequest failed: %v", err)
	}
	if req.Fee != "60000" {
		t.Errorf("Fee = %q, want %q", req.Fee, "60000")
	}
	if req.GlobalCourseID != 10 {
		t.Errorf("GlobalCourseID = %d, want 10", req.GlobalCourseID)
	}
	if req.WhoShouldChoose == nil {
		t.Error("WhoShouldChoose should not be nil")
	}
	if req.Features == nil {
		t.Error("Features should not be nil")
	}
	if req.FullTimeCourses == nil {
		t.Error("FullTimeCourses should not be nil")
	}
	if req.FeeItems == nil {
		t.Error("FeeItems should not be nil")
	}
}

func TestProgramResponseJSON(t *testing.T) {
	desc := "Custom course description"
	resp := ProgramResponse{
		ID:                  1,
		CreatedAt:           "2026-01-01T00:00:00Z",
		UpdatedAt:           "2026-01-01T00:00:00Z",
		InstitutionID:       10,
		InstitutionName:     "Test College",
		InstitutionLocation: "Kathmandu",
		InstitutionLink:     "https://test.edu",
		GlobalCourseID:      5,
		GlobalCourseTitle:   "BSc Computer Science",
		Fee:                 "50000",
		Eligibility:         "12th pass",
		Capacity:            100,
		Status:              "active",
		WhoShouldChoose: []education.PersonaItem{
			{Icon: "🎯", Title: "Science Lovers", ShortDesc: "For science enthusiasts"},
		},
		Features: []education.FeatureItem{
			{Title: "Lab Access", ShortDesc: "24/7 lab access"},
		},
		FullTimeCourses: []education.FullTimeCourse{
			{Course: "BSc", TotalFees: "200000", Seats: "60", StartDate: "2026-01", EndDate: "2030-01"},
		},
		FeeItems: []education.FeeItem{
			{Particular: "Tuition", Amount: "50000", Frequency: "yearly"},
		},
		Overrides: education.CourseOverrides{
			Description: &desc,
		},
		NullifiedFields: []string{"scholarships"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if parsed["globalCourseId"].(float64) != 5 {
		t.Errorf("globalCourseId = %v, want 5", parsed["globalCourseId"])
	}
	if parsed["globalCourseTitle"] != "BSc Computer Science" {
		t.Errorf("globalCourseTitle = %v, want BSc Computer Science", parsed["globalCourseTitle"])
	}
	if parsed["fee"] != "50000" {
		t.Errorf("fee = %v, want 50000", parsed["fee"])
	}
	if parsed["whoShouldChoose"] == nil {
		t.Error("whoShouldChoose should be present in JSON")
	}
	if parsed["features"] == nil {
		t.Error("features should be present in JSON")
	}
	if parsed["fullTimeCourses"] == nil {
		t.Error("fullTimeCourses should be present in JSON")
	}
	if parsed["feeItems"] == nil {
		t.Error("feeItems should be present in JSON")
	}
	if parsed["overrides"] == nil {
		t.Error("overrides should be present in JSON")
	}
	if parsed["nullifiedFields"] == nil {
		t.Error("nullifiedFields should be present in JSON")
	}
}

func TestCourseApprovalRequestJSON(t *testing.T) {
	input := `{
		"id": 1,
		"institutionId": 10,
		"title": "BSc Computer Science",
		"description": "A comprehensive CS program",
		"duration": "4 years",
		"level": "undergraduate",
		"affiliationId": 3,
		"bannerUrl": "https://example.com/banner.jpg",
		"careers": [{"title":"Software Engineer","icon":"💻","color":"blue"}],
		"faqs": [{"question":"Duration?","answer":"4 years"}],
		"eligibilityRows": [{"level":"undergraduate","stream":"science","eligibility":["12th pass"],"documents":["marksheet"]}],
		"admissionSteps": [{"title":"Apply","description":"Fill the form"}],
		"subjectGroups": [{"groupName":"Core CS","description":"Main subjects","subjects":["Math","CS"],"careers":["Developer"]}],
		"scholarshipDesc": "Merit-based scholarships available",
		"scholarshipNotes": "Up to 50% tuition waiver",
		"scholarships": [{"title":"Merit Scholarship","subtitle":"Top performers","coverage":"50% tuition","requirement":"GPA > 3.5"}],
		"fee": "200000",
		"eligibility": "12th pass with Science",
		"capacity": 60,
		"whoShouldChoose": [{"icon":"🎯","title":"Tech Enthusiasts","shortDesc":"For tech lovers"}],
		"features": [{"title":"Modern Labs","shortDesc":"State-of-the-art facilities"}],
		"fullTimeCourses": [{"course":"BSc CS","totalFees":"800000","seats":"60","startDate":"2026-08","endDate":"2030-05"}],
		"feeItems": [{"particular":"Tuition","amount":"50000","frequency":"semester","notes":"Includes lab fees"}],
		"status": "pending",
		"rejectionReason": ""
	}`

	var req CourseApprovalRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Unmarshal CourseApprovalRequest failed: %v", err)
	}

	if req.ID != 1 {
		t.Errorf("ID = %d, want 1", req.ID)
	}
	if req.InstitutionID != 10 {
		t.Errorf("InstitutionID = %d, want 10", req.InstitutionID)
	}
	if req.Title != "BSc Computer Science" {
		t.Errorf("Title = %q, want %q", req.Title, "BSc Computer Science")
	}
	if req.Description != "A comprehensive CS program" {
		t.Errorf("Description = %q, want expected", req.Description)
	}
	if req.Duration != "4 years" {
		t.Errorf("Duration = %q, want %q", req.Duration, "4 years")
	}
	if req.Level != "undergraduate" {
		t.Errorf("Level = %q, want %q", req.Level, "undergraduate")
	}
	if req.AffiliationID == nil || *req.AffiliationID != 3 {
		t.Errorf("AffiliationID = %v, want 3", req.AffiliationID)
	}
	if req.BannerURL != "https://example.com/banner.jpg" {
		t.Errorf("BannerURL = %q, want expected", req.BannerURL)
	}
	if len(req.Careers) != 1 || req.Careers[0].Title != "Software Engineer" {
		t.Errorf("Careers = %v, want [{Software Engineer}]", req.Careers)
	}
	if len(req.FAQs) != 1 || req.FAQs[0].Question != "Duration?" {
		t.Errorf("FAQs = %v, want [{Duration?}]", req.FAQs)
	}
	if len(req.EligibilityRows) != 1 || req.EligibilityRows[0].Level != "undergraduate" {
		t.Errorf("EligibilityRows = %v, want 1 row with level undergraduate", req.EligibilityRows)
	}
	if len(req.AdmissionSteps) != 1 || req.AdmissionSteps[0].Title != "Apply" {
		t.Errorf("AdmissionSteps = %v, want [{Apply}]", req.AdmissionSteps)
	}
	if len(req.SubjectGroups) != 1 || req.SubjectGroups[0].GroupName != "Core CS" {
		t.Errorf("SubjectGroups = %v, want [{Core CS}]", req.SubjectGroups)
	}
	if req.ScholarshipDesc != "Merit-based scholarships available" {
		t.Errorf("ScholarshipDesc = %q, want expected", req.ScholarshipDesc)
	}
	if len(req.Scholarships) != 1 || req.Scholarships[0].Title != "Merit Scholarship" {
		t.Errorf("Scholarships = %v, want [{Merit Scholarship}]", req.Scholarships)
	}
	if req.Fee != "200000" {
		t.Errorf("Fee = %q, want %q", req.Fee, "200000")
	}
	if req.Capacity != 60 {
		t.Errorf("Capacity = %d, want 60", req.Capacity)
	}
	if len(req.WhoShouldChoose) != 1 || req.WhoShouldChoose[0].Title != "Tech Enthusiasts" {
		t.Errorf("WhoShouldChoose = %v, want [{Tech Enthusiasts}]", req.WhoShouldChoose)
	}
	if len(req.Features) != 1 || req.Features[0].Title != "Modern Labs" {
		t.Errorf("Features = %v, want [{Modern Labs}]", req.Features)
	}
	if len(req.FullTimeCourses) != 1 || req.FullTimeCourses[0].Course != "BSc CS" {
		t.Errorf("FullTimeCourses = %v, want [{BSc CS}]", req.FullTimeCourses)
	}
	if len(req.FeeItems) != 1 || req.FeeItems[0].Particular != "Tuition" {
		t.Errorf("FeeItems = %v, want [{Tuition}]", req.FeeItems)
	}
	if req.Status != "pending" {
		t.Errorf("Status = %q, want %q", req.Status, "pending")
	}
	if req.ReviewedBy != nil {
		t.Errorf("ReviewedBy = %v, want nil", req.ReviewedBy)
	}
	if req.ReviewedAt != nil {
		t.Errorf("ReviewedAt = %v, want nil", req.ReviewedAt)
	}
}

func TestCourseApprovalRequestDefaults(t *testing.T) {
	req := CourseApprovalRequest{}

	if req.ID != 0 {
		t.Errorf("default ID = %d, want 0", req.ID)
	}
	if req.InstitutionID != 0 {
		t.Errorf("default InstitutionID = %d, want 0", req.InstitutionID)
	}
	if req.Status != "" {
		t.Errorf("default Status = %q, want empty", req.Status)
	}
	if req.Careers != nil {
		t.Errorf("default Careers = %v, want nil", req.Careers)
	}
	if req.WhoShouldChoose != nil {
		t.Errorf("default WhoShouldChoose = %v, want nil", req.WhoShouldChoose)
	}
	if req.Features != nil {
		t.Errorf("default Features = %v, want nil", req.Features)
	}
	if req.ReviewedBy != nil {
		t.Errorf("default ReviewedBy = %v, want nil", req.ReviewedBy)
	}
	if req.ReviewedAt != nil {
		t.Errorf("default ReviewedAt = %v, want nil", req.ReviewedAt)
	}
}

func TestCourseApprovalRequestMarshaling(t *testing.T) {
	reviewedAt := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	reviewedBy := uint(42)
	req := CourseApprovalRequest{
		ID:            1,
		InstitutionID: 10,
		Title:         "Test Course",
		Status:        "approved",
		ReviewedBy:    &reviewedBy,
		ReviewedAt:    &reviewedAt,
		Careers:       []education.CareerItem{{Title: "Engineer"}},
		WhoShouldChoose: []education.PersonaItem{{Icon: "🎯", Title: "Test"}},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed CourseApprovalRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if parsed.ID != 1 {
		t.Errorf("ID = %d, want 1", parsed.ID)
	}
	if parsed.Status != "approved" {
		t.Errorf("Status = %q, want %q", parsed.Status, "approved")
	}
	if parsed.ReviewedBy == nil || *parsed.ReviewedBy != 42 {
		t.Errorf("ReviewedBy = %v, want 42", parsed.ReviewedBy)
	}
	if parsed.ReviewedAt == nil {
		t.Error("ReviewedAt should not be nil")
	}
	if len(parsed.Careers) != 1 || parsed.Careers[0].Title != "Engineer" {
		t.Errorf("Careers = %v, want [{Engineer}]", parsed.Careers)
	}
	if len(parsed.WhoShouldChoose) != 1 || parsed.WhoShouldChoose[0].Title != "Test" {
		t.Errorf("WhoShouldChoose = %v, want [{Test}]", parsed.WhoShouldChoose)
	}
}

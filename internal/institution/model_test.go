package institution

import (
	"encoding/json"
	"testing"
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
		"globalCourseId": 5
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
	resp := ProgramResponse{
		ID:                  1,
		CreatedAt:           "2026-01-01T00:00:00Z",
		UpdatedAt:           "2026-01-01T00:00:00Z",
		InstitutionID:       10,
		InstitutionName:     "Test College",
		InstitutionLocation: "Kathmandu",
		InstitutionLink:     "https://test.edu",
		GlobalCourseID:      5,
		Fee:                 "50000",
		Eligibility:         "12th pass",
		Capacity:            100,
		Status:              "active",
		WhoShouldChoose:     []interface{}{map[string]interface{}{"icon": "🎯", "title": "Test"}},
		Features:            []interface{}{map[string]interface{}{"title": "Feature1"}},
		FullTimeCourses:     []interface{}{map[string]interface{}{"course": "BSc"}},
		FeeItems:            []interface{}{map[string]interface{}{"particular": "Tuition"}},
		Overrides:           map[string]interface{}{"fee": "45000"},
		NullifiedFields:     []string{"scholarships"},
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

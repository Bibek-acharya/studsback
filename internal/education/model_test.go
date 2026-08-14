package education

import (
	"encoding/json"
	"testing"
)

func TestPersonaItemSerialization(t *testing.T) {
	item := PersonaItem{
		Icon:      "star",
		Title:     "Great for Science Lovers",
		ShortDesc: "Perfect for students interested in research",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal PersonaItem: %v", err)
	}

	var decoded PersonaItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal PersonaItem: %v", err)
	}

	if decoded.Icon != item.Icon {
		t.Errorf("Icon mismatch: got %q, want %q", decoded.Icon, item.Icon)
	}
	if decoded.Title != item.Title {
		t.Errorf("Title mismatch: got %q, want %q", decoded.Title, item.Title)
	}
	if decoded.ShortDesc != item.ShortDesc {
		t.Errorf("ShortDesc mismatch: got %q, want %q", decoded.ShortDesc, item.ShortDesc)
	}
}

func TestPersonaItemJSONKeys(t *testing.T) {
	item := PersonaItem{Icon: "icon", Title: "title", ShortDesc: "desc"}
	data, _ := json.Marshal(item)

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["icon"]; !ok {
		t.Error("Missing JSON key 'icon'")
	}
	if _, ok := raw["title"]; !ok {
		t.Error("Missing JSON key 'title'")
	}
	if _, ok := raw["shortDesc"]; !ok {
		t.Error("Missing JSON key 'shortDesc'")
	}
}

func TestFeatureItemSerialization(t *testing.T) {
	item := FeatureItem{
		Title:     "Industry-Relevant Curriculum",
		ShortDesc: "Updated annually with industry input",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal FeatureItem: %v", err)
	}

	var decoded FeatureItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal FeatureItem: %v", err)
	}

	if decoded.Title != item.Title {
		t.Errorf("Title mismatch: got %q, want %q", decoded.Title, item.Title)
	}
	if decoded.ShortDesc != item.ShortDesc {
		t.Errorf("ShortDesc mismatch: got %q, want %q", decoded.ShortDesc, item.ShortDesc)
	}
}

func TestEligibilityRowSerialization(t *testing.T) {
	row := EligibilityRow{
		Level:       "+2",
		Stream:      "Science",
		Eligibility: []string{"Minimum 2.0 GPA", "Science background"},
		Documents:   []string{"Transcript", "Character Certificate"},
	}

	data, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("Failed to marshal EligibilityRow: %v", err)
	}

	var decoded EligibilityRow
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal EligibilityRow: %v", err)
	}

	if decoded.Level != row.Level {
		t.Errorf("Level mismatch: got %q, want %q", decoded.Level, row.Level)
	}
	if decoded.Stream != row.Stream {
		t.Errorf("Stream mismatch: got %q, want %q", decoded.Stream, row.Stream)
	}
	if len(decoded.Eligibility) != len(row.Eligibility) {
		t.Errorf("Eligibility length mismatch: got %d, want %d", len(decoded.Eligibility), len(row.Eligibility))
	}
	if len(decoded.Documents) != len(row.Documents) {
		t.Errorf("Documents length mismatch: got %d, want %d", len(decoded.Documents), len(row.Documents))
	}
}

func TestEligibilityRowEmptySlices(t *testing.T) {
	row := EligibilityRow{
		Level:       "Bachelor",
		Stream:      "Any",
		Eligibility: []string{},
		Documents:   []string{},
	}

	data, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("Failed to marshal EligibilityRow: %v", err)
	}

	var decoded EligibilityRow
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal EligibilityRow: %v", err)
	}

	if decoded.Eligibility == nil {
		t.Error("Eligibility should be empty slice, not nil")
	}
	if decoded.Documents == nil {
		t.Error("Documents should be empty slice, not nil")
	}
}

func TestAdmissionStepSerialization(t *testing.T) {
	step := AdmissionStep{
		Title:       "Submit Application",
		Description: "Fill out the online application form with required details",
	}

	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("Failed to marshal AdmissionStep: %v", err)
	}

	var decoded AdmissionStep
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal AdmissionStep: %v", err)
	}

	if decoded.Title != step.Title {
		t.Errorf("Title mismatch: got %q, want %q", decoded.Title, step.Title)
	}
	if decoded.Description != step.Description {
		t.Errorf("Description mismatch: got %q, want %q", decoded.Description, step.Description)
	}
}

func TestSubjectGroupSerialization(t *testing.T) {
	group := SubjectGroup{
		GroupName:   "Physics Group",
		Description: "Focus on physics and mathematics",
		Subjects:    []string{"Physics", "Mathematics", "Chemistry"},
		Careers:     []string{"Engineer", "Physicist", "Researcher"},
	}

	data, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("Failed to marshal SubjectGroup: %v", err)
	}

	var decoded SubjectGroup
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal SubjectGroup: %v", err)
	}

	if decoded.GroupName != group.GroupName {
		t.Errorf("GroupName mismatch: got %q, want %q", decoded.GroupName, group.GroupName)
	}
	if len(decoded.Subjects) != len(group.Subjects) {
		t.Errorf("Subjects length mismatch: got %d, want %d", len(decoded.Subjects), len(group.Subjects))
	}
	if len(decoded.Careers) != len(group.Careers) {
		t.Errorf("Careers length mismatch: got %d, want %d", len(decoded.Careers), len(group.Careers))
	}
}

func TestFeeItemSerialization(t *testing.T) {
	item := FeeItem{
		Particular: "Tuition Fee",
		Amount:     "50000",
		Frequency:  "per semester",
		Notes:      "Subject to annual revision",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal FeeItem: %v", err)
	}

	var decoded FeeItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal FeeItem: %v", err)
	}

	if decoded.Particular != item.Particular {
		t.Errorf("Particular mismatch: got %q, want %q", decoded.Particular, item.Particular)
	}
	if decoded.Amount != item.Amount {
		t.Errorf("Amount mismatch: got %q, want %q", decoded.Amount, item.Amount)
	}
	if decoded.Frequency != item.Frequency {
		t.Errorf("Frequency mismatch: got %q, want %q", decoded.Frequency, item.Frequency)
	}
	if decoded.Notes != item.Notes {
		t.Errorf("Notes mismatch: got %q, want %q", decoded.Notes, item.Notes)
	}
}

func TestScholarshipItemSerialization(t *testing.T) {
	item := ScholarshipItem{
		Title:       "Merit Scholarship",
		Subtitle:    "For top performers",
		Coverage:    "50% tuition waiver",
		Requirement: "GPA above 3.5",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal ScholarshipItem: %v", err)
	}

	var decoded ScholarshipItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ScholarshipItem: %v", err)
	}

	if decoded.Title != item.Title {
		t.Errorf("Title mismatch: got %q, want %q", decoded.Title, item.Title)
	}
	if decoded.Subtitle != item.Subtitle {
		t.Errorf("Subtitle mismatch: got %q, want %q", decoded.Subtitle, item.Subtitle)
	}
	if decoded.Coverage != item.Coverage {
		t.Errorf("Coverage mismatch: got %q, want %q", decoded.Coverage, item.Coverage)
	}
	if decoded.Requirement != item.Requirement {
		t.Errorf("Requirement mismatch: got %q, want %q", decoded.Requirement, item.Requirement)
	}
}

func TestFullTimeCourseSerialization(t *testing.T) {
	course := FullTimeCourse{
		Course:    "BSc Computer Science",
		TotalFees: "400000",
		Seats:     "60",
		StartDate: "2025-09-01",
		EndDate:   "2029-08-31",
	}

	data, err := json.Marshal(course)
	if err != nil {
		t.Fatalf("Failed to marshal FullTimeCourse: %v", err)
	}

	var decoded FullTimeCourse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal FullTimeCourse: %v", err)
	}

	if decoded.Course != course.Course {
		t.Errorf("Course mismatch: got %q, want %q", decoded.Course, course.Course)
	}
	if decoded.TotalFees != course.TotalFees {
		t.Errorf("TotalFees mismatch: got %q, want %q", decoded.TotalFees, course.TotalFees)
	}
	if decoded.Seats != course.Seats {
		t.Errorf("Seats mismatch: got %q, want %q", decoded.Seats, course.Seats)
	}
}

func TestFaqItemSerialization(t *testing.T) {
	item := FaqItem{
		Question: "What is the duration?",
		Answer:   "4 years for bachelor programs",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal FaqItem: %v", err)
	}

	var decoded FaqItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal FaqItem: %v", err)
	}

	if decoded.Question != item.Question {
		t.Errorf("Question mismatch: got %q, want %q", decoded.Question, item.Question)
	}
	if decoded.Answer != item.Answer {
		t.Errorf("Answer mismatch: got %q, want %q", decoded.Answer, item.Answer)
	}
}

func TestCareerItemSerialization(t *testing.T) {
	item := CareerItem{
		Title: "Software Engineer",
		Icon:  "code",
		Color: "#3B82F6",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal CareerItem: %v", err)
	}

	var decoded CareerItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal CareerItem: %v", err)
	}

	if decoded.Title != item.Title {
		t.Errorf("Title mismatch: got %q, want %q", decoded.Title, item.Title)
	}
	if decoded.Icon != item.Icon {
		t.Errorf("Icon mismatch: got %q, want %q", decoded.Icon, item.Icon)
	}
	if decoded.Color != item.Color {
		t.Errorf("Color mismatch: got %q, want %q", decoded.Color, item.Color)
	}
}

func TestCareerItemOmitempty(t *testing.T) {
	item := CareerItem{Title: "Engineer"}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal CareerItem: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["icon"]; ok {
		t.Error("Empty Icon should be omitted from JSON")
	}
	if _, ok := raw["color"]; ok {
		t.Error("Empty Color should be omitted from JSON")
	}
	if _, ok := raw["title"]; !ok {
		t.Error("Title should not be omitted from JSON")
	}
}

func TestCourseOverridesSerialization(t *testing.T) {
	desc := "Custom description"
	banner := "https://example.com/banner.jpg"

	overrides := CourseOverrides{
		Description: &desc,
		BannerURL:   &banner,
		Careers: []CareerItem{
			{Title: "Doctor", Icon: "heart", Color: "#EF4444"},
			{Title: "Surgeon", Icon: "scissors"},
		},
		FAQs: []FaqItem{
			{Question: "How long?", Answer: "5 years"},
		},
	}

	data, err := json.Marshal(overrides)
	if err != nil {
		t.Fatalf("Failed to marshal CourseOverrides: %v", err)
	}

	var decoded CourseOverrides
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal CourseOverrides: %v", err)
	}

	if decoded.Description == nil || *decoded.Description != desc {
		t.Errorf("Description mismatch")
	}
	if decoded.BannerURL == nil || *decoded.BannerURL != banner {
		t.Errorf("BannerURL mismatch")
	}
	if len(decoded.Careers) != 2 {
		t.Errorf("Careers length mismatch: got %d, want 2", len(decoded.Careers))
	}
	if len(decoded.FAQs) != 1 {
		t.Errorf("FAQs length mismatch: got %d, want 1", len(decoded.FAQs))
	}
}

func TestCourseOverridesNilFields(t *testing.T) {
	overrides := CourseOverrides{}

	data, err := json.Marshal(overrides)
	if err != nil {
		t.Fatalf("Failed to marshal CourseOverrides: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["description"]; ok {
		t.Error("Nil Description should be omitted from JSON")
	}
	if _, ok := raw["bannerUrl"]; ok {
		t.Error("Nil BannerURL should be omitted from JSON")
	}
	if _, ok := raw["careers"]; ok {
		t.Error("Nil Careers should be omitted from JSON")
	}
	if _, ok := raw["faqs"]; ok {
		t.Error("Nil FAQs should be omitted from JSON")
	}
}

func TestCourseOverridesPartialUpdate(t *testing.T) {
	desc := "New description"
	overrides := CourseOverrides{
		Description: &desc,
	}

	data, err := json.Marshal(overrides)
	if err != nil {
		t.Fatalf("Failed to marshal CourseOverrides: %v", err)
	}

	var decoded CourseOverrides
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal CourseOverrides: %v", err)
	}

	if decoded.Description == nil || *decoded.Description != desc {
		t.Errorf("Description should be set")
	}
	if decoded.BannerURL != nil {
		t.Errorf("BannerURL should be nil")
	}
	if decoded.Careers != nil {
		t.Errorf("Careers should be nil")
	}
	if decoded.FAQs != nil {
		t.Errorf("FAQs should be nil")
	}
}

func TestResolvedCourseSerialization(t *testing.T) {
	affID := uint(5)
	rc := ResolvedCourse{
		ID:                       1,
		Title:                    "BSc Computer Science",
		Duration:                 "4 Years",
		Level:                    "Bachelor",
		AffiliationID:            &affID,
		AffiliationName:          "Tribhuvan University",
		NonUniversityAffiliation: "",
		Description:              "A comprehensive CS program",
		BannerURL:                "https://example.com/banner.jpg",
		Careers: []CareerItem{
			{Title: "Software Engineer", Icon: "code", Color: "#3B82F6"},
		},
		FAQs: []FaqItem{
			{Question: "Duration?", Answer: "4 years"},
		},
		EligibilityRows: []EligibilityRow{
			{Level: "+2", Stream: "Science", Eligibility: []string{"2.0 GPA"}, Documents: []string{"Transcript"}},
		},
		AdmissionSteps: []AdmissionStep{
			{Title: "Apply", Description: "Submit form"},
		},
		SubjectGroups: []SubjectGroup{
			{GroupName: "Physics", Description: "Physics group", Subjects: []string{"Physics"}, Careers: []string{"Engineer"}},
		},
		ScholarshipDesc:  "Merit-based scholarships available",
		ScholarshipNotes: "Apply before deadline",
		Scholarships: []ScholarshipItem{
			{Title: "Merit", Subtitle: "Top performers", Coverage: "50%", Requirement: "GPA 3.5+"},
		},
		InstitutionID: 10,
		Fee:           "500000",
		Eligibility:   "+2 Science",
		Capacity:      60,
		WhoShouldChoose: []PersonaItem{
			{Icon: "star", Title: "Science Lovers", ShortDesc: "For science enthusiasts"},
		},
		Features: []FeatureItem{
			{Title: "Lab Access", ShortDesc: "24/7 lab facilities"},
		},
		FullTimeCourses: []FullTimeCourse{
			{Course: "BSc CS", TotalFees: "400000", Seats: "60", StartDate: "2025-09-01", EndDate: "2029-08-31"},
		},
		FeeItems: []FeeItem{
			{Particular: "Tuition", Amount: "50000", Frequency: "semester", Notes: "Annual revision"},
		},
		Status: "published",
	}

	data, err := json.Marshal(rc)
	if err != nil {
		t.Fatalf("Failed to marshal ResolvedCourse: %v", err)
	}

	var decoded ResolvedCourse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ResolvedCourse: %v", err)
	}

	if decoded.ID != rc.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, rc.ID)
	}
	if decoded.Title != rc.Title {
		t.Errorf("Title mismatch: got %q, want %q", decoded.Title, rc.Title)
	}
	if decoded.AffiliationID == nil || *decoded.AffiliationID != affID {
		t.Errorf("AffiliationID mismatch")
	}
	if decoded.AffiliationName != rc.AffiliationName {
		t.Errorf("AffiliationName mismatch: got %q, want %q", decoded.AffiliationName, rc.AffiliationName)
	}
	if decoded.InstitutionID != rc.InstitutionID {
		t.Errorf("InstitutionID mismatch: got %d, want %d", decoded.InstitutionID, rc.InstitutionID)
	}
	if decoded.Capacity != rc.Capacity {
		t.Errorf("Capacity mismatch: got %d, want %d", decoded.Capacity, rc.Capacity)
	}
	if len(decoded.Careers) != len(rc.Careers) {
		t.Errorf("Careers length mismatch: got %d, want %d", len(decoded.Careers), len(rc.Careers))
	}
	if len(decoded.FAQs) != len(rc.FAQs) {
		t.Errorf("FAQs length mismatch: got %d, want %d", len(decoded.FAQs), len(rc.FAQs))
	}
	if len(decoded.EligibilityRows) != len(rc.EligibilityRows) {
		t.Errorf("EligibilityRows length mismatch: got %d, want %d", len(decoded.EligibilityRows), len(rc.EligibilityRows))
	}
	if len(decoded.AdmissionSteps) != len(rc.AdmissionSteps) {
		t.Errorf("AdmissionSteps length mismatch: got %d, want %d", len(decoded.AdmissionSteps), len(rc.AdmissionSteps))
	}
	if len(decoded.SubjectGroups) != len(rc.SubjectGroups) {
		t.Errorf("SubjectGroups length mismatch: got %d, want %d", len(decoded.SubjectGroups), len(rc.SubjectGroups))
	}
	if len(decoded.Scholarships) != len(rc.Scholarships) {
		t.Errorf("Scholarships length mismatch: got %d, want %d", len(decoded.Scholarships), len(rc.Scholarships))
	}
	if len(decoded.WhoShouldChoose) != len(rc.WhoShouldChoose) {
		t.Errorf("WhoShouldChoose length mismatch: got %d, want %d", len(decoded.WhoShouldChoose), len(rc.WhoShouldChoose))
	}
	if len(decoded.Features) != len(rc.Features) {
		t.Errorf("Features length mismatch: got %d, want %d", len(decoded.Features), len(rc.Features))
	}
	if len(decoded.FullTimeCourses) != len(rc.FullTimeCourses) {
		t.Errorf("FullTimeCourses length mismatch: got %d, want %d", len(decoded.FullTimeCourses), len(rc.FullTimeCourses))
	}
	if len(decoded.FeeItems) != len(rc.FeeItems) {
		t.Errorf("FeeItems length mismatch: got %d, want %d", len(decoded.FeeItems), len(rc.FeeItems))
	}
	if decoded.Status != rc.Status {
		t.Errorf("Status mismatch: got %q, want %q", decoded.Status, rc.Status)
	}
}

func TestResolvedCourseJSONKeys(t *testing.T) {
	rc := ResolvedCourse{
		ID:              1,
		AffiliationName: "TU",
		WhoShouldChoose: []PersonaItem{},
		Features:        []FeatureItem{},
		FullTimeCourses: []FullTimeCourse{},
		FeeItems:        []FeeItem{},
	}

	data, _ := json.Marshal(rc)

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	expectedKeys := []string{
		"id", "title", "duration", "level", "affiliationId", "affiliationName",
		"nonUniversityAffiliation", "description", "bannerUrl", "careers", "faqs",
		"eligibilityRows", "admissionSteps", "subjectGroups", "scholarshipDesc",
		"scholarshipNotes", "scholarships", "institutionId", "fee", "eligibility",
		"capacity", "whoShouldChoose", "features", "fullTimeCourses", "feeItems", "status",
	}

	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("Missing JSON key: %q", key)
		}
	}
}

func TestAffiliationSerialization(t *testing.T) {
	aff := Affiliation{
		ID:   1,
		Name: "Tribhuvan University",
	}

	data, err := json.Marshal(aff)
	if err != nil {
		t.Fatalf("Failed to marshal Affiliation: %v", err)
	}

	var decoded Affiliation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Affiliation: %v", err)
	}

	if decoded.ID != aff.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, aff.ID)
	}
	if decoded.Name != aff.Name {
		t.Errorf("Name mismatch: got %q, want %q", decoded.Name, aff.Name)
	}
}

func TestAffiliationTableName(t *testing.T) {
	aff := Affiliation{}
	if aff.TableName() != "affiliations" {
		t.Errorf("TableName mismatch: got %q, want %q", aff.TableName(), "affiliations")
	}
}

func TestCourseNewJSONBFields(t *testing.T) {
	course := Course{
		WhoShouldChoose: []byte(`[{"icon":"star","title":"Test","shortDesc":"desc"}]`),
		Features:        []byte(`[{"title":"Feature","shortDesc":"desc"}]`),
		EligibilityRows: []byte(`[{"level":"+2","stream":"Science","eligibility":["2.0"],"documents":["Transcript"]}]`),
		AdmissionSteps:  []byte(`[{"title":"Step 1","description":"Apply"}]`),
		SubjectGroups:   []byte(`[{"groupName":"Physics","description":"Physics group","subjects":["Physics"],"careers":["Engineer"]}]`),
		FeeItems:        []byte(`[{"particular":"Tuition","amount":"50000","frequency":"semester","notes":"notes"}]`),
		Scholarships:    []byte(`[{"title":"Merit","subtitle":"Top","coverage":"50%","requirement":"GPA 3.5+"}]`),
		FullTimeCourses: []byte(`[{"course":"BSc","totalFees":"400000","seats":"60","startDate":"2025-09-01","endDate":"2029-08-31"}]`),
		FAQs:            []byte(`[{"question":"Q?","answer":"A!"}]`),
	}

	var whoShouldChoose []PersonaItem
	if err := json.Unmarshal(course.WhoShouldChoose, &whoShouldChoose); err != nil {
		t.Errorf("Failed to unmarshal WhoShouldChoose: %v", err)
	}
	if len(whoShouldChoose) != 1 {
		t.Errorf("WhoShouldChoose length mismatch: got %d, want 1", len(whoShouldChoose))
	}

	var features []FeatureItem
	if err := json.Unmarshal(course.Features, &features); err != nil {
		t.Errorf("Failed to unmarshal Features: %v", err)
	}
	if len(features) != 1 {
		t.Errorf("Features length mismatch: got %d, want 1", len(features))
	}

	var eligibilityRows []EligibilityRow
	if err := json.Unmarshal(course.EligibilityRows, &eligibilityRows); err != nil {
		t.Errorf("Failed to unmarshal EligibilityRows: %v", err)
	}
	if len(eligibilityRows) != 1 {
		t.Errorf("EligibilityRows length mismatch: got %d, want 1", len(eligibilityRows))
	}

	var admissionSteps []AdmissionStep
	if err := json.Unmarshal(course.AdmissionSteps, &admissionSteps); err != nil {
		t.Errorf("Failed to unmarshal AdmissionSteps: %v", err)
	}
	if len(admissionSteps) != 1 {
		t.Errorf("AdmissionSteps length mismatch: got %d, want 1", len(admissionSteps))
	}

	var subjectGroups []SubjectGroup
	if err := json.Unmarshal(course.SubjectGroups, &subjectGroups); err != nil {
		t.Errorf("Failed to unmarshal SubjectGroups: %v", err)
	}
	if len(subjectGroups) != 1 {
		t.Errorf("SubjectGroups length mismatch: got %d, want 1", len(subjectGroups))
	}

	var feeItems []FeeItem
	if err := json.Unmarshal(course.FeeItems, &feeItems); err != nil {
		t.Errorf("Failed to unmarshal FeeItems: %v", err)
	}
	if len(feeItems) != 1 {
		t.Errorf("FeeItems length mismatch: got %d, want 1", len(feeItems))
	}

	var scholarships []ScholarshipItem
	if err := json.Unmarshal(course.Scholarships, &scholarships); err != nil {
		t.Errorf("Failed to unmarshal Scholarships: %v", err)
	}
	if len(scholarships) != 1 {
		t.Errorf("Scholarships length mismatch: got %d, want 1", len(scholarships))
	}

	var fullTimeCourses []FullTimeCourse
	if err := json.Unmarshal(course.FullTimeCourses, &fullTimeCourses); err != nil {
		t.Errorf("Failed to unmarshal FullTimeCourses: %v", err)
	}
	if len(fullTimeCourses) != 1 {
		t.Errorf("FullTimeCourses length mismatch: got %d, want 1", len(fullTimeCourses))
	}

	var faqs []FaqItem
	if err := json.Unmarshal(course.FAQs, &faqs); err != nil {
		t.Errorf("Failed to unmarshal FAQs: %v", err)
	}
	if len(faqs) != 1 {
		t.Errorf("FAQs length mismatch: got %d, want 1", len(faqs))
	}
}

func TestCourseNewFieldsJSONKeys(t *testing.T) {
	course := Course{}
	data, _ := json.Marshal(course)

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	expectedKeys := []string{
		"whoShouldChoose", "features", "eligibilityRows", "admissionSteps",
		"subjectGroups", "feeItems", "scholarshipDesc", "scholarshipNotes",
		"scholarships", "fullTimeCourses", "faqs", "bannerUrl",
	}

	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("Missing JSON key in Course: %q", key)
		}
	}
}

func TestCreateCourseRequestValidate_BachelorRequiresAffiliationID(t *testing.T) {
	affID := uint(1)
	tests := []struct {
		name    string
		req     CreateCourseRequest
		wantErr bool
	}{
		{
			name: "Bachelor's with affiliation ID passes",
			req: CreateCourseRequest{
				Title:         "Test",
				Level:         "Bachelor's",
				AffiliationID: &affID,
			},
			wantErr: false,
		},
		{
			name: "Bachelor's without affiliation ID fails",
			req: CreateCourseRequest{
				Title: "Test",
				Level: "Bachelor's",
			},
			wantErr: true,
		},
		{
			name: "Master's with affiliation ID passes",
			req: CreateCourseRequest{
				Title:         "Test",
				Level:         "Master's",
				AffiliationID: &affID,
			},
			wantErr: false,
		},
		{
			name: "Master's without affiliation ID fails",
			req: CreateCourseRequest{
				Title: "Test",
				Level: "Master's",
			},
			wantErr: true,
		},
		{
			name: "+2 with non_university_affiliation passes",
			req: CreateCourseRequest{
				Title:                    "Test",
				Level:                    "+2",
				NonUniversityAffiliation: "NEB",
			},
			wantErr: false,
		},
		{
			name: "+2 without non_university_affiliation fails",
			req: CreateCourseRequest{
				Title: "Test",
				Level: "+2",
			},
			wantErr: true,
		},
		{
			name: "Diploma (CTEVT) with non_university_affiliation passes",
			req: CreateCourseRequest{
				Title:                    "Test",
				Level:                    "Diploma (CTEVT)",
				NonUniversityAffiliation: "CTEVT",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateCourseRequestValidate_ClearsConflictingFields(t *testing.T) {
	affID := uint(1)

	t.Run("Bachelor's clears NonUniversityAffiliation", func(t *testing.T) {
		req := CreateCourseRequest{
			Title:                    "Test",
			Level:                    "Bachelor's",
			AffiliationID:            &affID,
			NonUniversityAffiliation: "should be cleared",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		req.Normalize()
		if req.NonUniversityAffiliation != "" {
			t.Errorf("expected NonUniversityAffiliation to be empty, got %q", req.NonUniversityAffiliation)
		}
	})

	t.Run("Secondary clears AffiliationID", func(t *testing.T) {
		req := CreateCourseRequest{
			Title:                    "Test",
			Level:                    "+2",
			AffiliationID:            &affID,
			NonUniversityAffiliation: "NEB",
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		req.Normalize()
		if req.AffiliationID != nil {
			t.Errorf("expected AffiliationID to be nil, got %v", req.AffiliationID)
		}
	})
}

func TestCreateCourseRequestNormalize_SecondaryLevelClearsAffiliationID(t *testing.T) {
	affID := uint(42)
	levels := []string{"+2", "A-Level", "TSLC (CTEVT)", "Diploma (CTEVT)", "PCL"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			req := CreateCourseRequest{
				Title:                    "Test",
				Level:                    level,
				AffiliationID:            &affID,
				NonUniversityAffiliation: "NEB",
			}
			if err := req.Validate(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			req.Normalize()
			if req.AffiliationID != nil {
				t.Errorf("expected AffiliationID to be nil for %s, got %v", level, req.AffiliationID)
			}
			if req.NonUniversityAffiliation != "NEB" {
				t.Errorf("expected NonUniversityAffiliation to be preserved for %s", level)
			}
		})
	}
}

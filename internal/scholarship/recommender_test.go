package scholarship

import (
	"encoding/json"
	"testing"
)

func TestNormalizeAndMatch(t *testing.T) {
	if !fuzzyMatch("Bachelor's in CS", "bachelor") {
		t.Error("expected fuzzy match for bachelor")
	}
	if !fuzzyMatch("Bachelors degree", "bachelor") {
		t.Error("expected fuzzy match for bachelors")
	}
	if fuzzyMatch("Medical degree", "engineering") {
		t.Error("false positive for engineering")
	}
	if !fuzzyMatch("Computer Science", "cs") {
		t.Error("expected fuzzy match for cs -> computer science")
	}
}

func TestScoreEducationLevelFuzzy(t *testing.T) {
	s := Scholarship{DegreeLevel: "Bachelor's Degree"}
	score := scoreEducationLevel(s, "undergraduate")
	if score <= 0 {
		t.Error("expected non-zero score for undergraduate matching bachelor's")
	}
}

func TestScoreFieldOfStudyFuzzy(t *testing.T) {
	fos, _ := json.Marshal([]string{"Computer Science", "IT"})
	s := Scholarship{FieldOfStudy: fos}
	score := scoreFieldOfStudy(s, "cs")
	if score <= 0 {
		t.Error("expected non-zero score for cs matching computer science")
	}
}

func TestPercentileNormalization(t *testing.T) {
	raw := []float64{10, 20, 30, 40, 50}
	norm := normalizePercentile(raw)
	if len(norm) != 5 {
		t.Fatalf("expected 5 normalized values, got %d", len(norm))
	}
	if norm[0] != 0.0 || norm[4] != 1.0 {
		t.Errorf("expected min=0 max=1, got min=%f max=%f", norm[0], norm[4])
	}
}

func TestScoreTalentsAchievements(t *testing.T) {
	s := Scholarship{Description: "Looking for students with coding skills and olympiad achievements"}
	talentScore := scoreTalents(s, []string{"coding", "public_speaking"})
	if talentScore <= 0 {
		t.Error("expected non-zero talent score for coding match")
	}
	achievementScore := scoreAchievements(s, []string{"science_olympiad"})
	if achievementScore <= 0 {
		t.Error("expected non-zero achievement score for olympiad match")
	}
}

func TestExtractMinGPAFromText(t *testing.T) {
	gpa := extractMinGPAFromText("Minimum 3.0 GPA required")
	if gpa != 3.0 {
		t.Errorf("expected 3.0, got %f", gpa)
	}
	gpa = extractMinGPAFromText("CGPA 2.5 or above")
	if gpa != 2.5 {
		t.Errorf("expected 2.5, got %f", gpa)
	}
	gpa = extractMinGPAFromText("Score 30.0 or above")
	if gpa != 0 {
		t.Errorf("expected 0, got %f (false match on 30.0)", gpa)
	}
}

func TestScoreProfileCompatibility(t *testing.T) {
	fos, _ := json.Marshal([]string{"Computer Science"})
	s := Scholarship{
		Description:  "Computer Science scholarship for undergraduate students",
		FieldOfStudy: fos,
	}
	entries := []EducationEntryData{
		{Stream: "Science", Grade: "3.5"},
	}
	prefs := &PreferencesData{
		Preferences: map[string]interface{}{"fields": []interface{}{"cs"}},
	}
	score := scoreProfileCompatibility(s, entries, prefs, nil)
	if score <= 0 {
		t.Errorf("expected non-zero profile compatibility score, got %d", score)
	}
}

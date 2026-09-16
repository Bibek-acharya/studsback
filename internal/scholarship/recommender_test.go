package scholarship

import (
	"encoding/json"
	"math"
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

func TestRecommendationWeightsSumToOne(t *testing.T) {
	for _, hasProfile := range []bool{false, true} {
		weights := recommendationWeights(hasProfile)
		sum := 0.0
		for _, w := range weights {
			sum += w
		}
		if diff := math.Abs(sum - 1.0); diff > 1e-9 {
			t.Errorf("expected weights to sum to 1.0 (hasProfile=%v), got %f", hasProfile, sum)
		}
	}
}

func TestScoreTalentsUsesSelectedValues(t *testing.T) {
	debate := Scholarship{Title: "National Debate Championship Grant", Description: "Funding for students active in debate and oratory."}
	unrelated := Scholarship{Title: "General Merit Grant", Description: "Open grant for all deserving students."}

	withDebate := scoreTalents(debate, []string{"public_speaking"})
	withUnrelated := scoreTalents(unrelated, []string{"public_speaking"})

	if withDebate <= withUnrelated {
		t.Errorf("expected debate scholarship to score higher for public_speaking talent: got %d vs %d", withDebate, withUnrelated)
	}
	if withDebate <= 0 {
		t.Errorf("expected positive talent score when scholarship mentions debate, got %d", withDebate)
	}
	if withUnrelated != 0 {
		t.Errorf("expected zero talent score when scholarship text is unrelated, got %d", withUnrelated)
	}
}

func TestScoreAchievementsUsesSelectedValues(t *testing.T) {
	olympiad := Scholarship{Title: "Science Olympiad Excellence Award", Description: "Award for national science olympiad winners."}
	unrelated := Scholarship{Title: "Community Service Grant", Description: "For students with volunteering history."}

	withOlympiad := scoreAchievements(olympiad, []string{"olympiad"})
	withUnrelated := scoreAchievements(unrelated, []string{"olympiad"})

	if withOlympiad <= withUnrelated {
		t.Errorf("expected olympiad scholarship to score higher: got %d vs %d", withOlympiad, withUnrelated)
	}
}

func TestScoreWillingnessMatchesRequirements(t *testing.T) {
	essay := Scholarship{Title: "Written Application Grant", Description: "Applicants must submit an essay with their application."}
	noReq := Scholarship{Title: "Simple Grant", Description: "No additional documents needed."}

	// Student willing to write an essay scores higher for the essay scholarship.
	yes := scoreWillingness(essay, "yes", "no", "no")
	if yes != 5 {
		t.Errorf("expected max willingness for covered essay requirement, got %d", yes)
	}

	// Student NOT willing to write an essay scores low for the essay scholarship.
	no := scoreWillingness(essay, "no", "no", "no")
	if no >= yes {
		t.Errorf("expected unwilling student to score lower than willing one, got %d vs %d", no, yes)
	}

	// No detected requirements -> neutral score for both willing and unwilling.
	neutralYes := scoreWillingness(noReq, "yes", "no", "no")
	neutralNo := scoreWillingness(noReq, "no", "no", "no")
	if neutralYes != 3 || neutralNo != 3 {
		t.Errorf("expected neutral score 3 when no requirements detected, got %d and %d", neutralYes, neutralNo)
	}
}

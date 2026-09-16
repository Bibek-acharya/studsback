package college

import "testing"

// broadCollege offers programs across every bucket so each field option can
// find its keywords in the college text.
var broadCollege = College{
	Name:        "Test Polytechnic College",
	Description: "Offers programs in engineering, law, nursing, economics, management, science, computer applications and information technology",
}

// TestScorePreferredField_Boosted verifies that real UI field options score
// above the floor (5) when the college offers matching programs.
func TestScorePreferredField_Boosted(t *testing.T) {
	tests := []struct {
		name  string
		field string
	}{
		{"CS & IT option", "Computer Science & Information Technology"},
		{"Engineering", "Engineering"},
		{"Nursing", "Nursing"},
		{"Law & Legal Studies", "Law & Legal Studies"},
		{"Economics", "Economics"},
		{"Management", "Management"},
		{"Science", "Science"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score, _ := scorePreferredField(broadCollege, tc.field)
			if score <= 5 {
				t.Errorf("scorePreferredField(%q) = %d, want > 5 (floor)", tc.field, score)
			}
		})
	}
}

// TestScorePreferredField_Floor verifies that an unmatched field scores the
// floor.
func TestScorePreferredField_Floor(t *testing.T) {
	c := College{Name: "Plain College", Description: "General degrees"}

	score, _ := scorePreferredField(c, "Underwater Basket Weaving")
	if score != 5 {
		t.Errorf("scorePreferredField = %d, want floor 5", score)
	}
}

// TestScorePreferredField_NoFalsePositive guards against the "it" bucket
// matching unrelated fields like "Politics" (which contains "it").
func TestScorePreferredField_NoFalsePositive(t *testing.T) {
	// "Political Science" contains "it" but must boost via the science bucket,
	// not a spurious IT match.
	score, _ := scorePreferredField(broadCollege, "Political Science")
	if score != 20 {
		t.Errorf("scorePreferredField(Political Science) = %d, want 20", score)
	}

	// "Politics" selects the IT bucket via the "it" substring, but the college
	// text has no IT keyword, so it stays at floor.
	c := College{Name: "Plain College", Description: "General degrees"}
	score, _ = scorePreferredField(c, "Politics")
	if score != 5 {
		t.Errorf("scorePreferredField(Politics) = %d, want floor 5", score)
	}
}

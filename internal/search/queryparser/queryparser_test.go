package queryparser

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantQ      string
		wantCat    string
		wantLoc    string
		wantUni    string
		wantIntent string
	}{
		{
			name:       "top colleges in kathmandu",
			input:      "top colleges in kathmandu",
			wantQ:      "",
			wantCat:    "college",
			wantLoc:    "Kathmandu",
			wantIntent: "top",
		},
		{
			name:       "best computer science colleges in kathmandu",
			input:      "best computer science colleges in kathmandu",
			wantQ:      "computer science",
			wantCat:    "college",
			wantLoc:    "Kathmandu",
			wantIntent: "top",
		},
		{
			name:       "cheap engineering colleges in Kathmandu",
			input:      "cheap engineering colleges in Kathmandu",
			wantQ:      "engineering",
			wantCat:    "college",
			wantLoc:    "Kathmandu",
			wantIntent: "affordable",
		},
		{
			name:       "latest education news",
			input:      "latest education news",
			wantQ:      "education",
			wantCat:    "news",
			wantIntent: "latest",
		},
		{
			name:    "Kathmandu University",
			input:   "Kathmandu University",
			wantQ:   "Kathmandu",
			wantCat: "university",
		},
		{
			name:    "scholarships at TU",
			input:   "scholarships at TU",
			wantQ:   "",
			wantCat: "scholarship",
			wantUni: "Tribhuvan University",
		},
		{
			name:    "CSIT colleges",
			input:   "CSIT colleges",
			wantQ:   "CSIT",
			wantCat: "college",
		},
		{
			name:    "events near Pokhara",
			input:   "events near Pokhara",
			wantQ:   "",
			wantCat: "event",
			wantLoc: "Pokhara",
		},
		{
			name:    "colleges in chitwan",
			input:   "colleges in chitwan",
			wantQ:   "",
			wantCat: "college",
			wantLoc: "Chitwan",
		},
		{
			name:  "empty query",
			input: "",
			wantQ: "",
		},
		{
			name:  "plain text no intent",
			input: "computer science",
			wantQ: "computer science",
		},
		{
			name:       "intent does not match inside another word",
			input:      "renewable energy courses",
			wantQ:      "renewable energy",
			wantCat:    "course",
			wantIntent: "",
		},
		{
			name:       "longest intent phrase is removed",
			input:      "highest rated colleges",
			wantQ:      "",
			wantCat:    "college",
			wantIntent: "top",
		},
		{
			name:    "purbanchal alias",
			input:   "colleges at POU",
			wantQ:   "",
			wantCat: "college",
			wantUni: "Purbanchal University",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.input)
			if got.Query != tt.wantQ {
				t.Errorf("Query = %q, want %q", got.Query, tt.wantQ)
			}
			if got.Category != tt.wantCat {
				t.Errorf("Category = %q, want %q", got.Category, tt.wantCat)
			}
			if got.Filters.Location != tt.wantLoc {
				t.Errorf("Location = %q, want %q", got.Filters.Location, tt.wantLoc)
			}
			if got.Filters.University != tt.wantUni {
				t.Errorf("University = %q, want %q", got.Filters.University, tt.wantUni)
			}
			if got.Intent != tt.wantIntent {
				t.Errorf("Intent = %q, want %q", got.Intent, tt.wantIntent)
			}
		})
	}
}

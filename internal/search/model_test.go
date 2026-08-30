package search

import "testing"

func TestResolveCategoryKeyNormalizesAliases(t *testing.T) {
	tests := map[string]string{
		"course":         "course",
		"courses":        "course",
		"institution":    "college",
		"admissions":     "admission_page",
		"admission_page": "admission_page",
	}

	for input, expected := range tests {
		if got := resolveCategoryKey("", input); got != expected {
			t.Errorf("resolveCategoryKey(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestResolveCategoryKeyUsesWholeWords(t *testing.T) {
	if got := resolveCategoryKey("example questions", ""); got != "" {
		t.Fatalf("expected no category for substring match, got %q", got)
	}
	if got := resolveCategoryKey("upcoming entrance exams", ""); got != "exam" {
		t.Fatalf("expected exam category, got %q", got)
	}
}

func TestCategoryMetaKeySeparatesRetrievalAndDisplayVocabulary(t *testing.T) {
	if got := categoryMetaKey("admission_page"); got != "admissions" {
		t.Fatalf("expected admissions metadata, got %q", got)
	}
}

package queryparser

import (
	"strings"
	"time"
)

// StructuredQuery is the output of query understanding.
type StructuredQuery struct {
	Query      string            // Remaining semantic text for Meilisearch/pgvector
	Category   string            // Detected entity type (college, course, etc.) — empty if ambiguous
	Filters    SearchFilters     // Extracted structured filters
	Intent     string            // Detected intent (top, latest, affordable, etc.)
	OriginalQ  string            // Original user query before parsing
}

// SearchFilters are structured filters extracted from the query.
type SearchFilters struct {
	Location   string
	Type       string
	RatingMin  float64
	University string
}

// entitySynonyms maps user-facing terms to canonical entity types.
var entitySynonyms = map[string]string{
	"college":       "college",
	"colleges":      "college",
	"institute":     "institution",
	"institutes":    "institution",
	"institution":   "institution",
	"institutions":  "institution",
	"university":    "university",
	"universities":  "university",
	"course":        "course",
	"courses":       "course",
	"program":       "course",
	"programs":      "course",
	"scholarship":   "scholarship",
	"scholarships":  "scholarship",
	"grant":         "scholarship",
	"grants":        "scholarship",
	"exam":          "exam",
	"exams":         "exam",
	"entrance":      "exam",
	"event":         "event",
	"events":        "event",
	"fair":          "event",
	"fairs":         "event",
	"webinar":       "event",
	"workshop":      "event",
	"news":          "news",
	"article":       "news",
	"articles":      "news",
	"blog":          "blog",
	"blogs":         "blog",
	"admission":     "admission_page",
	"admissions":    "admission_page",
}

// intentKeywords maps intent terms to canonical intents.
var intentKeywords = map[string][]string{
	"top":        {"top", "best", "best rated", "highest rated", "leading", "popular", "good", "recommended", "finest", "premier", "top rated"},
	"latest":     {"latest", "recent", "new", "newest", "upcoming", "current", "today", "this week", "this month"},
	"affordable": {"cheap", "affordable", "low cost", "budget", "inexpensive", "low fee", "free"},
	"nearby":     {"nearby", "near me", "close", "closest", "nearest"},
}

// knownLocations — Nepali districts and major cities, longest first for matching.
var knownLocations = []string{
	"Achham", "Arghakhanchi", "Baglung", "Baitadi", "Bajhang", "Bajura",
	"Banke", "Bara", "Bardiya", "Bhaktapur", "Bhojpur",
	"Biratnagar", "Birgunj", "Birtamod", "Chitwan", "Dadeldhura", "Dailekh",
	"Dang", "Darchula", "Dhading", "Dhankuta", "Dharan", "Dolakha", "Dolpa",
	"Doti", "Gorkha", "Gulmi", "Humla", "Ilam", "Jajarkot", "Jhapa",
	"Jumla", "Kailali", "Kanchanpur", "Kapilvastu", "Kaski", "Kathmandu",
	"Kavrepalanchok", "Kirtipur", "Lalitpur", "Lamjung",
	"Mahottari", "Makwanpur", "Manang", "Morang", "Mugu", "Mustang",
	"Myagdi", "Nawalparasi", "Nepalgunj", "Nuwakot", "Okhaldhunga",
	"Palpa", "Panchthar", "Parbat", "Pokhara", "Pyuthan",
	"Ramechhap", "Rasuwa", "Rautahat", "Rolpa", "Rupandehi", "Sankhuwasabha",
	"Saptari", "Sarlahi", "Sindhuli", "Sindhupalchok", "Siraha", "Solukhumbu",
	"Sunsari", "Surkhet", "Syangja", "Tanahu", "Taplejung", "Terhathum",
	"Udayapur",
}

// universityKeywords maps abbreviations and names to university affiliations.
var universityKeywords = map[string]string{
	"tu":  "Tribhuvan University",
	"ku":  "Kathmandu University",
	"pu":  "Pokhara University",
	"pu)": "Purbanchal University",
	"pou": "Purbanchal University",
}

// Parse converts a natural language query into a StructuredQuery.
func Parse(q string) StructuredQuery {
	original := q
	sq := StructuredQuery{OriginalQ: original}

	lower := strings.ToLower(strings.TrimSpace(q))
	words := strings.Fields(lower)
	origWords := strings.Fields(strings.TrimSpace(q))
	remaining := make([]string, len(words))
	copy(remaining, words)
	origRemaining := make([]string, len(origWords))
	copy(origRemaining, origWords)

	// 1. Extract intent (check early so intent words don't pollute other extraction)
	intent, remaining, origRemaining := extractIntent(remaining, origRemaining)
	sq.Intent = intent

	// 2. Extract location
	location, remaining, origRemaining := extractLocation(remaining, origRemaining)
	sq.Filters.Location = location

	// 3. Extract university affiliation
	uni, remaining, origRemaining := extractUniversity(remaining, origRemaining)
	sq.Filters.University = uni

	// 4. Extract entity/category
	category, remaining, origRemaining := extractCategory(remaining, origRemaining)
	sq.Category = category

	// 5. Remaining words become the semantic query (preserve original case)
	semantic := strings.TrimSpace(strings.Join(origRemaining, " "))
	semantic = cleanSemanticQuery(semantic)
	sq.Query = semantic

	return sq
}

// extractIntent scans for intent keywords and returns the detected intent + remaining words.
func extractIntent(words, origWords []string) (string, []string, []string) {
	lower := strings.Join(words, " ")

	for intent, keywords := range intentKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				remaining := removePhrase(words, kw)
				origRemaining := removePhrase(origWords, kw)
				return intent, remaining, origRemaining
			}
		}
	}
	return "", words, origWords
}

// extractLocation checks for location patterns and returns the location + remaining words.
func extractLocation(words, origWords []string) (string, []string, []string) {
	lower := strings.Join(words, " ")

	// Pattern 1: "in <location>", "near <location>", "at <location>", "of <location>"
	prepositions := []string{"in", "near", "at", "of", "around", "located in"}
	for _, prep := range prepositions {
		for _, loc := range knownLocations {
			pattern := prep + " " + strings.ToLower(loc)
			if strings.Contains(lower, pattern) {
				remaining := removePhrase(words, pattern)
				origRemaining := removePhrase(origWords, pattern)
				return loc, remaining, origRemaining
			}
		}
	}

	// Pattern 2: location at end of query — "colleges kathmandu"
	for _, loc := range knownLocations {
		lowerLoc := strings.ToLower(loc)
		if strings.HasSuffix(lower, lowerLoc) {
			idx := strings.LastIndex(lower, lowerLoc)
			if idx > 0 && lower[idx-1] != ' ' {
				continue
			}
			fields := strings.Fields(lowerLoc)
			remaining := words[:len(words)-len(fields)]
			origRemaining := origWords[:len(origWords)-len(fields)]
			return loc, remaining, origRemaining
		}
	}

	// Pattern 3: location at start — "kathmandu colleges"
	// Only if the NEXT word is NOT an entity type (to avoid "Kathmandu University" → location=Kathmandu)
	for _, loc := range knownLocations {
		lowerLoc := strings.ToLower(loc)
		if strings.HasPrefix(lower, lowerLoc+" ") {
			fields := strings.Fields(lowerLoc)
			nextIdx := len(fields)
			if nextIdx < len(words) {
				nextWord := words[nextIdx]
				if _, isEntity := entitySynonyms[nextWord]; isEntity {
					continue // Don't extract — it's part of a name like "Kathmandu University"
				}
			}
			remaining := words[nextIdx:]
			origRemaining := origWords[nextIdx:]
			return loc, remaining, origRemaining
		}
	}

	return "", words, origWords
}

// extractUniversity checks for university abbreviations/names.
func extractUniversity(words, origWords []string) (string, []string, []string) {
	lower := strings.Join(words, " ")

	for abbrev, fullName := range universityKeywords {
		lowerAbbrev := strings.ToLower(abbrev)
		patterns := []string{
			"at " + lowerAbbrev,
			"of " + lowerAbbrev,
			"from " + lowerAbbrev,
			lowerAbbrev + " ",
			" " + lowerAbbrev,
		}
		for _, pat := range patterns {
			if strings.Contains(lower, pat) {
				remaining := removePhrase(words, abbrev)
				origRemaining := removePhrase(origWords, abbrev)
				return fullName, remaining, origRemaining
			}
		}
	}

	return "", words, origWords
}

// extractCategory detects entity type from remaining words.
func extractCategory(words, origWords []string) (string, []string, []string) {
	for i, word := range words {
		cleaned := strings.Trim(word, ".,!?;:")
		if entityType, ok := entitySynonyms[cleaned]; ok {
			if i == 0 || i == len(words)-1 || isBeforeLocationMarker(words, i) {
				remaining := make([]string, 0, len(words)-1)
				remaining = append(remaining, words[:i]...)
				remaining = append(remaining, words[i+1:]...)
				origRemaining := make([]string, 0, len(origWords)-1)
				origRemaining = append(origRemaining, origWords[:i]...)
				origRemaining = append(origRemaining, origWords[i+1:]...)
				return entityType, remaining, origRemaining
			}
		}
	}

	for i := 0; i < len(words)-1; i++ {
		twoWord := words[i] + " " + words[i+1]
		if entityType, ok := entitySynonyms[twoWord]; ok {
			if i == 0 || i+1 == len(words)-1 {
				remaining := make([]string, 0, len(words)-2)
				remaining = append(remaining, words[:i]...)
				remaining = append(remaining, words[i+2:]...)
				origRemaining := make([]string, 0, len(origWords)-2)
				origRemaining = append(origRemaining, origWords[:i]...)
				origRemaining = append(origRemaining, origWords[i+2:]...)
				return entityType, remaining, origRemaining
			}
		}
	}

	return "", words, origWords
}

// isBeforeLocationMarker checks if a word is followed by a preposition (suggesting it's a category).
func isBeforeLocationMarker(words []string, idx int) bool {
	if idx+1 >= len(words) {
		return true // at end
	}
	next := strings.Trim(words[idx+1], ".,!?;:")
	prepositions := map[string]bool{
		"in": true, "near": true, "at": true, "of": true,
		"around": true, "located": true,
	}
	return prepositions[next]
}

// removePhrase removes a phrase (case-insensitive) from a word list.
func removePhrase(words []string, phrase string) []string {
	phraseWords := strings.Fields(strings.ToLower(phrase))
	result := make([]string, 0, len(words))

	i := 0
	for i < len(words) {
		matched := false
		if i+len(phraseWords) <= len(words) {
			match := true
			for j, pw := range phraseWords {
				if strings.ToLower(strings.Trim(words[i+j], ".,!?;:")) != pw {
					match = false
					break
				}
			}
			if match {
				matched = true
				i += len(phraseWords)
			}
		}
		if !matched {
			result = append(result, words[i])
			i++
		}
	}

	return result
}

// cleanSemanticQuery removes leftover syntax words from the semantic query.
func cleanSemanticQuery(q string) string {
	syntaxWords := []string{"in", "near", "at", "of", "around", "located", "the", "a", "an", "and", "or", "for", "to", "is", "are", "was"}
	words := strings.Fields(q)
	var result []string
	for _, w := range words {
		cleaned := strings.Trim(w, ".,!?;:")
		isSyntax := false
		for _, sw := range syntaxWords {
			if strings.ToLower(cleaned) == sw {
				isSyntax = true
				break
			}
		}
		if !isSyntax {
			result = append(result, w)
		}
	}
	return strings.TrimSpace(strings.Join(result, " "))
}

// IntentBoost returns a ranking score adjustment based on the detected intent and entity type.
func IntentBoost(intent, entityType string) float64 {
	switch intent {
	case "top":
		switch entityType {
		case "college", "university", "institution":
			return 0.05
		case "course":
			return 0.03
		default:
			return 0.02
		}
	case "latest":
		return 0.04
	default:
		return 0.0
	}
}

// ShouldSearchOnlyCategory returns true if the query clearly targets a single entity type.
func ShouldSearchOnlyCategory(sq StructuredQuery) bool {
	return sq.Category != ""
}

// CandidateScore pairs a candidate with metadata for intent-aware processing.
type CandidateScore struct {
	ID        uint
	Score     float64
	CreatedAt time.Time
}

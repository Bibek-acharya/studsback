package search

import "strings"

type SearchItem struct {
	ID              uint     `json:"id"`
	Type            string   `json:"type"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Image           string   `json:"image"`
	Featured        bool     `json:"featured"`
	Verified        bool     `json:"verified"`
	Rating          float64  `json:"rating"`
	InstitutionType string   `json:"institutionType"`
	Location        string   `json:"location"`
	University      string   `json:"university"`
	Website         string   `json:"website"`
	Slug            string   `json:"slug"`
	Tags            []string `gorm:"-" json:"tags"`
}

type SearchCategory struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Related     []string `json:"related"`
	Tabs        []string `json:"tabs"`
	Key         string   `json:"key"`
}

type SearchResponse struct {
	Items           []SearchItem              `json:"items"`
	Category        *SearchCategory           `json:"category"`
	CategoryKey     string                    `json:"categoryKey"`
	Meta            PaginationMeta            `json:"meta"`
	Facets          map[string]map[string]int `json:"facets,omitempty"`
	RetrievalErrors []string                  `json:"retrievalErrors,omitempty"`
	IsVectorEnabled bool                      `json:"isVectorEnabled"`
	Quality         string                    `json:"quality"` // "full", "keyword-only", "degraded", "error"
}

type PaginationMeta struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
	Pages int   `json:"pages"`
}

var categoryMeta = map[string]SearchCategory{
	"colleges": {
		Title:       "Colleges",
		Description: "Explore college listings, admission details, and student reviews",
		Related:     []string{"engineering", "medical", "business", "arts", "science", "law", "management", "pharmacy"},
		Tabs:        []string{"Discover", "Engineering", "Medical", "Management", "Arts & Humanities", "Science", "Law", "Pharmacy"},
	},
	"courses": {
		Title:       "Courses",
		Description: "Browse courses by subject, level, and institution",
		Related:     []string{"computer science", "business administration", "mechanical engineering", "mbbs", "mba", "data science", "ai & ml", "cybersecurity"},
		Tabs:        []string{"All Courses", "Undergraduate", "Postgraduate", "Diploma", "Certificate", "Online", "Distance Learning"},
	},
	"exams": {
		Title:       "Exams",
		Description: "Find exam dates, syllabus, preparation resources and more",
		Related:     []string{"entrance exams", "competitive exams", "language proficiency", "graduate exams", "undergraduate exams"},
		Tabs:        []string{"All Exams", "Entrance", "Graduate", "Language", "Certification", "Government Jobs"},
	},
	"scholarships": {
		Title:       "Scholarships",
		Description: "Discover scholarships, grants, and financial aid opportunities",
		Related:     []string{"fully funded", "merit based", "need based", "international", "women scholarships", "sports quota"},
		Tabs:        []string{"All Scholarships", "Merit Based", "Need Based", "International", "Government", "Private"},
	},
	"news": {
		Title:       "News & Blog",
		Description: "Stay updated with our latest announcements and stories",
		Related:     []string{"education news", "exam updates", "admission news", "policy changes", "career guidance"},
		Tabs:        []string{"All News", "Education", "Exams", "Admissions", "Career", "Policy"},
	},
	"events": {
		Title:       "Events",
		Description: "Upcoming education fairs, webinars, workshops and more",
		Related:     []string{"education fairs", "webinars", "workshops", "career fairs", "college fests"},
		Tabs:        []string{"All Events", "Fairs", "Webinars", "Workshops", "Conferences", "Career Fairs"},
	},
	"blogs": {
		Title:       "Blogs",
		Description: "Read expert articles, student experiences, and career guides",
		Related:     []string{"career guides", "study tips", "student life", "exam preparation", "college reviews"},
		Tabs:        []string{"All Blogs", "Career", "Study Tips", "Student Life", "Reviews", "Guides"},
	},
	"universities": {
		Title:       "Universities",
		Description: "Explore universities, programs, and research opportunities",
		Related:     []string{"tribhuvan university", "kathmandu university", "pokhara university", "engineering", "management", "science"},
		Tabs:        []string{"All Universities", "Government", "Private", "Affiliated", "Research"},
	},
	"admissions": {
		Title:       "Admissions",
		Description: "Browse admission openings, programs, and application deadlines",
		Related:     []string{"bachelor admission", "master admission", "MBBS", "engineering", "management", "diploma"},
		Tabs:        []string{"All Admissions", "Bachelor", "Master", "Diploma", "Certificate", "Open"},
	},
}

var categoryAliases = map[string]string{
	"college":        "college",
	"colleges":       "college",
	"institute":      "college",
	"institutes":     "college",
	"institution":    "college",
	"institutions":   "college",
	"course":         "course",
	"courses":        "course",
	"program":        "course",
	"programs":       "course",
	"exam":           "exam",
	"exams":          "exam",
	"entrance":       "exam",
	"scholarship":    "scholarship",
	"scholarships":   "scholarship",
	"grant":          "scholarship",
	"grants":         "scholarship",
	"news":           "news",
	"article":        "news",
	"articles":       "news",
	"event":          "event",
	"events":         "event",
	"blog":           "blog",
	"blogs":          "blog",
	"university":     "university",
	"universities":   "university",
	"admission":      "admission_page",
	"admissions":     "admission_page",
	"admission_page": "admission_page",
}

var categoryMetaKeys = map[string]string{
	"college":        "colleges",
	"course":         "courses",
	"exam":           "exams",
	"scholarship":    "scholarships",
	"news":           "news",
	"event":          "events",
	"blog":           "blogs",
	"university":     "universities",
	"admission_page": "admissions",
}

func resolveCategoryKey(q string, cat string) string {
	if cat != "" {
		if canonical, ok := categoryAliases[strings.ToLower(strings.TrimSpace(cat))]; ok {
			return canonical
		}
	}
	if q == "" {
		return ""
	}
	for _, word := range strings.Fields(strings.ToLower(q)) {
		word = strings.Trim(word, ".,!?;:")
		if canonical, ok := categoryAliases[word]; ok {
			return canonical
		}
	}
	return ""
}

func categoryMetaKey(category string) string {
	return categoryMetaKeys[category]
}

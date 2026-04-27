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
	Items       []SearchItem    `json:"items"`
	Category    *SearchCategory `json:"category"`
	CategoryKey string          `json:"categoryKey"`
	Meta        PaginationMeta  `json:"meta"`
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
}

var categoryKeywordMap = map[string]string{
	"college":      "colleges",
	"colleges":     "colleges",
	"course":       "courses",
	"courses":      "courses",
	"exam":         "exams",
	"exams":        "exams",
	"scholarship":  "scholarships",
	"scholarships": "scholarships",
	"news":         "news",
	"events":       "events",
	"event":        "events",
	"blog":         "blogs",
	"blogs":        "blogs",
}

func resolveCategoryKey(q string, cat string) string {
	if cat != "" {
		if _, ok := categoryMeta[cat]; ok {
			return cat
		}
	}
	if q == "" {
		return ""
	}
	lowerQ := strings.ToLower(q)
	for keyword, category := range categoryKeywordMap {
		if strings.Contains(lowerQ, keyword) {
			return category
		}
	}
	return ""
}

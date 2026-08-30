package retrieval

import "context"

type EntityType string

const (
	EntityCollege       EntityType = "college"
	EntityCourse        EntityType = "course"
	EntityScholarship   EntityType = "scholarship"
	EntityNews          EntityType = "news"
	EntityEvent         EntityType = "event"
	EntityExam          EntityType = "exam"
	EntityBlog          EntityType = "blog"
	EntityUniversity    EntityType = "university"
	EntityAdmissionPage EntityType = "admission_page"
	EntityInstitution   EntityType = "institution"
)

// EntityToIndexName maps singular entity types to plural index/table names.
var EntityToIndexName = map[EntityType]string{
	EntityCollege:       "colleges",
	EntityCourse:        "courses",
	EntityScholarship:   "scholarships",
	EntityNews:          "news",
	EntityEvent:         "events",
	EntityExam:          "exams",
	EntityBlog:          "blogs",
	EntityUniversity:    "universities",
	EntityAdmissionPage: "admission_pages",
	EntityInstitution:   "institutions",
}

type Candidate struct {
	ID                       uint       `json:"id"`
	Type                     EntityType `json:"type"`
	Title                    string     `json:"title"`
	Description              string     `json:"description,omitempty"`
	Image                    string     `json:"image,omitempty"`
	Banner                   string     `json:"banner,omitempty"`
	Logo                     string     `json:"logo,omitempty"`
	Slug                     string     `json:"slug,omitempty"`
	Rating                   float64    `json:"rating,omitempty"`
	Reviews                  int        `json:"reviews,omitempty"`
	Programs                 int        `json:"programs,omitempty"`
	Colleges                 int        `json:"colleges,omitempty"`
	EntityRank               int        `json:"entity_rank,omitempty"`
	Location                 string     `json:"location,omitempty"`
	URL                      string     `json:"url,omitempty"`
	EntityType               string     `json:"entity_type,omitempty"`
	InstitutionName          string     `json:"institution_name,omitempty"`
	Featured                 bool       `json:"featured,omitempty"`
	Verified                 bool       `json:"verified,omitempty"`
	Claimed                  bool       `json:"claimed,omitempty"`
	Popular                  bool       `json:"popular,omitempty"`
	Website                  string     `json:"website,omitempty"`
	University               string     `json:"university,omitempty"`
	NonUniversityAffiliation string     `json:"non_university_affiliation,omitempty"`
	Duration                 string     `json:"duration,omitempty"`
	Field                    string     `json:"field,omitempty"`
	EstimatedFee             string     `json:"estimated_fee,omitempty"`
	Rank                     int        `json:"-"`
	Score                    float64    `json:"-"`
	LexicalScore             float64    `json:"-"`
	VectorScore              float64    `json:"-"`
}

type SearchFilters struct {
	Category   string
	Location   string
	Type       string
	RatingMin  float64
	University string
}

type SearchRequest struct {
	Query   string
	Filters SearchFilters
	Limit   int
}

type Retriever interface {
	Search(ctx context.Context, req SearchRequest) ([]Candidate, error)
}

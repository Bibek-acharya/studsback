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
	ID              uint       `json:"id"`
	Type            EntityType `json:"type"`
	Title           string     `json:"title"`
	Description     string     `json:"description,omitempty"`
	Image           string     `json:"image,omitempty"`
	Slug            string     `json:"slug,omitempty"`
	Rating          float64    `json:"rating,omitempty"`
	Location        string     `json:"location,omitempty"`
	URL             string     `json:"url,omitempty"`
	EntityType      string     `json:"entity_type,omitempty"`
	InstitutionName string     `json:"institution_name,omitempty"`
	Rank            int        `json:"-"`
	Score           float64    `json:"-"`
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

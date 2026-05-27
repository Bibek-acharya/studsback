package search

import (
	"fmt"
	"math"
	"strings"

	"studsphere/backend/internal/embedding"

	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Search(q string, cat string, page int, limit int) (*SearchResponse, error) {
	categoryKey := resolveCategoryKey(q, cat)

	var items []SearchItem
	if q != "" && embedding.IsEnabled() {
		items = s.vectorSearch(q, categoryKey)
	}
	if len(items) == 0 {
		items = s.keywordSearch(q, categoryKey)
	}

	total := int64(len(items))
	pages := int64(math.Ceil(float64(total) / float64(limit)))

	start := (page - 1) * limit
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	if start > len(items) {
		items = []SearchItem{}
	} else {
		items = items[start:end]
	}

	var catPtr *SearchCategory
	if categoryKey != "" {
		if meta, ok := categoryMeta[categoryKey]; ok {
			m := meta
			m.Key = categoryKey
			catPtr = &m
		}
	}

	return &SearchResponse{
		Items:       items,
		Category:    catPtr,
		CategoryKey: categoryKey,
		Meta: PaginationMeta{
			Page:  page,
			Limit: limit,
			Total: total,
			Pages: int(pages),
		},
	}, nil
}

func (s *Service) vectorSearch(q string, categoryKey string) []SearchItem {
	vec, err := embedding.GenerateEmbedding(q)
	if err != nil || len(vec) == 0 {
		return nil
	}

	vectorStr := embedding.Float32SliceToPgVector(vec)
	tables := categoryTables(categoryKey)

	var items []SearchItem
	for _, table := range tables {
		selectSQL := searchSelectForTable(table)
		if selectSQL == "" {
			continue
		}
		var results []SearchItem
		sql := fmt.Sprintf("SELECT %s FROM %s WHERE embedding IS NOT NULL ORDER BY embedding <=> '%s'::vector LIMIT 30", selectSQL, table, vectorStr)
		if err := s.db.Raw(sql).Scan(&results).Error; err != nil {
			continue
		}
		items = append(items, results...)
	}
	return items
}

func (s *Service) keywordSearch(q string, categoryKey string) []SearchItem {
	var items []SearchItem
	tables := categoryTables(categoryKey)

	for _, table := range tables {
		switch table {
		case "colleges":
			items = append(items, s.searchColleges(q)...)
		case "courses":
			items = append(items, s.searchCourses(q)...)
		case "exams":
			items = append(items, s.searchExams(q)...)
		case "scholarships":
			items = append(items, s.searchScholarships(q)...)
		case "news":
			items = append(items, s.searchNews(q)...)
		case "events":
			items = append(items, s.searchEvents(q)...)
		case "blogs":
			items = append(items, s.searchBlogs(q)...)
		}
	}

	return items
}

func (s *Service) searchColleges(q string) []SearchItem {
	var results []SearchItem
	query := s.db.Table("colleges").Select(searchSelectColleges())
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("LOWER(COALESCE(name, '')) LIKE LOWER(?) OR LOWER(COALESCE(location, '')) LIKE LOWER(?) OR LOWER(COALESCE(description, '')) LIKE LOWER(?)", like, like, like)
	}
	query.Limit(30).Find(&results)
	return results
}

func (s *Service) searchCourses(q string) []SearchItem {
	var results []SearchItem
	query := s.db.Table("courses").Select(searchSelectCourses())
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("LOWER(COALESCE(title, '')) LIKE LOWER(?) OR LOWER(COALESCE(field, '')) LIKE LOWER(?) OR LOWER(COALESCE(affiliation, '')) LIKE LOWER(?)", like, like, like)
	}
	query.Limit(30).Find(&results)
	return results
}

func (s *Service) searchExams(q string) []SearchItem {
	var results []SearchItem
	query := s.db.Table("exams").Select(searchSelectExams())
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("LOWER(COALESCE(title, '')) LIKE LOWER(?) OR LOWER(COALESCE(description, '')) LIKE LOWER(?)", like, like)
	}
	query.Limit(30).Find(&results)
	return results
}

func (s *Service) searchScholarships(q string) []SearchItem {
	var results []SearchItem
	query := s.db.Table("scholarships").Select(searchSelectScholarships())
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("LOWER(COALESCE(title, '')) LIKE LOWER(?) OR LOWER(COALESCE(provider, '')) LIKE LOWER(?) OR LOWER(COALESCE(description, '')) LIKE LOWER(?)", like, like, like)
	}
	query.Limit(30).Find(&results)
	return results
}

func (s *Service) searchNews(q string) []SearchItem {
	var results []SearchItem
	query := s.db.Table("news").Select(searchSelectNews())
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("LOWER(COALESCE(title, '')) LIKE LOWER(?) OR LOWER(COALESCE(excerpt, '')) LIKE LOWER(?)", like, like)
	}
	query.Limit(30).Find(&results)
	return results
}

func (s *Service) searchEvents(q string) []SearchItem {
	var results []SearchItem
	query := s.db.Table("events").Select(searchSelectEvents())
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("LOWER(COALESCE(title, '')) LIKE LOWER(?) OR LOWER(COALESCE(description, '')) LIKE LOWER(?)", like, like)
	}
	query.Limit(30).Find(&results)
	return results
}

func (s *Service) searchBlogs(q string) []SearchItem {
	var results []SearchItem
	query := s.db.Table("blogs").Select(searchSelectBlogs()).Where("published = ?", true)
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("LOWER(COALESCE(title, '')) LIKE LOWER(?) OR LOWER(COALESCE(excerpt, '')) LIKE LOWER(?)", like, like)
	}
	query.Limit(30).Find(&results)
	return results
}

func categoryTables(categoryKey string) []string {
	if categoryKey != "" {
		if t := categoryToTable(categoryKey); t != "" {
			return []string{t}
		}
		return []string{}
	}
	return allTables()
}

func categoryToTable(categoryKey string) string {
	switch categoryKey {
	case "colleges":
		return "colleges"
	case "courses":
		return "courses"
	case "exams":
		return "exams"
	case "scholarships":
		return "scholarships"
	case "news":
		return "news"
	case "events":
		return "events"
	case "blogs":
		return "blogs"
	default:
		return ""
	}
}

func allTables() []string {
	return []string{"colleges", "courses", "exams", "scholarships", "news", "events", "blogs"}
}

func searchSelectForTable(table string) string {
	switch table {
	case "colleges":
		return searchSelectColleges()
	case "courses":
		return searchSelectCourses()
	case "exams":
		return searchSelectExams()
	case "scholarships":
		return searchSelectScholarships()
	case "news":
		return searchSelectNews()
	case "events":
		return searchSelectEvents()
	case "blogs":
		return searchSelectBlogs()
	default:
		return ""
	}
}

func searchSelectColleges() string {
	return "id, 'college' as type, COALESCE(name, '') as title, COALESCE(description, '') as description, COALESCE(image_url, '') as image, featured, verified, COALESCE(rating, 0) as rating, COALESCE(college_type, '') as institution_type, COALESCE(location, '') as location, COALESCE(affiliation, '') as university, COALESCE(website, '') as website, COALESCE(name, '') as slug"
}

func searchSelectCourses() string {
	return "id, 'course' as type, COALESCE(title, '') as title, COALESCE(description, '') as description, '' as image, false as featured, false as verified, 0 as rating, COALESCE(level, '') as institution_type, COALESCE(location, '') as location, COALESCE(affiliation, '') as university, '' as website, '' as slug"
}

func searchSelectExams() string {
	return "id, 'exam' as type, COALESCE(title, '') as title, COALESCE(description, '') as description, COALESCE(image_url, '') as image, false as featured, false as verified, 0 as rating, COALESCE(type, '') as institution_type, '' as location, COALESCE(university, '') as university, '' as website, COALESCE(slug, '') as slug"
}

func searchSelectScholarships() string {
	return "id, 'scholarship' as type, COALESCE(title, '') as title, COALESCE(description, '') as description, COALESCE(image_url, '') as image, false as featured, false as verified, 0 as rating, COALESCE(scholarship_type, '') as institution_type, COALESCE(location, '') as location, COALESCE(provider, '') as university, '' as website, '' as slug"
}

func searchSelectNews() string {
	return "id, 'news' as type, COALESCE(title, '') as title, COALESCE(excerpt, '') as description, COALESCE(image, '') as image, false as featured, false as verified, 0 as rating, COALESCE(category, '') as institution_type, '' as location, COALESCE(source, '') as university, '' as website, '' as slug"
}

func searchSelectEvents() string {
	return "id, 'event' as type, COALESCE(title, '') as title, COALESCE(description, '') as description, COALESCE(image, '') as image, COALESCE(trending, false) as featured, false as verified, 0 as rating, COALESCE(category, '') as institution_type, COALESCE(location, '') as location, COALESCE(organizer, '') as university, '' as website, '' as slug"
}

func searchSelectBlogs() string {
	return "id, 'blog' as type, COALESCE(title, '') as title, COALESCE(excerpt, '') as description, COALESCE(image, '') as image, COALESCE(featured, false) as featured, false as verified, 0 as rating, COALESCE(category, '') as institution_type, '' as location, COALESCE(author, '') as university, '' as website, COALESCE(slug, '') as slug"
}

func parseTags(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

package search

import (
	"fmt"
	"log"
	"math"
	"strings"

	"studsphere/backend/internal/embedding"
	"studsphere/backend/internal/shared/config"

	"github.com/pgvector/pgvector-go"
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

	useVector := config.IsPostgreSQL() && config.IsPGVectorReady() && embedding.IsEnabled()
	if useVector && q != "" {
		count := s.countEmbeddings(categoryKey)
		useVector = count > 0
	}

	var items []SearchItem
	if useVector && q != "" {
		items = s.vectorSearch(q, categoryKey)
	} else {
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

func (s *Service) countEmbeddings(categoryKey string) int64 {
	var total int64
	table := categoryToTable(categoryKey)
	if table == "" {
		for _, t := range allTables() {
			var c int64
			s.db.Table(t).Where("embedding IS NOT NULL").Count(&c)
			total += c
		}
		return total
	}
	s.db.Table(table).Where("embedding IS NOT NULL").Count(&total)
	return total
}

func (s *Service) vectorSearch(q string, categoryKey string) []SearchItem {
	queryVec, err := embedding.GenerateEmbedding(q)
	if err != nil {
		log.Printf("Vector search: embedding failed, falling back to keyword: %v", err)
		return s.keywordSearch(q, categoryKey)
	}

	dim := config.AppConfig.VectorDimension
	if len(queryVec) < dim {
		padded := make([]float32, dim)
		copy(padded, queryVec)
		queryVec = padded
	}

	pgVec := pgvector.NewVector(queryVec)

	var allItems []SearchItem
	tables := categoryTables(categoryKey)

	for _, table := range tables {
		items := s.vectorSearchTable(table, q, pgVec)
		allItems = append(allItems, items...)
	}

	return allItems
}

func (s *Service) vectorSearchTable(table string, q string, queryVec pgvector.Vector) []SearchItem {
	selectCols := vectorSelectForTable(table)

	var results []SearchItem

	likeClause := ""
	if q != "" {
		likeQ := strings.ReplaceAll(q, "'", "''")
		switch table {
		case "colleges":
			likeClause = fmt.Sprintf("AND (LOWER(name) LIKE LOWER('%%%s%%') OR LOWER(COALESCE(location, '')) LIKE LOWER('%%%s%%') OR LOWER(COALESCE(description, '')) LIKE LOWER('%%%s%%'))", likeQ, likeQ, likeQ)
		case "courses":
			likeClause = fmt.Sprintf("AND (LOWER(title) LIKE LOWER('%%%s%%') OR LOWER(COALESCE(field, '')) LIKE LOWER('%%%s%%'))", likeQ, likeQ)
		case "exams":
			likeClause = fmt.Sprintf("AND (LOWER(title) LIKE LOWER('%%%s%%') OR LOWER(COALESCE(description, '')) LIKE LOWER('%%%s%%'))", likeQ, likeQ)
		case "scholarships":
			likeClause = fmt.Sprintf("AND (LOWER(title) LIKE LOWER('%%%s%%') OR LOWER(COALESCE(provider, '')) LIKE LOWER('%%%s%%'))", likeQ, likeQ)
		case "news":
			likeClause = fmt.Sprintf("AND (LOWER(title) LIKE LOWER('%%%s%%') OR LOWER(COALESCE(excerpt, '')) LIKE LOWER('%%%s%%'))", likeQ, likeQ)
		case "events":
			likeClause = fmt.Sprintf("AND (LOWER(title) LIKE LOWER('%%%s%%') OR LOWER(COALESCE(description, '')) LIKE LOWER('%%%s%%'))", likeQ, likeQ)
		case "blogs":
			likeClause = fmt.Sprintf("AND (LOWER(title) LIKE LOWER('%%%s%%') OR LOWER(COALESCE(excerpt, '')) LIKE LOWER('%%%s%%'))", likeQ, likeQ)
		}
	}

	sql := fmt.Sprintf(`
		SELECT %s, 1 - (embedding <=> ?) AS vector_score
		FROM %s
		WHERE embedding IS NOT NULL %s
		ORDER BY vector_score DESC
		LIMIT 30
	`, selectCols, table, likeClause)

	s.db.Raw(sql, queryVec).Scan(&results)
	return results
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

func vectorSelectForTable(table string) string {
	base := searchSelectColleges()
	switch table {
	case "colleges":
		base = searchSelectColleges()
	case "courses":
		base = searchSelectCourses()
	case "exams":
		base = searchSelectExams()
	case "scholarships":
		base = searchSelectScholarships()
	case "news":
		base = searchSelectNews()
	case "events":
		base = searchSelectEvents()
	case "blogs":
		base = searchSelectBlogs()
	}
	// Remove the leading "id" and rebuild without
	return base
}

func parseTags(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

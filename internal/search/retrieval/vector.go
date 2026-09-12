package retrieval

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"studsphere/backend/internal/embedding"

	"gorm.io/gorm"
)

const vectorSimilarityThreshold = 0.55

type EmbeddingGenerator interface {
	GenerateEmbedding(text string) ([]float32, error)
}

type VectorRetriever struct {
	db               *gorm.DB
	embeddingService EmbeddingGenerator

	// ponytail: tiny TTL cache; repeat queries (pagination, retry, re-search)
	// must not pay the embedding API roundtrip again. Cleared when full.
	mu       sync.RWMutex
	embCache map[string]embCacheEntry
}

type embCacheEntry struct {
	vec []float32
	exp time.Time
}

const (
	embCacheTTL  = 10 * time.Minute
	embCacheSize = 500
)

func NewVectorRetriever(db *gorm.DB, emb EmbeddingGenerator) *VectorRetriever {
	return &VectorRetriever{
		db:               db,
		embeddingService: emb,
		embCache:         make(map[string]embCacheEntry),
	}
}

type vectorTable struct {
	table   string
	entity  EntityType
	selects string
}

var vectorTables = []vectorTable{
	{"colleges", EntityCollege, "id, COALESCE(name,'') as title, COALESCE(description,'') as description, COALESCE(name,'') as slug, COALESCE(image_url,'') as image, COALESCE(location,'') as location, COALESCE(rating,0) as rating"},
	{"courses", EntityCourse, "id, COALESCE(title,'') as title, COALESCE(description,'') as description, '' as slug, '' as image, '' as location, 0 as rating"},
	{"scholarships", EntityScholarship, "id, COALESCE(title,'') as title, COALESCE(description,'') as description, '' as slug, '' as image, COALESCE(location,'') as location, 0 as rating"},
	{"news", EntityNews, "id, COALESCE(title,'') as title, COALESCE(content,'') as description, '' as slug, '' as image, '' as location, 0 as rating"},
	{"events", EntityEvent, "id, COALESCE(title,'') as title, COALESCE(description,'') as description, '' as slug, COALESCE(image,'') as image, COALESCE(location,'') as location, 0 as rating"},
	{"exams", EntityExam, "id, COALESCE(title,'') as title, COALESCE(description,'') as description, '' as slug, '' as image, '' as location, 0 as rating"},
	{"blogs", EntityBlog, "id, COALESCE(title,'') as title, COALESCE(content,'') as description, '' as slug, '' as image, '' as location, 0 as rating"},
	{"universities", EntityUniversity, "id, COALESCE(name,'') as title, COALESCE(description,'') as description, COALESCE(name,'') as slug, COALESCE(logo,'') as image, COALESCE(location,'') as location, COALESCE(rating,0) as rating"},
	{"admission_pages", EntityAdmissionPage, "id, COALESCE(title,'') as title, COALESCE(title,'') as description, COALESCE(title,'') as slug, '' as image, COALESCE(institution_location,'') as location, 0 as rating"},
	{"institution_users", EntityInstitution, "id, COALESCE(institution_name,'') as title, COALESCE(about,'') as description, '' as slug, COALESCE(logo_url,'') as image, COALESCE(district,'') as location, 0 as rating"},
}

func (r *VectorRetriever) Search(ctx context.Context, req SearchRequest) ([]Candidate, error) {
	vec, err := r.cachedEmbedding(req.Query)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	vectorStr := embedding.Float32SliceToPgVector(vec)
	tables := r.resolveTables(req.Filters.Category)

	results := make([][]Candidate, len(tables))
	errs := make([]error, len(tables))
	var wg sync.WaitGroup
	for i, vt := range tables {
		wg.Add(1)
		go func(i int, vt vectorTable) {
			defer wg.Done()
			items, err := r.searchTable(ctx, vt, vectorStr, req)
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = items
		}(i, vt)
	}
	wg.Wait()

	var candidates []Candidate
	var searchErrors []error
	for i := range tables {
		if errs[i] != nil {
			searchErrors = append(searchErrors, errs[i])
			continue
		}
		candidates = append(candidates, results[i]...)
	}
	if len(searchErrors) == len(tables) {
		return nil, errors.Join(searchErrors...)
	}

	return candidates, nil
}

func (r *VectorRetriever) cachedEmbedding(query string) ([]float32, error) {
	key := strings.TrimSpace(query)

	r.mu.RLock()
	if e, ok := r.embCache[key]; ok && time.Now().Before(e.exp) {
		vec := e.vec
		r.mu.RUnlock()
		return vec, nil
	}
	r.mu.RUnlock()

	vec, err := r.embeddingService.GenerateEmbedding(query)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	if len(r.embCache) >= embCacheSize {
		r.embCache = make(map[string]embCacheEntry)
	}
	r.embCache[key] = embCacheEntry{vec: vec, exp: time.Now().Add(embCacheTTL)}
	r.mu.Unlock()

	return vec, nil
}

func (r *VectorRetriever) resolveTables(category string) []vectorTable {
	if category == "" {
		return vectorTables
	}
	// "college" searches both colleges AND institutions
	if category == "college" {
		var result []vectorTable
		for _, vt := range vectorTables {
			if vt.entity == EntityCollege || vt.entity == EntityInstitution {
				result = append(result, vt)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	for _, vt := range vectorTables {
		if string(vt.entity) == category || vt.table == category {
			return []vectorTable{vt}
		}
	}
	return vectorTables
}

func (r *VectorRetriever) searchTable(ctx context.Context, vt vectorTable, vectorStr string, req SearchRequest) ([]Candidate, error) {
	type row struct {
		ID          uint
		Title       string
		Description string
		Slug        string
		Image       string
		Location    string
		Rating      float64
		Distance    float64
	}

	var rows []row
	query := r.db.WithContext(ctx).
		Select(vt.selects+", (embedding <=> ?::vector) AS distance", vectorStr).
		Table(vt.table).
		Where("embedding IS NOT NULL AND deleted_at IS NULL")

	if req.Filters.Location != "" {
		switch vt.entity {
		case EntityCollege, EntityScholarship, EntityEvent, EntityUniversity:
			query = query.Where("location = ?", req.Filters.Location)
		case EntityAdmissionPage:
			query = query.Where("institution_location = ?", req.Filters.Location)
		case EntityInstitution:
			query = query.Where("district = ?", req.Filters.Location)
		}
	}
	if req.Filters.RatingMin > 0 && (vt.entity == EntityCollege || vt.entity == EntityUniversity) {
		query = query.Where("rating >= ?", req.Filters.RatingMin)
	}

	if err := query.Order(gorm.Expr("embedding <=> ?::vector", vectorStr)).
		Limit(req.Limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("vector search %s: %w", vt.table, err)
	}

	candidates := make([]Candidate, 0, len(rows))
	for rank, row := range rows {
		similarity := 1 - row.Distance
		if similarity < vectorSimilarityThreshold {
			continue
		}
		candidates = append(candidates, Candidate{
			ID:          row.ID,
			Type:        vt.entity,
			Title:       row.Title,
			Description: row.Description,
			Slug:        row.Slug,
			Image:       row.Image,
			Location:    row.Location,
			Rating:      row.Rating,
			Rank:        rank + 1,
			VectorScore: similarity,
		})
	}

	return candidates, nil
}

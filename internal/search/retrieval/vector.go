package retrieval

import (
	"context"
	"fmt"

	"studsphere/backend/internal/embedding"

	"gorm.io/gorm"
)

type EmbeddingGenerator interface {
	GenerateEmbedding(text string) ([]float32, error)
}

type VectorRetriever struct {
	db               *gorm.DB
	embeddingService EmbeddingGenerator
}

func NewVectorRetriever(db *gorm.DB, emb EmbeddingGenerator) *VectorRetriever {
	return &VectorRetriever{
		db:               db,
		embeddingService: emb,
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
	{"site_pages", EntitySitePage, "id, COALESCE(title,'') as title, COALESCE(content,'') as description, COALESCE(slug,'') as slug, '' as image, '' as location, 0 as rating"},
	{"universities", EntityUniversity, "id, COALESCE(name,'') as title, COALESCE(description,'') as description, COALESCE(name,'') as slug, COALESCE(logo,'') as image, COALESCE(location,'') as location, COALESCE(rating,0) as rating"},
	{"admission_pages", EntityAdmissionPage, "id, COALESCE(title,'') as title, COALESCE(title,'') as description, COALESCE(title,'') as slug, '' as image, COALESCE(institution_location,'') as location, 0 as rating"},
	{"institution_users", EntityInstitution, "id, COALESCE(institution_name,'') as title, COALESCE(about,'') as description, '' as slug, COALESCE(logo_url,'') as image, COALESCE(district,'') as location, 0 as rating"},
}

func (r *VectorRetriever) Search(ctx context.Context, req SearchRequest) ([]Candidate, error) {
	vec, err := r.embeddingService.GenerateEmbedding(req.Query)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	vectorStr := embedding.Float32SliceToPgVector(vec)
	tables := r.resolveTables(req.Filters.Category)

	var candidates []Candidate
	for _, vt := range tables {
		items, err := r.searchTable(ctx, vt, vectorStr, req)
		if err != nil {
			continue
		}
		candidates = append(candidates, items...)
	}

	return candidates, nil
}

func (r *VectorRetriever) resolveTables(category string) []vectorTable {
	if category == "" {
		return vectorTables
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
	}

	var rows []row
	query := r.db.WithContext(ctx).
		Select(vt.selects).
		Table(vt.table).
		Where("embedding IS NOT NULL AND deleted_at IS NULL")

	if req.Filters.Location != "" {
		query = query.Where("location = ?", req.Filters.Location)
	}
	if req.Filters.RatingMin > 0 {
		query = query.Where("rating >= ?", req.Filters.RatingMin)
	}

	if err := query.Order(gorm.Expr("embedding <=> ?::vector", vectorStr)).
		Limit(req.Limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("vector search %s: %w", vt.table, err)
	}

	candidates := make([]Candidate, 0, len(rows))
	for rank, row := range rows {
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
		})
	}

	return candidates, nil
}

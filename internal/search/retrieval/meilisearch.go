package retrieval

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/meilisearch/meilisearch-go"
)

var allEntities = []EntityType{
	EntityCollege, EntityCourse, EntityScholarship,
	EntityNews, EntityEvent, EntityExam, EntityBlog, EntitySitePage,
	EntityUniversity, EntityAdmissionPage,
}

type IndexClient interface {
	Search(query string, request *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error)
}

type MeilisearchIndexProvider interface {
	Index(indexUID string) IndexClient
}

type MeilisearchRetriever struct {
	provider    MeilisearchIndexProvider
	indexPrefix string
}

func NewMeilisearchRetriever(provider MeilisearchIndexProvider, indexPrefix string) *MeilisearchRetriever {
	return &MeilisearchRetriever{
		provider:    provider,
		indexPrefix: indexPrefix,
	}
}

func (r *MeilisearchRetriever) Search(ctx context.Context, req SearchRequest) ([]Candidate, error) {
	entities := r.resolveEntities(req.Filters.Category)

	type indexResult struct {
		entity EntityType
		hits   []map[string]interface{}
		err    error
	}

	results := make([]indexResult, len(entities))
	var wg sync.WaitGroup

	for i, entity := range entities {
		wg.Add(1)
		go func(idx int, e EntityType) {
			defer wg.Done()
			hits, err := r.searchIndex(ctx, e, req)
			results[idx] = indexResult{entity: e, hits: hits, err: err}
		}(i, entity)
	}

	wg.Wait()

	var candidates []Candidate
	for _, res := range results {
		if res.err != nil {
			continue
		}
		for rank, hit := range res.hits {
			c := hitToCandidate(hit, res.entity)
			c.Rank = rank + 1
			candidates = append(candidates, c)
		}
	}

	return candidates, nil
}

func (r *MeilisearchRetriever) resolveEntities(category string) []EntityType {
	if category == "" {
		return allEntities
	}
	for _, e := range allEntities {
		if string(e) == category || EntityToIndexName[e] == category {
			return []EntityType{e}
		}
	}
	return allEntities
}

func (r *MeilisearchRetriever) searchIndex(ctx context.Context, entity EntityType, req SearchRequest) ([]map[string]interface{}, error) {
	indexName := r.indexName(entity)
	idx := r.provider.Index(indexName)

	searchReq := &meilisearch.SearchRequest{
		Query:                 req.Query,
		Limit:                 int64(req.Limit),
		AttributesToRetrieve: []string{"*"},
		ShowRankingScore:      true,
	}

	filters := r.buildFilters(entity, req.Filters)
	if filters != "" {
		searchReq.Filter = filters
	}

	resp, err := idx.Search(req.Query, searchReq)
	if err != nil {
		return nil, fmt.Errorf("meilisearch search %s: %w", indexName, err)
	}

	hits := make([]map[string]interface{}, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		if m, ok := hit.(map[string]interface{}); ok {
			hits = append(hits, m)
		}
	}
	return hits, nil
}

func (r *MeilisearchRetriever) buildFilters(entity EntityType, f SearchFilters) string {
	var parts []string

	if f.Location != "" {
		switch entity {
		case EntityAdmissionPage:
			parts = append(parts, fmt.Sprintf("institution_location = %q", f.Location))
		default:
			parts = append(parts, fmt.Sprintf("location = %q", f.Location))
		}
	}
	if f.Type != "" {
		switch entity {
		case EntityCollege:
			parts = append(parts, fmt.Sprintf("college_type = %q", f.Type))
		case EntityUniversity:
			parts = append(parts, fmt.Sprintf("type = %q", f.Type))
		default:
			parts = append(parts, fmt.Sprintf("type = %q", f.Type))
		}
	}
	if f.RatingMin > 0 {
		parts = append(parts, fmt.Sprintf("rating >= %f", f.RatingMin))
	}
	if f.University != "" {
		parts = append(parts, fmt.Sprintf("affiliation = %q", f.University))
	}

	return strings.Join(parts, " AND ")
}

func hitToCandidate(hit map[string]interface{}, entity EntityType) Candidate {
	c := Candidate{Type: entity}

	if id, ok := hit["id"]; ok {
		switch v := id.(type) {
		case float64:
			c.ID = uint(v)
		case int:
			c.ID = uint(v)
		}
	}

	if v, ok := hit["name"].(string); ok {
		c.Title = v
	} else if v, ok := hit["title"].(string); ok {
		c.Title = v
	}

	if v, ok := hit["description"].(string); ok {
		c.Description = v
	} else if v, ok := hit["excerpt"].(string); ok {
		c.Description = v
	}

	if v, ok := hit["image"].(string); ok {
		c.Image = v
	} else if v, ok := hit["logo"].(string); ok {
		c.Image = v
	} else if v, ok := hit["cover"].(string); ok {
		c.Image = v
	} else if v, ok := hit["image_url"].(string); ok {
		c.Image = v
	}

	if v, ok := hit["slug"].(string); ok {
		c.Slug = v
	} else if entity == EntityUniversity {
		if v, ok := hit["name"].(string); ok {
			c.Slug = slugify(v)
		}
	} else if entity == EntityAdmissionPage {
		if v, ok := hit["title"].(string); ok {
			c.Slug = slugify(v)
		}
	}

	if v, ok := hit["rating"].(float64); ok {
		c.Rating = v
	}

	if v, ok := hit["location"].(string); ok {
		c.Location = v
	} else if v, ok := hit["institution_location"].(string); ok {
		c.Location = v
	}

	if v, ok := hit["type"].(string); ok {
		c.EntityType = v
	} else if v, ok := hit["level"].(string); ok {
		c.EntityType = v
	}

	if v, ok := hit["institution_name"].(string); ok {
		c.InstitutionName = v
	}

	if v, ok := hit["_rankingScore"].(float64); ok {
		c.Score = v
	}

	return c
}

func slugify(s string) string {
	result := make([]rune, 0, len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == ' ' {
			if r == ' ' {
				result = append(result, '-')
			} else {
				result = append(result, r)
			}
		}
	}
	return strings.Trim(string(result), "-")
}

func (r *MeilisearchRetriever) indexName(entity EntityType) string {
	if name, ok := EntityToIndexName[entity]; ok {
		return r.indexPrefix + name
	}
	return r.indexPrefix + string(entity)
}

func (r *MeilisearchRetriever) GetFacets(ctx context.Context, req SearchRequest, facetAttributes []string) map[string]map[string]int {
	entities := r.resolveEntities(req.Filters.Category)
	aggregated := make(map[string]map[string]int)
	for _, attr := range facetAttributes {
		aggregated[attr] = make(map[string]int)
	}

	type facetResult struct {
		distribution map[string]int
	}

	results := make([]facetResult, len(entities))
	var wg sync.WaitGroup

	for i, entity := range entities {
		wg.Add(1)
		go func(idx int, e EntityType) {
			defer wg.Done()
			indexName := r.indexName(e)
			idxClient := r.provider.Index(indexName)

			searchReq := &meilisearch.SearchRequest{
				Query:  req.Query,
				Limit:  0,
				Facets: facetAttributes,
			}
			filters := r.buildFilters(e, req.Filters)
			if filters != "" {
				searchReq.Filter = filters
			}

			resp, err := idxClient.Search(req.Query, searchReq)
			if err != nil {
				return
			}

			dist := make(map[string]int)
			if fd, ok := resp.FacetDistribution.(map[string]interface{}); ok {
				for attr, vals := range fd {
					if m, ok := vals.(map[string]interface{}); ok {
						for val, count := range m {
							if c, ok := count.(float64); ok {
								dist[attr+":"+val] += int(c)
							}
						}
					}
				}
			}
			results[idx] = facetResult{distribution: dist}
		}(i, entity)
	}

	wg.Wait()

	for _, res := range results {
		for key, count := range res.distribution {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				aggregated[parts[0]][parts[1]] += count
			}
		}
	}

	return aggregated
}

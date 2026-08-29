package search

import (
	"context"
	"log"
	"math"
	"sort"
	"strconv"
	"time"

	"studsphere/backend/internal/search/queryparser"
	"studsphere/backend/internal/search/ranking"
	"studsphere/backend/internal/search/retrieval"

	"gorm.io/gorm"
)

type SearchService struct {
	meiliRetriever   *retrieval.MeilisearchRetriever
	vecRetriever     *retrieval.VectorRetriever
	rrf              *ranking.RRFScorer
	exactMatcher     *ranking.ExactMatcher
	businessBoost    *ranking.BusinessBooster
	db               *gorm.DB
	embeddingEnabled bool
}

type HybridSearchRequest struct {
	Query      string
	Category   string
	Location   string
	Type       string
	RatingMin  float64
	University string
	Sort       string
	Intent     string
	Page       int
	Limit      int
}

func NewSearchService(
	db *gorm.DB,
	meiliRetriever *retrieval.MeilisearchRetriever,
	vecRetriever *retrieval.VectorRetriever,
	embeddingEnabled bool,
) *SearchService {
	return &SearchService{
		db:               db,
		meiliRetriever:   meiliRetriever,
		vecRetriever:     vecRetriever,
		embeddingEnabled: embeddingEnabled,
		rrf:              ranking.NewRRFScorer(60),
		exactMatcher:     ranking.NewExactMatcher(ranking.ExactMatchConfig{
			TitleExact:  0.5,
			TitlePhrase: 0.2,
			TitlePrefix: 0.1,
			TokenMatch:  0.05,
			DescMatch:   0.005,
		}),
		businessBoost: ranking.NewBusinessBooster(ranking.BusinessBoostConfig{
			News:  ranking.EntityBoost{FreshnessBoost: 0.03},
			Event: ranking.EntityBoost{FreshnessBoost: 0.05},
			Blog:  ranking.EntityBoost{FreshnessBoost: 0.01},
		}),
	}
}

func (s *SearchService) Search(ctx context.Context, req HybridSearchRequest) *SearchResponse {
	start := time.Now()

	if req.Page < 1 {
		req.Page = 1
	}
	if req.Page > 5 {
		req.Page = 5
	}
	if req.Limit < 1 {
		req.Limit = 20
	}
	if req.Limit > 50 {
		req.Limit = 50
	}

	cat := resolveCategoryKey(req.Query, req.Category)

	retrievalReq := retrieval.SearchRequest{
		Query: req.Query,
		Filters: retrieval.SearchFilters{
			Category:   cat,
			Location:   req.Location,
			Type:       req.Type,
			RatingMin:  req.RatingMin,
			University: req.University,
		},
		Limit: 100,
	}

	meiliCh := make(chan []retrieval.Candidate, 1)
	vecCh := make(chan []retrieval.Candidate, 1)
	facetsCh := make(chan map[string]map[string]int, 1)

	go func() {
		candidates, err := s.meiliRetriever.Search(ctx, retrievalReq)
		if err != nil {
			log.Printf("search: meilisearch error: %v", err)
			meiliCh <- nil
			return
		}
		meiliCh <- candidates
	}()

	if s.embeddingEnabled {
		go func() {
			candidates, err := s.vecRetriever.Search(ctx, retrievalReq)
			if err != nil {
				log.Printf("search: vector error: %v", err)
				vecCh <- nil
				return
			}
			vecCh <- candidates
		}()
	} else {
		vecCh <- nil
	}

	go func() {
		facets := s.meiliRetriever.GetFacets(ctx, retrievalReq, []string{"location", "college_type", "rating"})
		facetsCh <- facets
	}()

	meiliResults := <-meiliCh
	vecResults := <-vecCh
	facets := <-facetsCh

	merged := mergeCandidates(meiliResults, vecResults)
	sourceRanks := buildSourceRanks(meiliResults, vecResults)
	ranked := s.rrf.RankCandidates(merged, sourceRanks)
	ranked = s.exactMatcher.Boost(ranked, req.Query)
	ranked = s.businessBoost.Boost(ranked)

	// Apply intent-aware boost
	if req.Intent != "" {
		intentBoost := queryparser.IntentBoost(req.Intent, cat)
		if intentBoost > 0 {
			for i := range ranked {
				ranked[i].Score += intentBoost
			}
		}
	}

	if req.Sort != "" && req.Sort != "relevance" {
		applySort(ranked, req.Sort)
	}

	// Filter out low-relevance candidates
	// Threshold ensures only meaningful matches appear in results
	minScore := 0.001
	var filtered []retrieval.Candidate
	for _, c := range ranked {
		if c.Score > minScore || req.Query == "" {
			filtered = append(filtered, c)
		}
	}
	ranked = filtered

	total := len(ranked)
	startIdx := (req.Page - 1) * req.Limit
	endIdx := startIdx + req.Limit
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}
	paged := ranked[startIdx:endIdx]

	items := make([]SearchItem, len(paged))
	for i, c := range paged {
		items[i] = SearchItem{
			ID:              c.ID,
			Type:            string(c.Type),
			Title:           c.Title,
			Description:     c.Description,
			Image:           c.Image,
			Slug:            c.Slug,
			Rating:          c.Rating,
			Location:        c.Location,
			InstitutionType: c.EntityType,
			University:      c.University,
			Website:         c.Website,
			Featured:        c.Featured,
			Verified:        c.Verified,
		}
	}

	var catPtr *SearchCategory
	if cat != "" {
		if meta, ok := categoryMeta[cat]; ok {
			m := meta
			m.Key = cat
			catPtr = &m
		}
	}

	pages := int(math.Ceil(float64(total) / float64(req.Limit)))

	log.Printf("search: q=%q cat=%s intent=%s location=%s results=%d latency=%s", req.Query, cat, req.Intent, req.Location, total, time.Since(start))

	return &SearchResponse{
		Items:       items,
		Category:    catPtr,
		CategoryKey: cat,
		Meta: PaginationMeta{
			Page:  req.Page,
			Limit: req.Limit,
			Total: int64(total),
			Pages: pages,
		},
		Facets: facets,
	}
}

func candidateKey(c retrieval.Candidate) string {
	return string(c.Type) + ":" + strconv.FormatUint(uint64(c.ID), 10)
}

func mergeCandidates(meili, vec []retrieval.Candidate) []retrieval.Candidate {
	seen := make(map[string]bool)
	var result []retrieval.Candidate

	for _, c := range meili {
		key := candidateKey(c)
		if !seen[key] {
			seen[key] = true
			result = append(result, c)
		}
	}
	for _, c := range vec {
		key := candidateKey(c)
		if !seen[key] {
			seen[key] = true
			result = append(result, c)
		}
	}

	return result
}

func buildSourceRanks(meili, vec []retrieval.Candidate) map[string][]int {
	ranks := make(map[string][]int)
	for _, c := range meili {
		key := candidateKey(c)
		ranks[key] = append(ranks[key], c.Rank)
	}
	for _, c := range vec {
		key := candidateKey(c)
		ranks[key] = append(ranks[key], c.Rank)
	}
	return ranks
}

func applySort(candidates []retrieval.Candidate, sortBy string) {
	switch sortBy {
	case "rating_desc":
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Rating > candidates[j].Rating
		})
	case "created_at_desc":
		// Already in insertion order from Meilisearch (newest first by default)
	case "title_asc":
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Title < candidates[j].Title
		})
	}
}

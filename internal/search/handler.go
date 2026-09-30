package search

import (
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"studsphere/backend/internal/embedding"
	"studsphere/backend/internal/search/queryparser"
	"studsphere/backend/internal/search/retrieval"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	searchService *SearchService
	meiliClient   *MeiliClient
	historyRepo   *SearchHistoryRepository
}

func NewHandler(searchService *SearchService) *Handler {
	return &Handler{searchService: searchService}
}

func NewHybridHandler(searchService *SearchService, meiliClient *MeiliClient) *Handler {
	return &Handler{
		searchService: searchService,
		meiliClient:   meiliClient,
	}
}

func (h *Handler) SetHistoryRepository(repo *SearchHistoryRepository) {
	h.historyRepo = repo
}

func (h *Handler) Search(c *gin.Context) {
	rawQ := c.Query("q")
	cat := c.Query("cat")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	sort := c.DefaultQuery("sort", "relevance")

	location := c.Query("location")
	entitytype := c.Query("type")
	ratingMin, _ := strconv.ParseFloat(c.DefaultQuery("rating_min", "0"), 64)
	university := c.Query("university")
	includeFacets := c.DefaultQuery("facets", "") == "1"

	// Validate query length
	if len(rawQ) == 0 {
		vectorEnabled := false
		if h.searchService != nil {
			vectorEnabled = h.searchService.IsEmbeddingEnabled()
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Empty query",
			"data": gin.H{
				"items":           []SearchItem{},
				"meta":            PaginationMeta{Page: 1, Limit: limit, Total: 0, Pages: 0},
				"category":        nil,
				"categoryKey":     "",
				"facets":          map[string]map[string]int{},
				"retrievalErrors": []string{},
				"isVectorEnabled": vectorEnabled,
				"quality":         "success",
			},
		})
		return
	}

	if len(rawQ) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Query must be at least 2 characters",
		})
		return
	}

	if len(rawQ) > 256 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Query too long (max 256 characters)",
		})
		return
	}

	// Run query understanding
	parsed := queryparser.Parse(rawQ)

	// Merge explicit query params with parsed results (explicit params win)
	if location == "" {
		location = parsed.Filters.Location
	}
	if university == "" {
		university = parsed.Filters.University
	}
	if cat == "" && parsed.Category != "" {
		cat = parsed.Category
	}

	// Use the semantic remainder as the search query
	q := parsed.Query
	if q == "" && parsed.Category == "" && parsed.Intent == "" && parsed.Filters.Location == "" && parsed.Filters.University == "" {
		q = rawQ
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	if h.searchService != nil {
		result := h.searchService.Search(c.Request.Context(), HybridSearchRequest{
			Query:         q,
			Category:      cat,
			Location:      location,
			Type:          entitytype,
			RatingMin:     ratingMin,
			University:    university,
			Sort:          sort,
			Intent:        parsed.Intent,
			Page:          page,
			Limit:         limit,
			IncludeFacets: includeFacets,
		})

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Search results retrieved successfully",
			"data": gin.H{
				"items":           result.Items,
				"category":        result.Category,
				"categoryKey":     result.CategoryKey,
				"meta":            result.Meta,
				"facets":          result.Facets,
				"quality":         result.Quality,
				"retrievalErrors": result.RetrievalErrors,
				"isVectorEnabled": result.IsVectorEnabled,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Search service not available",
		"data":    gin.H{"items": []SearchItem{}, "meta": PaginationMeta{Page: page, Limit: limit}},
	})
}

func (h *Handler) Suggest(c *gin.Context) {
	q := c.Query("q")
	cat := c.Query("cat")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))

	if q == "" {
		c.JSON(http.StatusOK, gin.H{"suggestions": []interface{}{}})
		return
	}
	if limit < 1 || limit > 10 {
		limit = 5
	}

	if h.meiliClient == nil || !h.meiliClient.IsHealthy() {
		c.JSON(http.StatusOK, gin.H{"suggestions": []interface{}{}})
		return
	}

	// Meilisearch-only for suggestions (no pgvector)
	ctx := c.Request.Context()
	adapter := &serviceManagerAdapter{sm: h.meiliClient.Client}
	meiliRetriever := retrieval.NewMeilisearchRetriever(adapter, h.meiliClient.IndexPrefix)

	catNormalized := resolveCategoryKey(q, cat)
	suggestionReq := retrieval.SearchRequest{
		Query: q,
		Filters: retrieval.SearchFilters{
			Category: catNormalized,
		},
		Limit: limit * 4,
	}

	hits, err := meiliRetriever.Search(ctx, suggestionReq)
	if err != nil {
		log.Printf("suggest: meilisearch error: %v", err)
		c.JSON(http.StatusOK, gin.H{"suggestions": []interface{}{}})
		return
	}

	type suggestion struct {
		Type  string `json:"type"`
		ID    string `json:"id"`
		Label string `json:"label"`
		URL   string `json:"url"`
	}

	var suggestions []suggestion
	for _, item := range hits {
		slug := item.Slug
		if slug == "" {
			slug = strconv.FormatUint(uint64(item.ID), 10)
		}
		entityName := retrieval.EntityToIndexName[item.Type]
		if entityName == "" {
			entityName = string(item.Type) + "s"
		}
		suggestions = append(suggestions, suggestion{
			Type:  string(item.Type),
			ID:    strconv.FormatUint(uint64(item.ID), 10),
			Label: item.Title,
			URL:   "/" + entityName + "/" + slug,
		})
	}

	// Re-sort so text-prefix matches appear first (suggestion feel).
	lowerQ := strings.ToLower(q)
	sort.SliceStable(suggestions, func(i, j int) bool {
		li, lj := strings.ToLower(suggestions[i].Label), strings.ToLower(suggestions[j].Label)
		iPrefix := strings.HasPrefix(li, lowerQ)
		jPrefix := strings.HasPrefix(lj, lowerQ)
		if iPrefix != jPrefix {
			return iPrefix // prefix matches first
		}
		iHas := strings.Contains(li, lowerQ)
		jHas := strings.Contains(lj, lowerQ)
		if iHas != jHas {
			return iHas // contains matches second
		}
		return li < lj // alphabetical within group
	})
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

// Reindex triggers a full embedding reindex across the platform.
//
// The role check comes FIRST, before embedding.IsEnabled(), and that ordering is
// deliberate: IsEnabled is a configuration flag and the disabled branch answers
// 202 with a message naming it, so checking afterwards would let any
// authenticated tenant read the platform's embedding configuration by branching
// on the status code. access.go has the full argument.
//
// The force flag is the destructive one — ReindexAllForce nulls every embedding
// column on all seventeen tables and re-embeds them, including other tenants'
// rows. Both flags are refused together; there is no safe variant worth
// separating.
func (h *Handler) Reindex(c *gin.Context) {
	force := c.DefaultQuery("force", "false") == "true"

	viewer := ViewerFrom(c)
	if err := h.searchService.authorizeReindex(viewer, force); err != nil {
		response.Error(c, http.StatusForbidden, ErrForbidden.Error())
		return
	}

	if !embedding.IsEnabled() {
		response.Success(c, http.StatusAccepted, "Embedding is not enabled. Set EMBEDDING_ENABLED=true in .env", nil)
		return
	}
	if embedding.GetReindexProgress().Running {
		response.Error(c, http.StatusConflict, "An embedding reindex is already running")
		return
	}
	db := config.GetDB()
	go func() {
		var err error
		if force {
			err = embedding.ReindexAllForce(db)
		} else {
			err = embedding.ReindexAll()
		}
		if err != nil {
			log.Printf("Reindex error: %v", err)
		}
	}()
	msg := "Embedding reindex started in background"
	if force {
		msg = "Full AI retrain started — clearing and regenerating all embeddings"
	}
	response.Success(c, http.StatusAccepted, msg, nil)
}

// ReindexStatus reports progress on the running sweep — which table, how many
// rows processed, and the last error.
//
// Platform-admin only, on the same reasoning as the reindex itself: it is the
// operational half of the same feature, it names a concrete table and row count
// for a sweep spanning every tenant, and there is no non-admin consumer of it.
// Leaving it on roleMW would have meant closing the door on the operation and
// leaving the window open on its progress.
func (h *Handler) ReindexStatus(c *gin.Context) {
	if err := h.searchService.authorizeReindex(ViewerFrom(c), false); err != nil {
		response.Error(c, http.StatusForbidden, ErrForbidden.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": embedding.GetReindexProgress()})
}

func (h *Handler) GetVectorStatus(c *gin.Context) {
	pgvectorReady := false
	if !config.IsSQLite {
		var result int
		if err := config.GetDB().Raw("SELECT count(*) FROM pg_extension WHERE extname = 'vector'").Scan(&result).Error; err == nil && result > 0 {
			pgvectorReady = true
		}
	}
	meiliReady := h.meiliClient != nil && h.meiliClient.IsHealthy()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"embedding_enabled": embedding.IsEnabled(),
			"pgvector_ready":    pgvectorReady,
			"meilisearch_ready": meiliReady,
			"message":           "Vector search status",
		},
	})
}

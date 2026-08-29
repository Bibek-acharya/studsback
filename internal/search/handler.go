package search

import (
	"log"
	"net/http"
	"strconv"

	"studsphere/backend/internal/embedding"
	"studsphere/backend/internal/search/retrieval"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	searchService *SearchService
	meiliClient   *MeiliClient
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

func (h *Handler) Search(c *gin.Context) {
	q := c.Query("q")
	cat := c.Query("cat")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	sort := c.DefaultQuery("sort", "relevance")

	location := c.Query("location")
	entitytype := c.Query("type")
	ratingMin, _ := strconv.ParseFloat(c.DefaultQuery("rating_min", "0"), 64)
	university := c.Query("university")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	if h.searchService != nil {
		result := h.searchService.Search(c.Request.Context(), HybridSearchRequest{
			Query:      q,
			Category:   cat,
			Location:   location,
			Type:       entitytype,
			RatingMin:  ratingMin,
			University: university,
			Sort:       sort,
			Page:       page,
			Limit:      limit,
		})

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Search results retrieved successfully",
			"data": gin.H{
				"items":       result.Items,
				"category":    result.Category,
				"categoryKey": result.CategoryKey,
				"meta":        result.Meta,
				"facets":      result.Facets,
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
		Limit: limit,
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

	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

func (h *Handler) Reindex(c *gin.Context) {
	if !embedding.IsEnabled() {
		response.Success(c, http.StatusAccepted, "Embedding is not enabled. Set EMBEDDING_ENABLED=true in .env", nil)
		return
	}
	db := config.GetDB()
	force := c.DefaultQuery("force", "false") == "true"
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

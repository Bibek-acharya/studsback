package search

import (
	"net/http"
	"strconv"

	"studsphere/backend/internal/embedding"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Search(c *gin.Context) {
	q := c.Query("q")
	cat := c.Query("cat")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 12
	}

	useVector := config.IsPostgreSQL() && config.IsPGVectorReady() && embedding.IsEnabled()
	vectorInfo := gin.H{
		"enabled":     useVector,
		"provider":    "openai",
		"model":       config.AppConfig.EmbeddingModel,
		"dimensions":  config.AppConfig.VectorDimension,
	}

	if q == "" && !useVector {
		// If no query and no vectors, skip vector info
		vectorInfo = nil
	}

	result, err := h.service.Search(q, cat, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Search failed")
		return
	}

	responseData := gin.H{
		"items":       result.Items,
		"category":    result.Category,
		"categoryKey": result.CategoryKey,
		"meta":        result.Meta,
		"vector":      vectorInfo,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Search results retrieved successfully",
		"data":    responseData,
	})
}

func (h *Handler) Reindex(c *gin.Context) {
	if !embedding.IsEnabled() {
		response.Error(c, http.StatusBadRequest, "Embedding service is not configured. Set EMBEDDING_ENABLED=true and EMBEDDING_API_KEY in .env")
		return
	}

	go func() {
		if err := embedding.ReindexAll(); err != nil {
			c.Error(err)
		}
	}()

	response.Success(c, http.StatusAccepted, "Embedding reindex started in background. Check server logs for progress.", nil)
}

func (h *Handler) GetVectorStatus(c *gin.Context) {
	enabled := embedding.IsEnabled()
	pgReady := config.IsPostgreSQL() && config.IsPGVectorReady()

	status := gin.H{
		"embedding_enabled":  enabled,
		"pgvector_ready":     pgReady,
		"model":              config.AppConfig.EmbeddingModel,
		"dimensions":         config.AppConfig.VectorDimension,
		"is_postgresql":      config.IsPostgreSQL(),
	}

	if pgReady && enabled {
		var vectorsCount int64
		for _, table := range allTables() {
			var c int64
			config.GetDB().Table(table).Where("embedding IS NOT NULL").Count(&c)
			vectorsCount += c
		}
		status["total_vectors"] = vectorsCount
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

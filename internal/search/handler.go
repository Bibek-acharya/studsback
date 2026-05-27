package search

import (
	"log"
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
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Search results retrieved successfully",
		"data":    responseData,
	})
}

func (h *Handler) Reindex(c *gin.Context) {
	if !embedding.IsEnabled() {
		response.Success(c, http.StatusAccepted, "Embedding is not enabled. Set EMBEDDING_ENABLED=true in .env", nil)
		return
	}
	go func() {
		if err := embedding.ReindexAll(); err != nil {
			log.Printf("Reindex error: %v", err)
		}
	}()
	response.Success(c, http.StatusAccepted, "Embedding reindex started in background. Check server logs for progress.", nil)
}

func (h *Handler) GetVectorStatus(c *gin.Context) {
	pgvectorReady := false
	if !config.IsSQLite {
		var result int
		if err := config.GetDB().Raw("SELECT count(*) FROM pg_extension WHERE extname = 'vector'").Scan(&result).Error; err == nil && result > 0 {
			pgvectorReady = true
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"embedding_enabled": embedding.IsEnabled(),
			"pgvector_ready":    pgvectorReady,
			"message":           "Vector search is enabled",
		},
	})
}

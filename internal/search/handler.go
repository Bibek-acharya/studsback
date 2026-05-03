package search

import (
	"net/http"
	"strconv"

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
	response.Success(c, http.StatusAccepted, "Reindex is not available (vector search disabled)", nil)
}

func (h *Handler) GetVectorStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"embedding_enabled": false,
			"pgvector_ready":    false,
			"message":           "Vector search has been disabled",
		},
	})
}

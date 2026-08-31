package search

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SearchHistory struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	UserID     uint      `gorm:"uniqueIndex:idx_search_histories_user_query" json:"user_id"`
	Query      string    `gorm:"size:256;uniqueIndex:idx_search_histories_user_query" json:"query"`
	SearchedAt time.Time `json:"searched_at"`
}

type SearchHistoryRepository struct {
	db *gorm.DB
}

func NewSearchHistoryRepository(db *gorm.DB) *SearchHistoryRepository {
	return &SearchHistoryRepository{db: db}
}

func (r *SearchHistoryRepository) Save(userID uint, query string) error {
	now := time.Now()
	entry := SearchHistory{UserID: userID, Query: query, SearchedAt: now}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "query"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"searched_at": now}),
	}).Create(&entry).Error
}

func (r *SearchHistoryRepository) List(userID uint, limit int) ([]SearchHistory, error) {
	var entries []SearchHistory
	if err := r.db.Where("user_id = ?", userID).
		Order("searched_at desc").
		Limit(limit).
		Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (h *Handler) GetSearchHistory(c *gin.Context) {
	userID, _ := c.Get("user_id")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	entries, err := h.historyRepo.List(userID.(uint), limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve search history")
		return
	}

	queries := make([]string, 0, len(entries))
	for _, entry := range entries {
		queries = append(queries, entry.Query)
	}

	response.Success(c, http.StatusOK, "Search history retrieved successfully", gin.H{"queries": queries})
}

func (h *Handler) SaveSearchHistory(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		Query string `json:"query" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Query is required")
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" || len(query) > 256 {
		response.Error(c, http.StatusBadRequest, "Query must be 1-256 characters")
		return
	}

	if err := h.historyRepo.Save(userID.(uint), query); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to save search history")
		return
	}

	response.Success(c, http.StatusOK, "Search history saved successfully", nil)
}

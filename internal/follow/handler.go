package follow

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Follow(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	targetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target ID"})
		return
	}

	targetType := c.DefaultQuery("type", "institution")
	if targetType != "institution" && targetType != "university" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target type"})
		return
	}

	if err := h.service.Follow(userID, uint(targetID), targetType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to follow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Followed successfully"})
}

func (h *Handler) Unfollow(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	targetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target ID"})
		return
	}

	targetType := c.DefaultQuery("type", "institution")

	h.service.Unfollow(userID, uint(targetID), targetType)
	c.JSON(http.StatusOK, gin.H{"message": "Unfollowed successfully"})
}

func (h *Handler) Status(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"following": false})
		return
	}

	targetID, err := strconv.ParseUint(c.Param("targetId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target ID"})
		return
	}

	targetType := c.DefaultQuery("type", "institution")

	following, _ := h.service.IsFollowing(userID, uint(targetID), targetType)
	c.JSON(http.StatusOK, gin.H{"following": following})
}

func (h *Handler) FollowedList(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	targetType := c.DefaultQuery("type", "institution")

	var ids []uint
	var err error
	if targetType == "university" {
		ids, err = h.service.GetFollowedUniversities(userID)
	} else {
		ids, err = h.service.GetFollowedInstitutions(userID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch followed list"})
		return
	}
	if ids == nil {
		ids = []uint{}
	}
	c.JSON(http.StatusOK, gin.H{"target_ids": ids})
}

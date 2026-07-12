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

	instID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid institution ID"})
		return
	}

	if err := h.service.Follow(userID, uint(instID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to follow institution"})
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

	instID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid institution ID"})
		return
	}

	h.service.Unfollow(userID, uint(instID))
	c.JSON(http.StatusOK, gin.H{"message": "Unfollowed successfully"})
}

func (h *Handler) Status(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"following": false})
		return
	}

	instID, err := strconv.ParseUint(c.Param("institutionId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid institution ID"})
		return
	}

	following, _ := h.service.IsFollowing(userID, uint(instID))
	c.JSON(http.StatusOK, gin.H{"following": following})
}

func (h *Handler) FollowedList(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	ids, _ := h.service.GetFollowedInstitutions(userID)
	if ids == nil {
		ids = []uint{}
	}
	c.JSON(http.StatusOK, gin.H{"institution_ids": ids})
}

package review

import (
	"errors"
	"net/http"
	"strconv"

	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func getUserID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	return userID.(uint)
}

func (h *Handler) SubmitReview(c *gin.Context) {
	userID := getUserID(c)

	var req CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	review, err := h.service.SubmitReview(userID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Review submitted successfully", gin.H{"review": review})
}

func (h *Handler) GetUserReviews(c *gin.Context) {
	userID := getUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := h.service.GetUserReviews(userID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Reviews fetched successfully", result)
}

func (h *Handler) GetCollegeReviews(c *gin.Context) {
	collegeID, err := strconv.ParseUint(c.Param("collegeId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid college ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := h.service.GetCollegeReviews(uint(collegeID), page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Reviews fetched successfully", result)
}

func (h *Handler) UpdateReview(c *gin.Context) {
	userID := getUserID(c)

	reviewID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid review ID")
		return
	}

	var req UpdateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	review, err := h.service.UpdateReview(uint(reviewID), userID, req)
	if err != nil {
		if err.Error() == "review not found" {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Review updated successfully", gin.H{"review": review})
}

func (h *Handler) DeleteReview(c *gin.Context) {
	userID := getUserID(c)

	reviewID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid review ID")
		return
	}

	if err := h.service.DeleteReview(uint(reviewID), userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "Review not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Review deleted successfully", nil)
}

func (h *Handler) MarkHelpful(c *gin.Context) {
	userID := getUserID(c)

	reviewID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid review ID")
		return
	}

	helpfulCount, err := h.service.MarkHelpful(uint(reviewID), userID)
	if err != nil {
		if err.Error() == "review not found" {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Marked as helpful", gin.H{"helpful_count": helpfulCount})
}

func (h *Handler) ReportReview(c *gin.Context) {
	userID := getUserID(c)

	reviewID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid review ID")
		return
	}

	var req ReportReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ReportReview(uint(reviewID), userID, req.Reason); err != nil {
		if err.Error() == "review not found" {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Review reported successfully", nil)
}

func (h *Handler) SubmitUniversityReview(c *gin.Context) {
	userID := getUserID(c)

	var req CreateUniversityReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	review, err := h.service.SubmitUniversityReview(userID, req)
	if err != nil {
		if err.Error() == "you have already reviewed this university" {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Review submitted successfully", gin.H{"review": review})
}

func (h *Handler) GetUniversityReviews(c *gin.Context) {
	universityID, err := strconv.ParseUint(c.Param("universityId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid university ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := h.service.GetUniversityReviews(uint(universityID), page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Reviews fetched successfully", result)
}

func (h *Handler) GetMyUniversityReview(c *gin.Context) {
	userID := getUserID(c)

	universityID, err := strconv.ParseUint(c.Param("universityId"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid university ID")
		return
	}

	review, err := h.service.GetUserUniversityReview(userID, uint(universityID))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Review not found")
		return
	}

	response.Success(c, http.StatusOK, "Review fetched successfully", gin.H{"review": review})
}

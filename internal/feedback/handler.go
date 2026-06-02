package feedback

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

func getUserID(c *gin.Context) uint {
	id, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	return id.(uint)
}

func (h *Handler) SubmitFeedback(c *gin.Context) {
	var req CreateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := getUserID(c)
	if userID == 0 {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	alreadySubmitted, err := h.service.HasUserSubmitted(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to check feedback status")
		return
	}
	if alreadySubmitted {
		response.Error(c, http.StatusConflict, "You have already submitted feedback")
		return
	}

	_, err = h.service.SubmitFeedback(userID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to submit feedback")
		return
	}

	response.Success(c, http.StatusCreated, "Feedback submitted successfully", nil)
}

func (h *Handler) ListFeedback(c *gin.Context) {
	feedbacks, err := h.service.ListFeedback()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch feedback")
		return
	}

	if feedbacks == nil {
		feedbacks = []FeedbackResponse{}
	}

	response.Success(c, http.StatusOK, "Feedback fetched successfully", feedbacks)
}

func (h *Handler) DeleteFeedback(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid feedback ID")
		return
	}

	if err := h.service.DeleteFeedback(uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete feedback")
		return
	}

	response.Success(c, http.StatusOK, "Feedback deleted successfully", nil)
}

func (h *Handler) GetPublicFeedbacks(c *gin.Context) {
	limit := 10
	feedbacks, err := h.service.ListPublicFeedback(limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch feedback")
		return
	}

	if feedbacks == nil {
		feedbacks = []FeedbackResponse{}
	}

	response.Success(c, http.StatusOK, "Testimonials fetched successfully", feedbacks)
}

func (h *Handler) CheckFeedbackStatus(c *gin.Context) {
	userID := getUserID(c)
	if userID == 0 {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	submitted, err := h.service.HasUserSubmitted(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to check feedback status")
		return
	}

	response.Success(c, http.StatusOK, "Feedback status", map[string]bool{"submitted": submitted})
}

func (h *Handler) SubmitTestimonial(c *gin.Context) {
	var req CreateTestimonialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := h.service.SubmitTestimonial(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to submit testimonial")
		return
	}

	response.Success(c, http.StatusCreated, "Testimonial submitted successfully", nil)
}

package tools

import (
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetScholarshipFinderRecommendations(c *gin.Context) {
	var req ScholarshipFinderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	recommendations, err := h.service.GetScholarshipRecommendations(req)
	if err != nil {
		response.Error(c, 500, "Failed to load scholarships")
		return
	}

	response.Success(c, 200, "Scholarship recommendations generated", ScholarshipRecommendationsResponse{
		Recommendations: recommendations,
	})
}

func (h *Handler) GetCollegeRecommenderRecommendations(c *gin.Context) {
	var req CollegeRecommenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	recommendations, err := h.service.GetCollegeRecommendations(req)
	if err != nil {
		response.Error(c, 500, "Failed to load colleges")
		return
	}

	response.Success(c, 200, "College recommendations generated", CollegeRecommendationsResponse{
		Recommendations: recommendations,
	})
}

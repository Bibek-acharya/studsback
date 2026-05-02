package tools

import (
	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"

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

func (h *Handler) UploadLogo(c *gin.Context) {
	header, err := c.FormFile("logo")
	if err != nil {
		response.Error(c, 400, "Failed to get logo file")
		return
	}

	url, err := utils.SaveUploadedImage(header, "tools/logo")
	if err != nil {
		response.Error(c, 500, "Failed to upload logo: "+err.Error())
		return
	}

	response.Success(c, 200, "Logo uploaded successfully", map[string]string{
		"url": url,
	})
}

func (h *Handler) GetLogoURL(c *gin.Context) {
	url := emailqueue.GetStudSphereLogoURL()
	response.Success(c, 200, "Logo URL retrieved", map[string]string{
		"url": url,
	})
}

func (h *Handler) ServeLogo(c *gin.Context) {
	logoData, contentType, err := utils.ReadLatestUploadedImage("tools/logo")
	if err != nil {
		// Fallback: return a simple SVG placeholder
		svgLogo := `<svg xmlns="http://www.w3.org/2000/svg" width="150" height="40" viewBox="0 0 150 40">
			<rect width="150" height="40" fill="#2563eb" rx="8"/>
			<text x="75" y="25" text-anchor="middle" fill="white" font-family="Arial, sans-serif" font-size="14" font-weight="bold">StudSphere</text>
		</svg>`

		c.Header("Content-Type", "image/svg+xml")
		c.Header("Cache-Control", "public, max-age=3600") // Cache for 1 hour
		c.Data(200, "image/svg+xml", []byte(svgLogo))
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	c.Data(200, contentType, logoData)
}

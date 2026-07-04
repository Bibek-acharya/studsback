package college

import (
	"net/http"
	"strconv"

	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service         *Service
	institutionRepo *institution.Repository
}

func NewHandler(service *Service, institutionRepo *institution.Repository) *Handler {
	return &Handler{service: service, institutionRepo: institutionRepo}
}

func (h *Handler) GetColleges(c *gin.Context) {
	filters := CollegeFilters{
		UniversityID:    c.Query("universityId"),
		Location:        c.Query("location"),
		Affiliation:     c.Query("affiliation"),
		Type:            c.Query("type"),
		Academic:        c.QueryArray("academic"),
		Program:         c.QueryArray("program"),
		Province:        c.QueryArray("province"),
		District:        c.QueryArray("district"),
		Local:           c.QueryArray("local"),
		Scholarship:     c.QueryArray("scholarship"),
		Facilities:      c.QueryArray("facilities"),
		FeeMax:          0,
		Verified:        c.Query("verified"),
		Popular:         c.Query("popular"),
		DirectAdmission: c.Query("directAdmission") == "true",
		MinRating:       c.Query("minRating"),
		Search:          c.Query("search"),
		CourseID:        c.Query("courseId"),
		Sort:            c.DefaultQuery("sort", "rating"),
		Order:           c.DefaultQuery("order", "DESC"),
	}

	if feeMax, err := strconv.Atoi(c.Query("feeMax")); err == nil && feeMax > 0 {
		filters.FeeMax = feeMax
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	filters.Page = page
	filters.PageSize = pageSize

	result, err := h.service.GetColleges(filters)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Colleges retrieved successfully", result)
}

func (h *Handler) GetCollegeByID(c *gin.Context) {
	collegeID := c.Param("id")
	parsedID, err := strconv.ParseUint(collegeID, 10, 64)
	if err != nil || parsedID == 0 {
		response.Error(c, 400, "Invalid college ID")
		return
	}

	result, err := h.service.GetCollegeByID(uint(parsedID))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "College retrieved successfully", result)
}

func (h *Handler) CreateCollege(c *gin.Context) {
	var req CreateCollegeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.CreateCollege(req)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 201, "College created successfully", result)
}

func (h *Handler) UploadCollegeImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		response.Error(c, 400, "No image file provided")
		return
	}

	urls, err := h.service.UploadCollegeImage(file)
	if err != nil {
		response.Error(c, 500, "Failed to upload image")
		return
	}

	response.Success(c, 200, "Image uploaded successfully", gin.H{"url": urls[0]})
}

func (h *Handler) UpdateCollege(c *gin.Context) {
	collegeID := c.Param("id")
	parsedID, err := strconv.ParseUint(collegeID, 10, 64)
	if err != nil || parsedID == 0 {
		response.Error(c, 400, "Invalid college ID")
		return
	}

	var req UpdateCollegeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.UpdateCollege(uint(parsedID), req)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 200, "College updated successfully", result)
}

func (h *Handler) DeleteCollege(c *gin.Context) {
	collegeID := c.Param("id")
	parsedID, err := strconv.ParseUint(collegeID, 10, 64)
	if err != nil || parsedID == 0 {
		response.Error(c, 400, "Invalid college ID")
		return
	}

	if err := h.service.DeleteCollege(uint(parsedID)); err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "College deleted successfully", nil)
}

func (h *Handler) ApproveCollege(c *gin.Context) {
	collegeID := c.Param("id")
	parsedID, err := strconv.ParseUint(collegeID, 10, 64)
	if err != nil || parsedID == 0 {
		response.Error(c, 400, "Invalid college ID")
		return
	}

	result, err := h.service.ApproveCollege(uint(parsedID))
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 200, "College approved successfully", result)
}

func (h *Handler) ToggleCollegeFeatured(c *gin.Context) {
	collegeID := c.Param("id")
	parsedID, err := strconv.ParseUint(collegeID, 10, 64)
	if err != nil || parsedID == 0 {
		response.Error(c, 400, "Invalid college ID")
		return
	}

	result, err := h.service.ToggleCollegeFeatured(uint(parsedID))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "College featured status updated", result)
}

func (h *Handler) GetFeaturedColleges(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := h.service.GetFeaturedColleges(limit)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Featured colleges retrieved successfully", result)
}

func (h *Handler) GetCollegeFilterCounts(c *gin.Context) {
	level := c.Query("level")
	result, err := h.service.GetCollegeFilterCounts(level)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "College filter counts retrieved successfully", result)
}

func (h *Handler) RecommendColleges(c *gin.Context) {
	var req CollegeRecommenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "Invalid request: "+err.Error())
		return
	}

	var userID *uint
	if uid, exists := c.Get("user_id"); exists && uid != nil {
		if id, ok := uid.(uint); ok && id > 0 {
			userID = &id
		}
	}

	recommendations, err := h.service.RecommendColleges(req, userID)
	if err != nil {
		response.Error(c, 500, "Failed to get recommendations")
		return
	}

	response.Success(c, 200, "Recommendations retrieved successfully", CollegeRecommendResponse{
		Recommendations: recommendations,
	})
}

func (h *Handler) GetMapColleges(c *gin.Context) {
	var north, south, east, west float64
	var err error

	if q := c.Query("north"); q != "" {
		north, err = strconv.ParseFloat(q, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "Invalid north parameter")
			return
		}
	}
	if q := c.Query("south"); q != "" {
		south, err = strconv.ParseFloat(q, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "Invalid south parameter")
			return
		}
	}
	if q := c.Query("east"); q != "" {
		east, err = strconv.ParseFloat(q, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "Invalid east parameter")
			return
		}
	}
	if q := c.Query("west"); q != "" {
		west, err = strconv.ParseFloat(q, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "Invalid west parameter")
			return
		}
	}

	colleges, err := h.service.GetMapColleges(north, south, east, west)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch map colleges")
		return
	}
	response.Success(c, http.StatusOK, "Colleges retrieved", gin.H{"colleges": colleges})
}

func (h *Handler) UpdateCollegeLocation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid college ID")
		return
	}

	var req struct {
		Latitude  float64 `json:"latitude" binding:"required"`
		Longitude float64 `json:"longitude" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := h.service.UpdateCollegeLocation(uint(id), req.Latitude, req.Longitude); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Location updated", gin.H{"id": id, "latitude": req.Latitude, "longitude": req.Longitude})
}

func (h *Handler) UpdateInstitutionCollegeLocation(c *gin.Context) {
	userID := c.GetUint("user_id")
	instUser, err := h.institutionRepo.FindInstitutionUserByID(userID)
	if err != nil || instUser.CollegeID == 0 {
		response.Error(c, http.StatusForbidden, "No college associated with your account")
		return
	}

	var req struct {
		Latitude  float64 `json:"latitude" binding:"required"`
		Longitude float64 `json:"longitude" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := h.service.UpdateCollegeLocation(instUser.CollegeID, req.Latitude, req.Longitude); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "College location updated", gin.H{
		"college_id": instUser.CollegeID,
		"latitude":   req.Latitude,
		"longitude":  req.Longitude,
	})
}

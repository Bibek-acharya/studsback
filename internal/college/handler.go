package college

import (
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
	result, err := h.service.GetCollegeFilterCounts()
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "College filter counts retrieved successfully", result)
}

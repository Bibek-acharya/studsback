package education

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

func (h *Handler) GetEducationRankings(c *gin.Context) {
	colleges, err := h.service.GetEducationRankings()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch rankings")
		return
	}

	response.Success(c, http.StatusOK, "Education rankings retrieved successfully", gin.H{"colleges": colleges})
}

func (h *Handler) GetEducationExams(c *gin.Context) {
	exams, err := h.service.GetEducationExams()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch exams")
		return
	}

	response.Success(c, http.StatusOK, "Education exams retrieved successfully", gin.H{"exams": exams})
}

func (h *Handler) GetEducationExamByID(c *gin.Context) {
	id := c.Param("id")
	exam, err := h.service.GetEducationExamByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Exam not found")
		return
	}

	response.Success(c, http.StatusOK, "Education exam retrieved successfully", exam)
}

func (h *Handler) GetEducationCourses(c *gin.Context) {
	courses, err := h.service.GetEducationCourses()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	response.Success(c, http.StatusOK, "Education courses retrieved successfully", gin.H{"courses": courses})
}

func (h *Handler) GetEducationCourseByID(c *gin.Context) {
	id := c.Param("id")
	course, err := h.service.GetEducationCourseByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	response.Success(c, http.StatusOK, "Education course retrieved successfully", course)
}

func (h *Handler) GetEducationCourseDetailsByID(c *gin.Context) {
	id := c.Param("id")
	details, err := h.service.GetEducationCourseDetailsByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	response.Success(c, http.StatusOK, "Education course details retrieved successfully", details)
}

func (h *Handler) GetEducationNews(c *gin.Context) {
	news, err := h.service.GetEducationNews()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch news")
		return
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", gin.H{"news": news})
}

func (h *Handler) GetEducationNewsByID(c *gin.Context) {
	id := c.Param("id")
	news, err := h.service.GetEducationNewsByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "News article not found")
		return
	}

	response.Success(c, http.StatusOK, "News article retrieved successfully", news)
}

func (h *Handler) GetEducationEvents(c *gin.Context) {
	events, err := h.service.GetEducationEvents()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch events")
		return
	}

	response.Success(c, http.StatusOK, "Events retrieved successfully", gin.H{"events": events})
}

func (h *Handler) GetEducationEventByID(c *gin.Context) {
	id := c.Param("id")
	event, err := h.service.GetEducationEventByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	response.Success(c, http.StatusOK, "Event retrieved successfully", event)
}

func (h *Handler) GetEducationBlogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	category := c.Query("category")
	search := c.Query("search")

	blogs, meta, err := h.service.GetEducationBlogs(page, limit, category, search)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch blogs")
		return
	}

	response.Success(c, http.StatusOK, "Blogs retrieved successfully", gin.H{
		"blogs": blogs,
		"meta":  meta,
	})
}

func (h *Handler) GetEducationBlogByID(c *gin.Context) {
	id := c.Param("id")
	blogWithRelated, err := h.service.GetEducationBlogByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Blog post not found")
		return
	}

	response.Success(c, http.StatusOK, "Blog post retrieved successfully", blogWithRelated)
}

// ─── Admin CRUD Handlers ─────────────────────────────────────────────────────

func (h *Handler) AdminGetBlogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	category := c.Query("category")
	search := c.Query("search")

	blogs, meta, err := h.service.GetAllBlogsAdmin(page, limit, category, search)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch blogs")
		return
	}

	response.Success(c, http.StatusOK, "Blogs retrieved successfully", gin.H{
		"blogs": blogs,
		"meta":  meta,
	})
}

func (h *Handler) CreateBlog(c *gin.Context) {
	var req CreateBlogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	blog, err := h.service.CreateBlog(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create blog")
		return
	}

	response.Success(c, http.StatusCreated, "Blog created successfully", blog)
}

func (h *Handler) UpdateBlog(c *gin.Context) {
	id := c.Param("id")
	var req UpdateBlogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	blog, err := h.service.UpdateBlog(id, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update blog")
		return
	}

	response.Success(c, http.StatusOK, "Blog updated successfully", blog)
}

func (h *Handler) DeleteBlog(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteBlog(id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete blog")
		return
	}

	response.Success(c, http.StatusOK, "Blog deleted successfully", nil)
}

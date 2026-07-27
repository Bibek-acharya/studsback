package education

import (
	"fmt"
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

func (h *Handler) SearchGlobalCourses(c *gin.Context) {
	query := c.Query("q")
	courses, err := h.service.SearchGlobalCourses(query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to search courses")
		return
	}
	response.Success(c, http.StatusOK, "Courses retrieved successfully", gin.H{"courses": courses})
}

func (h *Handler) GetEducationCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	level := c.Query("level")
	field := c.Query("field")
	affiliation := c.Query("affiliation")

	// If no filtering params, use old endpoint
	if search == "" && level == "" && field == "" && affiliation == "" {
		courses, err := h.service.GetEducationCourses()
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
			return
		}
		response.Success(c, http.StatusOK, "Education courses retrieved successfully", gin.H{"courses": courses})
		return
	}

	courses, meta, err := h.service.GetEducationCoursesPaginated(page, limit, search, level, field, affiliation)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	response.Success(c, http.StatusOK, "Education courses retrieved successfully", gin.H{
		"courses": courses,
		"meta":    meta,
	})
}

func (h *Handler) GetCourseFilterCounts(c *gin.Context) {
	counts, err := h.service.GetCourseFilterCounts()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch filter counts")
		return
	}

	response.Success(c, http.StatusOK, "Filter counts retrieved successfully", gin.H{
		"level_counts":       counts.LevelCount,
		"field_counts":       counts.FieldCount,
		"affiliation_counts": counts.AffiliationCount,
		"total":              counts.Total,
	})
}

func (h *Handler) GetInstitutionCourses(c *gin.Context) {
	instID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid institution ID")
		return
	}

	courses, err := h.service.GetInstitutionCourses(uint(instID))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch institution courses")
		return
	}

	response.Success(c, http.StatusOK, "Institution courses retrieved successfully", gin.H{"courses": courses})
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
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	category := c.Query("category")
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "newest")

	// If no filtering params, use old endpoint
	if category == "" && search == "" {
		news, err := h.service.GetEducationNews()
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to fetch news")
			return
		}
		response.Success(c, http.StatusOK, "News retrieved successfully", gin.H{"news": news})
		return
	}

	news, meta, err := h.service.GetEducationNewsFiltered(page, limit, category, search, sort)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch news")
		return
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", gin.H{
		"news": news,
		"meta": meta,
	})
}

func (h *Handler) GetNewsFilterCounts(c *gin.Context) {
	counts, err := h.service.GetNewsFilterCounts()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch filter counts")
		return
	}

	response.Success(c, http.StatusOK, "Filter counts retrieved successfully", gin.H{
		"category_counts": counts.CategoryCounts,
		"total":           counts.Total,
	})
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

func (h *Handler) GetEducationNewsBySlug(c *gin.Context) {
	slug := c.Param("slug")
	news, err := h.service.GetEducationNewsBySlug(slug)
	if err != nil {
		response.Error(c, 404, "News not found")
		return
	}
	response.Success(c, 200, "News retrieved successfully", news)
}

func (h *Handler) GetEducationEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	category := c.Query("category")
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "newest")
	featuredStr := c.Query("featured")

	// If no filtering params, use old endpoint
	if category == "" && search == "" && featuredStr == "" {
		events, err := h.service.GetEducationEvents()
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to fetch events")
			return
		}
		response.Success(c, http.StatusOK, "Events retrieved successfully", gin.H{"events": events})
		return
	}

	events, meta, err := h.service.GetEducationEventsFiltered(page, limit, category, search, sort, featuredStr)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch events")
		return
	}

	response.Success(c, http.StatusOK, "Events retrieved successfully", gin.H{
		"events": events,
		"meta":   meta,
	})
}

func (h *Handler) GetEventFilterCounts(c *gin.Context) {
	counts, err := h.service.GetEventFilterCounts()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch filter counts")
		return
	}

	response.Success(c, http.StatusOK, "Filter counts retrieved successfully", gin.H{
		"category_counts": counts.CategoryCounts,
		"total":           counts.Total,
	})
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

func (h *Handler) GetEducationEventBySlug(c *gin.Context) {
	slug := c.Param("slug")
	event, err := h.service.GetEducationEventBySlug(slug)
	if err != nil {
		response.Error(c, 404, "Event not found")
		return
	}
	response.Success(c, 200, "Event retrieved successfully", event)
}

func (h *Handler) GetEducationBlogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	category := c.Query("category")
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "newest")
	tags := c.Query("tags")

	blogs, meta, err := h.service.GetEducationBlogs(page, limit, category, search, sort, tags)
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

func (h *Handler) GetBlogFilterCounts(c *gin.Context) {
	counts, err := h.service.GetBlogFilterCounts()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch filter counts")
		return
	}

	response.Success(c, http.StatusOK, "Filter counts retrieved successfully", counts)
}

func (h *Handler) IncrementBlogView(c *gin.Context) {
	id := c.Param("id")
	err := h.service.IncrementBlogView(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Blog not found")
		return
	}

	response.Success(c, http.StatusOK, "View incremented successfully", nil)
}

// ─── Public Entrance Handlers ─────────────────────────────────────────────

func (h *Handler) GetPublicEntrances(c *gin.Context) {
	var req EntranceFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = EntranceFilterRequest{}
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.PageSize
	if limit < 1 || limit > 50 {
		limit = 10
	}

	// Handle arrays - use first element or join
	level := ""
	if len(req.AcademicLevel) > 0 {
		level = req.AcademicLevel[0]
	}
	stream := ""
	if len(req.Stream) > 0 {
		stream = req.Stream[0]
	}
	status := ""
	if len(req.Status) > 0 {
		status = req.Status[0]
	}

	entrances, total, err := h.service.GetPublicEntrances(page, limit, req.Search, level, stream, status)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch entrances")
		return
	}

	response.Success(c, http.StatusOK, "Entrances retrieved successfully", gin.H{
		"entrances": entrances,
		"total":     total,
		"page":      page,
		"pageSize":  limit,
	})
}

func (h *Handler) GetEntranceFilterCounts(c *gin.Context) {
	counts, err := h.service.GetEntranceFilterCounts()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch filter counts")
		return
	}

	response.Success(c, http.StatusOK, "Filter counts retrieved successfully", counts)
}

func (h *Handler) GetPublicEntranceByID(c *gin.Context) {
	id := c.Param("id")
	entrance, err := h.service.GetPublicEntranceByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Entrance not found")
		return
	}

	response.Success(c, http.StatusOK, "Entrance retrieved successfully", entrance)
}

// ─── Admin CRUD Handlers ─────────────────────────────────────────────────────

func (h *Handler) AdminGetBlogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	category := c.Query("category")
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "newest")

	blogs, meta, err := h.service.GetAllBlogsAdmin(page, limit, category, search, sort)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch blogs")
		return
	}

	response.Success(c, http.StatusOK, "Blogs retrieved successfully", gin.H{
		"blogs": blogs,
		"meta":  meta,
	})
}

func (h *Handler) AdminGetBlogByID(c *gin.Context) {
	id := c.Param("id")
	blog, err := h.service.GetBlogByIDAdmin(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Blog not found")
		return
	}

	response.Success(c, http.StatusOK, "Blog retrieved successfully", blog)
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

func (h *Handler) AdminGetEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	var universityID *uint
	if uid, err := strconv.ParseUint(c.Query("university_id"), 10, 64); err == nil && uid > 0 {
		id := uint(uid)
		universityID = &id
	}

	events, meta, err := h.service.GetAllEventsAdmin(page, limit, universityID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch events")
		return
	}

	response.Success(c, http.StatusOK, "Events retrieved successfully", gin.H{
		"events": events,
		"meta":   meta,
	})
}

func (h *Handler) AdminGetEventByID(c *gin.Context) {
	id := c.Param("id")
	event, err := h.service.GetEducationEventByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	response.Success(c, http.StatusOK, "Event retrieved successfully", event)
}

func (h *Handler) CreateEvent(c *gin.Context) {
	var req EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.CreateEvent(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create event")
		return
	}

	response.Success(c, http.StatusCreated, "Event created successfully", event)
}

func (h *Handler) UpdateEvent(c *gin.Context) {
	id := c.Param("id")
	var req UpdateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.UpdateEvent(id, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update event")
		return
	}

	response.Success(c, http.StatusOK, "Event updated successfully", event)
}

func (h *Handler) DeleteEvent(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteEvent(id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete event")
		return
	}

	response.Success(c, http.StatusOK, "Event deleted successfully", nil)
}

func (h *Handler) ToggleEventFeatured(c *gin.Context) {
	id := c.Param("id")
	event, err := h.service.ToggleEventFeatured(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to toggle featured")
		return
	}

	response.Success(c, http.StatusOK, "Featured toggled successfully", event)
}

func (h *Handler) UploadBlogImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "No image file provided")
		return
	}

	urls, err := h.service.UploadBlogImage(file)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to upload image")
		return
	}

	response.Success(c, http.StatusOK, "Image uploaded successfully", gin.H{"url": urls[0]})
}

// ─── Admin News CRUD Handlers ─────────────────────────────────────────────────

func (h *Handler) AdminGetNews(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	category := c.Query("category")
	search := c.Query("search")
	var universityID *uint
	if uid, err := strconv.ParseUint(c.Query("university_id"), 10, 64); err == nil && uid > 0 {
		id := uint(uid)
		universityID = &id
	}

	news, meta, err := h.service.GetAllNewsAdmin(page, limit, category, search, universityID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch news")
		return
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", gin.H{
		"news": news,
		"meta": meta,
	})
}

func (h *Handler) AdminCreateNews(c *gin.Context) {
	var req CreateNewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	news, err := h.service.CreateNewsAdmin(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create news")
		return
	}

	response.Success(c, http.StatusCreated, "News created successfully", news)
}

func (h *Handler) AdminGetNewsByID(c *gin.Context) {
	id := c.Param("id")
	news, err := h.service.GetNewsByIDAdmin(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "News not found")
		return
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", news)
}

func (h *Handler) AdminUpdateNews(c *gin.Context) {
	id := c.Param("id")
	var req UpdateNewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	news, err := h.service.UpdateNewsAdmin(id, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update news")
		return
	}

	response.Success(c, http.StatusOK, "News updated successfully", news)
}

func (h *Handler) AdminDeleteNews(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteNewsAdmin(id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete news")
		return
	}

	response.Success(c, http.StatusOK, "News deleted successfully", nil)
}

func (h *Handler) AdminUploadNewsImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "No image file provided")
		return
	}

	url, err := h.service.UploadNewsImage(file)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to upload image")
		return
	}

	response.Success(c, http.StatusOK, "Image uploaded successfully", gin.H{"url": url})
}

// ─── Admin Course CRUD Handlers ──────────────────────────────────────────────

func (h *Handler) AdminCreateCourse(c *gin.Context) {
	var req CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	course, err := h.service.CreateCourse(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create course")
		return
	}

	response.Success(c, http.StatusCreated, "Course created successfully", course)
}

func (h *Handler) AdminListCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	courses, meta, err := h.service.GetAllCoursesAdmin(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	response.Success(c, http.StatusOK, "Courses retrieved successfully", gin.H{
		"courses": courses,
		"meta":    meta,
	})
}

func (h *Handler) AdminListPendingCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	courses, meta, err := h.service.GetPendingCourses(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch pending courses")
		return
	}

	response.Success(c, http.StatusOK, "Pending courses retrieved successfully", gin.H{
		"courses": courses,
		"meta":    meta,
	})
}

func (h *Handler) AdminGetCourse(c *gin.Context) {
	id := c.Param("id")
	course, err := h.service.GetCourseByIDAdmin(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	response.Success(c, http.StatusOK, "Course retrieved successfully", course)
}

func (h *Handler) AdminUpdateCourse(c *gin.Context) {
	id := c.Param("id")
	var req UpdateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	course, err := h.service.UpdateCourse(id, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update course")
		return
	}

	response.Success(c, http.StatusOK, "Course updated successfully", course)
}

func (h *Handler) AdminDeleteCourse(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteCourse(id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete course")
		return
	}

	response.Success(c, http.StatusOK, "Course deleted successfully", nil)
}

func (h *Handler) AdminPublishCourse(c *gin.Context) {
	id := c.Param("id")
	course, err := h.service.PublishCourse(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to publish course")
		return
	}

	response.Success(c, http.StatusOK, "Course published successfully", course)
}

func (h *Handler) GetBlogComments(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	comments, err := h.service.GetBlogComments(uint(id))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch comments")
		return
	}

	response.Success(c, http.StatusOK, "Comments retrieved successfully", comments)
}

func (h *Handler) CreateBlogComment(c *gin.Context) {
	var input BlogCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	blogID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid blog ID")
		return
	}
	input.BlogID = uint(blogID)

	comment, err := h.service.CreateBlogComment(input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, fmt.Sprintf("Failed to create comment: %v", err))
		return
	}

	response.Success(c, http.StatusCreated, "Comment posted successfully", comment)
}

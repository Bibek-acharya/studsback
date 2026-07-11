package institution

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

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

func getInstID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	return userID.(uint)
}

func (h *Handler) GetDashboard(c *gin.Context) {
	instID := getInstID(c)

	data, err := h.service.GetDashboard(instID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch dashboard data")
		return
	}

	response.Success(c, http.StatusOK, "Dashboard data retrieved successfully", data)
}

func (h *Handler) GetAnalytics(c *gin.Context) {
	instID := getInstID(c)

	data, err := h.service.GetAnalytics(instID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch analytics data")
		return
	}

	response.Success(c, http.StatusOK, "Analytics data retrieved successfully", data)
}

func (h *Handler) GetProfile(c *gin.Context) {
	instID := getInstID(c)

	data, err := h.service.GetProfile(instID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Institution not found")
		return
	}

	response.Success(c, http.StatusOK, "Profile retrieved successfully", data)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	instID := getInstID(c)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	data, err := h.service.UpdateProfile(instID, req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Institution not found")
		return
	}

	response.Success(c, http.StatusOK, "Profile updated successfully", data)
}

func (h *Handler) UpdatePassword(c *gin.Context) {
	instID := getInstID(c)

	var req UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdatePassword(instID, req); err != nil {
		if err.Error() == "institution not found" {
			response.Error(c, http.StatusNotFound, err.Error())
		} else if err.Error() == "current password is incorrect" {
			response.Error(c, http.StatusUnauthorized, err.Error())
		} else {
			response.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Password updated successfully", nil)
}

func (h *Handler) GetPrograms(c *gin.Context) {
	instID := getInstID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	programs, total, err := h.service.GetPrograms(instID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch programs")
		return
	}

	var resp []ProgramResponse
	for _, p := range programs {
		resp = append(resp, toProgramResponse(p))
	}

	response.Success(c, http.StatusOK, "Programs retrieved successfully", gin.H{
		"programs": resp,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetProgramByID(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid program ID")
		return
	}

	program, err := h.service.GetProgramByID(instID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Program not found")
		return
	}

	response.Success(c, http.StatusOK, "Program retrieved successfully", toProgramResponse(*program))
}

func (h *Handler) CreateProgram(c *gin.Context) {
	instID := getInstID(c)

	var req CreateProgramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	program, err := h.service.CreateProgram(instID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create program")
		return
	}

	response.Success(c, http.StatusCreated, "Program created successfully", toProgramResponse(*program))
}

func (h *Handler) UpdateProgram(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid program ID")
		return
	}

	var req UpdateProgramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	program, err := h.service.UpdateProgram(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Program updated successfully", toProgramResponse(*program))
}

func (h *Handler) DeleteProgram(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid program ID")
		return
	}

	if err := h.service.DeleteProgram(instID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Program not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to delete program")
		}
		return
	}

	response.Success(c, http.StatusOK, "Program deleted successfully", nil)
}

func (h *Handler) GetMedia(c *gin.Context) {
	instID := getInstID(c)

	media, err := h.service.GetMedia(instID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch media")
		return
	}

	var resp []MediaResponse
	for _, m := range media {
		resp = append(resp, toMediaResponse(m))
	}

	response.Success(c, http.StatusOK, "Media retrieved successfully", resp)
}

func (h *Handler) CreateMedia(c *gin.Context) {
	instID := getInstID(c)

	var req CreateMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	media, err := h.service.CreateMedia(instID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to upload media")
		return
	}

	response.Success(c, http.StatusCreated, "Media uploaded successfully", toMediaResponse(*media))
}

func (h *Handler) DeleteMedia(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid media ID")
		return
	}

	if err := h.service.DeleteMedia(instID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Media not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to delete media")
		}
		return
	}

	response.Success(c, http.StatusOK, "Media deleted successfully", nil)
}

func (h *Handler) GetCounsellingSessions(c *gin.Context) {
	instID := getInstID(c)

	sessions, err := h.service.GetCounsellingSessions(instID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch sessions")
		return
	}

	var resp []CounsellingSessionResponse
	for _, s := range sessions {
		resp = append(resp, toCounsellingSessionResponse(s))
	}

	response.Success(c, http.StatusOK, "Counselling sessions retrieved successfully", resp)
}

func (h *Handler) CreateCounsellingSession(c *gin.Context) {
	instID := getInstID(c)

	var req CreateCounsellingSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	session, err := h.service.CreateCounsellingSession(instID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create session")
		return
	}

	response.Success(c, http.StatusCreated, "Session created successfully", toCounsellingSessionResponse(*session))
}

func (h *Handler) DeleteCounsellingSession(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid session ID")
		return
	}

	if err := h.service.DeleteCounsellingSession(instID, uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Session deleted successfully", nil)
}

func (h *Handler) GetCounsellingBookings(c *gin.Context) {
	instID := getInstID(c)

	bookings, err := h.service.GetCounsellingBookings(instID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch bookings")
		return
	}

	var resp []CounsellingBookingResponse
	for _, b := range bookings {
		resp = append(resp, toCounsellingBookingResponse(b))
	}

	response.Success(c, http.StatusOK, "Counselling bookings retrieved successfully", resp)
}

func (h *Handler) UpdateBookingStatus(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid booking ID")
		return
	}

	var req UpdateBookingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	booking, err := h.service.UpdateBookingStatus(instID, uint(id), req.Status)
	if err != nil {
		if err.Error() == "booking not found" {
			response.Error(c, http.StatusNotFound, err.Error())
		} else {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Booking status updated successfully", booking)
}

func (h *Handler) GetEntrances(c *gin.Context) {
	instID := getInstID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	statusFilter := c.Query("status")
	entrances, total, err := h.service.GetEntrances(instID, statusFilter, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch entrances")
		return
	}

	var resp []EntranceResponse
	for _, e := range entrances {
		resp = append(resp, ToEntranceResponse(e))
	}

	response.Success(c, http.StatusOK, "Entrances retrieved successfully", gin.H{
		"entrances": resp,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetEntranceByID(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid entrance ID")
		return
	}

	entrance, err := h.service.GetEntranceByID(instID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Entrance not found")
		return
	}

	response.Success(c, http.StatusOK, "Entrance retrieved successfully", ToEntranceResponse(*entrance))
}

func (h *Handler) CreateEntrance(c *gin.Context) {
	instID := getInstID(c)

	var req CreateEntranceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	entrance, err := h.service.CreateEntrance(instID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Entrance created successfully", ToEntranceResponse(*entrance))
}

func (h *Handler) UpdateEntrance(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid entrance ID")
		return
	}

	var req UpdateEntranceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	entrance, err := h.service.UpdateEntrance(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Entrance updated successfully", ToEntranceResponse(*entrance))
}

func (h *Handler) DeleteEntrance(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid entrance ID")
		return
	}

	if err := h.service.DeleteEntrance(instID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Entrance not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to delete entrance")
		}
		return
	}

	response.Success(c, http.StatusOK, "Entrance deleted successfully", nil)
}

func (h *Handler) GetEntranceApplicants(c *gin.Context) {
	instID := getInstID(c)
	entranceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid entrance ID")
		return
	}

	applicants, err := h.service.GetEntranceApplicants(instID, uint(entranceID))
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	var resp []EntranceApplicantResponse
	for _, a := range applicants {
		resp = append(resp, toEntranceApplicantResponse(a))
	}

	response.Success(c, http.StatusOK, "Applicants retrieved successfully", resp)
}

func (h *Handler) GetEvents(c *gin.Context) {
	instID := getInstID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	events, total, err := h.service.GetEvents(instID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch events")
		return
	}

	var resp []EventResponse
	for _, e := range events {
		resp = append(resp, toEventResponse(e))
	}

	response.Success(c, http.StatusOK, "Events retrieved successfully", gin.H{
		"events": resp,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetEventByID(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	event, err := h.service.GetEventByID(instID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	response.Success(c, http.StatusOK, "Event retrieved successfully", toEventResponse(*event))
}

func (h *Handler) CreateEvent(c *gin.Context) {
	instID := getInstID(c)

	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.CreateEvent(instID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Event created successfully", toEventResponse(*event))
}

func (h *Handler) UpdateEvent(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	var req UpdateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.UpdateEvent(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Event updated successfully", toEventResponse(*event))
}

func (h *Handler) DeleteEvent(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	if err := h.service.DeleteEvent(instID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Event not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to delete event")
		}
		return
	}

	response.Success(c, http.StatusOK, "Event deleted successfully", nil)
}

func (h *Handler) GetNews(c *gin.Context) {
	instID := getInstID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	news, total, err := h.service.GetNews(instID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch news")
		return
	}

	var resp []NewsResponse
	for _, n := range news {
		resp = append(resp, toNewsResponse(n))
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", gin.H{
		"news": resp,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetNewsByID(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid news ID")
		return
	}

	news, err := h.service.GetNewsByID(instID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "News not found")
		return
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", toNewsResponse(*news))
}

func (h *Handler) ListPublicNews(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	news, total, err := h.service.ListPublicNews(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch news")
		return
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", gin.H{
		"news": news,
		"meta": gin.H{"total": total, "page": page, "limit": limit},
	})
}

func (h *Handler) ListPublicEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	events, total, err := h.service.ListPublicEvents(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch events")
		return
	}

	response.Success(c, http.StatusOK, "Events retrieved successfully", gin.H{
		"events": events,
		"meta":   gin.H{"total": total, "page": page, "limit": limit},
	})
}

func (h *Handler) GetPublicEventByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	event, err := h.service.GetPublicEventByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	response.Success(c, http.StatusOK, "Event retrieved successfully", toEventResponse(*event))
}

func (h *Handler) GetPublicEventBySlug(c *gin.Context) {
	slug := c.Param("slug")
	event, err := h.service.GetPublicEventBySlug(slug)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}
	response.Success(c, http.StatusOK, "Event retrieved successfully", toEventResponse(*event))
}

func (h *Handler) ListPublicScholarships(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	scholarships, total, err := h.service.ListPublicScholarships(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch scholarships")
		return
	}

	response.Success(c, http.StatusOK, "Scholarships retrieved successfully", gin.H{
		"scholarships": scholarships,
		"meta":         gin.H{"total": total, "page": page, "limit": limit},
	})
}

func (h *Handler) GetPublicScholarshipByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid scholarship ID")
		return
	}

	scholarship, err := h.service.GetPublicScholarshipByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Scholarship not found")
		return
	}

	response.Success(c, http.StatusOK, "Scholarship retrieved successfully", toScholarshipResponse(*scholarship))
}

func (h *Handler) GetPublicNewsByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid news ID")
		return
	}

	news, err := h.service.GetPublicNewsByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "News not found")
		return
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", toNewsResponse(*news))
}

func (h *Handler) GetPublicNewsBySlug(c *gin.Context) {
	slug := c.Param("slug")
	news, err := h.service.GetPublicNewsBySlug(slug)
	if err != nil {
		response.Error(c, http.StatusNotFound, "News not found")
		return
	}
	response.Success(c, http.StatusOK, "News retrieved successfully", toNewsResponse(*news))
}

func (h *Handler) CreateNews(c *gin.Context) {
	instID := getInstID(c)

	var req CreateNewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	news, err := h.service.CreateNews(instID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create news")
		return
	}

	response.Success(c, http.StatusCreated, "News created successfully", toNewsResponse(*news))
}

func (h *Handler) UpdateNews(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid news ID")
		return
	}

	var req UpdateNewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	news, err := h.service.UpdateNews(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "News updated successfully", toNewsResponse(*news))
}

func (h *Handler) DeleteNews(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid news ID")
		return
	}

	if err := h.service.DeleteNews(instID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "News not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to delete news")
		}
		return
	}

	response.Success(c, http.StatusOK, "News deleted successfully", nil)
}

func (h *Handler) GetBlogs(c *gin.Context) {
	instID := getInstID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	blogs, total, err := h.service.GetBlogs(instID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch blogs")
		return
	}

	var resp []BlogResponse
	for _, b := range blogs {
		resp = append(resp, toBlogResponse(b))
	}

	response.Success(c, http.StatusOK, "Blogs retrieved successfully", gin.H{
		"blogs": resp,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetBlogByID(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	blog, err := h.service.GetBlogByID(instID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Blog not found")
		return
	}

	response.Success(c, http.StatusOK, "Blog retrieved successfully", toBlogResponse(*blog))
}

func (h *Handler) CreateBlog(c *gin.Context) {
	instID := getInstID(c)

	var req CreateBlogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	blog, err := h.service.CreateBlog(instID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create blog")
		return
	}

	response.Success(c, http.StatusCreated, "Blog created successfully", toBlogResponse(*blog))
}

func (h *Handler) UpdateBlog(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	var req UpdateBlogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	blog, err := h.service.UpdateBlog(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Blog updated successfully", toBlogResponse(*blog))
}

func (h *Handler) DeleteBlog(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	if err := h.service.DeleteBlog(instID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Blog not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to delete blog")
		}
		return
	}

	response.Success(c, http.StatusOK, "Blog deleted successfully", nil)
}

func (h *Handler) ListPublicBlogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	blogs, total, err := h.service.ListPublicBlogs(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch blogs")
		return
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	var blogResponses []BlogResponse
	for _, b := range blogs {
		blogResponses = append(blogResponses, toBlogResponse(b))
	}

	response.Success(c, http.StatusOK, "Blogs retrieved successfully", gin.H{
		"blogs": blogResponses,
		"meta": gin.H{
			"total":      total,
			"page":       page,
			"limit":      limit,
			"totalPages": totalPages,
		},
	})
}

func (h *Handler) GetPublicBlogBySlug(c *gin.Context) {
	slug := c.Param("slug")
	blog, err := h.service.GetPublicBlogBySlug(slug)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Blog not found")
		return
	}
	response.Success(c, http.StatusOK, "Blog retrieved successfully", toBlogResponse(*blog))
}

func (h *Handler) GetQMS(c *gin.Context) {
	instID := getInstID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	qms, total, err := h.service.GetQMS(instID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch QMS records")
		return
	}

	var resp []QMSResponse
	for _, q := range qms {
		resp = append(resp, toQMSResponse(q))
	}

	response.Success(c, http.StatusOK, "QMS records retrieved successfully", gin.H{
		"qms": resp,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetQMSByID(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid QMS ID")
		return
	}

	qms, err := h.service.GetQMSByID(instID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "QMS record not found")
		return
	}

	response.Success(c, http.StatusOK, "QMS record retrieved successfully", toQMSResponse(*qms))
}

func (h *Handler) CreateQMS(c *gin.Context) {
	instID := getInstID(c)

	var req CreateQMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	qms, err := h.service.CreateQMS(instID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create QMS record")
		return
	}

	response.Success(c, http.StatusCreated, "QMS record created successfully", toQMSResponse(*qms))
}

func (h *Handler) UpdateQMS(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid QMS ID")
		return
	}

	var req UpdateQMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	qms, err := h.service.UpdateQMS(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "QMS record updated successfully", toQMSResponse(*qms))
}

func (h *Handler) DeleteQMS(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid QMS ID")
		return
	}

	if err := h.service.DeleteQMS(instID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "QMS record not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to delete QMS record")
		}
		return
	}

	response.Success(c, http.StatusOK, "QMS record deleted successfully", nil)
}

func (h *Handler) GetMessages(c *gin.Context) {
	instID := getInstID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	messages, total, err := h.service.GetMessages(instID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch messages")
		return
	}

	var resp []MessageResponse
	for _, m := range messages {
		resp = append(resp, toMessageResponse(m))
	}

	response.Success(c, http.StatusOK, "Messages retrieved successfully", gin.H{
		"messages": resp,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetMessageByID(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid message ID")
		return
	}

	message, err := h.service.GetMessageByID(instID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Message not found")
		return
	}

	response.Success(c, http.StatusOK, "Message retrieved successfully", toMessageResponse(*message))
}

func (h *Handler) CreateMessage(c *gin.Context) {
	instID := getInstID(c)

	var req CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	message, err := h.service.CreateMessage(instID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to send message")
		return
	}

	response.Success(c, http.StatusCreated, "Message sent successfully", toMessageResponse(*message))
}

func (h *Handler) CreateInquiry(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid institution ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req CreateInquiryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	message, err := h.service.CreateInquiry(uint(id), userID.(uint), req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to send inquiry")
		return
	}

	response.Success(c, http.StatusCreated, "Inquiry sent successfully", toMessageResponse(*message))
}

func (h *Handler) GetMessageStudents(c *gin.Context) {
	instID := getInstID(c)

	contacts, err := h.service.GetMessageStudents(instID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch student contacts")
		return
	}

	response.Success(c, http.StatusOK, "Student contacts retrieved successfully", contacts)
}

func (h *Handler) UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	folder := c.DefaultQuery("folder", "institution")

	url, err := utils.SaveUploadedImage(file, folder)
	if err != nil {
		url, err = utils.SaveUploadedDocument(file, folder)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "Failed to upload file: "+err.Error())
			return
		}
	}

	response.Success(c, http.StatusOK, "File uploaded successfully", gin.H{
		"url": url,
	})
}

func (h *Handler) GetSettings(c *gin.Context) {
	instID := getInstID(c)

	settings, err := h.service.GetSettings(instID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch settings")
		return
	}

	response.Success(c, http.StatusOK, "Settings retrieved successfully", settings)
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	instID := getInstID(c)

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	settings, err := h.service.UpdateSettings(instID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	response.Success(c, http.StatusOK, "Settings updated successfully", settings)
}

func (h *Handler) ListPublicInstitutions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "18"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 18
	}

	search := c.Query("search")
	location := c.Query("location")
	instType := c.Query("type")

	results, total, err := h.service.ListPublicInstitutions(page, limit, search, location, instType)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch institutions")
		return
	}

	response.Success(c, http.StatusOK, "Institutions retrieved successfully", gin.H{
		"institutions": results,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetPublicInstitution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid institution ID")
		return
	}

	result, err := h.service.GetPublicInstitution(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Institution not found")
		return
	}

	response.Success(c, http.StatusOK, "Institution retrieved successfully", result)
}

func (h *Handler) GetPublicCounsellingSessions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid institution ID")
		return
	}

	sessions, err := h.service.GetPublicCounsellingSessions(uint(id))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch counselling sessions")
		return
	}

	response.Success(c, http.StatusOK, "Counselling sessions retrieved successfully", sessions)
}

func (h *Handler) CreatePublicCounsellingBooking(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req PublicCounsellingBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	booking, err := h.service.CreatePublicBooking(userID.(uint), req)
	if err != nil {
		status := http.StatusInternalServerError
		switch err.Error() {
		case "session_mode must be 'online' or 'in_person'":
			status = http.StatusBadRequest
		case "session not found":
			status = http.StatusNotFound
		case "session is not available for booking":
			status = http.StatusConflict
		case "no available seats in this session":
			status = http.StatusConflict
		case "you have already booked this session":
			status = http.StatusConflict
		}
		response.Error(c, status, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Counselling session booked successfully", toCounsellingBookingResponse(*booking))
}

func (h *Handler) GetScholarships(c *gin.Context) {
	instID := getInstID(c)

	scholarships, err := h.service.GetScholarships(instID)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	var resp []ScholarshipResponse
	for _, s := range scholarships {
		resp = append(resp, toScholarshipResponse(s))
	}

	response.Success(c, http.StatusOK, "Scholarships retrieved successfully", resp)
}

func (h *Handler) CreateScholarship(c *gin.Context) {
	instID := getInstID(c)

	var req CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	scholarship, err := h.service.CreateScholarship(instID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Scholarship created successfully", toScholarshipResponse(*scholarship))
}

func (h *Handler) UpdateScholarship(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid scholarship ID")
		return
	}

	var req UpdateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	scholarship, err := h.service.UpdateScholarship(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Scholarship updated successfully", toScholarshipResponse(*scholarship))
}

func (h *Handler) DeleteScholarship(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid scholarship ID")
		return
	}

	if err := h.service.DeleteScholarship(instID, uint(id)); err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Scholarship deleted successfully", nil)
}

func (h *Handler) GetAdmissions(c *gin.Context) {
	instID := getInstID(c)
	status := c.Query("status")

	admissions, err := h.service.GetAdmissions(instID, status)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	var resp []AdmissionResponse
	for _, a := range admissions {
		resp = append(resp, toAdmissionResponse(a))
	}

	response.Success(c, http.StatusOK, "Admissions retrieved successfully", resp)
}

func (h *Handler) UpdateAdmissionStatus(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid admission ID")
		return
	}

	var req UpdateAdmissionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	admission, err := h.service.UpdateAdmissionStatus(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Admission status updated successfully", toAdmissionResponse(*admission))
}

func (h *Handler) GetScholarshipApplications(c *gin.Context) {
	instID := getInstID(c)
	status := c.Query("status")

	applications, err := h.service.GetScholarshipApplications(instID, status)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	var resp []ScholarshipApplicationResponse
	for _, a := range applications {
		resp = append(resp, toScholarshipApplicationResponse(a))
	}

	response.Success(c, http.StatusOK, "Scholarship applications retrieved successfully", resp)
}

func (h *Handler) UpdateScholarshipApplicationStatus(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application ID")
		return
	}

	var req UpdateScholarshipApplicationStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	application, err := h.service.UpdateScholarshipApplicationStatus(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Application status updated successfully", toScholarshipApplicationResponse(*application))
}

// --- Admission Page Handlers ---

func (h *Handler) CreateAdmissionPage(c *gin.Context) {
	instID := getInstID(c)

	var req CreateAdmissionPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.service.CreateAdmissionPage(instID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create admission")
		return
	}

	response.Success(c, http.StatusCreated, "Admission created successfully", page)
}

func (h *Handler) GetAdmissionPages(c *gin.Context) {
	instID := getInstID(c)
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	pages, total, err := h.service.GetAdmissionPages(instID, status, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch admissions")
		return
	}

	response.Success(c, http.StatusOK, "Admissions retrieved successfully", gin.H{
		"admissions": pages,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetAdmissionPage(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid admission ID")
		return
	}

	page, err := h.service.GetAdmissionPage(instID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Admission not found")
		return
	}

	response.Success(c, http.StatusOK, "Admission retrieved successfully", page)
}

func (h *Handler) UpdateAdmissionPage(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid admission ID")
		return
	}

	var req UpdateAdmissionPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.service.UpdateAdmissionPage(instID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Admission updated successfully", page)
}

func (h *Handler) DeleteAdmissionPage(c *gin.Context) {
	instID := getInstID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid admission ID")
		return
	}

	if err := h.service.DeleteAdmissionPage(instID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Admission not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to delete admission")
		}
		return
	}

	response.Success(c, http.StatusOK, "Admission deleted successfully", nil)
}

func (h *Handler) GetPublishedAdmissionPages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	pages, total, err := h.service.GetPublishedAdmissionPages(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch published admissions")
		return
	}

	response.Success(c, http.StatusOK, "Published admissions retrieved successfully", gin.H{
		"admissions": pages,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetPublishedAdmissionInstitutions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "18"))
	level := c.Query("level")
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 18
	}

	result, err := h.service.GetPublishedAdmissionInstitutions(page, limit, level)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch published admission institutions")
		return
	}

	response.Success(c, http.StatusOK, "Published admission institutions retrieved successfully", result)
}

func (h *Handler) GetPublishedAdmissionInstitutionByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid institution ID")
		return
	}

	result, err := h.service.GetPublishedAdmissionInstitutionByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Published admission institution not found")
		return
	}

	response.Success(c, http.StatusOK, "Published admission institution retrieved successfully", result)
}

func toProgramResponse(p InstitutionProgram) ProgramResponse {
	var data interface{}
	if p.Data != nil {
		json.Unmarshal([]byte(*p.Data), &data)
	}
	return ProgramResponse{
		ID:            p.ID,
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     p.UpdatedAt.Format(time.RFC3339),
		InstitutionID: p.InstitutionID,
		Name:          p.Name,
		Description:   p.Description,
		Duration:      p.Duration,
		Fee:           p.Fee,
		Eligibility:   p.Eligibility,
		Capacity:      p.Capacity,
		BannerURL:     p.BannerURL,
		Data:          data,
		Status:        p.Status,
	}
}

func toMediaResponse(m InstitutionMedia) MediaResponse {
	return MediaResponse{
		ID:            m.ID,
		CreatedAt:     m.CreatedAt.Format(time.RFC3339),
		InstitutionID: m.InstitutionID,
		URL:           m.URL,
		Type:          m.Type,
		Title:         m.Title,
	}
}

func toCounsellingSessionResponse(s InstitutionCounsellingSession) CounsellingSessionResponse {
	return CounsellingSessionResponse{
		ID:            s.ID,
		CreatedAt:     s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     s.UpdatedAt.Format(time.RFC3339),
		InstitutionID: s.InstitutionID,
		Title:         s.Title,
		Description:   s.Description,
		ScheduledAt:   s.ScheduledAt.Format(time.RFC3339),
		Duration:      s.Duration,
		MaxSeats:      s.MaxSeats,
		BookedSeats:   s.BookedSeats,
		Status:        s.Status,
	}
}

func toCounsellingBookingResponse(b InstitutionCounsellingBooking) CounsellingBookingResponse {
	studentName := b.StudentName
	if studentName == "" {
		studentName = fmt.Sprintf("User #%d", b.UserID)
	}

	resp := CounsellingBookingResponse{
		ID:               b.ID,
		CreatedAt:        b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        b.UpdatedAt.Format(time.RFC3339),
		SessionID:        b.SessionID,
		UserID:           b.UserID,
		Status:           b.Status,
		Notes:            b.Notes,
		StudentName:      studentName,
		StudentPhone:     b.StudentPhone,
		StudentEmail:     b.StudentEmail,
		ProgramLevel:     b.ProgramLevel,
		InterestedCourse: b.InterestedCourse,
		SessionMode:      b.SessionMode,
	}

	if b.Session.ID != 0 {
		s := toCounsellingSessionResponse(b.Session)
		resp.Session = &s
	}

	return resp
}

func ToEntranceResponse(e InstitutionEntrance) EntranceResponse {
	var questions interface{}
	if e.Questions != nil {
		json.Unmarshal([]byte(*e.Questions), &questions)
	}
	return EntranceResponse{
		ID:                     e.ID,
		CreatedAt:              e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              e.UpdatedAt.Format(time.RFC3339),
		InstitutionID:          e.InstitutionID,
		InstitutionName:        e.InstitutionName,
		InstitutionLocation:    e.InstitutionLocation,
		InstitutionLink:        e.InstitutionLink,
		InstitutionAffiliation: e.InstitutionAffiliation,
		Title:                  e.Title,
		Description:            e.Description,
		Program:                e.Program,
		Date:                   e.Date.Format("2006-01-02"),
		StartTime:              e.StartTime,
		EndTime:                e.EndTime,
		Duration:               e.Duration,
		TotalMarks:             e.TotalMarks,
		PassingMarks:           e.PassingMarks,
		TotalSeats:             e.TotalSeats,
		FilledSeats:            e.FilledSeats,
		Instructions:           e.Instructions,
		HeroBanner:             e.HeroBanner,
		Questions:              questions,
		Status:                 e.Status,
		ApplicationFee:         e.ApplicationFee,
		OverviewDetails:        json.RawMessage(e.OverviewDetails),
		ExamDateSchedules:      json.RawMessage(e.ExamDateSchedules),
		EligibilityList:        json.RawMessage(e.EligibilityList),
		ApplicationSteps:       json.RawMessage(e.ApplicationSteps),
		ExamPattern:            json.RawMessage(e.ExamPattern),
		SubjectMarks:           json.RawMessage(e.SubjectMarks),
		ModelSets:              json.RawMessage(e.ModelSets),
		UpcomingDates:          json.RawMessage(e.UpcomingDates),
		ContactPersons:         json.RawMessage(e.ContactPersons),
		Faqs:                   json.RawMessage(e.Faqs),
		Email:                  e.Email,
		ContactNumber:          e.ContactNumber,
		SocialLinks:            json.RawMessage(e.SocialLinks),
		ApplicationLink:        e.ApplicationLink,
		NoticeFile:             e.NoticeFile,
		EmbeddedMap:            e.EmbeddedMap,
		RequiredDocuments:      json.RawMessage(e.RequiredDocuments),
		ExaminationSchedule:    json.RawMessage(e.ExaminationSchedule),
		ProgramsOffered:        json.RawMessage(e.ProgramsOffered),
	}
}

func toEntranceApplicantResponse(a InstitutionEntranceApplicant) EntranceApplicantResponse {
	return EntranceApplicantResponse{
		ID:         a.ID,
		CreatedAt:  a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  a.UpdatedAt.Format(time.RFC3339),
		EntranceID: a.EntranceID,
		UserID:     a.UserID,
		Status:     a.Status,
		Score:      a.Score,
		Rank:       a.Rank,
	}
}

func toEventResponse(e InstitutionEvent) EventResponse {
	var tags []string
	if e.Tags != nil && *e.Tags != "" {
		json.Unmarshal([]byte(*e.Tags), &tags)
	}
	var startDate *string
	if e.StartDate != nil {
		t := e.StartDate.Format(time.RFC3339)
		startDate = &t
	}
	var endDate *string
	if e.EndDate != nil {
		t := e.EndDate.Format(time.RFC3339)
		endDate = &t
	}
	return EventResponse{
		ID:                 e.ID,
		CreatedAt:          e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          e.UpdatedAt.Format(time.RFC3339),
		InstitutionID:      e.InstitutionID,
		Name:               e.Name,
		ShortDesc:          e.ShortDesc,
		Description:        e.Description,
		ImageURL:           e.ImageURL,
		EventType:          e.EventType,
		Category:           e.Category,
		MaxParticipants:    e.MaxParticipants,
		OnlineLink:         e.OnlineLink,
		OrganizedBy:        e.OrganizedBy,
		ContactPerson:      e.ContactPerson,
		ContactEmail:       e.ContactEmail,
		StartDate:          startDate,
		EndDate:            endDate,
		Location:           e.Location,
		Tags:               tags,
		EnableRegistration: e.EnableRegistration,
		Status:             e.Status,
	}
}

func toNewsResponse(n InstitutionNews) NewsResponse {
	var tags []string
	if n.Tags != nil && *n.Tags != "" {
		json.Unmarshal([]byte(*n.Tags), &tags)
	}
	var publishedAt *string
	if n.PublishedAt != nil {
		t := n.PublishedAt.Format(time.RFC3339)
		publishedAt = &t
	}
	var publishDate *string
	if n.PublishDate != nil {
		pd := *n.PublishDate
		publishDate = &pd
	}
	return NewsResponse{
		ID:            n.ID,
		CreatedAt:     n.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     n.UpdatedAt.Format(time.RFC3339),
		InstitutionID: n.InstitutionID,
		Title:         n.Title,
		ShortDesc:     n.ShortDesc,
		Content:       n.Content,
		ImageURL:      n.ImageURL,
		NewsType:      n.NewsType,
		PublishedBy:   n.PublishedBy,
		PublishDate:   publishDate,
		Tags:          tags,
		AllowComments: n.AllowComments,
		Status:        n.Status,
		PublishedAt:   publishedAt,
	}
}

func toBlogResponse(b InstitutionBlog) BlogResponse {
	var publishedAt *string
	if b.PublishedAt != nil {
		s := b.PublishedAt.Format(time.RFC3339)
		publishedAt = &s
	}
	return BlogResponse{
		ID:            b.ID,
		Slug:          b.Slug,
		CreatedAt:     b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     b.UpdatedAt.Format(time.RFC3339),
		InstitutionID: b.InstitutionID,
		Title:         b.Title,
		Content:       b.Content,
		Excerpt:       b.Excerpt,
		Image:         b.Image,
		Category:      b.Category,
		BlogCategory:  b.BlogCategory,
		ReadTime:      b.ReadTime,
		Tags:          b.Tags,
		Status:        b.Status,
		PublishedAt:   publishedAt,
	}
}

func toQMSResponse(q InstitutionQMS) QMSResponse {
	return QMSResponse{
		ID:            q.ID,
		CreatedAt:     q.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     q.UpdatedAt.Format(time.RFC3339),
		InstitutionID: q.InstitutionID,
		Title:         q.Title,
		Description:   q.Description,
		Category:      q.Category,
		Status:        q.Status,
		Score:         q.Score,
		Documents:     q.Documents,
	}
}

func toMessageResponse(m InstitutionMessage) MessageResponse {
	return MessageResponse{
		ID:            m.ID,
		CreatedAt:     m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     m.UpdatedAt.Format(time.RFC3339),
		InstitutionID: m.InstitutionID,
		UserID:        m.UserID,
		Subject:       m.Subject,
		Content:       m.Content,
		Read:          m.Read,
		Direction:     m.Direction,
	}
}

func toScholarshipResponse(s Scholarship) ScholarshipResponse {
	var fieldOfStudy []string
	if len(s.FieldOfStudy) > 0 {
		json.Unmarshal(s.FieldOfStudy, &fieldOfStudy)
	}

	return ScholarshipResponse{
		ID:              s.ID,
		CreatedAt:       s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       s.UpdatedAt.Format(time.RFC3339),
		InstitutionID:   s.InstitutionID,
		Title:           s.Title,
		ShortDesc:       s.ShortDesc,
		Provider:        s.Provider,
		Location:        s.Location,
		Value:           s.Value,
		Deadline:        s.Deadline.Format("2006-01-02"),
		DegreeLevel:     s.DegreeLevel,
		FundingType:     s.FundingType,
		ScholarshipType: s.ScholarshipType,
		Description:     s.Description,
		ImageURL:        s.ImageURL,
		FieldOfStudy:    fieldOfStudy,
		Status:          s.Status,
	}
}

func toAdmissionResponse(a Admission) AdmissionResponse {
	resp := AdmissionResponse{
		ID:                a.ID,
		CreatedAt:         a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         a.UpdatedAt.Format(time.RFC3339),
		UserID:            a.UserID,
		CollegeID:         a.CollegeID,
		ProgramName:       a.ProgramName,
		ProgramLevel:      a.ProgramLevel,
		StudentName:       a.StudentName,
		StudentEmail:      a.StudentEmail,
		StudentPhone:      a.StudentPhone,
		Gender:            a.Gender,
		Address:           a.Address,
		City:              a.City,
		LastQualification: a.LastQualification,
		Institution:       a.Institution,
		GPA:               a.GPA,
		EntranceScore:     a.EntranceScore,
		Statement:         a.Statement,
		Status:            a.Status,
		Notes:             a.Notes,
		ReviewedBy:        a.ReviewedBy,
	}

	if a.DateOfBirth != nil {
		dob := a.DateOfBirth.Format("2006-01-02")
		resp.DateOfBirth = &dob
	}

	if a.ReviewedAt != nil {
		reviewedAt := a.ReviewedAt.Format(time.RFC3339)
		resp.ReviewedAt = &reviewedAt
	}

	if a.College.ID != 0 {
		resp.College = &CollegeDTO{
			ID:   a.College.ID,
			Name: a.College.Name,
		}
	}

	if a.User != nil && a.User.ID != 0 {
		resp.User = &UserDTO{
			ID:    a.User.ID,
			Email: a.User.Email,
		}
	}

	return resp
}

func toScholarshipApplicationResponse(a ScholarshipApplication) ScholarshipApplicationResponse {
	var scholarshipResp *ScholarshipResponse
	if a.Scholarship.ID != 0 {
		s := toScholarshipResponse(a.Scholarship)
		scholarshipResp = &s
	}

	var userResp *UserDTO
	if a.User.ID != 0 {
		userResp = &UserDTO{
			ID:    a.User.ID,
			Email: a.User.Email,
		}
	}

	return ScholarshipApplicationResponse{
		ID:            a.ID,
		CreatedAt:     a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     a.UpdatedAt.Format(time.RFC3339),
		ScholarshipID: a.ScholarshipID,
		UserID:        a.UserID,
		Status:        a.Status,
		CoverLetter:   a.CoverLetter,
		Documents:     a.Documents,
		Scholarship:   scholarshipResp,
		User:          userResp,
	}
}

func (h *Handler) GetPublicInstitutionFilterCounts(c *gin.Context) {
	result, err := h.service.GetPublicInstitutionFilterCounts()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch institution filter counts")
		return
	}

	response.Success(c, http.StatusOK, "Institution filter counts retrieved successfully", result)
}

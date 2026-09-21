package system

import (
	"errors"
	"net/http"
	"strconv"

	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) SubmitContactInquiry(c *gin.Context) {
	var req ContactInquiryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	inquiry, err := h.service.SubmitContactInquiry(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Inquiry submitted successfully", toContactInquiryResponse(inquiry))
}

func (h *Handler) GetContactInquiries(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	inquiryType := c.Query("type")

	inquiries, total, err := h.service.GetContactInquiries(page, limit, status, inquiryType)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve inquiries")
		return
	}

	responses := make([]ContactInquiryResponse, len(inquiries))
	for i, inq := range inquiries {
		responses[i] = toContactInquiryResponse(&inq)
	}

	response.Success(c, http.StatusOK, "Inquiries retrieved successfully", gin.H{
		"inquiries": responses,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetContactInquiryByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	inquiry, err := h.service.GetContactInquiryByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Inquiry not found")
		return
	}

	response.Success(c, http.StatusOK, "Inquiry retrieved successfully", toContactInquiryResponse(inquiry))
}

func (h *Handler) UpdateContactInquiryStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req ContactInquiryStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	inquiry, err := h.service.UpdateContactInquiryStatus(uint(id), req.Status)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Inquiry status updated successfully", toContactInquiryResponse(inquiry))
}

func (h *Handler) DeleteContactInquiry(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	if err := h.service.DeleteContactInquiry(uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Inquiry not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete inquiry")
		return
	}

	response.Success(c, http.StatusOK, "Inquiry deleted successfully", nil)
}

func (h *Handler) GetAds(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	pageFilter := c.Query("page")
	positionFilter := c.Query("position")
	activeStr := c.Query("active")

	var active *bool
	if activeStr != "" {
		val := activeStr == "true"
		active = &val
	}

	ads, total, err := h.service.GetAds(page, limit, pageFilter, positionFilter, active)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve ads")
		return
	}

	responses := make([]AdResponse, len(ads))
	for i, ad := range ads {
		responses[i] = toAdResponse(&ad)
	}

	response.Success(c, http.StatusOK, "Ads retrieved successfully", gin.H{
		"ads": responses,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetActiveAds(c *gin.Context) {
	page := c.Query("page")
	position := c.Query("position")

	ads, err := h.service.GetActiveAds(page, position)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve active ads")
		return
	}

	responses := make([]AdResponse, len(ads))
	for i, ad := range ads {
		responses[i] = toAdResponse(&ad)
	}

	response.Success(c, http.StatusOK, "Active ads retrieved successfully", responses)
}

func (h *Handler) GetAdByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	ad, err := h.service.GetAdByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Ad not found")
		return
	}

	response.Success(c, http.StatusOK, "Ad retrieved successfully", toAdResponse(ad))
}

func (h *Handler) CreateAd(c *gin.Context) {
	var req AdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	ad, err := h.service.CreateAd(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Ad created successfully", toAdResponse(ad))
}

func (h *Handler) UpdateAd(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req AdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	ad, err := h.service.UpdateAd(uint(id), req)
	if err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Ad not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update ad")
		return
	}

	response.Success(c, http.StatusOK, "Ad updated successfully", toAdResponse(ad))
}

func (h *Handler) DeleteAd(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	if err := h.service.DeleteAd(uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Ad not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete ad")
		return
	}

	response.Success(c, http.StatusOK, "Ad deleted successfully", nil)
}

func (h *Handler) TrackAdClick(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	ad, err := h.service.TrackAdClick(uint(id))
	if err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Ad not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to track ad click")
		return
	}

	response.Success(c, http.StatusOK, "Ad click tracked", gin.H{"link_url": ad.LinkURL})
}

func (h *Handler) GetCarousels(c *gin.Context) {
	page := c.DefaultQuery("page", "landing")
	activeStr := c.Query("active")

	var active *bool
	if activeStr != "" {
		val := activeStr == "true"
		active = &val
	}

	slides, err := h.service.GetCarousels(page, active)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve carousel slides")
		return
	}

	responses := make([]CarouselSlideResponse, len(slides))
	for i, slide := range slides {
		responses[i] = toCarouselSlideResponse(&slide)
	}

	response.Success(c, http.StatusOK, "Carousel slides retrieved successfully", responses)
}

func (h *Handler) GetCarouselSlideByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	slide, err := h.service.GetCarouselSlideByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Slide not found")
		return
	}

	response.Success(c, http.StatusOK, "Slide retrieved successfully", toCarouselSlideResponse(slide))
}

func (h *Handler) CreateCarouselSlide(c *gin.Context) {
	var req CarouselSlideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	slide, err := h.service.CreateCarouselSlide(req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Slide created successfully", toCarouselSlideResponse(slide))
}

func (h *Handler) UpdateCarouselSlide(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req CarouselSlideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	slide, err := h.service.UpdateCarouselSlide(uint(id), req)
	if err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Slide not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update slide")
		return
	}

	response.Success(c, http.StatusOK, "Slide updated successfully", toCarouselSlideResponse(slide))
}

func (h *Handler) DeleteCarouselSlide(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	if err := h.service.DeleteCarouselSlide(uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Slide not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete slide")
		return
	}

	response.Success(c, http.StatusOK, "Slide deleted successfully", nil)
}

func (h *Handler) ReorderCarouselSlides(c *gin.Context) {
	var req CarouselReorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	items := make([]struct {
		ID    uint
		Order int
	}, len(req.Slides))
	for i, item := range req.Slides {
		items[i] = struct {
			ID    uint
			Order int
		}{ID: item.ID, Order: item.Order}
	}

	if err := h.service.ReorderCarouselSlides(items); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to reorder slides")
		return
	}

	response.Success(c, http.StatusOK, "Slides reordered successfully", nil)
}

func (h *Handler) GetPublicNotifications(c *gin.Context) {
	notifications, err := h.service.GetActivePublicNotifications()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve notifications")
		return
	}

	responses := make([]PublicNotificationResponse, len(notifications))
	for i, n := range notifications {
		responses[i] = notification.ToPublicNotificationResponse(&n)
	}

	response.Success(c, http.StatusOK, "Notifications retrieved successfully", responses)
}

func toContactInquiryResponse(inquiry *ContactInquiry) ContactInquiryResponse {
	return ContactInquiryResponse{
		ID:            inquiry.ID,
		InstitutionID: inquiry.InstitutionID,
		Name:          inquiry.Name,
		Email:         inquiry.Email,
		Phone:         inquiry.Phone,
		Subject:       inquiry.Subject,
		Message:       inquiry.Message,
		Type:          inquiry.Type,
		Status:        inquiry.Status,
		CreatedAt:     inquiry.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     inquiry.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func toAdResponse(ad *Ad) AdResponse {
	var startDate, endDate string
	if !ad.StartDate.IsZero() {
		startDate = ad.StartDate.Format("2006-01-02T15:04:05Z")
	}
	if !ad.EndDate.IsZero() {
		endDate = ad.EndDate.Format("2006-01-02T15:04:05Z")
	}

	resp := AdResponse{
		ID:          ad.ID,
		Title:       ad.Title,
		ImageURL:    ad.ImageURL,
		LinkURL:     ad.LinkURL,
		Location:    ad.Location,
		Page:        ad.Page,
		Position:    ad.Position,
		StartDate:   startDate,
		EndDate:     endDate,
		Active:      ad.Active,
		Clicks:      ad.Clicks,
		Impressions: ad.Impressions,
		Priority:    ad.Priority,
		CollegeID:   ad.CollegeID,
		CourseID:    ad.CourseID,
		Description: ad.Description,
		Accent:      ad.Accent,
		CreatedAt:   ad.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   ad.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if ad.CollegeID != nil && ad.College != nil {
		resp.CollegeName = ad.College.Name
		resp.CollegeImage = ad.College.ImageURL
		resp.CollegeRating = ad.College.Rating
		resp.CollegeLocation = ad.College.Location
		resp.CollegeWebsite = ad.College.Website
	}
	if ad.CourseID != nil && ad.Course != nil {
		resp.CourseTitle = ad.Course.Title
		resp.CourseLevel = ad.Course.Level
		resp.CourseDuration = ad.Course.Duration
		resp.CourseField = ad.Course.FieldStudy
		resp.CourseBannerURL = ad.Course.BannerURL
		resp.CourseEstFee = ad.Course.EstFee
		resp.CourseAffiliation = ad.Course.Affiliation
	}

	return resp
}

func toCarouselSlideResponse(slide *CarouselSlide) CarouselSlideResponse {
	return CarouselSlideResponse{
		ID:          slide.ID,
		Page:        slide.Page,
		Title:       slide.Title,
		Subtitle:    slide.Subtitle,
		Description: slide.Description,
		ImageURL:    slide.ImageURL,
		LinkURL:     slide.LinkURL,
		ButtonText:  slide.ButtonText,
		Order:       slide.Order,
		Active:      slide.Active,
		CreatedAt:   slide.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   slide.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// Landing Course handlers

func (h *Handler) GetPublicLandingCourses(c *gin.Context) {
	courses, err := h.service.GetPublicLandingCourses()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve landing courses")
		return
	}
	response.Success(c, http.StatusOK, "Landing courses retrieved", courses)
}

func (h *Handler) GetAdminLandingCourses(c *gin.Context) {
	courses, err := h.service.GetAdminLandingCourses()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve landing courses")
		return
	}
	response.Success(c, http.StatusOK, "Landing courses retrieved", courses)
}

func (h *Handler) UpdateLandingField(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid field ID")
		return
	}
	var req UpdateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.UpdateLandingField(uint(id), req); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update field")
		return
	}
	response.Success(c, http.StatusOK, "Field updated successfully", nil)
}

func (h *Handler) ReorderLandingFields(c *gin.Context) {
	var req ReorderFieldsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.ReorderLandingFields(req); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to reorder fields")
		return
	}
	response.Success(c, http.StatusOK, "Fields reordered successfully", nil)
}

func (h *Handler) LinkLandingInstitution(c *gin.Context) {
	var req LinkInstitutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	inst, err := h.service.LinkInstitution(req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, http.StatusCreated, "Institution linked successfully", inst)
}

func (h *Handler) UnlinkLandingInstitution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}
	if err := h.service.UnlinkInstitution(uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to unlink institution")
		return
	}
	response.Success(c, http.StatusOK, "Institution unlinked successfully", nil)
}

func (h *Handler) ReorderLandingInstitutions(c *gin.Context) {
	var req ReorderInstitutionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.ReorderLandingInstitutions(req); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to reorder institutions")
		return
	}
	response.Success(c, http.StatusOK, "Institutions reordered successfully", nil)
}

func (h *Handler) SearchLandingInstitutions(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		response.Error(c, http.StatusBadRequest, "Search query is required")
		return
	}
	results, err := h.service.SearchInstitutionsForLanding(query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to search institutions")
		return
	}
	response.Success(c, http.StatusOK, "Search results", results)
}

// Course-finder ad card handlers

func courseAdErrorStatus(err error) int {
	var ve *validationError
	if errors.As(err, &ve) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func (h *Handler) GetCourseAdCards(c *gin.Context) {
	// Admin listing: all cards (including inactive) for the position.
	cards, err := h.service.GetCourseAdCards(c.Query("position"), false)
	if err != nil {
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Course ad cards retrieved", cards)
}

func (h *Handler) GetActiveCourseAdCards(c *gin.Context) {
	// Public listing: active cards only, priority desc, id desc, max 10.
	cards, err := h.service.GetCourseAdCards(c.Query("position"), true)
	if err != nil {
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Course ad cards retrieved", cards)
}

func (h *Handler) GetCourseAdCardByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}
	card, err := h.service.GetCourseAdCard(uint(id))
	if err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Course ad card not found")
			return
		}
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Course ad card retrieved", card)
}

func (h *Handler) CreateCourseAdCard(c *gin.Context) {
	var req CourseAdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	card, err := h.service.CreateCourseAdCard(req)
	if err != nil {
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusCreated, "Course ad card created", card)
}

func (h *Handler) UpdateCourseAdCard(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req CourseAdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	card, err := h.service.UpdateCourseAdCard(uint(id), req)
	if err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Course ad card not found")
			return
		}
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Course ad card updated", card)
}

func (h *Handler) DeleteCourseAdCard(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	if err := h.service.DeleteCourseAdCard(uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Course ad card not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete course ad card")
		return
	}
	response.Success(c, http.StatusOK, "Course ad card deleted", nil)
}

func (h *Handler) TrackCourseAdCardClick(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	if err := h.service.TrackCourseAdCardClick(uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Course ad card not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to track course ad click")
		return
	}
	response.Success(c, http.StatusOK, "Course ad click tracked", nil)
}

// Advertise request handlers

func authContextID(c *gin.Context) (uint, bool) {
	idVal, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := idVal.(uint)
	return id, ok
}

func authContextEmail(c *gin.Context) string {
	emailVal, exists := c.Get("user_email")
	if !exists {
		return ""
	}
	email, _ := emailVal.(string)
	return email
}

func (h *Handler) SubmitAdvertiseRequest(c *gin.Context) {
	institutionID, ok := authContextID(c)
	if !ok || institutionID == 0 {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req AdvertiseRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Email == "" {
		req.Email = authContextEmail(c)
	}

	resp, err := h.service.SubmitAdvertiseRequest(institutionID, req)
	if err != nil {
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusCreated, "Advertise request submitted successfully", resp)
}

func (h *Handler) GetInstitutionAdvertiseRequests(c *gin.Context) {
	institutionID, ok := authContextID(c)
	if !ok || institutionID == 0 {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	responses, err := h.service.GetInstitutionAdvertiseRequests(institutionID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve advertise requests")
		return
	}
	response.Success(c, http.StatusOK, "Advertise requests retrieved successfully", responses)
}

func (h *Handler) GetAdvertiseRequests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.Query("status")

	responses, total, err := h.service.GetAdvertiseRequests(page, limit, status)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve advertise requests")
		return
	}
	response.Success(c, http.StatusOK, "Advertise requests retrieved successfully", gin.H{
		"requests": responses,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) UpdateAdvertiseRequestStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req AdvertiseStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.service.UpdateAdvertiseRequestStatus(uint(id), req)
	if err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Advertise request not found")
			return
		}
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Advertise request status updated successfully", resp)
}

func toCourseAdCardResponse(card *CourseAdCard) CourseAdCardResponse {
	resp := CourseAdCardResponse{
		ID:           card.ID,
		Position:     card.Position,
		Subtitle:     card.Subtitle,
		Institutions: []CourseAdInstitutionResponse{},
		MouCompanies: []CourseAdMouResponse{},
		Active:       card.Active,
		Priority:     card.Priority,
		Clicks:       card.Clicks,
	}
	if card.Course != nil {
		resp.Course = &CourseAdCourseResponse{
			ID:           card.Course.ID,
			Title:        card.Course.Title,
			Level:        card.Course.Level,
			Duration:     card.Course.Duration,
			FieldOfStudy: card.Course.FieldStudy,
			Affiliation:  card.Course.Affiliation,
			EstFee:       card.Course.EstFee,
			BannerURL:    card.Course.BannerURL,
			Location:     card.Course.Location,
			Description:  card.Course.Description,
		}
	}
	for _, inst := range card.InstitutionInf {
		resp.Institutions = append(resp.Institutions, CourseAdInstitutionResponse{
			ID:       inst.ID,
			Name:     inst.Name,
			ImageURL: inst.ImageURL,
			Rating:   inst.Rating,
			Location: inst.Location,
			Website:  inst.Website,
			Slug:     inst.Slug,
		})
	}
	for _, mou := range card.MouCompanies {
		resp.MouCompanies = append(resp.MouCompanies, CourseAdMouResponse{
			ID:         mou.ID,
			Name:       mou.Name,
			LogoURL:    mou.LogoURL,
			CompanyURL: mou.CompanyURL,
		})
	}
	return resp
}

// College-finder page ad handlers

func (h *Handler) GetPublicCollegeAdTrending(c *gin.Context) {
	grouped, err := h.service.GetPublicCollegeAdTrending()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve trending college ads")
		return
	}
	response.Success(c, http.StatusOK, "Trending college ads retrieved", grouped)
}

func (h *Handler) GetCollegeAdTrending(c *gin.Context) {
	// Admin listing: all items (including inactive) for the kind filter.
	items, err := h.service.GetCollegeAdTrending(c.Query("kind"), false)
	if err != nil {
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Trending college ads retrieved", items)
}

func (h *Handler) CreateCollegeAdTrending(c *gin.Context) {
	var req CollegeAdTrendingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	item, err := h.service.CreateCollegeAdTrending(req)
	if err != nil {
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusCreated, "Trending college ad created", item)
}

func (h *Handler) UpdateCollegeAdTrending(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req CollegeAdTrendingUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	item, err := h.service.UpdateCollegeAdTrending(uint(id), req)
	if err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Trending college ad not found")
			return
		}
		response.Error(c, courseAdErrorStatus(err), err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Trending college ad updated", item)
}

func (h *Handler) DeleteCollegeAdTrending(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid ID")
		return
	}

	if err := h.service.DeleteCollegeAdTrending(uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Trending college ad not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete trending college ad")
		return
	}
	response.Success(c, http.StatusOK, "Trending college ad deleted", nil)
}

func (h *Handler) SubmitCollegeAdFeedback(c *gin.Context) {
	var req CollegeAdFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.SubmitCollegeAdFeedback(req); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "ok", nil)
}

func (h *Handler) GetCollegeAdFeedback(c *gin.Context) {
	feedback, err := h.service.GetCollegeAdFeedback()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve feedback")
		return
	}
	response.Success(c, http.StatusOK, "Feedback retrieved", feedback)
}

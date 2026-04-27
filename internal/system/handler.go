package system

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
	activeStr := c.Query("active")

	var active *bool
	if activeStr != "" {
		val := activeStr == "true"
		active = &val
	}

	ads, total, err := h.service.GetAds(page, limit, pageFilter, active)
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

	ads, err := h.service.GetActiveAds(page)
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
		responses[i] = toPublicNotificationResponse(&n)
	}

	response.Success(c, http.StatusOK, "Notifications retrieved successfully", responses)
}

func toContactInquiryResponse(inquiry *ContactInquiry) ContactInquiryResponse {
	return ContactInquiryResponse{
		ID:        inquiry.ID,
		Name:      inquiry.Name,
		Email:     inquiry.Email,
		Phone:     inquiry.Phone,
		Subject:   inquiry.Subject,
		Message:   inquiry.Message,
		Type:      inquiry.Type,
		Status:    inquiry.Status,
		CreatedAt: inquiry.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: inquiry.UpdatedAt.Format("2006-01-02T15:04:05Z"),
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

	return AdResponse{
		ID:          ad.ID,
		Title:       ad.Title,
		ImageURL:    ad.ImageURL,
		LinkURL:     ad.LinkURL,
		Page:        ad.Page,
		Position:    ad.Position,
		StartDate:   startDate,
		EndDate:     endDate,
		Active:      ad.Active,
		Clicks:      ad.Clicks,
		Impressions: ad.Impressions,
		Priority:    ad.Priority,
		CreatedAt:   ad.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   ad.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
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

func toPublicNotificationResponse(n *PublicNotification) PublicNotificationResponse {
	return PublicNotificationResponse{
		ID:        n.ID,
		CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Title:     n.Title,
		Message:   n.Message,
		Type:      n.Type,
		Link:      n.Link,
		Icon:      n.Icon,
		Color:     n.Color,
		BgColor:   n.BgColor,
	}
}

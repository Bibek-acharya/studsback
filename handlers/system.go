package handlers

import (
	"net/http"
	"strconv"
	"time"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SubmitContactInquiry(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Email   string `json:"email" binding:"required,email"`
		Phone   string `json:"phone"`
		Subject string `json:"subject" binding:"required"`
		Message string `json:"message" binding:"required"`
		Type    string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Type == "" {
		req.Type = "general"
	}

	inquiry := models.ContactInquiry{
		Name:    req.Name,
		Email:   req.Email,
		Phone:   req.Phone,
		Subject: req.Subject,
		Message: req.Message,
		Type:    req.Type,
		Status:  "new",
	}

	if err := config.GetDB().Create(&inquiry).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to submit inquiry")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Inquiry submitted successfully", inquiry)
}

func GetContactInquiries(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	inquiryType := c.Query("type")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := config.GetDB().Model(&models.ContactInquiry{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if inquiryType != "" {
		query = query.Where("type = ?", inquiryType)
	}

	var total int64
	query.Count(&total)

	var inquiries []models.ContactInquiry
	query.Order("created_at desc").Offset(offset).Limit(limit).Find(&inquiries)

	utils.SuccessResponse(c, http.StatusOK, "Inquiries retrieved successfully", gin.H{
		"inquiries": inquiries,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetContactInquiryByID(c *gin.Context) {
	id := c.Param("id")

	var inquiry models.ContactInquiry
	if err := config.GetDB().First(&inquiry, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Inquiry not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Inquiry retrieved successfully", inquiry)
}

func UpdateContactInquiryStatus(c *gin.Context) {
	id := c.Param("id")

	var inquiry models.ContactInquiry
	if err := config.GetDB().First(&inquiry, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Inquiry not found")
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	validStatuses := map[string]bool{
		"new": true, "read": true, "in_progress": true, "resolved": true, "closed": true,
	}
	if !validStatuses[req.Status] {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid status")
		return
	}

	config.GetDB().Model(&inquiry).Update("status", req.Status)

	utils.SuccessResponse(c, http.StatusOK, "Inquiry status updated successfully", inquiry)
}

func DeleteContactInquiry(c *gin.Context) {
	id := c.Param("id")

	result := config.GetDB().Delete(&models.ContactInquiry{}, id)
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete inquiry")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Inquiry not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Inquiry deleted successfully", nil)
}

func GetAds(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	pageFilter := c.Query("page")
	active := c.Query("active")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := config.GetDB().Model(&models.Ad{})
	if pageFilter != "" {
		query = query.Where("page = ?", pageFilter)
	}
	if active != "" {
		query = query.Where("active = ?", active == "true")
	}

	var total int64
	query.Count(&total)

	var ads []models.Ad
	query.Order("priority desc, created_at desc").Offset(offset).Limit(limit).Find(&ads)

	utils.SuccessResponse(c, http.StatusOK, "Ads retrieved successfully", gin.H{
		"ads": ads,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetActiveAds(c *gin.Context) {
	page := c.Query("page")

	query := config.GetDB().Model(&models.Ad{}).
		Where("active = ? AND start_date <= ? AND (end_date IS NULL OR end_date >= ?)", true, time.Now(), time.Now())

	if page != "" {
		query = query.Where("page = ?", page)
	}

	var ads []models.Ad
	query.Order("priority desc, created_at desc").Find(&ads)

	config.GetDB().Model(&models.Ad{}).
		Where("id IN ?", func() []uint {
			ids := make([]uint, len(ads))
			for i, ad := range ads {
				ids[i] = ad.ID
			}
			return ids
		}()).Update("impressions", gorm.Expr("impressions + 1"))

	utils.SuccessResponse(c, http.StatusOK, "Active ads retrieved successfully", ads)
}

func GetAdByID(c *gin.Context) {
	id := c.Param("id")

	var ad models.Ad
	if err := config.GetDB().First(&ad, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Ad not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Ad retrieved successfully", ad)
}

func CreateAd(c *gin.Context) {
	var req struct {
		Title     string `json:"title" binding:"required"`
		ImageURL  string `json:"image_url"`
		LinkURL   string `json:"link_url"`
		Page      string `json:"page" binding:"required"`
		Position  string `json:"position"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Active    *bool  `json:"active"`
		Priority  int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	ad := models.Ad{
		Title:    req.Title,
		ImageURL: req.ImageURL,
		LinkURL:  req.LinkURL,
		Page:     req.Page,
		Position: req.Position,
		Active:   true,
		Priority: req.Priority,
	}

	if req.StartDate != "" {
		if t, _ := parseTime(req.StartDate); !t.IsZero() {
			ad.StartDate = t
		}
	}
	if req.EndDate != "" {
		if t, _ := parseTime(req.EndDate); !t.IsZero() {
			ad.EndDate = t
		}
	}
	if req.Active != nil {
		ad.Active = *req.Active
	}

	if err := config.GetDB().Create(&ad).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create ad")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Ad created successfully", ad)
}

func UpdateAd(c *gin.Context) {
	id := c.Param("id")

	var ad models.Ad
	if err := config.GetDB().First(&ad, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Ad not found")
		return
	}

	var req struct {
		Title     string `json:"title"`
		ImageURL  string `json:"image_url"`
		LinkURL   string `json:"link_url"`
		Page      string `json:"page"`
		Position  string `json:"position"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Active    *bool  `json:"active"`
		Priority  int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}
	if req.LinkURL != "" {
		updates["link_url"] = req.LinkURL
	}
	if req.Page != "" {
		updates["page"] = req.Page
	}
	if req.Position != "" {
		updates["position"] = req.Position
	}
	if req.StartDate != "" {
		if t, _ := parseTime(req.StartDate); !t.IsZero() {
			updates["start_date"] = t
		}
	}
	if req.EndDate != "" {
		if t, _ := parseTime(req.EndDate); !t.IsZero() {
			updates["end_date"] = t
		}
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if req.Priority != 0 {
		updates["priority"] = req.Priority
	}

	config.GetDB().Model(&ad).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Ad updated successfully", ad)
}

func DeleteAd(c *gin.Context) {
	id := c.Param("id")

	result := config.GetDB().Delete(&models.Ad{}, id)
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete ad")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Ad not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Ad deleted successfully", nil)
}

func TrackAdClick(c *gin.Context) {
	id := c.Param("id")

	var ad models.Ad
	if err := config.GetDB().First(&ad, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Ad not found")
		return
	}

	config.GetDB().Model(&ad).Updates(map[string]interface{}{
		"clicks":      ad.Clicks + 1,
		"impressions": ad.Impressions + 1,
	})

	utils.SuccessResponse(c, http.StatusOK, "Ad click tracked", gin.H{"link_url": ad.LinkURL})
}

func GetCarousels(c *gin.Context) {
	page := c.DefaultQuery("page", "landing")
	active := c.Query("active")

	query := config.GetDB().Model(&models.CarouselSlide{}).Where("page = ?", page)
	if active != "" {
		query = query.Where("active = ?", active == "true")
	}

	var slides []models.CarouselSlide
	query.Order("order asc, created_at desc").Find(&slides)

	utils.SuccessResponse(c, http.StatusOK, "Carousel slides retrieved successfully", slides)
}

func GetCarouselSlideByID(c *gin.Context) {
	id := c.Param("id")

	var slide models.CarouselSlide
	if err := config.GetDB().First(&slide, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Slide not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Slide retrieved successfully", slide)
}

func CreateCarouselSlide(c *gin.Context) {
	var req struct {
		Page        string `json:"page"`
		Title       string `json:"title"`
		Subtitle    string `json:"subtitle"`
		Description string `json:"description"`
		ImageURL    string `json:"image_url"`
		LinkURL     string `json:"link_url"`
		ButtonText  string `json:"button_text"`
		Order       int    `json:"order"`
		Active      *bool  `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Page == "" {
		req.Page = "landing"
	}

	slide := models.CarouselSlide{
		Page:        req.Page,
		Title:       req.Title,
		Subtitle:    req.Subtitle,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		LinkURL:     req.LinkURL,
		ButtonText:  req.ButtonText,
		Order:       req.Order,
		Active:      true,
	}

	if req.Active != nil {
		slide.Active = *req.Active
	}

	if err := config.GetDB().Create(&slide).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create slide")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Slide created successfully", slide)
}

func UpdateCarouselSlide(c *gin.Context) {
	id := c.Param("id")

	var slide models.CarouselSlide
	if err := config.GetDB().First(&slide, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Slide not found")
		return
	}

	var req struct {
		Page        string `json:"page"`
		Title       string `json:"title"`
		Subtitle    string `json:"subtitle"`
		Description string `json:"description"`
		ImageURL    string `json:"image_url"`
		LinkURL     string `json:"link_url"`
		ButtonText  string `json:"button_text"`
		Order       int    `json:"order"`
		Active      *bool  `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Page != "" {
		updates["page"] = req.Page
	}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Subtitle != "" {
		updates["subtitle"] = req.Subtitle
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}
	if req.LinkURL != "" {
		updates["link_url"] = req.LinkURL
	}
	if req.ButtonText != "" {
		updates["button_text"] = req.ButtonText
	}
	if req.Order != 0 {
		updates["order"] = req.Order
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}

	config.GetDB().Model(&slide).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Slide updated successfully", slide)
}

func DeleteCarouselSlide(c *gin.Context) {
	id := c.Param("id")

	result := config.GetDB().Delete(&models.CarouselSlide{}, id)
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete slide")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Slide not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Slide deleted successfully", nil)
}

func ReorderCarouselSlides(c *gin.Context) {
	var req struct {
		SlideOrders []struct {
			ID    uint `json:"id" binding:"required"`
			Order int  `json:"order" binding:"required"`
		} `json:"slides" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	tx := config.GetDB().Begin()
	for _, item := range req.SlideOrders {
		if err := tx.Model(&models.CarouselSlide{}).Where("id = ?", item.ID).Update("order", item.Order).Error; err != nil {
			tx.Rollback()
			utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to reorder slides")
			return
		}
	}
	tx.Commit()

	utils.SuccessResponse(c, http.StatusOK, "Slides reordered successfully", nil)
}

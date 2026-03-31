package handlers

import (
	"net/http"
	"strconv"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
)

func getInstitutionID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	return uint(userID.(float64))
}

func GetInstitutionDashboard(c *gin.Context) {
	instID := getInstitutionID(c)

	var totalPrograms int64
	config.GetDB().Model(&models.InstitutionProgram{}).Where("institution_id = ?", instID).Count(&totalPrograms)

	var totalStudents int64
	config.GetDB().Model(&models.InstitutionEntranceApplicant{}).
		Joins("JOIN institution_entrances ON institution_entrances.id = institution_entrance_applicants.entrance_id").
		Where("institution_entrances.institution_id = ?", instID).
		Distinct("institution_entrance_applicants.user_id").Count(&totalStudents)

	var activeEntrances int64
	config.GetDB().Model(&models.InstitutionEntrance{}).
		Where("institution_id = ? AND status = ?", instID, "upcoming").Count(&activeEntrances)

	var pendingBookings int64
	config.GetDB().Model(&models.InstitutionCounsellingBooking{}).
		Joins("JOIN institution_counselling_sessions ON institution_counselling_sessions.id = institution_counselling_bookings.session_id").
		Where("institution_counselling_sessions.institution_id = ? AND institution_counselling_bookings.status = ?", instID, "pending").
		Count(&pendingBookings)

	var unreadMessages int64
	config.GetDB().Model(&models.InstitutionMessage{}).
		Where("institution_id = ? AND read = ?", instID, false).Count(&unreadMessages)

	utils.SuccessResponse(c, http.StatusOK, "Dashboard data retrieved successfully", gin.H{
		"total_programs":   totalPrograms,
		"total_students":   totalStudents,
		"active_entrances": activeEntrances,
		"pending_bookings": pendingBookings,
		"unread_messages":  unreadMessages,
	})
}

func GetInstitutionAnalytics(c *gin.Context) {
	instID := getInstitutionID(c)

	var programs []models.InstitutionProgram
	config.GetDB().Where("institution_id = ?", instID).Find(&programs)

	programStats := []gin.H{}
	for _, p := range programs {
		var entrances int64
		config.GetDB().Model(&models.InstitutionEntrance{}).
			Where("institution_id = ?", instID).Count(&entrances)
		programStats = append(programStats, gin.H{
			"id":        p.ID,
			"name":      p.Name,
			"status":    p.Status,
			"entrances": entrances,
		})
	}

	var totalApplicants int64
	config.GetDB().Model(&models.InstitutionEntranceApplicant{}).
		Joins("JOIN institution_entrances ON institution_entrances.id = institution_entrance_applicants.entrance_id").
		Where("institution_entrances.institution_id = ?", instID).Count(&totalApplicants)

	utils.SuccessResponse(c, http.StatusOK, "Analytics data retrieved successfully", gin.H{
		"program_stats":    programStats,
		"total_applicants": totalApplicants,
	})
}

func GetInstitutionPrograms(c *gin.Context) {
	instID := getInstitutionID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.InstitutionProgram{}).Where("institution_id = ?", instID).Count(&total)

	var programs []models.InstitutionProgram
	config.GetDB().Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&programs)

	utils.SuccessResponse(c, http.StatusOK, "Programs retrieved successfully", gin.H{
		"programs": programs,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetInstitutionProgramByID(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var program models.InstitutionProgram
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&program).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Program not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Program retrieved successfully", program)
}

func CreateInstitutionProgram(c *gin.Context) {
	instID := getInstitutionID(c)

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Duration    string `json:"duration"`
		Fee         string `json:"fee"`
		Eligibility string `json:"eligibility"`
		Capacity    int    `json:"capacity"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	program := models.InstitutionProgram{
		InstitutionID: instID,
		Name:          req.Name,
		Description:   req.Description,
		Duration:      req.Duration,
		Fee:           req.Fee,
		Eligibility:   req.Eligibility,
		Capacity:      req.Capacity,
		Status:        "active",
	}

	if err := config.GetDB().Create(&program).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create program")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Program created successfully", program)
}

func UpdateInstitutionProgram(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var program models.InstitutionProgram
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&program).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Program not found")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Duration    string `json:"duration"`
		Fee         string `json:"fee"`
		Eligibility string `json:"eligibility"`
		Capacity    int    `json:"capacity"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Duration != "" {
		updates["duration"] = req.Duration
	}
	if req.Fee != "" {
		updates["fee"] = req.Fee
	}
	if req.Eligibility != "" {
		updates["eligibility"] = req.Eligibility
	}
	if req.Capacity > 0 {
		updates["capacity"] = req.Capacity
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	config.GetDB().Model(&program).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Program updated successfully", program)
}

func DeleteInstitutionProgram(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	result := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).Delete(&models.InstitutionProgram{})
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete program")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Program not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Program deleted successfully", nil)
}

func GetInstitutionProfile(c *gin.Context) {
	instID := getInstitutionID(c)

	var instUser models.InstitutionUser
	if err := config.GetDB().First(&instUser, instID).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Institution not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Profile retrieved successfully", gin.H{
		"id":                  instUser.ID,
		"institution_name":    instUser.InstitutionName,
		"email":               instUser.Email,
		"registration_number": instUser.RegistrationNumber,
		"role":                instUser.Role,
	})
}

func UpdateInstitutionProfile(c *gin.Context) {
	instID := getInstitutionID(c)

	var instUser models.InstitutionUser
	if err := config.GetDB().First(&instUser, instID).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Institution not found")
		return
	}

	var req struct {
		InstitutionName    string `json:"institution_name"`
		RegistrationNumber string `json:"registration_number"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.InstitutionName != "" {
		updates["institution_name"] = req.InstitutionName
	}
	if req.RegistrationNumber != "" {
		updates["registration_number"] = req.RegistrationNumber
	}

	config.GetDB().Model(&instUser).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Profile updated successfully", gin.H{
		"id":                  instUser.ID,
		"institution_name":    instUser.InstitutionName,
		"email":               instUser.Email,
		"registration_number": instUser.RegistrationNumber,
		"role":                instUser.Role,
	})
}

func GetInstitutionMedia(c *gin.Context) {
	instID := getInstitutionID(c)

	var media []models.InstitutionMedia
	config.GetDB().Where("institution_id = ?", instID).Order("created_at desc").Find(&media)

	utils.SuccessResponse(c, http.StatusOK, "Media retrieved successfully", media)
}

func CreateInstitutionMedia(c *gin.Context) {
	instID := getInstitutionID(c)

	var req struct {
		URL   string `json:"url" binding:"required"`
		Type  string `json:"type" binding:"required"`
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	media := models.InstitutionMedia{
		InstitutionID: instID,
		URL:           req.URL,
		Type:          req.Type,
		Title:         req.Title,
	}

	if err := config.GetDB().Create(&media).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to upload media")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Media uploaded successfully", media)
}

func DeleteInstitutionMedia(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	result := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).Delete(&models.InstitutionMedia{})
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete media")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Media not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Media deleted successfully", nil)
}

func GetInstitutionCounsellingSessions(c *gin.Context) {
	instID := getInstitutionID(c)

	var sessions []models.InstitutionCounsellingSession
	config.GetDB().Where("institution_id = ?", instID).Order("scheduled_at asc").Find(&sessions)

	utils.SuccessResponse(c, http.StatusOK, "Counselling sessions retrieved successfully", sessions)
}

func GetInstitutionCounsellingBookings(c *gin.Context) {
	instID := getInstitutionID(c)

	var bookings []models.InstitutionCounsellingBooking
	config.GetDB().
		Joins("JOIN institution_counselling_sessions ON institution_counselling_sessions.id = institution_counselling_bookings.session_id").
		Where("institution_counselling_sessions.institution_id = ?", instID).
		Preload("Session").
		Order("institution_counselling_bookings.created_at desc").
		Find(&bookings)

	utils.SuccessResponse(c, http.StatusOK, "Counselling bookings retrieved successfully", bookings)
}

func UpdateInstitutionCounsellingBookingStatus(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var booking models.InstitutionCounsellingBooking
	if err := config.GetDB().
		Joins("JOIN institution_counselling_sessions ON institution_counselling_sessions.id = institution_counselling_bookings.session_id").
		Where("institution_counselling_bookings.id = ? AND institution_counselling_sessions.institution_id = ?", id, instID).
		First(&booking).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Booking not found")
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
		"pending": true, "confirmed": true, "cancelled": true, "completed": true,
	}
	if !validStatuses[req.Status] {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid status")
		return
	}

	config.GetDB().Model(&booking).Update("status", req.Status)

	utils.SuccessResponse(c, http.StatusOK, "Booking status updated successfully", booking)
}

func GetInstitutionEntrances(c *gin.Context) {
	instID := getInstitutionID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.InstitutionEntrance{}).Where("institution_id = ?", instID).Count(&total)

	var entrances []models.InstitutionEntrance
	config.GetDB().Where("institution_id = ?", instID).
		Order("date desc").Offset(offset).Limit(limit).Find(&entrances)

	utils.SuccessResponse(c, http.StatusOK, "Entrances retrieved successfully", gin.H{
		"entrances": entrances,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetInstitutionEntranceByID(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var entrance models.InstitutionEntrance
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&entrance).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Entrance not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Entrance retrieved successfully", entrance)
}

func CreateInstitutionEntrance(c *gin.Context) {
	instID := getInstitutionID(c)

	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Date        string `json:"date" binding:"required"`
		Duration    int    `json:"duration"`
		TotalSeats  int    `json:"total_seats"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	date, _ := parseTime(req.Date)

	entrance := models.InstitutionEntrance{
		InstitutionID: instID,
		Title:         req.Title,
		Description:   req.Description,
		Date:          date,
		Duration:      req.Duration,
		TotalSeats:    req.TotalSeats,
		Status:        "upcoming",
	}

	if err := config.GetDB().Create(&entrance).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create entrance")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Entrance created successfully", entrance)
}

func UpdateInstitutionEntrance(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var entrance models.InstitutionEntrance
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&entrance).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Entrance not found")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Date        string `json:"date"`
		Duration    int    `json:"duration"`
		TotalSeats  int    `json:"total_seats"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Date != "" {
		if t, _ := parseTime(req.Date); !t.IsZero() {
			updates["date"] = t
		}
	}
	if req.Duration > 0 {
		updates["duration"] = req.Duration
	}
	if req.TotalSeats > 0 {
		updates["total_seats"] = req.TotalSeats
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	config.GetDB().Model(&entrance).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Entrance updated successfully", entrance)
}

func DeleteInstitutionEntrance(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	result := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).Delete(&models.InstitutionEntrance{})
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete entrance")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Entrance not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Entrance deleted successfully", nil)
}

func GetInstitutionEntranceApplicants(c *gin.Context) {
	instID := getInstitutionID(c)
	entranceID := c.Param("id")

	var entrance models.InstitutionEntrance
	if err := config.GetDB().Where("id = ? AND institution_id = ?", entranceID, instID).First(&entrance).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Entrance not found")
		return
	}

	var applicants []models.InstitutionEntranceApplicant
	config.GetDB().Where("entrance_id = ?", entranceID).Order("rank asc").Find(&applicants)

	utils.SuccessResponse(c, http.StatusOK, "Applicants retrieved successfully", applicants)
}

func GetInstitutionEvents(c *gin.Context) {
	instID := getInstitutionID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.InstitutionEvent{}).Where("institution_id = ?", instID).Count(&total)

	var events []models.InstitutionEvent
	config.GetDB().Where("institution_id = ?", instID).
		Order("date desc").Offset(offset).Limit(limit).Find(&events)

	utils.SuccessResponse(c, http.StatusOK, "Events retrieved successfully", gin.H{
		"events": events,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetInstitutionEventByID(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var event models.InstitutionEvent
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&event).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Event not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Event retrieved successfully", event)
}

func CreateInstitutionEvent(c *gin.Context) {
	instID := getInstitutionID(c)

	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Date        string `json:"date" binding:"required"`
		Location    string `json:"location"`
		Image       string `json:"image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	date, _ := parseTime(req.Date)

	event := models.InstitutionEvent{
		InstitutionID: instID,
		Title:         req.Title,
		Description:   req.Description,
		Date:          date,
		Location:      req.Location,
		Image:         req.Image,
		Status:        "upcoming",
	}

	if err := config.GetDB().Create(&event).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create event")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Event created successfully", event)
}

func UpdateInstitutionEvent(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var event models.InstitutionEvent
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&event).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Event not found")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Date        string `json:"date"`
		Location    string `json:"location"`
		Image       string `json:"image"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Date != "" {
		if t, _ := parseTime(req.Date); !t.IsZero() {
			updates["date"] = t
		}
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.Image != "" {
		updates["image"] = req.Image
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	config.GetDB().Model(&event).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Event updated successfully", event)
}

func DeleteInstitutionEvent(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	result := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).Delete(&models.InstitutionEvent{})
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete event")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Event not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Event deleted successfully", nil)
}

func GetInstitutionNews(c *gin.Context) {
	instID := getInstitutionID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.InstitutionNews{}).Where("institution_id = ?", instID).Count(&total)

	var news []models.InstitutionNews
	config.GetDB().Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&news)

	utils.SuccessResponse(c, http.StatusOK, "News retrieved successfully", gin.H{
		"news": news,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetInstitutionNewsByID(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var news models.InstitutionNews
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&news).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "News not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "News retrieved successfully", news)
}

func CreateInstitutionNews(c *gin.Context) {
	instID := getInstitutionID(c)

	var req struct {
		Title    string `json:"title" binding:"required"`
		Content  string `json:"content"`
		Excerpt  string `json:"excerpt"`
		Image    string `json:"image"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	news := models.InstitutionNews{
		InstitutionID: instID,
		Title:         req.Title,
		Content:       req.Content,
		Excerpt:       req.Excerpt,
		Image:         req.Image,
		Category:      req.Category,
		Published:     true,
	}

	if err := config.GetDB().Create(&news).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create news")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "News created successfully", news)
}

func UpdateInstitutionNews(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var news models.InstitutionNews
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&news).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "News not found")
		return
	}

	var req struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Excerpt  string `json:"excerpt"`
		Image    string `json:"image"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.Excerpt != "" {
		updates["excerpt"] = req.Excerpt
	}
	if req.Image != "" {
		updates["image"] = req.Image
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}

	config.GetDB().Model(&news).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "News updated successfully", news)
}

func DeleteInstitutionNews(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	result := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).Delete(&models.InstitutionNews{})
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete news")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "News not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "News deleted successfully", nil)
}

func GetInstitutionQMS(c *gin.Context) {
	instID := getInstitutionID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.InstitutionQMS{}).Where("institution_id = ?", instID).Count(&total)

	var qms []models.InstitutionQMS
	config.GetDB().Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&qms)

	utils.SuccessResponse(c, http.StatusOK, "QMS records retrieved successfully", gin.H{
		"qms": qms,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetInstitutionQMSByID(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var qms models.InstitutionQMS
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&qms).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "QMS record not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "QMS record retrieved successfully", qms)
}

func CreateInstitutionQMS(c *gin.Context) {
	instID := getInstitutionID(c)

	var req struct {
		Title       string  `json:"title" binding:"required"`
		Description string  `json:"description"`
		Category    string  `json:"category"`
		Score       float64 `json:"score"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	qms := models.InstitutionQMS{
		InstitutionID: instID,
		Title:         req.Title,
		Description:   req.Description,
		Category:      req.Category,
		Score:         req.Score,
		Status:        "pending",
	}

	if err := config.GetDB().Create(&qms).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create QMS record")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "QMS record created successfully", qms)
}

func UpdateInstitutionQMS(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var qms models.InstitutionQMS
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&qms).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "QMS record not found")
		return
	}

	var req struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Category    string  `json:"category"`
		Status      string  `json:"status"`
		Score       float64 `json:"score"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Score > 0 {
		updates["score"] = req.Score
	}

	config.GetDB().Model(&qms).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "QMS record updated successfully", qms)
}

func DeleteInstitutionQMS(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	result := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).Delete(&models.InstitutionQMS{})
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete QMS record")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "QMS record not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "QMS record deleted successfully", nil)
}

func GetInstitutionMessages(c *gin.Context) {
	instID := getInstitutionID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.InstitutionMessage{}).Where("institution_id = ?", instID).Count(&total)

	var messages []models.InstitutionMessage
	config.GetDB().Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&messages)

	utils.SuccessResponse(c, http.StatusOK, "Messages retrieved successfully", gin.H{
		"messages": messages,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetInstitutionMessageByID(c *gin.Context) {
	instID := getInstitutionID(c)
	id := c.Param("id")

	var message models.InstitutionMessage
	if err := config.GetDB().Where("id = ? AND institution_id = ?", id, instID).First(&message).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Message not found")
		return
	}

	if !message.Read {
		config.GetDB().Model(&message).Update("read", true)
	}

	utils.SuccessResponse(c, http.StatusOK, "Message retrieved successfully", message)
}

func CreateInstitutionMessage(c *gin.Context) {
	instID := getInstitutionID(c)

	var req struct {
		UserID  uint   `json:"user_id" binding:"required"`
		Subject string `json:"subject" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	message := models.InstitutionMessage{
		InstitutionID: instID,
		UserID:        req.UserID,
		Subject:       req.Subject,
		Content:       req.Content,
		Direction:     "outbound",
	}

	if err := config.GetDB().Create(&message).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to send message")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Message sent successfully", message)
}

func GetInstitutionMessageStudents(c *gin.Context) {
	instID := getInstitutionID(c)

	type StudentContact struct {
		UserID  uint   `json:"user_id"`
		Name    string `json:"name"`
		LastMsg string `json:"last_message"`
		Unread  int    `json:"unread"`
	}

	var messages []models.InstitutionMessage
	config.GetDB().Where("institution_id = ?", instID).Order("created_at desc").Find(&messages)

	contactMap := map[uint]*StudentContact{}
	for _, msg := range messages {
		if _, exists := contactMap[msg.UserID]; !exists {
			var user models.User
			config.GetDB().First(&user, msg.UserID)
			contactMap[msg.UserID] = &StudentContact{
				UserID:  msg.UserID,
				Name:    user.FirstName + " " + user.LastName,
				LastMsg: msg.Content,
			}
		}
		if !msg.Read && msg.Direction == "inbound" {
			contactMap[msg.UserID].Unread++
		}
	}

	contacts := make([]StudentContact, 0, len(contactMap))
	for _, c := range contactMap {
		contacts = append(contacts, *c)
	}

	utils.SuccessResponse(c, http.StatusOK, "Student contacts retrieved successfully", contacts)
}

func GetInstitutionSettings(c *gin.Context) {
	instID := getInstitutionID(c)

	var settings models.InstitutionSettings
	if err := config.GetDB().Where("institution_id = ?", instID).FirstOrCreate(&settings, models.InstitutionSettings{
		InstitutionID: instID,
	}).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch settings")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Settings retrieved successfully", settings)
}

func UpdateInstitutionSettings(c *gin.Context) {
	instID := getInstitutionID(c)

	var settings models.InstitutionSettings
	config.GetDB().Where("institution_id = ?", instID).FirstOrCreate(&settings, models.InstitutionSettings{
		InstitutionID: instID,
	})

	var req struct {
		EmailNotifs   bool   `json:"email_notifications"`
		Timezone      string `json:"timezone"`
		Language      string `json:"language"`
		PublicProfile bool   `json:"public_profile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	config.GetDB().Model(&settings).Updates(map[string]interface{}{
		"email_notifs":   req.EmailNotifs,
		"timezone":       req.Timezone,
		"language":       req.Language,
		"public_profile": req.PublicProfile,
	})

	utils.SuccessResponse(c, http.StatusOK, "Settings updated successfully", settings)
}

func UpdateInstitutionPassword(c *gin.Context) {
	instID := getInstitutionID(c)

	var instUser models.InstitutionUser
	if err := config.GetDB().First(&instUser, instID).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Institution not found")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := instUser.CheckPassword(req.CurrentPassword); err != nil {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Current password is incorrect")
		return
	}

	if err := instUser.HashPassword(req.NewPassword); err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	config.GetDB().Model(&instUser).Update("password", instUser.Password)

	utils.SuccessResponse(c, http.StatusOK, "Password updated successfully", nil)
}

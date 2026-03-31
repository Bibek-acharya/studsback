package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseTime(s string) (time.Time, error) {
	formats := []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, nil
}

func toJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func getProviderID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	idStr := strconv.FormatUint(uint64(userID.(float64)), 10)
	id, _ := strconv.Atoi(idStr)
	return uint(id)
}

func GetScholarshipProviderDashboard(c *gin.Context) {
	providerID := getProviderID(c)

	var totalScholarships int64
	config.GetDB().Model(&models.ProviderScholarship{}).Where("provider_id = ?", providerID).Count(&totalScholarships)

	var totalApplications int64
	config.GetDB().Model(&models.ProviderApplication{}).
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_scholarships.provider_id = ?", providerID).Count(&totalApplications)

	var pendingApplications int64
	config.GetDB().Model(&models.ProviderApplication{}).
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_scholarships.provider_id = ? AND provider_applications.status = ?", providerID, "pending").
		Count(&pendingApplications)

	var totalInterviews int64
	config.GetDB().Model(&models.ProviderInterview{}).Where("provider_id = ?", providerID).Count(&totalInterviews)

	var unreadMessages int64
	config.GetDB().Model(&models.ProviderMessage{}).Where("provider_id = ? AND read = ?", providerID, false).Count(&unreadMessages)

	utils.SuccessResponse(c, http.StatusOK, "Dashboard data retrieved successfully", gin.H{
		"total_scholarships":   totalScholarships,
		"total_applications":   totalApplications,
		"pending_applications": pendingApplications,
		"total_interviews":     totalInterviews,
		"unread_messages":      unreadMessages,
	})
}

func GetScholarshipProviderAnalytics(c *gin.Context) {
	providerID := getProviderID(c)

	var applications []models.ProviderApplication
	config.GetDB().
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_scholarships.provider_id = ?", providerID).
		Find(&applications)

	statusCounts := map[string]int{}
	for _, app := range applications {
		statusCounts[app.Status]++
	}

	var scholarships []models.ProviderScholarship
	config.GetDB().Where("provider_id = ?", providerID).Find(&scholarships)

	scholarshipStats := []gin.H{}
	for _, s := range scholarships {
		var appCount int64
		config.GetDB().Model(&models.ProviderApplication{}).Where("scholarship_id = ?", s.ID).Count(&appCount)
		scholarshipStats = append(scholarshipStats, gin.H{
			"id":           s.ID,
			"title":        s.Title,
			"applications": appCount,
			"status":       s.Status,
		})
	}

	utils.SuccessResponse(c, http.StatusOK, "Analytics data retrieved successfully", gin.H{
		"status_breakdown":   statusCounts,
		"total_applications": len(applications),
		"scholarship_stats":  scholarshipStats,
	})
}

func CreateProviderScholarship(c *gin.Context) {
	providerID := getProviderID(c)

	var req models.CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	scholarship := models.ProviderScholarship{
		ProviderID:      providerID,
		Title:           req.Title,
		Description:     req.Description,
		Location:        req.Location,
		Value:           req.Value,
		DegreeLevel:     req.DegreeLevel,
		FundingType:     req.FundingType,
		ScholarshipType: req.ScholarshipType,
		FieldOfStudy:    toJSON(req.FieldOfStudy),
		Status:          "draft",
	}

	if req.Deadline != "" {
		if deadline, err := parseTime(req.Deadline); err == nil {
			scholarship.Deadline = deadline
		}
	}

	if err := config.GetDB().Create(&scholarship).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create scholarship")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Scholarship created successfully", scholarship)
}

func GetProviderScholarships(c *gin.Context) {
	providerID := getProviderID(c)

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
	config.GetDB().Model(&models.ProviderScholarship{}).Where("provider_id = ?", providerID).Count(&total)

	var scholarships []models.ProviderScholarship
	config.GetDB().Where("provider_id = ?", providerID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&scholarships)

	utils.SuccessResponse(c, http.StatusOK, "Scholarships retrieved successfully", gin.H{
		"scholarships": scholarships,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetProviderScholarshipByID(c *gin.Context) {
	providerID := getProviderID(c)
	id := c.Param("id")

	var scholarship models.ProviderScholarship
	if err := config.GetDB().Where("id = ? AND provider_id = ?", id, providerID).First(&scholarship).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.ErrorResponse(c, http.StatusNotFound, "Scholarship not found")
			return
		}
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch scholarship")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Scholarship retrieved successfully", scholarship)
}

func UpdateProviderScholarship(c *gin.Context) {
	providerID := getProviderID(c)
	id := c.Param("id")

	var scholarship models.ProviderScholarship
	if err := config.GetDB().Where("id = ? AND provider_id = ?", id, providerID).First(&scholarship).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Scholarship not found")
		return
	}

	var req models.CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{
		"title":            req.Title,
		"description":      req.Description,
		"location":         req.Location,
		"value":            req.Value,
		"degree_level":     req.DegreeLevel,
		"funding_type":     req.FundingType,
		"scholarship_type": req.ScholarshipType,
		"field_of_study":   toJSON(req.FieldOfStudy),
	}

	if req.Deadline != "" {
		if deadline, err := parseTime(req.Deadline); err == nil {
			updates["deadline"] = deadline
		}
	}

	config.GetDB().Model(&scholarship).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Scholarship updated successfully", scholarship)
}

func DeleteProviderScholarship(c *gin.Context) {
	providerID := getProviderID(c)
	id := c.Param("id")

	result := config.GetDB().Where("id = ? AND provider_id = ?", id, providerID).Delete(&models.ProviderScholarship{})
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete scholarship")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Scholarship not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Scholarship deleted successfully", nil)
}

func GetProviderApplications(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.Query("status")
	scholarshipID := c.Query("scholarship_id")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := config.GetDB().Model(&models.ProviderApplication{}).
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_scholarships.provider_id = ?", providerID)

	if status != "" {
		query = query.Where("provider_applications.status = ?", status)
	}
	if scholarshipID != "" {
		query = query.Where("provider_applications.scholarship_id = ?", scholarshipID)
	}

	var total int64
	query.Count(&total)

	var applications []models.ProviderApplication
	query.Preload("Scholarship").Order("created_at desc").Offset(offset).Limit(limit).Find(&applications)

	utils.SuccessResponse(c, http.StatusOK, "Applications retrieved successfully", gin.H{
		"applications": applications,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetProviderApplicationByID(c *gin.Context) {
	providerID := getProviderID(c)
	id := c.Param("id")

	var application models.ProviderApplication
	if err := config.GetDB().
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_applications.id = ? AND provider_scholarships.provider_id = ?", id, providerID).
		Preload("Scholarship").
		First(&application).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Application not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Application retrieved successfully", application)
}

func EvaluateProviderApplication(c *gin.Context) {
	providerID := getProviderID(c)
	id := c.Param("id")

	var application models.ProviderApplication
	if err := config.GetDB().
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_applications.id = ? AND provider_scholarships.provider_id = ?", id, providerID).
		First(&application).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Application not found")
		return
	}

	var req struct {
		Score   int    `json:"score"`
		Notes   string `json:"notes"`
		Passing bool   `json:"passing"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	config.GetDB().Model(&application).Updates(map[string]interface{}{
		"evaluation_notes": req.Notes,
	})

	utils.SuccessResponse(c, http.StatusOK, "Application evaluated successfully", application)
}

func UpdateProviderApplicationStatus(c *gin.Context) {
	providerID := getProviderID(c)
	id := c.Param("id")

	var application models.ProviderApplication
	if err := config.GetDB().
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_applications.id = ? AND provider_scholarships.provider_id = ?", id, providerID).
		First(&application).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Application not found")
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
		"pending": true, "under_review": true, "approved": true,
		"rejected": true, "shortlisted": true,
	}
	if !validStatuses[req.Status] {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid status")
		return
	}

	config.GetDB().Model(&application).Update("status", req.Status)

	utils.SuccessResponse(c, http.StatusOK, "Application status updated successfully", application)
}

func GetProviderInterviews(c *gin.Context) {
	providerID := getProviderID(c)

	var interviews []models.ProviderInterview
	config.GetDB().Where("provider_id = ?", providerID).
		Order("scheduled_at asc").Find(&interviews)

	utils.SuccessResponse(c, http.StatusOK, "Interviews retrieved successfully", interviews)
}

func CreateProviderInterview(c *gin.Context) {
	providerID := getProviderID(c)

	var req struct {
		ApplicationID uint   `json:"application_id" binding:"required"`
		ScheduledAt   string `json:"scheduled_at" binding:"required"`
		Duration      int    `json:"duration"`
		Type          string `json:"type"`
		Location      string `json:"location"`
		Link          string `json:"link"`
		Notes         string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	scheduledAt, err := parseTime(req.ScheduledAt)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid scheduled_at format")
		return
	}

	interview := models.ProviderInterview{
		ProviderID:    providerID,
		ApplicationID: req.ApplicationID,
		ScheduledAt:   scheduledAt,
		Duration:      req.Duration,
		Type:          req.Type,
		Location:      req.Location,
		Link:          req.Link,
		Notes:         req.Notes,
		Status:        "scheduled",
	}

	if err := config.GetDB().Create(&interview).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create interview")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Interview scheduled successfully", interview)
}

func UpdateProviderInterview(c *gin.Context) {
	providerID := getProviderID(c)
	id := c.Param("id")

	var interview models.ProviderInterview
	if err := config.GetDB().Where("id = ? AND provider_id = ?", id, providerID).First(&interview).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Interview not found")
		return
	}

	var req struct {
		ScheduledAt string `json:"scheduled_at"`
		Duration    int    `json:"duration"`
		Type        string `json:"type"`
		Location    string `json:"location"`
		Link        string `json:"link"`
		Status      string `json:"status"`
		Notes       string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.ScheduledAt != "" {
		if t, err := parseTime(req.ScheduledAt); err == nil {
			updates["scheduled_at"] = t
		}
	}
	if req.Duration > 0 {
		updates["duration"] = req.Duration
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.Link != "" {
		updates["link"] = req.Link
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}

	config.GetDB().Model(&interview).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Interview updated successfully", interview)
}

func GetProviderMessages(c *gin.Context) {
	providerID := getProviderID(c)

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
	config.GetDB().Model(&models.ProviderMessage{}).Where("provider_id = ?", providerID).Count(&total)

	var messages []models.ProviderMessage
	config.GetDB().Where("provider_id = ?", providerID).
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

func CreateProviderMessage(c *gin.Context) {
	providerID := getProviderID(c)

	var req struct {
		UserID  uint   `json:"user_id" binding:"required"`
		Subject string `json:"subject" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	message := models.ProviderMessage{
		ProviderID: providerID,
		UserID:     req.UserID,
		Subject:    req.Subject,
		Content:    req.Content,
		Direction:  "outbound",
	}

	if err := config.GetDB().Create(&message).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to send message")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Message sent successfully", message)
}

func GetProviderMessageByID(c *gin.Context) {
	providerID := getProviderID(c)
	id := c.Param("id")

	var message models.ProviderMessage
	if err := config.GetDB().Where("id = ? AND provider_id = ?", id, providerID).First(&message).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Message not found")
		return
	}

	if !message.Read {
		config.GetDB().Model(&message).Update("read", true)
	}

	utils.SuccessResponse(c, http.StatusOK, "Message retrieved successfully", message)
}

func GetProviderProfile(c *gin.Context) {
	providerID := getProviderID(c)

	var provider models.ScholarshipProviderUser
	if err := config.GetDB().First(&provider, providerID).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Provider not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Profile retrieved successfully", gin.H{
		"id":                  provider.ID,
		"provider_name":       provider.ProviderName,
		"registration_number": provider.RegistrationNumber,
		"email":               provider.Email,
		"role":                provider.Role,
	})
}

func UpdateProviderProfile(c *gin.Context) {
	providerID := getProviderID(c)

	var provider models.ScholarshipProviderUser
	if err := config.GetDB().First(&provider, providerID).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Provider not found")
		return
	}

	var req struct {
		ProviderName       string `json:"provider_name"`
		RegistrationNumber string `json:"registration_number"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.ProviderName != "" {
		updates["provider_name"] = req.ProviderName
	}
	if req.RegistrationNumber != "" {
		updates["registration_number"] = req.RegistrationNumber
	}

	config.GetDB().Model(&provider).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Profile updated successfully", gin.H{
		"id":                  provider.ID,
		"provider_name":       provider.ProviderName,
		"registration_number": provider.RegistrationNumber,
		"email":               provider.Email,
	})
}

func GetProviderSettings(c *gin.Context) {
	providerID := getProviderID(c)

	var settings models.ProviderSettings
	if err := config.GetDB().Where("provider_id = ?", providerID).FirstOrCreate(&settings, models.ProviderSettings{
		ProviderID: providerID,
	}).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch settings")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Settings retrieved successfully", settings)
}

func UpdateProviderSettings(c *gin.Context) {
	providerID := getProviderID(c)

	var settings models.ProviderSettings
	config.GetDB().Where("provider_id = ?", providerID).FirstOrCreate(&settings, models.ProviderSettings{
		ProviderID: providerID,
	})

	var req struct {
		EmailNotifs bool   `json:"email_notifications"`
		SmsNotifs   bool   `json:"sms_notifications"`
		AutoReject  bool   `json:"auto_reject_expired"`
		Timezone    string `json:"timezone"`
		Language    string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	config.GetDB().Model(&settings).Updates(map[string]interface{}{
		"email_notifs": req.EmailNotifs,
		"sms_notifs":   req.SmsNotifs,
		"auto_reject":  req.AutoReject,
		"timezone":     req.Timezone,
		"language":     req.Language,
	})

	utils.SuccessResponse(c, http.StatusOK, "Settings updated successfully", settings)
}

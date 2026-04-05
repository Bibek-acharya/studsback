package scholarshipprovider

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

func getProviderID(c *gin.Context) uint {
	userID, _ := c.Get("user_id")
	return userID.(uint)
}

func (h *Handler) GetDashboard(c *gin.Context) {
	providerID := getProviderID(c)

	dashboard, err := h.service.GetDashboard(providerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch dashboard")
		return
	}

	response.Success(c, http.StatusOK, "Dashboard data retrieved successfully", dashboard)
}

func (h *Handler) GetAnalytics(c *gin.Context) {
	providerID := getProviderID(c)

	analytics, err := h.service.GetAnalytics(providerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch analytics")
		return
	}

	response.Success(c, http.StatusOK, "Analytics data retrieved successfully", analytics)
}

func (h *Handler) CreateScholarship(c *gin.Context) {
	providerID := getProviderID(c)

	var req CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	scholarship, err := h.service.CreateScholarship(providerID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create scholarship")
		return
	}

	response.Success(c, http.StatusCreated, "Scholarship created successfully", toScholarshipResponse(scholarship))
}

func (h *Handler) GetScholarships(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	scholarships, total, err := h.service.GetScholarships(providerID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch scholarships")
		return
	}

	responses := make([]ScholarshipResponse, len(scholarships))
	for i, s := range scholarships {
		responses[i] = toScholarshipResponse(&s)
	}

	response.Success(c, http.StatusOK, "Scholarships retrieved successfully", ScholarshipListResponse{
		Scholarships: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetScholarshipByID(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid scholarship ID")
		return
	}

	scholarship, err := h.service.GetScholarshipByID(providerID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Scholarship not found")
		return
	}

	response.Success(c, http.StatusOK, "Scholarship retrieved successfully", toScholarshipResponse(scholarship))
}

func (h *Handler) UpdateScholarship(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid scholarship ID")
		return
	}

	var req CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	scholarship, err := h.service.UpdateScholarship(providerID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Scholarship not found")
		return
	}

	response.Success(c, http.StatusOK, "Scholarship updated successfully", toScholarshipResponse(scholarship))
}

func (h *Handler) DeleteScholarship(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid scholarship ID")
		return
	}

	if err := h.service.DeleteScholarship(providerID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Scholarship not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete scholarship")
		return
	}

	response.Success(c, http.StatusOK, "Scholarship deleted successfully", nil)
}

func (h *Handler) GetApplications(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.Query("status")
	scholarshipID := c.Query("scholarship_id")

	applications, total, err := h.service.GetApplications(providerID, page, limit, status, scholarshipID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch applications")
		return
	}

	responses := make([]ApplicationResponse, len(applications))
	for i, a := range applications {
		responses[i] = toApplicationResponse(&a)
	}

	response.Success(c, http.StatusOK, "Applications retrieved successfully", ApplicationListResponse{
		Applications: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetApplicationByID(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application ID")
		return
	}

	application, err := h.service.GetApplicationByID(providerID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Application not found")
		return
	}

	response.Success(c, http.StatusOK, "Application retrieved successfully", toApplicationResponse(application))
}

func (h *Handler) EvaluateApplication(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application ID")
		return
	}

	var req EvaluateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	application, err := h.service.EvaluateApplication(providerID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Application not found")
		return
	}

	response.Success(c, http.StatusOK, "Application evaluated successfully", toApplicationResponse(application))
}

func (h *Handler) UpdateApplicationStatus(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application ID")
		return
	}

	var req UpdateApplicationStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	application, err := h.service.UpdateApplicationStatus(providerID, uint(id), req)
	if err != nil {
		if err.Error() == "invalid status" {
			response.Error(c, http.StatusBadRequest, "Invalid status")
			return
		}
		response.Error(c, http.StatusNotFound, "Application not found")
		return
	}

	response.Success(c, http.StatusOK, "Application status updated successfully", toApplicationResponse(application))
}

func (h *Handler) GetInterviews(c *gin.Context) {
	providerID := getProviderID(c)

	interviews, err := h.service.GetInterviews(providerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch interviews")
		return
	}

	responses := make([]InterviewResponse, len(interviews))
	for i, interview := range interviews {
		responses[i] = toInterviewResponse(&interview)
	}

	response.Success(c, http.StatusOK, "Interviews retrieved successfully", responses)
}

func (h *Handler) CreateInterview(c *gin.Context) {
	providerID := getProviderID(c)

	var req CreateInterviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	interview, err := h.service.CreateInterview(providerID, req)
	if err != nil {
		if err.Error() == "invalid scheduled_at format" {
			response.Error(c, http.StatusBadRequest, "Invalid scheduled_at format")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create interview")
		return
	}

	response.Success(c, http.StatusCreated, "Interview scheduled successfully", toInterviewResponse(interview))
}

func (h *Handler) UpdateInterview(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid interview ID")
		return
	}

	var req UpdateInterviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	interview, err := h.service.UpdateInterview(providerID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Interview not found")
		return
	}

	response.Success(c, http.StatusOK, "Interview updated successfully", toInterviewResponse(interview))
}

func (h *Handler) GetMessages(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	messages, total, err := h.service.GetMessages(providerID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch messages")
		return
	}

	responses := make([]MessageResponse, len(messages))
	for i, m := range messages {
		responses[i] = toMessageResponse(&m)
	}

	response.Success(c, http.StatusOK, "Messages retrieved successfully", MessageListResponse{
		Messages: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) CreateMessage(c *gin.Context) {
	providerID := getProviderID(c)

	var req CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	message, err := h.service.CreateMessage(providerID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to send message")
		return
	}

	response.Success(c, http.StatusCreated, "Message sent successfully", toMessageResponse(message))
}

func (h *Handler) GetMessageByID(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid message ID")
		return
	}

	message, err := h.service.GetMessageByID(providerID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Message not found")
		return
	}

	response.Success(c, http.StatusOK, "Message retrieved successfully", toMessageResponse(message))
}

func (h *Handler) GetProfile(c *gin.Context) {
	providerID := getProviderID(c)

	provider, err := h.service.GetProviderProfile(providerID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Provider not found")
		return
	}

	response.Success(c, http.StatusOK, "Profile retrieved successfully", ProfileResponse{
		ID:                 provider.ID,
		ProviderName:       provider.ProviderName,
		RegistrationNumber: provider.RegistrationNumber,
		Email:              provider.Email,
		Role:               provider.Role,
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	providerID := getProviderID(c)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	provider, err := h.service.UpdateProviderProfile(providerID, req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Provider not found")
		return
	}

	response.Success(c, http.StatusOK, "Profile updated successfully", ProfileResponse{
		ID:                 provider.ID,
		ProviderName:       provider.ProviderName,
		RegistrationNumber: provider.RegistrationNumber,
		Email:              provider.Email,
	})
}

func (h *Handler) GetSettings(c *gin.Context) {
	providerID := getProviderID(c)

	settings, err := h.service.GetProviderSettings(providerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch settings")
		return
	}

	response.Success(c, http.StatusOK, "Settings retrieved successfully", toSettingsResponse(settings))
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	providerID := getProviderID(c)

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	settings, err := h.service.UpdateProviderSettings(providerID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	response.Success(c, http.StatusOK, "Settings updated successfully", toSettingsResponse(settings))
}

func (h *Handler) GetNotifications(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	notifications, total, unreadCount, err := h.service.GetNotifications(providerID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch notifications")
		return
	}

	responses := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		responses[i] = toNotificationResponse(&n)
	}

	response.Success(c, http.StatusOK, "Notifications retrieved successfully", NotificationListResponse{
		Notifications: responses,
		UnreadCount:   unreadCount,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) MarkNotificationRead(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	if err := h.service.MarkNotificationRead(providerID, uint(id)); err != nil {
		response.Error(c, http.StatusNotFound, "Notification not found")
		return
	}

	response.Success(c, http.StatusOK, "Notification marked as read", nil)
}

func (h *Handler) MarkAllNotificationsRead(c *gin.Context) {
	providerID := getProviderID(c)

	if err := h.service.MarkAllNotificationsRead(providerID); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to mark notifications as read")
		return
	}

	response.Success(c, http.StatusOK, "All notifications marked as read", nil)
}

func toScholarshipResponse(s *ProviderScholarship) ScholarshipResponse {
	return ScholarshipResponse{
		ID:                  s.ID,
		CreatedAt:           s.CreatedAt,
		UpdatedAt:           s.UpdatedAt,
		ProviderID:          s.ProviderID,
		Title:               s.Title,
		Description:         s.Description,
		ImageURL:            s.ImageURL,
		Location:            s.Location,
		Value:               s.Value,
		Deadline:            s.Deadline,
		DegreeLevel:         s.DegreeLevel,
		FundingType:         s.FundingType,
		ScholarshipType:     s.ScholarshipType,
		FieldOfStudy:        s.FieldOfStudy,
		EligibilityCriteria: s.EligibilityCriteria,
		RequiredDocuments:   s.RequiredDocuments,
		Status:              s.Status,
		ApplicationsCount:   s.ApplicationsCount,
	}
}

func toApplicationResponse(a *ProviderApplication) ApplicationResponse {
	resp := ApplicationResponse{
		ID:                a.ID,
		CreatedAt:         a.CreatedAt,
		UpdatedAt:         a.UpdatedAt,
		ScholarshipID:     a.ScholarshipID,
		UserID:            a.UserID,
		FirstName:         a.FirstName,
		LastName:          a.LastName,
		Email:             a.Email,
		PhoneNumber:       a.PhoneNumber,
		Status:            a.Status,
		EvaluationNotes:   a.EvaluationNotes,
		Documents:         a.Documents,
		PersonalStatement: a.PersonalStatement,
	}

	if a.Scholarship.ID != 0 {
		schResp := toScholarshipResponse(&a.Scholarship)
		resp.Scholarship = &schResp
	}

	return resp
}

func toInterviewResponse(i *ProviderInterview) InterviewResponse {
	return InterviewResponse{
		ID:            i.ID,
		CreatedAt:     i.CreatedAt,
		UpdatedAt:     i.UpdatedAt,
		ApplicationID: i.ApplicationID,
		ProviderID:    i.ProviderID,
		ScheduledAt:   i.ScheduledAt,
		Duration:      i.Duration,
		Type:          i.Type,
		Location:      i.Location,
		Link:          i.Link,
		Status:        i.Status,
		Notes:         i.Notes,
	}
}

func toMessageResponse(m *ProviderMessage) MessageResponse {
	return MessageResponse{
		ID:         m.ID,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
		ProviderID: m.ProviderID,
		UserID:     m.UserID,
		Subject:    m.Subject,
		Content:    m.Content,
		Read:       m.Read,
		Direction:  m.Direction,
	}
}

func toSettingsResponse(s *ProviderSettings) SettingsResponse {
	return SettingsResponse{
		ID:          s.ID,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		ProviderID:  s.ProviderID,
		EmailNotifs: s.EmailNotifs,
		SmsNotifs:   s.SmsNotifs,
		AutoReject:  s.AutoReject,
		Timezone:    s.Timezone,
		Language:    s.Language,
	}
}

func toNotificationResponse(n *ProviderNotification) NotificationResponse {
	return NotificationResponse{
		ID:         n.ID,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
		ProviderID: n.ProviderID,
		Title:      n.Title,
		Message:    n.Message,
		Type:       n.Type,
		Read:       n.Read,
		Link:       n.Link,
	}
}

package scholarshipprovider

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetService() *Service {
	return h.service
}

func getProviderID(c *gin.Context) uint {
	providerID, ok := c.Get("provider_id")
	if ok && providerID.(uint) > 0 {
		return providerID.(uint)
	}
	userID, _ := c.Get("user_id")
	return userID.(uint)
}


func resolveProviderUploadFolder(folder string) (string, error) {
	switch folder {
	case "", "general":
		return "scholarship-provider/general", nil
	case "scholarship-banners", "scholarships":
		return "scholarship-provider/scholarship-banners", nil
	case "news":
		return "scholarship-provider/news", nil
	case "events":
		return "scholarship-provider/events", nil
	case "blogs":
		return "scholarship-provider/blogs", nil
	case "profile":
		return "scholarship-provider/profile", nil
	case "logos":
		return "scholarship-provider/logos", nil
	case "gallery":
		return "scholarship-provider/gallery", nil
	case "partners":
		return "scholarship-provider/partners", nil
	case "downloads":
		return "scholarship-provider/downloads", nil
	case "payments":
		return "scholarship-provider/payments", nil
	default:
		return "", errors.New("invalid upload folder")
	}
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

func (h *Handler) GetDetailedAnalytics(c *gin.Context) {
	providerID := getProviderID(c)

	var filters DetailedAnalyticsFilters
	filters.Province = c.Query("province")
	filters.District = c.Query("district")
	filters.SchoolType = c.Query("school_type")
	filters.ScholarshipStatus = c.Query("scholarship_status")

	analytics, err := h.service.GetDetailedAnalytics(providerID, filters)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch detailed analytics")
		return
	}

	response.Success(c, http.StatusOK, "Detailed analytics data retrieved successfully", analytics)
}

func (h *Handler) UploadImage(c *gin.Context) {
	providerID := getProviderID(c)
	_ = providerID

	folder, err := resolveProviderUploadFolder(c.Query("folder"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "No file provided")
		return
	}

	url, err := utils.SaveUploadedImage(header, folder)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "File uploaded successfully", gin.H{"url": url})
}

func (h *Handler) UploadDocument(c *gin.Context) {
	providerID := getProviderID(c)
	_ = providerID

	folder, err := resolveProviderUploadFolder(c.Query("folder"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "No file provided")
		return
	}

	url, err := utils.SaveUploadedDocument(header, folder)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "File uploaded successfully", gin.H{"url": url})
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

	// Debug: log incoming update request context to help diagnose 404s
	log.Printf("scholarshipprovider: UpdateScholarship request - providerID=%d, scholarshipID=%d, remote=%s", providerID, id, c.ClientIP())

	var req CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	scholarship, err := h.service.UpdateScholarship(providerID, uint(id), req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "Scholarship not found")
			return
		}
		log.Printf("scholarshipprovider: UpdateScholarship handler error: %v", err)
		response.Error(c, http.StatusInternalServerError, "Failed to update scholarship")
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

func (h *Handler) ExportApplications(c *gin.Context) {
	providerID := getProviderID(c)

	applications, _, err := h.service.GetApplications(providerID, 1, 10000, "", "")
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch applications for export")
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Applications"
	index, _ := f.NewSheet(sheetName)
	f.SetActiveSheet(index)

	headers := []string{"ID", "Scholarship", "Full Name", "Email", "Phone", "Status", "Score", "Province", "District", "Created At"}
	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	for i, a := range applications {
		row := i + 2
		scholarshipTitle := ""
		if a.Scholarship.ID != 0 {
			scholarshipTitle = a.Scholarship.Title
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), a.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), scholarshipTitle)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), a.FirstName+" "+a.LastName)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), a.Email)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), a.PhoneNumber)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), a.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), a.EvaluationScore)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), a.Province)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), a.District)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), a.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	f.DeleteSheet("Sheet1") // Remove default sheet

	c.Header("Content-Disposition", "attachment; filename=applications.xlsx")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Transfer-Encoding", "binary")
	
	if err := f.Write(c.Writer); err != nil {
		log.Printf("Failed to write excel file: %v", err)
	}
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

type ApprovePaymentRequest struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}

func (h *Handler) ApproveApplicationPayment(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application ID")
		return
	}

	var req ApprovePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err = h.service.ApproveApplicationPayment(providerID, uint(id), req.Approve, req.Reason)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if req.Approve {
		response.Success(c, http.StatusOK, "Payment approved and admit card sent", nil)
	} else {
		response.Success(c, http.StatusOK, "Payment rejected", nil)
	}
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
	userID, _ := c.Get("user_id")
	role, _ := c.Get("user_role")

	if role == "scholarship_provider_subuser" {
		user, err := h.service.GetAccessUser(userID.(uint))
		if err == nil {
			response.Success(c, http.StatusOK, "Profile retrieved successfully", ProfileResponse{
				ID:                 user.ID,
				ProviderName:       user.Name,
				RegistrationNumber: "SUBUSER",
				Email:              user.Email,
				Role:               user.Role,
				IsSubUser:          true,
				Permissions:        user.Permissions,
				ProviderID:         user.ProviderID,
			})
			return
		}
	}

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
		IsSubUser:          false,
		ProviderID:         provider.ID,
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

func (h *Handler) ChangePassword(c *gin.Context) {
	providerID := getProviderID(c)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ChangePassword(providerID, req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Password changed successfully", ChangePasswordResponse{
		Message: "Password changed successfully",
	})
}

func (h *Handler) ChangeEmail(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")
	isSubUser := userRole == "scholarship_provider_subuser"

	var req ChangeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ChangeEmail(userID.(uint), isSubUser, req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Email changed successfully", ChangeEmailResponse{
		Message: "Email changed successfully",
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

func unmarshalJSONB(data []byte) interface{} {
	var v interface{}
	json.Unmarshal(data, &v)
	if v == nil {
		return []interface{}{}
	}
	return v
}

func formatTimeOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func decodeJSONB[T any](data []byte) T {
	var value T
	if len(data) == 0 {
		return value
	}
	_ = json.Unmarshal(data, &value)
	return value
}

func toScholarshipResponse(s *ProviderScholarship) ScholarshipResponse {
	return ScholarshipResponse{
		ID:                       s.ID,
		CreatedAt:                s.CreatedAt,
		UpdatedAt:                s.UpdatedAt,
		ProviderID:               s.ProviderID,
		Title:                    s.Title,
		Provider:                 s.Provider,
		Description:              s.Description,
		ProviderName:             s.ProviderName,
		FundingTypeOther:         s.FundingTypeOther,
		ScholarshipTypeOther:     s.ScholarshipTypeOther,
		EducationLevel:           s.EducationLevel,
		EducationLevelOther:      s.EducationLevelOther,
		Location:                 s.Location,
		Value:                    s.Value,
		Deadline:                 formatTimeOrEmpty(s.Deadline),
		DegreeLevel:              s.DegreeLevel,
		FundingType:              s.FundingType,
		ScholarshipType:          s.ScholarshipType,
		FieldOfStudy:             decodeJSONB[[]string](s.FieldOfStudy),
		Status:                   s.Status,
		ApplicationStartDate:     formatTimeOrEmpty(s.ApplicationStartDate),
		ApplicationEndDate:       formatTimeOrEmpty(s.ApplicationEndDate),
		ApplyLink:                s.ApplyLink,
		Image:                    s.BannerBackgroundImageURL,
		CoverageArea:             s.CoverageArea,
		ContactEmail:             s.ContactEmail,
		PrimaryPhone:             s.PrimaryPhone,
		SecondaryPhone:           s.SecondaryPhone,
		WebsiteUrl:               s.WebsiteUrl,
		OfficeAddress:            s.OfficeAddress,
		MapUrl:                   s.MapUrl,
		AboutParagraph1:          s.AboutParagraph1,
		VideoTutorials:           decodeJSONB[[]VideoTutorial](s.VideoTutorials),
		JourneyTimeline:          decodeJSONB[[]JourneyTimelineItem](s.JourneyTimeline),
		Timeline:                 decodeJSONB[[]TimelineItem](s.Timeline),
		ScholarshipSectionTitle:  s.ScholarshipSectionTitle,
		ScholarshipSubtitle:      s.ScholarshipSubtitle,
		ScholarshipDescription1:  s.ScholarshipDescription1,
		ScholarshipDescription2:  s.ScholarshipDescription2,
		ScholarshipTypes:         decodeJSONB[[]ScholarshipTypeItem](s.ScholarshipTypes),
		ScholarshipTypesNew:      decodeJSONB[[]ScholarshipTypeItem](s.ScholarshipTypesNew),
		SelectionRubric:          decodeJSONB[[]SelectionRubricItem](s.SelectionRubric),
		SelectionRubricNew:       decodeJSONB[[]SelectionRubricItem](s.SelectionRubricNew),
		EligibilitySectionTitle:  s.EligibilitySectionTitle,
		EligibilitySubtitle:      s.EligibilitySubtitle,
		BasicEligibilityCriteria: decodeJSONB[[]string](s.BasicEligibilityCriteria),
		FullyFundedCriteria:      decodeJSONB[[]string](s.FullyFundedCriteria),
		PartiallyFundedCriteria:  decodeJSONB[[]string](s.PartiallyFundedCriteria),
		SelectionProcessSteps:    decodeJSONB[[]SelectionProcessStepItem](s.SelectionProcessSteps),
		RequiredDocuments:        decodeJSONB[[]string](s.RequiredDocuments),
		FAQs:                     decodeJSONB[[]FAQItem](s.FAQs),
		FAQsNew:                  decodeJSONB[[]FAQItem](s.FAQsNew),
		GalleryImages:            decodeJSONB[[]GalleryImageItem](s.GalleryImages),
		GalleryImagesNew:         decodeJSONB[[]GalleryImageItem](s.GalleryImagesNew),
		PartnerGroups:            decodeJSONB[[]PartnerOrganization](s.PartnerGroups),
		ExamCenters:              decodeJSONB[[]ExamCenterItem](s.ExamCenters),
		ExamCentersNew:           decodeJSONB[[]ExamCenterItem](s.ExamCentersNew),
		Downloads:                decodeJSONB[[]DownloadItem](s.Downloads),
		PaymentConfig: func() *PaymentConfig {
			if len(s.PaymentConfig) == 0 {
				return nil
			}
			var paymentConfig PaymentConfig
			if err := json.Unmarshal(s.PaymentConfig, &paymentConfig); err != nil {
				return nil
			}
			return &paymentConfig
		}(),
	}
}

func toApplicationResponse(a *ProviderApplication) ApplicationResponse {
	var paymentResp *PaymentResponse
	if a.Payment != nil {
		paymentResp = &PaymentResponse{
			ID:            a.Payment.ID,
			Method:        a.Payment.Method,
			Amount:        a.Payment.Amount,
			Status:        a.Payment.Status,
			ReceiptURL:    a.Payment.ReceiptURL,
			TransactionID: a.Payment.TransactionID,
		}
		if a.Payment.PaidAt != nil {
			paymentResp.PaidAt = a.Payment.PaidAt.Format(time.RFC3339)
		}
	}

	resp := ApplicationResponse{
		ID:                    a.ID,
		CreatedAt:             a.CreatedAt,
		UpdatedAt:             a.UpdatedAt,
		ScholarshipID:         a.ScholarshipID,
		UserID:                uintValueOrZero(a.UserID),
		FullName:              a.FullName,
		FirstName:             a.FirstName,
		LastName:              a.LastName,
		Email:                 a.Email,
		PhoneNumber:           a.PhoneNumber,
		Gender:                a.Gender,
		Ethnicity:             a.Ethnicity,
		EthnicityOther:        a.EthnicityOther,
		DateOfBirthBS:         a.DateOfBirthBS,
		DateOfBirthAD:         a.DateOfBirthAD,
		Age:                   a.Age,
		PhotoURL:              a.PhotoURL,
		SEEGPA:                a.SEEGPA,
		SchoolName:            a.SchoolName,
		SchoolProvince:        a.SchoolProvince,
		SchoolDistrict:        a.SchoolDistrict,
		SchoolMunicipality:    a.SchoolMunicipality,
		SchoolTole:            a.SchoolTole,
		PermanentProvince:     a.PermanentProvince,
		PermanentDistrict:     a.PermanentDistrict,
		PermanentMunicipality: a.PermanentMunicipality,
		PermanentWard:         a.PermanentWard,
		PermanentTole:         a.PermanentTole,
		TemporaryProvince:     a.TemporaryProvince,
		TemporaryDistrict:     a.TemporaryDistrict,
		TemporaryMunicipality: a.TemporaryMunicipality,
		TemporaryWard:         a.TemporaryWard,
		TemporaryTole:         a.TemporaryTole,
		GuardianName:          a.GuardianName,
		GuardianPhone:         a.GuardianPhone,
		GuardianEmail:         a.GuardianEmail,
		FatherOccupation:      a.FatherOccupation,
		FatherOccupationOther: a.FatherOccupationOther,
		MotherOccupation:      a.MotherOccupation,
		MotherOccupationOther: a.MotherOccupationOther,
		FamilyMonthlyIncome:   a.FamilyMonthlyIncome,
		FamilyMembersCount:    a.FamilyMembersCount,
		Status:                a.Status,
		EvaluationNotes:       a.EvaluationNotes,
		Documents:             a.Documents,
		PersonalStatement:     a.PersonalStatement,
		Province:              a.Province,
		District:              a.District,
		Stream:                a.Stream,
		GPA:                   a.GPA,
		SchoolType:            a.SchoolType,
		ExamCenter:            a.ExamCenter,
		Payment:               paymentResp,
	}

	if a.Scholarship.ID != 0 {
		schResp := toScholarshipResponse(&a.Scholarship)
		resp.Scholarship = &schResp
	}

	return resp
}

func uintValueOrZero(v *uint) uint {
	if v == nil {
		return 0
	}
	return *v
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

func toNewsResponse(n *ProviderNews) NewsResponse {
	var tags interface{}
	if n.Tags != nil {
		json.Unmarshal(n.Tags, &tags)
	}
	return NewsResponse{
		ID:            n.ID,
		CreatedAt:     n.CreatedAt,
		UpdatedAt:     n.UpdatedAt,
		ProviderID:    n.ProviderID,
		Title:         n.Title,
		ShortDesc:     n.ShortDesc,
		Content:       n.Content,
		ImageURL:      n.ImageURL,
		NewsType:      n.NewsType,
		PublishedBy:   n.PublishedBy,
		PublishDate:   n.PublishDate,
		Tags:          tags,
		AllowComments: n.AllowComments,
		Status:        n.Status,
		PublishedAt:   n.PublishedAt,
	}
}

func toEventResponse(e *ProviderEvent) EventResponse {
	var tags interface{}
	if e.Tags != nil {
		json.Unmarshal(e.Tags, &tags)
	}
	return EventResponse{
		ID:                 e.ID,
		CreatedAt:          e.CreatedAt,
		UpdatedAt:          e.UpdatedAt,
		ProviderID:         e.ProviderID,
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
		StartDate:          e.StartDate,
		EndDate:            e.EndDate,
		Location:           e.Location,
		Tags:               tags,
		EnableRegistration: e.EnableRegistration,
		Status:             e.Status,
		Attendees:          e.Attendees,
	}
}

func toBlogResponse(b *ProviderBlog) BlogResponse {
	return BlogResponse{
		ID:          b.ID,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
		ProviderID:  b.ProviderID,
		Title:       b.Title,
		Content:     b.Content,
		ImageURL:    b.ImageURL,
		Author:      b.Author,
		Status:      b.Status,
		PublishedAt: b.PublishedAt,
		Views:       b.Views,
		Likes:       b.Likes,
	}
}

func toCalendarEventResponse(e *ProviderCalendarEvent) CalendarEventResponse {
	return CalendarEventResponse{
		ID:          e.ID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		ProviderID:  e.ProviderID,
		Title:       e.Title,
		Description: e.Description,
		StartDate:   e.StartDate,
		EndDate:     e.EndDate,
		Color:       e.Color,
		IsAllDay:    e.IsAllDay,
	}
}

func toResultResponse(r *ProviderResult) ResultResponse {
	return ResultResponse{
		ID:            r.ID,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
		ProviderID:    r.ProviderID,
		ScholarshipID: r.ScholarshipID,
		Title:         r.Title,
		Status:        r.Status,
		PublishedAt:   r.PublishedAt,
		Results:       json.RawMessage(r.Results),
	}
}

func toAccessResponse(a *ProviderAccess) AccessResponse {
	return AccessResponse{
		ID:         a.ID,
		CreatedAt:  a.CreatedAt,
		UpdatedAt:  a.UpdatedAt,
		ProviderID: a.ProviderID,
		Email:      a.Email,
		Role:       a.Role,
		Status:     a.Status,
	}
}

func (h *Handler) CreateNews(c *gin.Context) {
	providerID := getProviderID(c)

	var req CreateNewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	news, err := h.service.CreateNews(providerID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create news")
		return
	}

	response.Success(c, http.StatusCreated, "News created successfully", toNewsResponse(news))
}

func (h *Handler) GetNews(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	news, total, err := h.service.GetNews(providerID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch news")
		return
	}

	responses := make([]NewsResponse, len(news))
	for i, n := range news {
		responses[i] = toNewsResponse(&n)
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", NewsListResponse{
		News: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetNewsByID(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid news ID")
		return
	}

	news, err := h.service.GetNewsByID(providerID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "News not found")
		return
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", toNewsResponse(news))
}

func (h *Handler) UpdateNews(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid news ID")
		return
	}

	var req CreateNewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	news, err := h.service.UpdateNews(providerID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "News not found")
		return
	}

	response.Success(c, http.StatusOK, "News updated successfully", toNewsResponse(news))
}

func (h *Handler) DeleteNews(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid news ID")
		return
	}

	if err := h.service.DeleteNews(providerID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "News not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete news")
		return
	}

	response.Success(c, http.StatusOK, "News deleted successfully", nil)
}

func (h *Handler) CreateEvent(c *gin.Context) {
	providerID := getProviderID(c)

	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.CreateEvent(providerID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create event")
		return
	}

	response.Success(c, http.StatusCreated, "Event created successfully", toEventResponse(event))
}

func (h *Handler) GetEvents(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	events, total, err := h.service.GetEvents(providerID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch events")
		return
	}

	responses := make([]EventResponse, len(events))
	for i, e := range events {
		responses[i] = toEventResponse(&e)
	}

	response.Success(c, http.StatusOK, "Events retrieved successfully", EventListResponse{
		Events: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetEventByID(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	event, err := h.service.GetEventByID(providerID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	response.Success(c, http.StatusOK, "Event retrieved successfully", toEventResponse(event))
}

func (h *Handler) UpdateEvent(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.UpdateEvent(providerID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	response.Success(c, http.StatusOK, "Event updated successfully", toEventResponse(event))
}

func (h *Handler) DeleteEvent(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	if err := h.service.DeleteEvent(providerID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Event not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete event")
		return
	}

	response.Success(c, http.StatusOK, "Event deleted successfully", nil)
}

func (h *Handler) CreateBlog(c *gin.Context) {
	providerID := getProviderID(c)

	var req CreateBlogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	blog, err := h.service.CreateBlog(providerID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create blog")
		return
	}

	response.Success(c, http.StatusCreated, "Blog created successfully", toBlogResponse(blog))
}

func (h *Handler) GetBlogs(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	blogs, total, err := h.service.GetBlogs(providerID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch blogs")
		return
	}

	responses := make([]BlogResponse, len(blogs))
	for i, b := range blogs {
		responses[i] = toBlogResponse(&b)
	}

	response.Success(c, http.StatusOK, "Blogs retrieved successfully", BlogListResponse{
		Blogs: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetBlogByID(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	blog, err := h.service.GetBlogByID(providerID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Blog not found")
		return
	}

	response.Success(c, http.StatusOK, "Blog retrieved successfully", toBlogResponse(blog))
}

func (h *Handler) UpdateBlog(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	var req CreateBlogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	blog, err := h.service.UpdateBlog(providerID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Blog not found")
		return
	}

	response.Success(c, http.StatusOK, "Blog updated successfully", toBlogResponse(blog))
}

func (h *Handler) DeleteBlog(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	if err := h.service.DeleteBlog(providerID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Blog not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete blog")
		return
	}

	response.Success(c, http.StatusOK, "Blog deleted successfully", nil)
}

func (h *Handler) CreateCalendarEvent(c *gin.Context) {
	providerID := getProviderID(c)

	var req CreateCalendarEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.CreateCalendarEvent(providerID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create calendar event")
		return
	}

	response.Success(c, http.StatusCreated, "Calendar event created successfully", toCalendarEventResponse(event))
}

func (h *Handler) GetCalendarEvents(c *gin.Context) {
	providerID := getProviderID(c)

	events, err := h.service.GetCalendarEvents(providerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch calendar events")
		return
	}

	responses := make([]CalendarEventResponse, len(events))
	for i, e := range events {
		responses[i] = toCalendarEventResponse(&e)
	}

	response.Success(c, http.StatusOK, "Calendar events retrieved successfully", responses)
}

func (h *Handler) GetCalendarEventByID(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid calendar event ID")
		return
	}

	event, err := h.service.GetCalendarEventByID(providerID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Calendar event not found")
		return
	}

	response.Success(c, http.StatusOK, "Calendar event retrieved successfully", toCalendarEventResponse(event))
}

func (h *Handler) UpdateCalendarEvent(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid calendar event ID")
		return
	}

	var req CreateCalendarEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.UpdateCalendarEvent(providerID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Calendar event not found")
		return
	}

	response.Success(c, http.StatusOK, "Calendar event updated successfully", toCalendarEventResponse(event))
}

func (h *Handler) DeleteCalendarEvent(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid calendar event ID")
		return
	}

	if err := h.service.DeleteCalendarEvent(providerID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Calendar event not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete calendar event")
		return
	}

	response.Success(c, http.StatusOK, "Calendar event deleted successfully", nil)
}

func (h *Handler) CreateResult(c *gin.Context) {
	providerID := getProviderID(c)

	var req CreateResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.CreateResult(providerID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create result")
		return
	}

	response.Success(c, http.StatusCreated, "Result created successfully", toResultResponse(result))
}

func (h *Handler) GetResults(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	results, total, err := h.service.GetResults(providerID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch results")
		return
	}

	responses := make([]ResultResponse, len(results))
	for i, r := range results {
		responses[i] = toResultResponse(&r)
	}

	response.Success(c, http.StatusOK, "Results retrieved successfully", ResultListResponse{
		Results: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetResultByID(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid result ID")
		return
	}

	result, err := h.service.GetResultByID(providerID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Result not found")
		return
	}

	response.Success(c, http.StatusOK, "Result retrieved successfully", toResultResponse(result))
}

func (h *Handler) UpdateResult(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid result ID")
		return
	}

	var req CreateResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.UpdateResult(providerID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Result not found")
		return
	}

	response.Success(c, http.StatusOK, "Result updated successfully", toResultResponse(result))
}

func (h *Handler) DeleteResult(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid result ID")
		return
	}

	if err := h.service.DeleteResult(providerID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Result not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete result")
		return
	}

	response.Success(c, http.StatusOK, "Result deleted successfully", nil)
}

func (h *Handler) CreateAccess(c *gin.Context) {
	providerID := getProviderID(c)

	var req CreateAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	access, err := h.service.CreateAccess(providerID, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create access")
		return
	}

	response.Success(c, http.StatusCreated, "Access created successfully", toAccessResponse(access))
}

func (h *Handler) GetAccess(c *gin.Context) {
	providerID := getProviderID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	access, total, err := h.service.GetAccess(providerID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch access")
		return
	}

	responses := make([]AccessResponse, len(access))
	for i, a := range access {
		responses[i] = toAccessResponse(&a)
	}

	response.Success(c, http.StatusOK, "Access retrieved successfully", AccessListResponse{
		Access: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetAccessByID(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid access ID")
		return
	}

	access, err := h.service.GetAccessByID(providerID, uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Access not found")
		return
	}

	response.Success(c, http.StatusOK, "Access retrieved successfully", toAccessResponse(access))
}

func (h *Handler) UpdateAccess(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid access ID")
		return
	}

	var req CreateAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	access, err := h.service.UpdateAccess(providerID, uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Access not found")
		return
	}

	response.Success(c, http.StatusOK, "Access updated successfully", toAccessResponse(access))
}

func (h *Handler) DeleteAccess(c *gin.Context) {
	providerID := getProviderID(c)
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid access ID")
		return
	}

	if err := h.service.DeleteAccess(providerID, uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Access not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete access")
		return
	}

	response.Success(c, http.StatusOK, "Access deleted successfully", nil)
}

func (h *Handler) GetPublicNews(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))

	news, total, err := h.service.GetPublishedNews(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch news")
		return
	}

	responses := make([]NewsResponse, len(news))
	for i, n := range news {
		responses[i] = toNewsResponse(&n)
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", NewsListResponse{
		News: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetPublicNewsByID(c *gin.Context) {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid news ID")
		return
	}

	news, err := h.service.GetPublishedNewsByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "News not found")
		return
	}

	response.Success(c, http.StatusOK, "News retrieved successfully", toNewsResponse(news))
}

func (h *Handler) GetPublicEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))

	events, total, err := h.service.GetPublishedEvents(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch events")
		return
	}

	responses := make([]EventResponse, len(events))
	for i, e := range events {
		responses[i] = toEventResponse(&e)
	}

	response.Success(c, http.StatusOK, "Events retrieved successfully", EventListResponse{
		Events: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetPublicBlogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))

	blogs, total, err := h.service.GetPublishedBlogs(page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch blogs")
		return
	}

	responses := make([]BlogResponse, len(blogs))
	for i, b := range blogs {
		responses[i] = toBlogResponse(&b)
	}

	response.Success(c, http.StatusOK, "Blogs retrieved successfully", BlogListResponse{
		Blogs: responses,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

func (h *Handler) GetPublicBlogByID(c *gin.Context) {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	blog, err := h.service.GetPublishedBlogByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Blog not found")
		return
	}

	response.Success(c, http.StatusOK, "Blog retrieved successfully", toBlogResponse(blog))
}

func (h *Handler) GetPublicEventByID(c *gin.Context) {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	event, err := h.service.GetPublishedEventByID(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	response.Success(c, http.StatusOK, "Event retrieved successfully", toEventResponse(event))
}

func generateToken(userID, providerID uint) (string, error) {
	claims := jwt.MapClaims{
		"user_id":     userID,
		"provider_id": providerID,
		"exp":         time.Now().Add(time.Hour * 24).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte("your-secret-key"))
}

func (h *Handler) CreateAccessUser(c *gin.Context) {
	providerIDVal, ok := c.Get("provider_id")
	var providerID uint
	if ok {
		providerID = providerIDVal.(uint)
	} else {
		providerID = getProviderID(c)
	}

	var req CreateAccessUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.service.CreateAccessUser(req, providerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Access user created successfully", user)
}

func (h *Handler) GetAccessUsers(c *gin.Context) {
	providerIDVal, ok := c.Get("provider_id")
	var providerID uint
	if ok {
		providerID = providerIDVal.(uint)
	} else {
		providerID = getProviderID(c)
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	users, err := h.service.GetAccessUsers(providerID, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch access users")
		return
	}

	response.Success(c, http.StatusOK, "Access users retrieved successfully", users)
}

func (h *Handler) GetAccessUser(c *gin.Context) {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid access user ID")
		return
	}

	user, err := h.service.GetAccessUser(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Access user not found")
		return
	}

	response.Success(c, http.StatusOK, "Access user retrieved successfully", user)
}

func (h *Handler) UpdateAccessUser(c *gin.Context) {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid access user ID")
		return
	}

	var req UpdateAccessUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.service.UpdateAccessUser(uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Access user not found")
		return
	}

	response.Success(c, http.StatusOK, "Access user updated successfully", user)
}

func (h *Handler) DeleteAccessUser(c *gin.Context) {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid access user ID")
		return
	}

	providerID := getProviderID(c)

	if err := h.service.DeleteAccessUser(uint(id), providerID); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete access user")
		return
	}

	response.Success(c, http.StatusOK, "Access user deleted successfully", nil)
}

func (h *Handler) UpdatePermissions(c *gin.Context) {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid access user ID")
		return
	}

	var req struct {
		Permissions []string `json:"permissions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdatePermissions(uint(id), req.Permissions); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Permissions updated successfully", nil)
}

func (h *Handler) ResetAccessUserPassword(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid access user ID")
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ResetAccessUserPassword(uint(id), req.NewPassword); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to reset password")
		return
	}

	response.Success(c, http.StatusOK, "Password reset successfully", nil)
}

func (h *Handler) LoginAccessUser(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	providerIDVal, ok := c.Get("provider_id")
	var providerID uint
	if !ok {
		providerID = getProviderID(c)
	} else {
		providerID = providerIDVal.(uint)
	}

	user, err := h.service.LoginAccessUser(req.Email, req.Password, providerID)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	token, err := generateToken(user.ID, user.ProviderID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	var permissions []string
	if user.Permissions != nil {
		permissions = user.Permissions
	}

	response.Success(c, http.StatusOK, "Login successful", gin.H{
		"user":        user,
		"token":       token,
		"permissions": permissions,
	})
}

func (h *Handler) LoginAccessUserPublic(c *gin.Context) {
	log.Printf("[HANDLER] LoginAccessUserPublic called")
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[HANDLER] Bind error: %v", err)
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[HANDLER] Calling service with email=%s", req.Email)
	user, err := h.service.LoginAccessUser(req.Email, req.Password, 0)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	token, err := generateToken(user.ID, user.ProviderID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	var permissions []string
	if user.Permissions != nil {
		permissions = user.Permissions
	}

	response.Success(c, http.StatusOK, "Login successful", gin.H{
		"user":        user,
		"token":       token,
		"permissions": permissions,
	})
}

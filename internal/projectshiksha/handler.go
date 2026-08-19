package projectshiksha

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"studsphere/backend/internal/shared/response"
)

// Handler handles HTTP requests for Project Shiksha
type Handler struct {
	service *Service
}

// NewHandler creates a new handler instance
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateApplication handles creating a new application
func (h *Handler) CreateApplication(c *gin.Context) {
	var req CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	app, err := h.service.CreateApplication(req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Application created successfully", toApplicationResponse(app))
}

// GetApplication handles retrieving a single application
func (h *Handler) GetApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application ID")
		return
	}

	app, err := h.service.GetApplication(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Application not found")
		return
	}

	response.Success(c, http.StatusOK, "Application retrieved successfully", toApplicationResponse(app))
}

// GetApplicationByRollNumber handles retrieving an application by roll number
func (h *Handler) GetApplicationByRollNumber(c *gin.Context) {
	rollNumber := c.Param("roll_number")
	
	app, err := h.service.GetApplicationByRollNumber(rollNumber)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Application not found")
		return
	}

	response.Success(c, http.StatusOK, "Application retrieved successfully", toApplicationResponse(app))
}

// ListApplications handles listing applications with pagination
func (h *Handler) ListApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.Query("status")
	paymentStatus := c.Query("payment_status")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	apps, total, err := h.service.ListApplications(page, limit, status, paymentStatus)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve applications")
		return
	}

	var appResponses []ApplicationResponse
	for _, app := range apps {
		appResponses = append(appResponses, toApplicationResponse(&app))
	}

	response.Success(c, http.StatusOK, "Applications retrieved successfully", ApplicationListResponse{
		Applications: appResponses,
		Total:        total,
		Page:         page,
		Limit:        limit,
	})
}

// UpdateApplicationStatus handles updating application status
func (h *Handler) UpdateApplicationStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application ID")
		return
	}

	var req UpdateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateApplicationStatus(uint(id), req.Status); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Application status updated successfully", nil)
}

// ProcessPayment handles payment processing
func (h *Handler) ProcessPayment(c *gin.Context) {
	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	payment, err := h.service.ProcessPayment(req.ApplicationID, req.PaymentMethod, req.Amount, req.TransactionID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Payment processed successfully", toPaymentResponse(payment))
}

// InitiateEsewaPayment handles eSewa payment initiation
func (h *Handler) InitiateEsewaPayment(c *gin.Context) {
	var req EsewaInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.InitiateEsewaPayment(req.ApplicationID, req.Amount)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "eSewa payment initiated", result)
}

// VerifyEsewaPayment handles eSewa payment verification
func (h *Handler) VerifyEsewaPayment(c *gin.Context) {
	var req EsewaVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	payment, err := h.service.VerifyEsewaPayment(req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "eSewa payment verified successfully", toPaymentResponse(payment))
}

// VerifyPayment handles payment verification (admin only)
func (h *Handler) VerifyPayment(c *gin.Context) {
	var req VerifyPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Get admin user ID from context (set by auth middleware)
	verifiedBy, _ := c.Get("userID")
	var verifiedByUint uint
	if id, ok := verifiedBy.(uint); ok {
		verifiedByUint = id
	}

	verified := req.Status == "verified"
	if err := h.service.VerifyBankPayment(req.PaymentID, verified, verifiedByUint); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Payment verified successfully", nil)
}

// GetStats handles retrieving statistics
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve statistics")
		return
	}

	response.Success(c, http.StatusOK, "Statistics retrieved successfully", stats)
}

// GetAdmitCard handles retrieving admit card
func (h *Handler) GetAdmitCard(c *gin.Context) {
	rollNumber := c.Param("roll_number")
	
	admitCard, err := h.service.GetAdmitCard(rollNumber)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Admit card retrieved successfully", admitCard)
}

// DeleteApplication handles deleting an application
func (h *Handler) DeleteApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid application ID")
		return
	}

	if err := h.service.DeleteApplication(uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete application")
		return
	}

	response.Success(c, http.StatusOK, "Application deleted successfully", nil)
}

// Helper functions to convert models to responses

func toApplicationResponse(app *ShikshaApplication) ApplicationResponse {
	return ApplicationResponse{
		ID:            app.ID,
		FullName:      app.FullName,
		Phone:         app.Phone,
		Email:         app.Email,
		PhotoURL:      app.PhotoURL,
		SEESchoolType: app.SEESchoolType,
		SchoolName:    app.SchoolName,
		PermDistrict:  app.PermDistrict,
		GuardianName:  app.GuardianName,
		GuardianPhone: app.GuardianPhone,
		FamilyIncome:  app.FamilyIncome,
		PaymentStatus: app.PaymentStatus,
		PaymentMethod: app.PaymentMethod,
		RollNumber:    app.RollNumber,
		Status:        app.Status,
		CreatedAt:     app.CreatedAt.Format(time.RFC3339),
	}
}

func toPaymentResponse(payment *ShikshaPayment) PaymentResponse {
	return PaymentResponse{
		ID:            payment.ID,
		ApplicationID: payment.ApplicationID,
		PaymentMethod: payment.PaymentMethod,
		Amount:        payment.Amount,
		Status:        payment.Status,
		TransactionID: payment.TransactionID,
		ScreenshotURL: payment.ScreenshotURL,
		CreatedAt:     payment.CreatedAt.Format(time.RFC3339),
	}
}

package counselling

import (
	"net/http"
	"studsphere/backend/internal/shared/response"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateCounsellingBooking(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateCounsellingBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	booking, err := h.service.CreateBooking(userID.(uint), req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "session_mode must be either 'online' or 'in_person'" ||
			err.Error() == "you already booked this date and time slot" {
			status = http.StatusBadRequest
			if err.Error() == "you already booked this date and time slot" {
				status = http.StatusConflict
			}
		}
		response.Error(c, status, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Counselling session booked successfully", toBookingResponse(booking))
}

func (h *Handler) GetMyCounsellingBookings(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	bookings, err := h.service.GetMyBookings(userID.(uint))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch counselling bookings")
		return
	}

	responses := make([]CounsellingBookingResponse, len(bookings))
	for i, b := range bookings {
		responses[i] = toBookingResponse(&b)
	}

	response.Success(c, http.StatusOK, "Counselling bookings retrieved successfully", gin.H{
		"bookings": responses,
	})
}

func toBookingResponse(b *CounsellingBooking) CounsellingBookingResponse {
	return CounsellingBookingResponse{
		ID:               b.ID,
		College:          b.College,
		ProgramLevel:     b.ProgramLevel,
		InterestedCourse: b.InterestedCourse,
		SessionMode:      b.SessionMode,
		SessionDate:      b.SessionDate,
		SessionTime:      b.SessionTime,
		StudentName:      b.StudentName,
		StudentPhone:     b.StudentPhone,
		StudentEmail:     b.StudentEmail,
		StudentNotes:     b.StudentNotes,
		Status:           b.Status,
		CreatedAt:        b.CreatedAt.Format(time.RFC3339),
	}
}

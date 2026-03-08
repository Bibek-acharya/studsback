package handlers

import (
	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
)

func CreateCounsellingBooking(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.ErrorResponse(c, 401, "Unauthorized")
		return
	}

	var req models.CreateCounsellingBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	if req.SessionMode != "online" && req.SessionMode != "in_person" {
		utils.ErrorResponse(c, 400, "session_mode must be either 'online' or 'in_person'")
		return
	}

	var existing models.CounsellingBooking
	err := config.GetDB().Where(
		"user_id = ? AND session_date = ? AND session_time = ?",
		userID,
		req.SessionDate,
		req.SessionTime,
	).First(&existing).Error
	if err == nil {
		utils.ErrorResponse(c, 409, "You already booked this date and time slot")
		return
	}

	booking := models.CounsellingBooking{
		UserID:           userID.(uint),
		College:          req.College,
		ProgramLevel:     req.ProgramLevel,
		InterestedCourse: req.InterestedCourse,
		SessionMode:      req.SessionMode,
		SessionDate:      req.SessionDate,
		SessionTime:      req.SessionTime,
		StudentName:      req.StudentName,
		StudentPhone:     req.StudentPhone,
		StudentEmail:     req.StudentEmail,
		StudentNotes:     req.StudentNotes,
		Status:           "pending",
	}

	if err := config.GetDB().Create(&booking).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create counselling booking")
		return
	}

	utils.SuccessResponse(c, 201, "Counselling session booked successfully", booking)
}

func GetMyCounsellingBookings(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.ErrorResponse(c, 401, "Unauthorized")
		return
	}

	var bookings []models.CounsellingBooking
	if err := config.GetDB().Where("user_id = ?", userID).Order("created_at DESC").Find(&bookings).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch counselling bookings")
		return
	}

	utils.SuccessResponse(c, 200, "Counselling bookings retrieved successfully", gin.H{
		"bookings": bookings,
	})
}
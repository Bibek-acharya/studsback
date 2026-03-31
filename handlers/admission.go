package handlers

import (
	"strconv"
	"time"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
)

// CreateAdmission creates a new admission application
func CreateAdmission(c *gin.Context) {
	var req models.CreateAdmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var college models.College
	if err := config.GetDB().First(&college, req.CollegeID).Error; err != nil {
		utils.ErrorResponse(c, 404, "College not found")
		return
	}

	admission := models.Admission{
		CollegeID:         req.CollegeID,
		ProgramName:       req.ProgramName,
		ProgramLevel:      req.ProgramLevel,
		StudentName:       req.StudentName,
		StudentEmail:      req.StudentEmail,
		StudentPhone:      req.StudentPhone,
		LastQualification: req.LastQualification,
		Institution:       req.Institution,
		GPA:               req.GPA,
		EntranceScore:     req.EntranceScore,
		Statement:         req.Statement,
		Gender:            req.Gender,
		Address:           req.Address,
		City:              req.City,
		Status:            "pending",
	}

	if req.DateOfBirth != "" {
		if dob, err := time.Parse("2006-01-02", req.DateOfBirth); err == nil {
			admission.DateOfBirth = &dob
		}
	}

	userId, exists := c.Get("user_id")
	if exists {
		if uid, ok := userId.(uint); ok {
			admission.UserID = &uid
		}
	}

	if err := config.GetDB().Create(&admission).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create admission application")
		return
	}

	utils.SuccessResponse(c, 201, "Admission application created successfully", admission)
}

// GetMyAdmissions retrieves all admissions for the authenticated user
func GetMyAdmissions(c *gin.Context) {
	userId, _ := c.Get("user_id")
	uid := userId.(uint)

	var admissions []models.Admission
	if err := config.GetDB().Where("user_id = ?", uid).Preload("College").Order("created_at DESC").Find(&admissions).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch admissions")
		return
	}

	utils.SuccessResponse(c, 200, "Admissions retrieved successfully", admissions)
}

// GetAdmission retrieves a single admission by ID
func GetAdmission(c *gin.Context) {
	admissionID := c.Param("id")
	parsedID, err := strconv.ParseUint(admissionID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid admission ID")
		return
	}

	var admission models.Admission
	if err := config.GetDB().Preload("College").Preload("User").First(&admission, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Admission not found")
		return
	}

	userId, exists := c.Get("user_id")
	if exists {
		if uid := userId.(uint); admission.UserID != nil && *admission.UserID != uid {
			role, _ := c.Get("user_role")
			roleStr := ""
			if r, ok := role.(string); ok {
				roleStr = r
			}
			if roleStr != "admin" && roleStr != "super_admin" {
				utils.ErrorResponse(c, 403, "You can only view your own admissions")
				return
			}
		}
	}

	utils.SuccessResponse(c, 200, "Admission retrieved successfully", admission)
}

// UpdateAdmission updates an existing admission application
func UpdateAdmission(c *gin.Context) {
	admissionID := c.Param("id")
	parsedID, err := strconv.ParseUint(admissionID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid admission ID")
		return
	}

	var admission models.Admission
	if err := config.GetDB().First(&admission, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Admission not found")
		return
	}

	userId, exists := c.Get("user_id")
	if exists {
		if uid := userId.(uint); admission.UserID != nil && *admission.UserID != uid {
			utils.ErrorResponse(c, 403, "You can only update your own admissions")
			return
		}
	}

	var req models.UpdateAdmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	if req.ProgramName != nil {
		admission.ProgramName = *req.ProgramName
	}
	if req.ProgramLevel != nil {
		admission.ProgramLevel = *req.ProgramLevel
	}
	if req.StudentName != nil {
		admission.StudentName = *req.StudentName
	}
	if req.StudentEmail != nil {
		admission.StudentEmail = *req.StudentEmail
	}
	if req.StudentPhone != nil {
		admission.StudentPhone = *req.StudentPhone
	}
	if req.DateOfBirth != nil {
		if dob, err := time.Parse("2006-01-02", *req.DateOfBirth); err == nil {
			admission.DateOfBirth = &dob
		}
	}
	if req.Gender != nil {
		admission.Gender = *req.Gender
	}
	if req.Address != nil {
		admission.Address = *req.Address
	}
	if req.City != nil {
		admission.City = *req.City
	}
	if req.LastQualification != nil {
		admission.LastQualification = *req.LastQualification
	}
	if req.Institution != nil {
		admission.Institution = *req.Institution
	}
	if req.GPA != nil {
		admission.GPA = *req.GPA
	}
	if req.EntranceScore != nil {
		admission.EntranceScore = *req.EntranceScore
	}
	if req.Statement != nil {
		admission.Statement = *req.Statement
	}

	if err := config.GetDB().Save(&admission).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update admission application")
		return
	}

	utils.SuccessResponse(c, 200, "Admission application updated successfully", admission)
}

// DeleteAdmission deletes an admission application
func DeleteAdmission(c *gin.Context) {
	admissionID := c.Param("id")
	parsedID, err := strconv.ParseUint(admissionID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid admission ID")
		return
	}

	var admission models.Admission
	if err := config.GetDB().First(&admission, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Admission not found")
		return
	}

	userId, exists := c.Get("user_id")
	if exists {
		if uid := userId.(uint); admission.UserID != nil && *admission.UserID != uid {
			utils.ErrorResponse(c, 403, "You can only delete your own admissions")
			return
		}
	}

	if err := config.GetDB().Unscoped().Delete(&admission).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to delete admission application")
		return
	}

	utils.SuccessResponse(c, 200, "Admission application deleted successfully", nil)
}

// UpdateAdmissionStatus updates the status of an admission application (admin only)
func UpdateAdmissionStatus(c *gin.Context) {
	admissionID := c.Param("id")
	parsedID, err := strconv.ParseUint(admissionID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid admission ID")
		return
	}

	var admission models.Admission
	if err := config.GetDB().First(&admission, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Admission not found")
		return
	}

	var req models.UpdateAdmissionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	userId, _ := c.Get("user_id")
	uid := userId.(uint)
	now := time.Now()
	admission.Status = req.Status
	admission.Notes = req.Notes
	admission.ReviewedBy = &uid
	admission.ReviewedAt = &now

	if err := config.GetDB().Save(&admission).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update admission status")
		return
	}

	utils.SuccessResponse(c, 200, "Admission status updated successfully", admission)
}

// GetCollegeAdmissions retrieves all admissions for a specific college (admin only)
func GetCollegeAdmissions(c *gin.Context) {
	collegeID := c.Param("collegeId")

	var admissions []models.Admission
	query := config.GetDB().Where("college_id = ?", collegeID).Preload("User")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&admissions).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch college admissions")
		return
	}

	utils.SuccessResponse(c, 200, "College admissions retrieved successfully", admissions)
}

// GetAllAdmissions retrieves all admissions (admin only)
func GetAllAdmissions(c *gin.Context) {
	var admissions []models.Admission
	query := config.GetDB().Preload("College").Preload("User")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if collegeID := c.Query("college_id"); collegeID != "" {
		query = query.Where("college_id = ?", collegeID)
	}

	if err := query.Order("created_at DESC").Find(&admissions).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch admissions")
		return
	}

	utils.SuccessResponse(c, 200, "Admissions retrieved successfully", admissions)
}

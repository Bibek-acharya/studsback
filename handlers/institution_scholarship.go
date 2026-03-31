package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
)

// InstitutionGetScholarships retrieves all scholarships for the authenticated institution
func InstitutionGetScholarships(c *gin.Context) {
	instId, _ := c.Get("user_id")
	instRole, _ := c.Get("user_role")

	if instRole.(string) != "institution" {
		utils.ErrorResponse(c, 403, "Only institutions can access this endpoint")
		return
	}

	var college models.College
	if err := config.GetDB().Where("university_id = ?", instId).First(&college).Error; err != nil {
		utils.ErrorResponse(c, 404, "No college found for this institution")
		return
	}

	var scholarships []models.Scholarship
	if err := config.GetDB().Where("location ILIKE ?", "%"+college.Name+"%").Find(&scholarships).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch scholarships")
		return
	}

	utils.SuccessResponse(c, 200, "Scholarships retrieved successfully", scholarships)
}

// InstitutionCreateScholarship creates a new scholarship for the institution
func InstitutionCreateScholarship(c *gin.Context) {
	instId, _ := c.Get("user_id")
	instRole, _ := c.Get("user_role")

	if instRole.(string) != "institution" {
		utils.ErrorResponse(c, 403, "Only institutions can access this endpoint")
		return
	}

	var college models.College
	if err := config.GetDB().Where("university_id = ?", instId).First(&college).Error; err != nil {
		utils.ErrorResponse(c, 404, "No college found for this institution")
		return
	}

	var req models.CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var deadline time.Time
	if req.Deadline != "" {
		var err error
		deadline, err = time.Parse("2006-01-02", req.Deadline)
		if err != nil {
			utils.ErrorResponse(c, 400, "Invalid deadline format (expected YYYY-MM-DD)")
			return
		}
	}

	fieldOfStudy, _ := json.Marshal(req.FieldOfStudy)

	scholarship := models.Scholarship{
		Title:           req.Title,
		Provider:        college.Name,
		Location:        req.Location,
		Value:           req.Value,
		Deadline:        deadline,
		DegreeLevel:     req.DegreeLevel,
		FundingType:     req.FundingType,
		ScholarshipType: req.ScholarshipType,
		Description:     req.Description,
		ImageURL:        req.ImageURL,
		FieldOfStudy:    fieldOfStudy,
	}

	if err := config.GetDB().Create(&scholarship).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create scholarship")
		return
	}

	utils.SuccessResponse(c, 201, "Scholarship created successfully", scholarship)
}

// InstitutionUpdateScholarship updates an existing scholarship for the institution
func InstitutionUpdateScholarship(c *gin.Context) {
	instId, _ := c.Get("user_id")
	instRole, _ := c.Get("user_role")

	if instRole.(string) != "institution" {
		utils.ErrorResponse(c, 403, "Only institutions can access this endpoint")
		return
	}

	scholarshipID := c.Param("id")
	parsedID, err := strconv.ParseUint(scholarshipID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid scholarship ID")
		return
	}

	var college models.College
	if err := config.GetDB().Where("university_id = ?", instId).First(&college).Error; err != nil {
		utils.ErrorResponse(c, 404, "No college found for this institution")
		return
	}

	var scholarship models.Scholarship
	if err := config.GetDB().First(&scholarship, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Scholarship not found")
		return
	}

	if scholarship.Provider != college.Name {
		utils.ErrorResponse(c, 403, "You can only update your own scholarships")
		return
	}

	var req models.CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	if req.Title != "" {
		scholarship.Title = req.Title
	}
	if req.Provider != "" {
		scholarship.Provider = req.Provider
	}
	if req.Location != "" {
		scholarship.Location = req.Location
	}
	if req.Value != "" {
		scholarship.Value = req.Value
	}
	if req.Deadline != "" {
		if deadline, err := time.Parse("2006-01-02", req.Deadline); err == nil {
			scholarship.Deadline = deadline
		}
	}
	if req.DegreeLevel != "" {
		scholarship.DegreeLevel = req.DegreeLevel
	}
	if req.FundingType != "" {
		scholarship.FundingType = req.FundingType
	}
	if req.ScholarshipType != "" {
		scholarship.ScholarshipType = req.ScholarshipType
	}
	if req.Description != "" {
		scholarship.Description = req.Description
	}
	if req.ImageURL != "" {
		scholarship.ImageURL = req.ImageURL
	}
	if len(req.FieldOfStudy) > 0 {
		if data, err := json.Marshal(req.FieldOfStudy); err == nil {
			scholarship.FieldOfStudy = data
		}
	}

	if err := config.GetDB().Save(&scholarship).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update scholarship")
		return
	}

	utils.SuccessResponse(c, 200, "Scholarship updated successfully", scholarship)
}

// InstitutionDeleteScholarship deletes a scholarship for the institution
func InstitutionDeleteScholarship(c *gin.Context) {
	instId, _ := c.Get("user_id")
	instRole, _ := c.Get("user_role")

	if instRole.(string) != "institution" {
		utils.ErrorResponse(c, 403, "Only institutions can access this endpoint")
		return
	}

	scholarshipID := c.Param("id")
	parsedID, err := strconv.ParseUint(scholarshipID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid scholarship ID")
		return
	}

	var college models.College
	if err := config.GetDB().Where("university_id = ?", instId).First(&college).Error; err != nil {
		utils.ErrorResponse(c, 404, "No college found for this institution")
		return
	}

	var scholarship models.Scholarship
	if err := config.GetDB().First(&scholarship, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Scholarship not found")
		return
	}

	if scholarship.Provider != college.Name {
		utils.ErrorResponse(c, 403, "You can only delete your own scholarships")
		return
	}

	if err := config.GetDB().Unscoped().Delete(&scholarship).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to delete scholarship")
		return
	}

	utils.SuccessResponse(c, 200, "Scholarship deleted successfully", nil)
}

// InstitutionGetAdmissions retrieves all admissions for the institution's college
func InstitutionGetAdmissions(c *gin.Context) {
	instId, _ := c.Get("user_id")
	instRole, _ := c.Get("user_role")

	if instRole.(string) != "institution" {
		utils.ErrorResponse(c, 403, "Only institutions can access this endpoint")
		return
	}

	var college models.College
	if err := config.GetDB().Where("university_id = ?", instId).First(&college).Error; err != nil {
		utils.ErrorResponse(c, 404, "No college found for this institution")
		return
	}

	var admissions []models.Admission
	query := config.GetDB().Where("college_id = ?", college.ID).Preload("User")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&admissions).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch admissions")
		return
	}

	utils.SuccessResponse(c, 200, "Admissions retrieved successfully", admissions)
}

// InstitutionUpdateAdmissionStatus updates the status of an admission for the institution
func InstitutionUpdateAdmissionStatus(c *gin.Context) {
	instId, _ := c.Get("user_id")
	instRole, _ := c.Get("user_role")

	if instRole.(string) != "institution" {
		utils.ErrorResponse(c, 403, "Only institutions can access this endpoint")
		return
	}

	admissionID := c.Param("id")
	parsedID, err := strconv.ParseUint(admissionID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid admission ID")
		return
	}

	var college models.College
	if err := config.GetDB().Where("university_id = ?", instId).First(&college).Error; err != nil {
		utils.ErrorResponse(c, 404, "No college found for this institution")
		return
	}

	var admission models.Admission
	if err := config.GetDB().First(&admission, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Admission not found")
		return
	}

	if admission.CollegeID != college.ID {
		utils.ErrorResponse(c, 403, "You can only manage admissions for your own college")
		return
	}

	var req models.UpdateAdmissionStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	now := time.Now()
	instUID := instId.(uint)
	admission.Status = req.Status
	admission.Notes = req.Notes
	admission.ReviewedBy = &instUID
	admission.ReviewedAt = &now

	if err := config.GetDB().Save(&admission).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update admission status")
		return
	}

	utils.SuccessResponse(c, 200, "Admission status updated successfully", admission)
}

// InstitutionGetScholarshipApplications retrieves all scholarship applications for the institution's scholarships
func InstitutionGetScholarshipApplications(c *gin.Context) {
	instId, _ := c.Get("user_id")
	instRole, _ := c.Get("user_role")

	if instRole.(string) != "institution" {
		utils.ErrorResponse(c, 403, "Only institutions can access this endpoint")
		return
	}

	var college models.College
	if err := config.GetDB().Where("university_id = ?", instId).First(&college).Error; err != nil {
		utils.ErrorResponse(c, 404, "No college found for this institution")
		return
	}

	var scholarships []models.Scholarship
	if err := config.GetDB().Where("provider = ?", college.Name).Find(&scholarships).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch scholarships")
		return
	}

	scholarshipIDs := make([]uint, len(scholarships))
	for i, s := range scholarships {
		scholarshipIDs[i] = s.ID
	}

	var applications []models.ScholarshipApplication
	query := config.GetDB().Where("scholarship_id IN ?", scholarshipIDs).Preload("Scholarship").Preload("User")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&applications).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch applications")
		return
	}

	utils.SuccessResponse(c, 200, "Scholarship applications retrieved successfully", applications)
}

// InstitutionUpdateScholarshipApplicationStatus updates the status of a scholarship application
func InstitutionUpdateScholarshipApplicationStatus(c *gin.Context) {
	instId, _ := c.Get("user_id")
	instRole, _ := c.Get("user_role")

	if instRole.(string) != "institution" {
		utils.ErrorResponse(c, 403, "Only institutions can access this endpoint")
		return
	}

	applicationID := c.Param("id")
	parsedID, err := strconv.ParseUint(applicationID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid application ID")
		return
	}

	var college models.College
	if err := config.GetDB().Where("university_id = ?", instId).First(&college).Error; err != nil {
		utils.ErrorResponse(c, 404, "No college found for this institution")
		return
	}

	var application models.ScholarshipApplication
	if err := config.GetDB().Preload("Scholarship").First(&application, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Application not found")
		return
	}

	if application.Scholarship.Provider != college.Name {
		utils.ErrorResponse(c, 403, "You can only manage applications for your own scholarships")
		return
	}

	var req struct {
		Status string `json:"status" binding:"required,oneof=pending under_review approved rejected shortlisted"`
		Notes  string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	application.Status = req.Status

	if err := config.GetDB().Save(&application).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update application status")
		return
	}

	utils.SuccessResponse(c, 200, "Application status updated successfully", application)
}

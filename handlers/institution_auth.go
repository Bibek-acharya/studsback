package handlers

import (
	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InstitutionRegister(c *gin.Context) {
	var req models.InstitutionRegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var existingByEmail models.InstitutionUser
	if err := config.GetDB().Where("email = ?", req.Email).First(&existingByEmail).Error; err == nil {
		utils.ErrorResponse(c, 409, "Institution account with this email already exists")
		return
	}

	var existingByReg models.InstitutionUser
	if err := config.GetDB().Where("registration_number = ?", req.RegistrationNumber).First(&existingByReg).Error; err == nil {
		utils.ErrorResponse(c, 409, "Institution with this registration number already exists")
		return
	}

	institutionUser := models.InstitutionUser{
		InstitutionName:    req.InstitutionName,
		RegistrationNumber: req.RegistrationNumber,
		Email:              req.Email,
		Role:               "institution",
	}

	if err := institutionUser.HashPassword(req.Password); err != nil {
		utils.ErrorResponse(c, 500, "Failed to hash password")
		return
	}

	if err := config.GetDB().Create(&institutionUser).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create institution account")
		return
	}

	token, err := utils.GenerateToken(institutionUser.ID, institutionUser.Email, institutionUser.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	utils.SuccessResponse(c, 201, "Institution account created successfully", gin.H{
		"user":  institutionUser,
		"token": token,
	})
}

func InstitutionLogin(c *gin.Context) {
	var req models.InstitutionLoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var institutionUser models.InstitutionUser
	if err := config.GetDB().Where("email = ?", req.Email).First(&institutionUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.ErrorResponse(c, 401, "Invalid email or password")
			return
		}
		utils.ErrorResponse(c, 500, "Failed to fetch institution account")
		return
	}

	if err := institutionUser.CheckPassword(req.Password); err != nil {
		utils.ErrorResponse(c, 401, "Invalid email or password")
		return
	}

	token, err := utils.GenerateToken(institutionUser.ID, institutionUser.Email, institutionUser.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	utils.SuccessResponse(c, 200, "Institution login successful", gin.H{
		"user":  institutionUser,
		"token": token,
	})
}

package handlers

import (
	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ScholarshipProviderRegister(c *gin.Context) {
	var req models.ScholarshipProviderRegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var existingByEmail models.ScholarshipProviderUser
	if err := config.GetDB().Where("email = ?", req.Email).First(&existingByEmail).Error; err == nil {
		utils.ErrorResponse(c, 409, "Scholarship provider account with this email already exists")
		return
	}

	var existingByReg models.ScholarshipProviderUser
	if err := config.GetDB().Where("registration_number = ?", req.RegistrationNumber).First(&existingByReg).Error; err == nil {
		utils.ErrorResponse(c, 409, "Scholarship provider with this registration number already exists")
		return
	}

	providerUser := models.ScholarshipProviderUser{
		ProviderName:       req.ProviderName,
		RegistrationNumber: req.RegistrationNumber,
		Email:              req.Email,
		Role:               "scholarship_provider",
	}

	if err := providerUser.HashPassword(req.Password); err != nil {
		utils.ErrorResponse(c, 500, "Failed to hash password")
		return
	}

	if err := config.GetDB().Create(&providerUser).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create scholarship provider account")
		return
	}

	token, err := utils.GenerateToken(providerUser.ID, providerUser.Email, providerUser.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	utils.SuccessResponse(c, 201, "Scholarship provider account created successfully", gin.H{
		"user":  providerUser,
		"token": token,
	})
}

func ScholarshipProviderLogin(c *gin.Context) {
	var req models.ScholarshipProviderLoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var providerUser models.ScholarshipProviderUser
	if err := config.GetDB().Where("email = ?", req.Email).First(&providerUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.ErrorResponse(c, 401, "Invalid email or password")
			return
		}
		utils.ErrorResponse(c, 500, "Failed to fetch scholarship provider account")
		return
	}

	if err := providerUser.CheckPassword(req.Password); err != nil {
		utils.ErrorResponse(c, 401, "Invalid email or password")
		return
	}

	token, err := utils.GenerateToken(providerUser.ID, providerUser.Email, providerUser.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	utils.SuccessResponse(c, 200, "Scholarship provider login successful", gin.H{
		"user":  providerUser,
		"token": token,
	})
}

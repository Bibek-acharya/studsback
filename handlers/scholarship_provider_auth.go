package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

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

func ScholarshipProviderGoogleLogin(c *gin.Context) {
	initGoogleConfig()
	state := "scholarship-provider-oauth-state"
	url := googleOauthConfig.AuthCodeURL(state)
	c.Redirect(302, url)
}

func ScholarshipProviderGoogleCallback(c *gin.Context) {
	initGoogleConfig()
	code := c.Query("code")
	if code == "" {
		utils.ErrorResponse(c, 400, "Code not found")
		return
	}

	token, err := googleOauthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to exchange token: "+err.Error())
		return
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to get user info: "+err.Error())
		return
	}
	defer resp.Body.Close()

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to read user info: "+err.Error())
		return
	}

	var googleUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(contents, &googleUser); err != nil {
		utils.ErrorResponse(c, 500, "Failed to parse user info: "+err.Error())
		return
	}

	var providerUser models.ScholarshipProviderUser
	db := config.GetDB()
	result := db.Where("email = ? OR google_id = ?", googleUser.Email, googleUser.ID).First(&providerUser)

	if result.Error != nil {
		providerUser = models.ScholarshipProviderUser{
			ProviderName:       googleUser.Name,
			RegistrationNumber: "GOOGLE-" + googleUser.ID,
			Email:              googleUser.Email,
			GoogleID:           &googleUser.ID,
			Role:               "scholarship_provider",
		}
		if err := db.Create(&providerUser).Error; err != nil {
			utils.ErrorResponse(c, 500, "Failed to create scholarship provider account: "+err.Error())
			return
		}
	} else {
		if providerUser.GoogleID == nil || *providerUser.GoogleID == "" {
			providerUser.GoogleID = &googleUser.ID
			db.Save(&providerUser)
		}
	}

	jwtToken, err := utils.GenerateToken(providerUser.ID, providerUser.Email, providerUser.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	userData, _ := json.Marshal(gin.H{
		"id":            providerUser.ID,
		"provider_name": providerUser.ProviderName,
		"email":         providerUser.Email,
		"role":          providerUser.Role,
	})
	redirectURL := fmt.Sprintf("%s/scholarship-providers/auth/google-callback?token=%s&user=%s",
		config.AppConfig.FrontendURL,
		jwtToken,
		url.QueryEscape(string(userData)),
	)

	c.Redirect(302, redirectURL)
}

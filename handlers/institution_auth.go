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

func InstitutionGoogleLogin(c *gin.Context) {
	initGoogleConfig()
	state := "institution-oauth-state"
	url := googleOauthConfig.AuthCodeURL(state)
	c.Redirect(302, url)
}

func InstitutionGoogleCallback(c *gin.Context) {
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

	var instUser models.InstitutionUser
	db := config.GetDB()
	result := db.Where("email = ? OR google_id = ?", googleUser.Email, googleUser.ID).First(&instUser)

	if result.Error != nil {
		instUser = models.InstitutionUser{
			InstitutionName:    googleUser.Name,
			RegistrationNumber: "GOOGLE-" + googleUser.ID,
			Email:              googleUser.Email,
			GoogleID:           &googleUser.ID,
			Role:               "institution",
		}
		if err := db.Create(&instUser).Error; err != nil {
			utils.ErrorResponse(c, 500, "Failed to create institution account: "+err.Error())
			return
		}
	} else {
		if instUser.GoogleID == nil || *instUser.GoogleID == "" {
			instUser.GoogleID = &googleUser.ID
			db.Save(&instUser)
		}
	}

	jwtToken, err := utils.GenerateToken(instUser.ID, instUser.Email, instUser.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	userData, _ := json.Marshal(gin.H{
		"id":               instUser.ID,
		"institution_name": instUser.InstitutionName,
		"email":            instUser.Email,
		"role":             instUser.Role,
	})
	redirectURL := fmt.Sprintf("%s/institutions/auth/google-callback?token=%s&user=%s",
		config.AppConfig.FrontendURL,
		jwtToken,
		url.QueryEscape(string(userData)),
	)

	c.Redirect(302, redirectURL)
}

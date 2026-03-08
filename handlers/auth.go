package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var googleOauthConfig *oauth2.Config

func initGoogleConfig() {
	googleOauthConfig = &oauth2.Config{
		RedirectURL:  config.AppConfig.GoogleRedirectURL,
		ClientID:     config.AppConfig.GoogleClientID,
		ClientSecret: config.AppConfig.GoogleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

// GoogleLogin redirects the user to Google's OAuth2 login page
func GoogleLogin(c *gin.Context) {
	initGoogleConfig()
	url := googleOauthConfig.AuthCodeURL("state-token")
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallback handles the callback from Google OAuth2
func GoogleCallback(c *gin.Context) {
	initGoogleConfig()
	code := c.Query("code")
	if code == "" {
		utils.ErrorResponse(c, 400, "Code not found")
		return
	}

	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to exchange token: "+err.Error())
		return
	}

	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to get user info: "+err.Error())
		return
	}
	defer response.Body.Close()

	contents, err := io.ReadAll(response.Body)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to read user info: "+err.Error())
		return
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
		Locale        string `json:"locale"`
	}

	if err := json.Unmarshal(contents, &googleUser); err != nil {
		utils.ErrorResponse(c, 500, "Failed to parse user info: "+err.Error())
		return
	}

	var user models.User
	db := config.GetDB()
	result := db.Where("email = ? OR google_id = ?", googleUser.Email, googleUser.ID).First(&user)

	if result.Error != nil {
		user = models.User{
			Email:     googleUser.Email,
			FirstName: googleUser.GivenName,
			LastName:  googleUser.FamilyName,
			GoogleID:  &googleUser.ID,
			Role:      "student",
		}
		if err := db.Create(&user).Error; err != nil {
			utils.ErrorResponse(c, 500, "Failed to create user: "+err.Error())
			return
		}
	} else {
		if user.GoogleID == nil || *user.GoogleID == "" {
			user.GoogleID = &googleUser.ID
			db.Save(&user)
		}
	}

	jwtToken, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	// Redirect back to frontend with token and essential user data
	userData, _ := json.Marshal(user)
	redirectURL := fmt.Sprintf("%s/auth/google-callback?token=%s&user=%s",
		config.AppConfig.FrontendURL,
		jwtToken,
		url.QueryEscape(string(userData)),
	)

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// Register handles user registration — creates the user and sends an OTP for email verification
func Register(c *gin.Context) {
	var req models.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := config.GetDB().Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		utils.ErrorResponse(c, 409, "User with this email already exists")
		return
	}

	// Create new user
	user := models.User{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      req.Role,
	}

	if user.Role == "" {
		user.Role = "student"
	}

	if req.EducationLevel != "" {
		now := time.Now()
		user.Preferences = &models.Preferences{
			Role: user.Role,
			Preferences: map[string]interface{}{
				"education_level": req.EducationLevel,
			},
			CompletedAt: &now,
		}
	}

	if err := user.HashPassword(req.Password); err != nil {
		utils.ErrorResponse(c, 500, "Failed to hash password")
		return
	}

	// Generate and send OTP
	otp, err := utils.GenerateOTP()
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate OTP")
		return
	}

	// Store user data in OTP store instead of creating in DB
	utils.StoreOTP(req.Email, otp, user)

	// Send OTP email (non-blocking: log error but don't fail registration)
	if emailErr := utils.SendOTPEmail(req.Email, otp); emailErr != nil {
		log.Printf("Warning: failed to send OTP email to %s: %v", req.Email, emailErr)
		// In development, log the OTP so it can be used even if email fails
		log.Printf("DEV OTP for %s: %s", req.Email, otp)
	}

	utils.SuccessResponse(c, 201, "Registration successful. Please verify your email with the OTP sent.", gin.H{
		"email":        user.Email,
		"requires_otp": true,
	})
}

// SendOTP sends a fresh OTP to an existing user's email (e.g. resend)
func SendOTP(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	otp, err := utils.GenerateOTP()
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate OTP")
		return
	}

	// Preserve existing data if any
	data := utils.GetOTPData(req.Email)
	utils.StoreOTP(req.Email, otp, data)

	if emailErr := utils.SendOTPEmail(req.Email, otp); emailErr != nil {
		log.Printf("Warning: failed to send OTP email to %s: %v", req.Email, emailErr)
		log.Printf("DEV OTP for %s: %s", req.Email, otp)
	}

	utils.SuccessResponse(c, 200, "OTP sent successfully", nil)
}

// VerifyOTP verifies the OTP and returns a JWT token on success
func VerifyOTP(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		OTP   string `json:"otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	valid, data := utils.VerifyOTP(req.Email, req.OTP)
	if !valid {
		utils.ErrorResponse(c, 400, "Invalid or expired OTP")
		return
	}

	// No data means something is wrong (maybe tried to verify without registering)
	if data == nil {
		utils.ErrorResponse(c, 400, "Registration data not found. Please register again.")
		return
	}

	// Recover user data and create in DB
	user, ok := data.(models.User)
	if !ok {
		utils.ErrorResponse(c, 500, "Failed to recover user data")
		return
	}

	if err := config.GetDB().Create(&user).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create user")
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	utils.SuccessResponse(c, 200, "Email verified successfully", gin.H{
		"user":  user,
		"token": token,
	})
}

// Login handles user login
func Login(c *gin.Context) {
	var req models.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var user models.User
	if err := config.GetDB().Where("email = ?", req.Email).First(&user).Error; err != nil {
		utils.ErrorResponse(c, 401, "Invalid email or password")
		return
	}

	if err := user.CheckPassword(req.Password); err != nil {
		utils.ErrorResponse(c, 401, "Invalid email or password")
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	utils.SuccessResponse(c, 200, "Login successful", gin.H{
		"user":  user,
		"token": token,
	})
}

// GetProfile returns the authenticated user's profile
func GetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var user models.User
	if err := config.GetDB().First(&user, userID).Error; err != nil {
		utils.ErrorResponse(c, 404, "User not found")
		return
	}

	utils.SuccessResponse(c, 200, "Profile retrieved successfully", user)
}

// SavePreferences saves user preferences from onboarding
func SavePreferences(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.ErrorResponse(c, 401, "Unauthorized")
		return
	}

	var req models.SavePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var user models.User
	if err := config.GetDB().First(&user, userID).Error; err != nil {
		utils.ErrorResponse(c, 404, "User not found")
		return
	}

	now := time.Now()
	prefs := &models.Preferences{
		Role:           req.PreferenceRole,
		PreferenceFlow: req.PreferenceFlow,
		Preferences:    req.Preferences,
		CompletedAt:    &now,
	}

	if err := config.GetDB().Model(&user).Update("preferences", prefs).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to save preferences")
		return
	}

	config.GetDB().First(&user, userID)

	utils.SuccessResponse(c, 200, "Preferences saved successfully", gin.H{
		"user": user,
	})
}

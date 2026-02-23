package handlers

import (
	"time"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
)

// Register handles user registration
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

	// Set default role if not provided
	if user.Role == "" {
		user.Role = "student"
	}

	// Hash password
	if err := user.HashPassword(req.Password); err != nil {
		utils.ErrorResponse(c, 500, "Failed to hash password")
		return
	}

	// Save user to database
	if err := config.GetDB().Create(&user).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create user")
		return
	}

	// Generate JWT token
	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		utils.ErrorResponse(c, 500, "Failed to generate token")
		return
	}

	utils.SuccessResponse(c, 201, "User registered successfully", gin.H{
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

	// Find user by email
	var user models.User
	if err := config.GetDB().Where("email = ?", req.Email).First(&user).Error; err != nil {
		utils.ErrorResponse(c, 401, "Invalid email or password")
		return
	}

	// Check password
	if err := user.CheckPassword(req.Password); err != nil {
		utils.ErrorResponse(c, 401, "Invalid email or password")
		return
	}

	// Generate JWT token
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

	// Find user
	var user models.User
	if err := config.GetDB().First(&user, userID).Error; err != nil {
		utils.ErrorResponse(c, 404, "User not found")
		return
	}

	// Create preferences object
	now := time.Now()
	prefs := &models.Preferences{
		Role:           req.PreferenceRole,
		PreferenceFlow: req.PreferenceFlow,
		Preferences:    req.Preferences,
		CompletedAt:    &now,
	}

	// Update user preferences in database
	if err := config.GetDB().Model(&user).Update("preferences", prefs).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to save preferences")
		return
	}

	// Fetch updated user to return fresh data
	config.GetDB().First(&user, userID)

	utils.SuccessResponse(c, 200, "Preferences saved successfully", gin.H{
		"user": user,
	})
}

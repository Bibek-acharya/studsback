package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.Register(req)
	if err != nil {
		response.Error(c, 409, err.Error())
		return
	}

	response.Success(c, 201, "Registration successful. Please verify your email with the OTP sent.", result)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.Login(req)
	if err != nil {
		response.Error(c, 401, err.Error())
		return
	}

	response.Success(c, 200, "Login successful", result)
}

func (h *Handler) SendOTP(c *gin.Context) {
	var req SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	otpType := req.Type
	if otpType == "" {
		otpType = "verification" // default
	}

	// For password_reset, check if email exists
	// For verification (registration), allow without checking
	if err := h.service.SendOTP(req.Email, otpType); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "OTP sent successfully", nil)
}

func (h *Handler) VerifyOTP(c *gin.Context) {
	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.VerifyOTP(req.Email, req.OTP)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 200, "Email verified successfully", result)
}

func (h *Handler) GoogleLogin(c *gin.Context) {
	googleConfig := &oauth2.Config{
		RedirectURL:  config.AppConfig.GoogleRedirectURL,
		ClientID:     config.AppConfig.GoogleClientID,
		ClientSecret: config.AppConfig.GoogleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	url := googleConfig.AuthCodeURL("state-token")
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *Handler) GoogleCallback(c *gin.Context) {
	googleConfig := &oauth2.Config{
		RedirectURL:  config.AppConfig.GoogleRedirectURL,
		ClientID:     config.AppConfig.GoogleClientID,
		ClientSecret: config.AppConfig.GoogleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	code := c.Query("code")
	if code == "" {
		response.Error(c, 400, "Code not found")
		return
	}

	token, err := googleConfig.Exchange(context.Background(), code)
	if err != nil {
		response.Error(c, 500, "Failed to exchange token: "+err.Error())
		return
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		response.Error(c, 500, "Failed to get user info: "+err.Error())
		return
	}
	defer resp.Body.Close()

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		response.Error(c, 500, "Failed to read user info: "+err.Error())
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
		response.Error(c, 500, "Failed to parse user info: "+err.Error())
		return
	}

	jwtToken, err := h.service.GoogleLoginOrRegister(googleUser.ID, googleUser.Email, googleUser.GivenName, googleUser.FamilyName)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	redirectURL := fmt.Sprintf("%s/auth/google-callback?token=%s",
		config.AppConfig.FrontendURL,
		jwtToken,
	)

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (h *Handler) GetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	result, err := h.service.GetProfile(userID.(uint))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Profile retrieved successfully", result)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.UpdateProfile(userID.(uint), req)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Profile updated successfully", result)
}

func (h *Handler) SavePreferences(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, 401, "Unauthorized")
		return
	}

	var req SavePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.SavePreferences(userID.(uint), req)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Preferences saved successfully", result)
}

func (h *Handler) InstitutionRegister(c *gin.Context) {
	var req InstitutionRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.InstitutionRegister(req)
	if err != nil {
		response.Error(c, 409, err.Error())
		return
	}

	response.Success(c, 201, "Institution account created successfully", result)
}

func (h *Handler) InstitutionLogin(c *gin.Context) {
	var req InstitutionLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.InstitutionLogin(req)
	if err != nil {
		response.Error(c, 401, err.Error())
		return
	}

	response.Success(c, 200, "Institution login successful", result)
}

func (h *Handler) InstitutionGoogleLogin(c *gin.Context) {
	googleConfig := &oauth2.Config{
		RedirectURL:  config.AppConfig.GoogleRedirectURL,
		ClientID:     config.AppConfig.GoogleClientID,
		ClientSecret: config.AppConfig.GoogleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	state := "institution-oauth-state"
	url := googleConfig.AuthCodeURL(state)
	c.Redirect(302, url)
}

func (h *Handler) InstitutionGoogleCallback(c *gin.Context) {
	googleConfig := &oauth2.Config{
		RedirectURL:  config.AppConfig.GoogleRedirectURL,
		ClientID:     config.AppConfig.GoogleClientID,
		ClientSecret: config.AppConfig.GoogleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	code := c.Query("code")
	if code == "" {
		response.Error(c, 400, "Code not found")
		return
	}

	token, err := googleConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		response.Error(c, 500, "Failed to exchange token: "+err.Error())
		return
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		response.Error(c, 500, "Failed to get user info: "+err.Error())
		return
	}
	defer resp.Body.Close()

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		response.Error(c, 500, "Failed to read user info: "+err.Error())
		return
	}

	var googleUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(contents, &googleUser); err != nil {
		response.Error(c, 500, "Failed to parse user info: "+err.Error())
		return
	}

	instUser, jwtToken, err := h.service.InstitutionGoogleLoginOrRegister(googleUser.ID, googleUser.Email, googleUser.Name)
	if err != nil {
		response.Error(c, 500, err.Error())
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

func (h *Handler) ScholarshipProviderRegister(c *gin.Context) {
	var req ScholarshipProviderRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.ScholarshipProviderRegister(req)
	if err != nil {
		response.Error(c, 409, err.Error())
		return
	}

	response.Success(c, 201, "Scholarship provider account created successfully", result)
}

func (h *Handler) ScholarshipProviderLogin(c *gin.Context) {
	var req ScholarshipProviderLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.ScholarshipProviderLogin(req)
	if err != nil {
		response.Error(c, 401, err.Error())
		return
	}

	response.Success(c, 200, "Scholarship provider login successful", result)
}

func (h *Handler) ScholarshipProviderGoogleLogin(c *gin.Context) {
	googleConfig := &oauth2.Config{
		RedirectURL:  config.AppConfig.GoogleRedirectURL,
		ClientID:     config.AppConfig.GoogleClientID,
		ClientSecret: config.AppConfig.GoogleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	state := "scholarship-provider-oauth-state"
	url := googleConfig.AuthCodeURL(state)
	c.Redirect(302, url)
}

func (h *Handler) ScholarshipProviderGoogleCallback(c *gin.Context) {
	googleConfig := &oauth2.Config{
		RedirectURL:  config.AppConfig.GoogleRedirectURL,
		ClientID:     config.AppConfig.GoogleClientID,
		ClientSecret: config.AppConfig.GoogleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	code := c.Query("code")
	if code == "" {
		response.Error(c, 400, "Code not found")
		return
	}

	token, err := googleConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		response.Error(c, 500, "Failed to exchange token: "+err.Error())
		return
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		response.Error(c, 500, "Failed to get user info: "+err.Error())
		return
	}
	defer resp.Body.Close()

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		response.Error(c, 500, "Failed to read user info: "+err.Error())
		return
	}

	var googleUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(contents, &googleUser); err != nil {
		response.Error(c, 500, "Failed to parse user info: "+err.Error())
		return
	}

	providerUser, jwtToken, err := h.service.ScholarshipProviderGoogleLoginOrRegister(googleUser.ID, googleUser.Email, googleUser.Name)
	if err != nil {
		response.Error(c, 500, err.Error())
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

func (h *Handler) SuperadminRegister(c *gin.Context) {
	var req SuperadminRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.SuperadminRegister(req)
	if err != nil {
		response.Error(c, 403, err.Error())
		return
	}

	response.Success(c, 201, "Superadmin account created successfully", result)
}

func (h *Handler) SuperadminLogin(c *gin.Context) {
	var req SuperadminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.SuperadminLogin(req)
	if err != nil {
		response.Error(c, 401, err.Error())
		return
	}

	response.Success(c, 200, "Superadmin login successful", result)
}

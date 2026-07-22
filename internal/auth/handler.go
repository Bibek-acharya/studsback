package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/scholarshipprovider"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/middleware"
	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuthStateStore stores OAuth state with redirect URLs
var spHandler *scholarshipprovider.Handler
var instService *institution.Service

func SetScholarshipProviderHandler(h *scholarshipprovider.Handler) {
	spHandler = h
}

func SetInstitutionService(s *institution.Service) {
	instService = s
}

type OAuthStateStore struct {
	states map[string]struct {
		redirectURL string
		expires     time.Time
	}
	mu sync.RWMutex
}

var stateStore = &OAuthStateStore{
	states: make(map[string]struct {
		redirectURL string
		expires     time.Time
	}),
}

// generateOAuthState creates a secure state token with embedded redirect URL
func generateOAuthState(redirectURL string) (string, error) {
	// Generate random bytes for state
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	state := base64.URLEncoding.EncodeToString(bytes)

	// Store state with redirect URL (expires in 10 minutes)
	stateStore.mu.Lock()
	stateStore.states[state] = struct {
		redirectURL string
		expires     time.Time
	}{
		redirectURL: redirectURL,
		expires:     time.Now().Add(10 * time.Minute),
	}
	stateStore.mu.Unlock()

	return state, nil
}

// validateOAuthState validates the state and returns the redirect URL
func validateOAuthState(state string) (string, bool) {
	stateStore.mu.RLock()
	defer stateStore.mu.RUnlock()

	if val, exists := stateStore.states[state]; exists {
		if time.Now().Before(val.expires) {
			return val.redirectURL, true
		}
		// Clean up expired state
		delete(stateStore.states, state)
	}
	return "", false
}

// cleanupStates removes expired states periodically
func cleanupStates() {
	for {
		time.Sleep(5 * time.Minute)
		stateStore.mu.Lock()
		for state, val := range stateStore.states {
			if time.Now().After(val.expires) {
				delete(stateStore.states, state)
			}
		}
		stateStore.mu.Unlock()
	}
}

func resolveOAuthRedirectURL(frontendURL, redirectURL string) string {
	frontendBase := strings.TrimRight(frontendURL, "/")
	if frontendBase == "" {
		frontendBase = "http://localhost:5173"
	}
	frontendParsed, err := url.Parse(frontendBase)
	if err != nil {
		return frontendBase + "/"
	}

	if redirectURL == "" {
		return frontendBase + "/"
	}

	if parsedURL, err := url.Parse(redirectURL); err == nil {
		if parsedURL.Path == "/login" {
			return frontendBase + "/"
		}

		if parsedURL.IsAbs() {
			if strings.EqualFold(parsedURL.Host, frontendParsed.Host) {
				return parsedURL.String()
			}
			return frontendBase + "/"
		}
	}

	if !strings.HasPrefix(redirectURL, "/") {
		redirectURL = "/" + redirectURL
	}

	if redirectURL == "/login" {
		return frontendBase + "/"
	}

	return frontendBase + redirectURL
}

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

	middleware.SetAuthCookie(c, result.Token)
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

	if result.Token != "" {
		middleware.SetAuthCookie(c, result.Token)
	}
	response.Success(c, 200, "Email verified successfully", result)
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	err := h.service.ResetPassword(req.Email, req.OTP, req.Password)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 200, "Password reset successfully", nil)
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

	// Get redirect URL from query parameter, default to home
	redirectURL := c.Query("redirect")
	redirectURL = resolveOAuthRedirectURL(config.AppConfig.FrontendURL, redirectURL)

	// Generate secure state with embedded redirect URL
	state, err := generateOAuthState(redirectURL)
	if err != nil {
		response.Error(c, 500, "Failed to generate state")
		return
	}

	// Build auth URL with optional prompt parameter for account selection
	url := googleConfig.AuthCodeURL(state)
	prompt := c.Query("prompt")
	if prompt == "select_account" {
		url += "&prompt=select_account"
	}

	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *Handler) redirectError(c *gin.Context, msg string) {
	frontendURL := strings.TrimRight(config.AppConfig.FrontendURL, "/")
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=%s", frontendURL, url.QueryEscape(msg)))
}

func (h *Handler) GoogleCallback(c *gin.Context) {
	// Validate state parameter (CSRF protection)
	state := c.Query("state")
	redirectURL, valid := validateOAuthState(state)
	if !valid {
		h.redirectError(c, "Invalid or expired state parameter")
		return
	}

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
		h.redirectError(c, "Code not found")
		return
	}

	token, err := googleConfig.Exchange(context.Background(), code)
	if err != nil {
		h.redirectError(c, "Failed to exchange token: "+err.Error())
		return
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		h.redirectError(c, "Failed to get user info: "+err.Error())
		return
	}
	defer resp.Body.Close()

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		h.redirectError(c, "Failed to read user info: "+err.Error())
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
		h.redirectError(c, "Failed to parse user info: "+err.Error())
		return
	}

	jwtToken, err := h.service.GoogleLoginOrRegister(googleUser.ID, googleUser.Email, googleUser.GivenName, googleUser.FamilyName, googleUser.Picture)
	if err != nil {
		h.redirectError(c, err.Error())
		return
	}

	middleware.SetAuthCookie(c, jwtToken)

	// Construct the callback URL with the token to sync with frontend localStorage
	frontendCallback := fmt.Sprintf("%s/login?token=%s",
		strings.TrimRight(config.AppConfig.FrontendURL, "/"),
		jwtToken)

	// Use the redirect URL from state (original destination)
	finalRedirectURL := redirectURL
	if finalRedirectURL == "" {
		finalRedirectURL = "/"
	}

	frontendCallback = fmt.Sprintf("%s&redirect=%s", frontendCallback, url.QueryEscape(finalRedirectURL))

	c.Redirect(http.StatusTemporaryRedirect, frontendCallback)
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

func (h *Handler) UploadProfilePicture(c *gin.Context) {
	userID, _ := c.Get("user_id")

	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, 400, "No file provided")
		return
	}

	if !strings.HasPrefix(file.Header.Get("Content-Type"), "image/") {
		response.Error(c, 400, "Only image files are allowed")
		return
	}

	url, err := utils.SaveUploadedImage(file, "profiles")
	if err != nil {
		response.Error(c, 500, "Failed to upload image: "+err.Error())
		return
	}

	result, err := h.service.UpdateProfile(userID.(uint), UpdateProfileRequest{ImageURL: url})
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Profile picture uploaded successfully", result)
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

	response.Success(c, 201, "Verification code sent to your email", result)
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

	middleware.SetAuthCookie(c, result.Token)
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

	// Get redirect URL from query parameter, default to institution dashboard
	redirectURL := c.Query("redirect")
	if redirectURL == "" {
		redirectURL = "/institutions/dashboard"
	}
	redirectURL = resolveOAuthRedirectURL(config.AppConfig.FrontendURL, redirectURL)

	// Generate secure state with embedded redirect URL
	state, err := generateOAuthState(redirectURL)
	if err != nil {
		response.Error(c, 500, "Failed to generate state")
		return
	}

	// Build auth URL with optional prompt parameter for account selection
	url := googleConfig.AuthCodeURL(state)
	prompt := c.Query("prompt")
	if prompt == "select_account" {
		url += "&prompt=select_account"
	}

	c.Redirect(302, url)
}

func (h *Handler) InstitutionGoogleCallback(c *gin.Context) {
	state := c.Query("state")
	redirectURL, valid := validateOAuthState(state)
	if !valid {
		h.redirectError(c, "Invalid or expired state parameter")
		return
	}

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
		h.redirectError(c, "Code not found")
		return
	}

	token, err := googleConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		h.redirectError(c, "Failed to exchange token: "+err.Error())
		return
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		h.redirectError(c, "Failed to get user info: "+err.Error())
		return
	}
	defer resp.Body.Close()

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		h.redirectError(c, "Failed to read user info: "+err.Error())
		return
	}

	var googleUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(contents, &googleUser); err != nil {
		h.redirectError(c, "Failed to parse user info: "+err.Error())
		return
	}

	_, jwtToken, err := h.service.InstitutionGoogleLoginOrRegister(googleUser.ID, googleUser.Email, googleUser.Name)
	if err != nil {
		h.redirectError(c, err.Error())
		return
	}

	middleware.SetAuthCookie(c, jwtToken)

	frontendCallback := fmt.Sprintf("%s/login?token=%s&role=institution",
		strings.TrimRight(config.AppConfig.FrontendURL, "/"),
		jwtToken)

	if redirectURL == "" {
		redirectURL = "/institutions/dashboard"
	}

	frontendCallback = fmt.Sprintf("%s&redirect=%s", frontendCallback, url.QueryEscape(redirectURL))

	c.Redirect(http.StatusTemporaryRedirect, frontendCallback)
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

	response.Success(c, 201, "Verification code sent to your email", result)
}

func (h *Handler) ScholarshipProviderLogin(c *gin.Context) {
	var req ScholarshipProviderLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.ScholarshipProviderLogin(req)
	if err != nil {
		if strings.Contains(err.Error(), "record not found") || strings.Contains(err.Error(), "Invalid email or password") {
			if spHandler != nil {
				user, spErr := spHandler.GetService().LoginAccessUser(req.Email, req.Password, 0)
				if spErr == nil {
					token, tokenErr := utils.GenerateToken(user.ID, user.Email, "scholarship_provider_subuser", user.ProviderID)
					if tokenErr == nil {
						middleware.SetAuthCookie(c, token)

						spHandler.GetService().CreateNotification(
							user.ProviderID,
							"New Login",
							fmt.Sprintf("Access user %s logged in.", user.Name),
							"system",
							"assign-access",
						)

						response.Success(c, 200, "Login successful", gin.H{
							"user": gin.H{
								"id":          user.ID,
								"email":       user.Email,
								"first_name":  user.Name,
								"last_name":   "",
								"role":        "scholarship_provider_subuser",
								"provider_id": user.ProviderID,
								"permissions": user.Permissions,
								"is_sub_user": true,
							},
							"token": token,
						})
						return
					}
				}
			}
		}
		response.Error(c, 401, err.Error())
		return
	}

	middleware.SetAuthCookie(c, result.Token)

	if spHandler != nil {
		if providerUser, ok := result.User.(*ScholarshipProviderUser); ok {
			spHandler.GetService().CreateNotification(
				providerUser.ID,
				"New Login",
				"You have successfully logged in.",
				"system",
				"sec-dashboard",
			)
		} else if providerUser, ok := result.User.(ScholarshipProviderUser); ok {
			spHandler.GetService().CreateNotification(
				providerUser.ID,
				"New Login",
				"You have successfully logged in.",
				"system",
				"sec-dashboard",
			)
		}
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

	// Get redirect URL from query parameter, default to scholarship provider dashboard
	redirectURL := c.Query("redirect")
	if redirectURL == "" {
		redirectURL = "/scholarship-provider/dashboard"
	}
	redirectURL = resolveOAuthRedirectURL(config.AppConfig.FrontendURL, redirectURL)

	// Generate secure state with embedded redirect URL
	state, err := generateOAuthState(redirectURL)
	if err != nil {
		response.Error(c, 500, "Failed to generate state")
		return
	}

	// Build auth URL with optional prompt parameter for account selection
	url := googleConfig.AuthCodeURL(state)
	prompt := c.Query("prompt")
	if prompt == "select_account" {
		url += "&prompt=select_account"
	}

	c.Redirect(302, url)
}

func (h *Handler) ScholarshipProviderGoogleCallback(c *gin.Context) {
	state := c.Query("state")
	redirectURL, valid := validateOAuthState(state)
	if !valid {
		h.redirectError(c, "Invalid or expired state parameter")
		return
	}

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
		h.redirectError(c, "Code not found")
		return
	}

	token, err := googleConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		h.redirectError(c, "Failed to exchange token: "+err.Error())
		return
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		h.redirectError(c, "Failed to get user info: "+err.Error())
		return
	}
	defer resp.Body.Close()

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		h.redirectError(c, "Failed to read user info: "+err.Error())
		return
	}

	var googleUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(contents, &googleUser); err != nil {
		h.redirectError(c, "Failed to parse user info: "+err.Error())
		return
	}

	providerUser, jwtToken, err := h.service.ScholarshipProviderGoogleLoginOrRegister(googleUser.ID, googleUser.Email, googleUser.Name)
	if err != nil {
		h.redirectError(c, err.Error())
		return
	}

	if spHandler != nil && providerUser != nil {
		spHandler.GetService().CreateNotification(
			providerUser.ID,
			"New Login",
			"You have successfully logged in via Google.",
			"system",
			"sec-dashboard",
		)
	}

	middleware.SetAuthCookie(c, jwtToken)

	frontendCallback := fmt.Sprintf("%s/login?token=%s&role=scholarship_provider",
		strings.TrimRight(config.AppConfig.FrontendURL, "/"),
		jwtToken)

	if redirectURL == "" {
		redirectURL = "/scholarship-provider/dashboard"
	}

	frontendCallback = fmt.Sprintf("%s&redirect=%s", frontendCallback, url.QueryEscape(redirectURL))

	c.Redirect(http.StatusTemporaryRedirect, frontendCallback)
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

	middleware.SetAuthCookie(c, result.Token)
	response.Success(c, 200, "Superadmin login successful", result)
}

func (h *Handler) ListPendingScholarshipProviders(c *gin.Context) {
	providers, err := h.service.ListPendingScholarshipProviders()
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	if providers == nil {
		providers = []ScholarshipProviderUser{}
	}

	response.Success(c, 200, "Pending providers retrieved successfully", providers)
}

func (h *Handler) ListVerifiedScholarshipProviders(c *gin.Context) {
	providers, err := h.service.ListVerifiedScholarshipProviders()
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	if providers == nil {
		providers = []ScholarshipProviderUser{}
	}

	response.Success(c, 200, "Verified providers retrieved successfully", providers)
}

func (h *Handler) ApproveScholarshipProvider(c *gin.Context) {
	var req ScholarshipProviderApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	switch req.Action {
	case "approved":
		if err := h.service.ApproveScholarshipProvider(req.ProviderID); err != nil {
			response.Error(c, 500, err.Error())
			return
		}
		response.Success(c, 200, "Provider approved successfully. Email sent with login credentials.", nil)
	case "rejected":
		if err := h.service.RejectScholarshipProvider(req.ProviderID); err != nil {
			response.Error(c, 500, err.Error())
			return
		}
		response.Success(c, 200, "Provider rejected successfully. Rejection email sent.", nil)
	default:
		response.Error(c, 400, "Invalid action. Use 'approved' or 'rejected'.")
	}
}

func (h *Handler) ListPendingInstitutions(c *gin.Context) {
	reqType := c.DefaultQuery("type", "")
	status := c.DefaultQuery("status", "pending")

	institutions, err := h.service.ListPendingInstitutionsFiltered(status, reqType)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	if institutions == nil {
		institutions = []InstitutionUser{}
	}

	response.Success(c, 200, "Pending institutions retrieved successfully", institutions)
}

func (h *Handler) ListVerifiedInstitutions(c *gin.Context) {
	var filter InstitutionFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.Error(c, 400, "Invalid filter parameters")
		return
	}

	institutions, counts, err := h.service.ListVerifiedInstitutionsFiltered(filter)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	if institutions == nil {
		institutions = []InstitutionUser{}
	}

	response.Success(c, 200, "Verified institutions retrieved successfully", gin.H{
		"institutions": institutions,
		"counts":       counts,
	})
}

func (h *Handler) ListRejectedInstitutions(c *gin.Context) {
	institutions, err := h.service.ListRejectedInstitutions()
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	if institutions == nil {
		institutions = []InstitutionUser{}
	}

	response.Success(c, 200, "Rejected institutions retrieved successfully", institutions)
}

func (h *Handler) CreateInstitution(c *gin.Context) {
	var req CreateInstitutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.CreateInstitution(req)
	if err != nil {
		response.Error(c, 409, err.Error())
		return
	}

	response.Success(c, 201, "Institution created successfully", result)
}

func (h *Handler) GetInstitution(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid institution ID")
		return
	}

	result, err := h.service.GetInstitution(uint(id))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Institution retrieved successfully", result)
}

func (h *Handler) UpdateInstitution(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid institution ID")
		return
	}

	var req UpdateInstitutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	if err := h.service.UpdateInstitution(uint(id), req); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Institution updated successfully", nil)
}

func (h *Handler) ApproveInstitution(c *gin.Context) {
	var req InstitutionApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	switch req.Action {
	case "approved":
		if err := h.service.ApproveInstitution(req.InstitutionID); err != nil {
			response.Error(c, 500, err.Error())
			return
		}
		response.Success(c, 200, "Institution approved successfully. Email sent with login credentials.", nil)
	case "rejected":
		if err := h.service.RejectInstitution(req.InstitutionID); err != nil {
			response.Error(c, 500, err.Error())
			return
		}
		response.Success(c, 200, "Institution rejected successfully. Rejection email sent.", nil)
	default:
		response.Error(c, 400, "Invalid action. Use 'approved' or 'rejected'.")
	}
}

func (h *Handler) UpdateInstitutionProfileAccess(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid institution ID")
		return
	}

	var req UpdateProfileAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	if err := h.service.UpdateInstitutionProfileAccess(uint(id), req.ProfileAccess); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Profile access updated successfully", nil)
}

func (h *Handler) ClaimRegister(c *gin.Context) {
	var req ClaimRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	result, err := h.service.ClaimRegister(req)
	if err != nil {
		response.Error(c, 409, err.Error())
		return
	}

	response.Success(c, 201, "Verification code sent to your email", result)
}

func (h *Handler) RejectClaimRequest(c *gin.Context) {
	var req RejectClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	if err := h.service.RejectClaimRequest(req.ClaimID, req.RejectionReason); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Claim request rejected successfully. Email sent.", nil)
}

func (h *Handler) RecordInstitutionPayment(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid institution ID")
		return
	}

	var req RecordPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	paymentDate, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		response.Error(c, 400, "Invalid payment date format (use YYYY-MM-DD)")
		return
	}

	if err := h.service.RecordInstitutionPayment(uint(id), paymentDate, req.PaidForDays, req.Amount, req.Remarks); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Payment recorded successfully", nil)
}

func (h *Handler) VerifyInstitution(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid institution ID")
		return
	}

	if err := h.service.VerifyInstitution(uint(id)); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Institution verified successfully", nil)
}

func (h *Handler) ToggleInstitutionFeatured(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid institution ID")
		return
	}

	if err := h.service.ToggleInstitutionFeatured(uint(id)); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Featured status toggled successfully", nil)
}

func (h *Handler) ApproveClaimRequest(c *gin.Context) {
	var req struct {
		InstitutionID uint `json:"institution_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	if err := h.service.ApproveClaimRequest(req.InstitutionID); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Claim approved successfully. Email sent with login credentials.", nil)
}

func (h *Handler) SuspendInstitution(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid institution ID")
		return
	}

	if err := h.service.SuspendInstitution(uint(id)); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Institution suspended successfully", nil)
}

func (h *Handler) DeleteInstitution(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid institution ID")
		return
	}

	if err := h.service.DeleteInstitution(uint(id)); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Institution deleted successfully", nil)
}

func (h *Handler) GetMyProfileAccess(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	access, err := h.service.GetInstitutionProfileAccess(id)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Profile access retrieved successfully", access)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ChangePassword(id, req.CurrentPassword, req.NewPassword); err != nil {
		if err.Error() == "invalid credentials" {
			response.Error(c, http.StatusUnauthorized, "Current password is incorrect")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to change password")
		return
	}

	response.Success(c, http.StatusOK, "Password changed successfully", nil)
}

func (h *Handler) GetEducationEntries(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	entries, err := h.service.GetEducationEntries(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve education entries")
		return
	}

	response.Success(c, http.StatusOK, "Education entries retrieved successfully", entries)
}

func (h *Handler) CreateEducationEntry(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	var req EducationEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := h.service.CreateEducationEntry(id, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create education entry")
		return
	}

	response.Success(c, http.StatusCreated, "Education entry created successfully", entry)
}

func (h *Handler) UpdateEducationEntry(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	entryID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid education entry ID")
		return
	}

	var req EducationEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := h.service.UpdateEducationEntry(uint(entryID), id, req)
	if err != nil {
		if err.Error() == "not found" {
			response.Error(c, http.StatusNotFound, "Education entry not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to update education entry")
		return
	}

	response.Success(c, http.StatusOK, "Education entry updated successfully", entry)
}

func (h *Handler) DeleteEducationEntry(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	entryID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid education entry ID")
		return
	}

	if err := h.service.DeleteEducationEntry(uint(entryID), id); err != nil {
		if err.Error() == "not found" {
			response.Error(c, http.StatusNotFound, "Education entry not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete education entry")
		return
	}

	response.Success(c, http.StatusOK, "Education entry deleted successfully", nil)
}

func (h *Handler) SuperadminUploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	folder := c.DefaultQuery("folder", "uploads")

	url, err := utils.SaveUploadedImage(file, folder)
	if err != nil {
		url, err = utils.SaveUploadedDocument(file, folder)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "Failed to upload file: "+err.Error())
			return
		}
	}

	response.Success(c, http.StatusOK, "File uploaded successfully", gin.H{
		"url": url,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	middleware.ClearAuthCookie(c)
	response.Success(c, 200, "Logged out successfully", nil)
}

func (h *Handler) GetSessions(c *gin.Context) {
	userID, _ := c.Get("user_id")

	sessions, err := h.service.GetUserSessions(userID.(uint))
	if err != nil {
		response.Error(c, 500, "Failed to retrieve sessions")
		return
	}

	if sessions == nil {
		sessions = []UserSession{}
	}

	response.Success(c, 200, "Sessions retrieved successfully", sessions)
}

func (h *Handler) RevokeSession(c *gin.Context) {
	userID, _ := c.Get("user_id")
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid session ID")
		return
	}

	if err := h.service.RevokeSession(uint(sessionID), userID.(uint)); err != nil {
		if err.Error() == "session not found" {
			response.Error(c, 404, "Session not found")
			return
		}
		response.Error(c, 500, "Failed to revoke session")
		return
	}

	response.Success(c, 200, "Session revoked successfully", nil)
}

func (h *Handler) RevokeAllSessions(c *gin.Context) {
	userID, _ := c.Get("user_id")

	if err := h.service.RevokeAllSessions(userID.(uint)); err != nil {
		response.Error(c, 500, "Failed to revoke sessions")
		return
	}

	response.Success(c, 200, "All other sessions revoked", nil)
}

func (h *Handler) GetDashboardStats(c *gin.Context) {
	stats, err := h.service.GetDashboardStats()
	if err != nil {
		response.Error(c, 500, "Failed to fetch dashboard stats")
		return
	}

	response.Success(c, 200, "Dashboard stats retrieved successfully", stats)
}

func (h *Handler) ListAllUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	users, total, err := h.service.ListAllUsers(search, page, limit)
	if err != nil {
		response.Error(c, 500, "Failed to fetch users")
		return
	}

	response.Success(c, 200, "Users retrieved successfully", gin.H{
		"users": users,
		"pagination": gin.H{
			"total":      total,
			"page":       page,
			"limit":      limit,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

func (h *Handler) GetUserDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid user ID")
		return
	}

	user, err := h.service.GetUserDetail(uint(id))
	if err != nil {
		response.Error(c, 404, "User not found")
		return
	}

	response.Success(c, 200, "User retrieved successfully", user)
}

func (h *Handler) SuspendUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid user ID")
		return
	}

	if err := h.service.SuspendUser(uint(id)); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 200, "User suspended successfully", nil)
}

func (h *Handler) ReinstateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid user ID")
		return
	}

	if err := h.service.ReinstateUser(uint(id)); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 200, "User reinstated successfully", nil)
}

func (h *Handler) GetUserEducation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid user ID")
		return
	}

	entries, err := h.service.GetEducationEntries(uint(id))
	if err != nil {
		response.Error(c, 500, "Failed to fetch education entries")
		return
	}

	response.Success(c, 200, "Education entries retrieved successfully", entries)
}

// --- Superadmin Program Handlers ---

func (h *Handler) ListAllPrograms(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	programs, total, err := instService.GetAllPrograms(page, limit)
	if err != nil {
		response.Error(c, 500, "Failed to fetch programs")
		return
	}

	if programs == nil {
		programs = []institution.InstitutionProgram{}
	}

	response.Success(c, 200, "Programs retrieved successfully", gin.H{
		"programs": programs,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetProgramForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid program ID")
		return
	}
	program, err := instService.GetProgramByIDOnly(uint(id))
	if err != nil {
		response.Error(c, 404, "Program not found")
		return
	}
	response.Success(c, 200, "Program retrieved successfully", program)
}

func (h *Handler) CreateProgramForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	var req SuperadminCreateProgramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	progReq := institution.CreateProgramRequest{
		Name:                req.Name,
		Description:         req.Description,
		Duration:            req.Duration,
		Fee:                 req.Fee,
		Eligibility:         req.Eligibility,
		Capacity:            req.Capacity,
		BannerURL:           req.BannerURL,
		InstitutionName:     req.InstitutionName,
		InstitutionLocation: req.InstitutionLocation,
		InstitutionLink:     req.InstitutionLink,
		Data:                req.Data,
		Status:              req.Status,
	}

	instID := uint(0)
	if req.InstitutionID != nil {
		instID = *req.InstitutionID
	}
	program, err := instService.CreateProgram(instID, progReq)
	if err != nil {
		response.Error(c, 500, "Failed to create program")
		return
	}

	response.Success(c, 201, "Program created successfully", program)
}

func (h *Handler) UpdateProgramForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid program ID")
		return
	}

	var req SuperadminUpdateProgramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	progReq := institution.UpdateProgramRequest{
		Name:                req.Name,
		Description:         req.Description,
		Duration:            req.Duration,
		Fee:                 req.Fee,
		Eligibility:         req.Eligibility,
		Capacity:            req.Capacity,
		BannerURL:           req.BannerURL,
		InstitutionName:     req.InstitutionName,
		InstitutionLocation: req.InstitutionLocation,
		InstitutionLink:     req.InstitutionLink,
		Data:                req.Data,
		Status:              req.Status,
	}

	instID := uint(0)
	if req.InstitutionID != nil {
		instID = *req.InstitutionID
	}
	program, err := instService.UpdateProgram(instID, uint(id), progReq)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Program updated successfully", program)
}

func (h *Handler) DeleteProgramForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid program ID")
		return
	}

	if err := instService.DeleteProgramByID(uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, 404, "Program not found")
		} else {
			response.Error(c, 500, "Failed to delete program")
		}
		return
	}

	response.Success(c, 200, "Program deleted successfully", nil)
}

// --- Superadmin Entrance Handlers ---

func (h *Handler) ListAllEntrances(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	statusFilter := c.Query("status")
	entrances, total, err := instService.GetAllEntrances(statusFilter, page, limit)
	if err != nil {
		response.Error(c, 500, "Failed to fetch entrances")
		return
	}

	if entrances == nil {
		entrances = []institution.InstitutionEntrance{}
	}

	response.Success(c, 200, "Entrances retrieved successfully", gin.H{
		"entrances": entrances,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetEntranceForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid entrance ID")
		return
	}

	entrance, err := instService.GetEntranceByIDOnly(uint(id))
	if err != nil {
		response.Error(c, 404, "Entrance not found")
		return
	}

	response.Success(c, 200, "Entrance retrieved successfully", institution.ToEntranceResponse(*entrance))
}

func (h *Handler) CreateEntranceForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	var req SuperadminCreateEntranceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	entReq := institution.CreateEntranceRequest{
		Title:                  req.Title,
		Description:            req.Description,
		Program:                req.Program,
		Date:                   req.Date,
		StartTime:              req.StartTime,
		EndTime:                req.EndTime,
		Duration:               req.Duration,
		TotalMarks:             req.TotalMarks,
		PassingMarks:           req.PassingMarks,
		TotalSeats:             req.TotalSeats,
		Instructions:           req.Instructions,
		HeroBanner:             req.HeroBanner,
		Questions:              req.Questions,
		Status:                 req.Status,
		InstitutionName:        req.InstitutionName,
		InstitutionLocation:    req.InstitutionLocation,
		InstitutionLink:        req.InstitutionLink,
		InstitutionAffiliation: req.InstitutionAffiliation,
		InstitutionLogo:        req.InstitutionLogo,
		ApplicationFee:         req.ApplicationFee,
		OverviewDetails:        req.OverviewDetails,
		ExamDateSchedules:      req.ExamDateSchedules,
		EligibilityList:        req.EligibilityList,
		ApplicationSteps:       req.ApplicationSteps,
		ExamPattern:            req.ExamPattern,
		SubjectMarks:           req.SubjectMarks,
		ModelSets:              req.ModelSets,
		UpcomingDates:          req.UpcomingDates,
		ContactPersons:         req.ContactPersons,
		Faqs:                   req.Faqs,
		Email:                  req.Email,
		ContactNumber:          req.ContactNumber,
		SocialLinks:            req.SocialLinks,
		ApplicationLink:        req.ApplicationLink,
		NoticeFile:             req.NoticeFile,
		EmbeddedMap:            req.EmbeddedMap,
		RequiredDocuments:      req.RequiredDocuments,
		ExaminationSchedule:    req.ExaminationSchedule,
		ProgramsOffered:        req.ProgramsOffered,
	}

	instID := uint(0)
	if req.InstitutionID != nil {
		instID = *req.InstitutionID
	}
	entrance, err := instService.CreateEntrance(instID, entReq)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 201, "Entrance created successfully", entrance)
}

func (h *Handler) UpdateEntranceForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid entrance ID")
		return
	}

	var req SuperadminUpdateEntranceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	entReq := institution.UpdateEntranceRequest{
		Title:                  req.Title,
		Description:            req.Description,
		Program:                req.Program,
		Date:                   req.Date,
		StartTime:              req.StartTime,
		EndTime:                req.EndTime,
		Duration:               req.Duration,
		TotalMarks:             req.TotalMarks,
		PassingMarks:           req.PassingMarks,
		TotalSeats:             req.TotalSeats,
		Instructions:           req.Instructions,
		HeroBanner:             req.HeroBanner,
		Questions:              req.Questions,
		Status:                 req.Status,
		InstitutionName:        req.InstitutionName,
		InstitutionLocation:    req.InstitutionLocation,
		InstitutionLink:        req.InstitutionLink,
		InstitutionAffiliation: req.InstitutionAffiliation,
		InstitutionLogo:        req.InstitutionLogo,
		ApplicationFee:         req.ApplicationFee,
		OverviewDetails:        req.OverviewDetails,
		ExamDateSchedules:      req.ExamDateSchedules,
		EligibilityList:        req.EligibilityList,
		ApplicationSteps:       req.ApplicationSteps,
		ExamPattern:            req.ExamPattern,
		SubjectMarks:           req.SubjectMarks,
		ModelSets:              req.ModelSets,
		UpcomingDates:          req.UpcomingDates,
		ContactPersons:         req.ContactPersons,
		Faqs:                   req.Faqs,
		Email:                  req.Email,
		ContactNumber:          req.ContactNumber,
		SocialLinks:            req.SocialLinks,
		ApplicationLink:        req.ApplicationLink,
		NoticeFile:             req.NoticeFile,
		EmbeddedMap:            req.EmbeddedMap,
		RequiredDocuments:      req.RequiredDocuments,
		ExaminationSchedule:    req.ExaminationSchedule,
		ProgramsOffered:        req.ProgramsOffered,
	}

	instID := uint(0)
	if req.InstitutionID != nil {
		instID = *req.InstitutionID
	}
	entrance, err := instService.UpdateEntrance(instID, uint(id), entReq)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Entrance updated successfully", entrance)
}

func (h *Handler) DeleteEntranceForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid entrance ID")
		return
	}

	if err := instService.DeleteEntranceByID(uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, 404, "Entrance not found")
		} else {
			response.Error(c, 500, "Failed to delete entrance")
		}
		return
	}

	response.Success(c, 200, "Entrance deleted successfully", nil)
}

func (h *Handler) GetEntranceApplicantsForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	entranceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid entrance ID")
		return
	}

	applicants, err := instService.GetEntranceApplicantsByID(uint(entranceID))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	if applicants == nil {
		applicants = []institution.InstitutionEntranceApplicant{}
	}

	response.Success(c, 200, "Applicants retrieved successfully", applicants)
}

// --- Superadmin Admission Page Handlers ---

func (h *Handler) ListAllAdmissionPages(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	statusFilter := c.Query("status")
	pages, total, err := instService.GetAllAdmissionPages(statusFilter, page, limit)
	if err != nil {
		response.Error(c, 500, "Failed to fetch admission pages")
		return
	}

	if pages == nil {
		pages = []institution.AdmissionPage{}
	}

	response.Success(c, 200, "Admission pages retrieved successfully", gin.H{
		"admission_pages": pages,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) CreateAdmissionPageForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	var req SuperadminCreateAdmissionPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	pageReq := institution.CreateAdmissionPageRequest{
		InstitutionName:     req.InstitutionName,
		InstitutionLocation: req.InstitutionLocation,
		InstitutionLink:     req.InstitutionLink,
		Data:                req.Data,
		Status:              req.Status,
	}

	instID := uint(0)
	if req.InstitutionID != nil {
		instID = *req.InstitutionID
	}
	page, err := instService.CreateAdmissionPage(instID, pageReq)
	if err != nil {
		response.Error(c, 500, "Failed to create admission page")
		return
	}

	response.Success(c, 201, "Admission page created successfully", page)
}

func (h *Handler) UpdateAdmissionPageForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid admission page ID")
		return
	}

	var req SuperadminUpdateAdmissionPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	pageReq := institution.UpdateAdmissionPageRequest{
		InstitutionName:     req.InstitutionName,
		InstitutionLocation: req.InstitutionLocation,
		InstitutionLink:     req.InstitutionLink,
		Data:                req.Data,
		Status:              req.Status,
	}

	instID := uint(0)
	if req.InstitutionID != nil {
		instID = *req.InstitutionID
	}
	page, err := instService.UpdateAdmissionPage(instID, uint(id), pageReq)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Admission page updated successfully", page)
}

func (h *Handler) DeleteAdmissionPageForInstitution(c *gin.Context) {
	if instService == nil {
		response.Error(c, 500, "Institution service not available")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "Invalid admission page ID")
		return
	}

	if err := instService.DeleteAdmissionPageByID(uint(id)); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, 404, "Admission page not found")
		} else {
			response.Error(c, 500, "Failed to delete admission page")
		}
		return
	}

	response.Success(c, 200, "Admission page deleted successfully", nil)
}

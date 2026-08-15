package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/shared/utils"

	"github.com/pquerna/otp/totp"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) emailExistsAcrossTypes(email string) bool {
	_, err := s.repo.FindUserByEmail(email)
	if err == nil {
		return true
	}
	_, err = s.repo.FindInstitutionUserByEmail(email)
	if err == nil {
		return true
	}
	_, err = s.repo.FindScholarshipProviderUserByEmail(email)
	return err == nil
}

func (s *Service) Register(req RegisterRequest) (*RegisterResponse, error) {
	if s.emailExistsAcrossTypes(req.Email) {
		return nil, errors.New("An account with this email already exists")
	}

	user := User{
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
		user.Preferences = &Preferences{
			Role: user.Role,
			Preferences: map[string]interface{}{
				"education_level": req.EducationLevel,
			},
			CompletedAt: &now,
		}
	}

	if err := user.HashPassword(req.Password); err != nil {
		return nil, errors.New("Failed to hash password")
	}

	otp, err := utils.GenerateOTP()
	if err != nil {
		return nil, errors.New("Failed to generate OTP")
	}

	utils.StoreOTP(req.Email, otp, user)

	// Don't send email here - frontend will call /send-otp after user clicks "Verify Account"

	return &RegisterResponse{
		Email:       user.Email,
		RequiresOTP: true,
	}, nil
}

func (s *Service) Login(req LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.FindUserByEmail(req.Email)
	if err != nil {
		return nil, errors.New("Invalid email or password")
	}

	if err := user.CheckPassword(req.Password); err != nil {
		return nil, errors.New("Invalid email or password")
	}

	if user.Status == "suspended" {
		return nil, errors.New("Your account has been suspended. Please contact support.")
	}

	now := time.Now()
	user.LastLoginAt = &now
	s.repo.SaveUser(user)

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, 0)
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	s.CreateOrUpdateSession(user.ID, req.IPAddress, req.UserAgent, "")

	if user.TOTPEnabled {
		totpToken, err := utils.GenerateTOTPToken(user.ID, user.Email, user.Role)
		if err != nil {
			return nil, errors.New("Failed to generate TOTP challenge")
		}
		return &LoginResponse{
			User:         nil,
			Token:        "",
			RequiresTOTP: true,
			TOTPToken:    totpToken,
		}, nil
	}

	return &LoginResponse{
		User:  user,
		Token: token,
	}, nil
}

func (s *Service) CreateOrUpdateSession(userID uint, ipAddress, userAgent, location string) {
	deviceName, deviceType, browser := parseUserAgent(userAgent)

	if location == "" && ipAddress != "" && !isPrivateIP(ipAddress) {
		if loc, err := lookupLocation(ipAddress); err == nil {
			location = loc
		}
	}

	var existing *UserSession
	sessions, err := s.repo.FindUserSessionsByUserID(userID)
	if err == nil {
		for _, sess := range sessions {
			if sess.IPAddress == ipAddress && sess.DeviceName == deviceName && sess.DeviceType == deviceType {
				existing = &sess
				break
			}
		}
	}

	now := time.Now()

	if existing != nil {
		existing.LastActiveAt = now
		if location != "" {
			existing.Location = location
		}
		if err := s.repo.db.Save(existing).Error; err != nil {
			log.Printf("auth: failed to update session: %v", err)
		}
	} else {
		session := &UserSession{
			UserID:       userID,
			DeviceName:   deviceName,
			DeviceType:   deviceType,
			Browser:      browser,
			IPAddress:    ipAddress,
			Location:     location,
			LastActiveAt: now,
		}
		if err := s.repo.CreateUserSession(session); err != nil {
			log.Printf("auth: failed to create session for user %d: %v", userID, err)
		}
	}
}

func parseUserAgent(ua string) (deviceName, deviceType, browser string) {
	ua = strings.ToLower(ua)
	deviceName = "Unknown Device"
	deviceType = "web"
	browser = "Unknown"

	switch {
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		deviceType = "mobile"
		deviceName = "Apple iOS"
	case strings.Contains(ua, "android"):
		deviceType = "mobile"
		deviceName = "Android"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os"):
		deviceName = "Mac"
	case strings.Contains(ua, "windows"):
		deviceName = "Windows PC"
	case strings.Contains(ua, "linux"):
		deviceName = "Linux"
	}

	switch {
	case strings.Contains(ua, "chrome/") && !strings.Contains(ua, "edg/"):
		browser = "Chrome"
	case strings.Contains(ua, "firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "safari/") && !strings.Contains(ua, "chrome/"):
		browser = "Safari"
	case strings.Contains(ua, "edg/"):
		browser = "Edge"
	case strings.Contains(ua, "opr/") || strings.Contains(ua, "opera"):
		browser = "Opera"
	}

	return
}

func (s *Service) SendOTP(email string, otpType string) error {
	// Default to verification type
	if otpType == "" {
		otpType = "verification"
	}

	if otpType == "password_reset" {
		_, userErr := s.repo.FindUserByEmail(email)
		_, providerErr := s.repo.FindScholarshipProviderUserByEmail(email)
		_, instErr := s.repo.FindInstitutionUserByEmail(email)
		if userErr != nil && providerErr != nil && instErr != nil {
			return errors.New("No account found with this email address")
		}
	}

	// For verification (registration): don't check if user exists
	// Send OTP anyway - user will be created after OTP verification

	otp, err := utils.GenerateOTP()
	if err != nil {
		return errors.New("Failed to generate OTP")
	}

	_, data := utils.GetOTPData(email)
	utils.StoreOTPWithType(email, otp, otpType, data)

	if emailErr := utils.SendOTPEmail(email, otp); emailErr != nil {
		log.Printf("Warning: failed to send OTP email to %s: %v", email, emailErr)
		log.Printf("DEV OTP for %s: %s", email, otp)
	}

	return nil
}

func (s *Service) VerifyOTP(email, otp string) (*LoginResponse, error) {
	valid, otpType, data := utils.VerifyOTP(email, otp)
	if !valid {
		return nil, errors.New("Invalid or expired OTP")
	}

	if otpType == "password_reset" {
		return nil, errors.New("Use /reset-password endpoint to complete password reset")
	}

	if data == nil {
		return nil, errors.New("Registration data not found. Please register again.")
	}

	if providerUser, ok := data.(ScholarshipProviderUser); ok {
		if s.emailExistsAcrossTypes(providerUser.Email) {
			return nil, errors.New("An account with this email already exists")
		}
		if err := s.repo.CreateScholarshipProviderUser(&providerUser); err != nil {
			return nil, errors.New("Failed to create scholarship provider account")
		}

		return &LoginResponse{
			User:  providerUser,
			Token: "",
		}, nil
	}

	if institutionUser, ok := data.(InstitutionUser); ok {
		if s.emailExistsAcrossTypes(institutionUser.Email) {
			return nil, errors.New("An account with this email already exists")
		}
		if err := s.repo.CreateInstitutionUser(&institutionUser); err != nil {
			return nil, errors.New("Failed to create institution account")
		}

		settings := institution.InstitutionSettings{
			InstitutionID: institutionUser.ID,
			PublicProfile: institutionUser.CollegeID > 0,
			EmailNotifs:   true,
		}
		_ = s.repo.CreateInstitutionSettings(&settings)

		return &LoginResponse{
			User:  institutionUser,
			Token: "",
		}, nil
	}

	user, ok := data.(User)
	if !ok {
		return nil, errors.New("Failed to recover user data")
	}

	if s.emailExistsAcrossTypes(user.Email) {
		return nil, errors.New("An account with this email already exists")
	}

	if err := s.repo.CreateUser(&user); err != nil {
		return nil, errors.New("Failed to create user")
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, 0)
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	return &LoginResponse{
		User:  user,
		Token: token,
	}, nil
}

func downloadAndSavePicture(url string) (string, error) {
	if url == "" {
		return "", nil
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%d.jpg", time.Now().UnixNano())
	if err := storage.UploadBytes("profiles/"+filename, data, "image/jpeg"); err != nil {
		return "", err
	}
	return "/uploads/profiles/" + filename, nil
}

type googleLoginResult struct {
	Token       string
	UserID      uint
	TOTPEnabled bool
}

func (s *Service) GoogleLoginOrRegister(googleID, email, givenName, familyName, picture string) (*googleLoginResult, error) {
	user, err := s.repo.FindUserByEmail(email)
	if err != nil {
		_, instErr := s.repo.FindInstitutionUserByEmail(email)
		_, provErr := s.repo.FindScholarshipProviderUserByEmail(email)
		if instErr == nil || provErr == nil {
			return nil, errors.New("This email is already registered for another account type.")
		}

		user = &User{
			Email:     email,
			FirstName: givenName,
			LastName:  familyName,
			GoogleID:  &googleID,
			Role:      "student",
		}
		if err := s.repo.CreateUser(user); err != nil {
			return nil, errors.New("Failed to create user: " + err.Error())
		}
	} else {
		if user.GoogleID == nil || *user.GoogleID == "" {
			user.GoogleID = &googleID
		}
		s.repo.SaveUser(user)
	}

	if user.ImageURL == "" || !strings.HasPrefix(user.ImageURL, "/uploads/") {
		localPic, _ := downloadAndSavePicture(picture)
		if localPic != "" {
			user.ImageURL = localPic
			s.repo.SaveUser(user)
		}
	}

	token, err := utils.GenerateTokenWithClaims(utils.TokenOptions{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		ImageURL:  user.ImageURL,
	})
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	return &googleLoginResult{Token: token, UserID: user.ID, TOTPEnabled: user.TOTPEnabled}, nil
}

func (s *Service) GetProfile(userID uint) (*ProfileResponse, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	return &ProfileResponse{
		ID:             user.ID,
		Email:          user.Email,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		MiddleName:     user.MiddleName,
		Phone:          user.Phone,
		AlternatePhone: user.AlternatePhone,
		DateOfBirth:    user.DateOfBirth,
		Gender:         user.Gender,
		Nationality:    user.Nationality,
		Address:        user.Address,
		Bio:            user.Bio,
		Role:           user.Role,
		GoogleID:       user.GoogleID,
		ImageURL:       user.ImageURL,
		Preferences:    user.Preferences,
	}, nil
}

func (s *Service) UpdateProfile(userID uint, req UpdateProfileRequest) (*ProfileResponse, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	user.MiddleName = req.MiddleName
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	user.AlternatePhone = req.AlternatePhone
	if req.DateOfBirth != "" {
		user.DateOfBirth = req.DateOfBirth
	}
	if req.Gender != "" {
		user.Gender = req.Gender
	}
	if req.Nationality != "" {
		user.Nationality = req.Nationality
	}
	if req.Address != "" {
		user.Address = req.Address
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}
	if req.ImageURL != "" {
		user.ImageURL = req.ImageURL
	}

	if err := s.repo.SaveUser(user); err != nil {
		return nil, errors.New("Failed to update profile")
	}

	return &ProfileResponse{
		ID:             user.ID,
		Email:          user.Email,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		MiddleName:     user.MiddleName,
		Phone:          user.Phone,
		AlternatePhone: user.AlternatePhone,
		DateOfBirth:    user.DateOfBirth,
		Gender:         user.Gender,
		Nationality:    user.Nationality,
		Address:        user.Address,
		Bio:            user.Bio,
		Role:           user.Role,
		GoogleID:       user.GoogleID,
		ImageURL:       user.ImageURL,
		Preferences:    user.Preferences,
	}, nil
}

func (s *Service) SavePreferences(userID uint, req SavePreferencesRequest) (*PreferencesResponse, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	now := time.Now()
	prefs := &Preferences{
		Role:                req.PreferenceRole,
		PreferenceFlow:      req.PreferenceFlow,
		Preferences:         req.Preferences,
		CompletedAt:         &now,
		OnboardingCompleted: true,
	}

	if err := s.repo.UpdatePreferences(user, prefs); err != nil {
		return nil, errors.New("Failed to save preferences")
	}

	user, err = s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	return &PreferencesResponse{
		User: *user,
	}, nil
}

func (s *Service) SaveInstitutionPreferences(userID uint, req SaveInstitutionPreferencesRequest) (*InstitutionUser, error) {
	institution, err := s.repo.FindInstitutionUserByID(userID)
	if err != nil {
		return nil, errors.New("Institution not found")
	}

	now := time.Now()
	institution.Preferences = &Preferences{
		Role:                "institution",
		PreferenceFlow:      "onboarding",
		Preferences:         req.Preferences,
		CompletedAt:         &now,
		OnboardingCompleted: true,
	}

	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return nil, errors.New("Failed to save preferences")
	}

	institution, err = s.repo.FindInstitutionUserByID(userID)
	if err != nil {
		return nil, errors.New("Institution not found")
	}

	return institution, nil
}

func (s *Service) InstitutionRegister(req InstitutionRegisterRequest) (*RegisterResponse, error) {
	if s.emailExistsAcrossTypes(req.Email) {
		return nil, errors.New("An account with this email already exists")
	}

	_, err := s.repo.FindInstitutionUserByRegistrationNumber(req.RegistrationNumber)
	if err == nil {
		return nil, errors.New("Institution with this registration number already exists")
	}

	institutionUser := InstitutionUser{
		InstitutionName:          req.InstitutionName,
		RegistrationNumber:       req.RegistrationNumber,
		Email:                    req.Email,
		ContactNumber:            req.ContactNumber,
		Province:                 req.Province,
		District:                 req.District,
		LocalBody:                req.LocalBody,
		OrganizationType:         req.OrganizationType,
		PANNumber:                req.PANNumber,
		WebsiteURL:               req.WebsiteURL,
		ContactPerson:            req.ContactPerson,
		ContactPersonDesignation: req.ContactPersonDesignation,
		ContactPersonPhone:       req.ContactPersonPhone,
		Role:                     "institution",
		Status:                   "pending",
	}

	otp, err := utils.GenerateOTP()
	if err != nil {
		return nil, errors.New("Failed to generate OTP")
	}

	utils.StoreOTP(req.Email, otp, institutionUser)

	return &RegisterResponse{
		Email:       institutionUser.Email,
		RequiresOTP: true,
	}, nil
}

func (s *Service) InstitutionLogin(req InstitutionLoginRequest) (*LoginResponse, error) {
	institutionUser, err := s.repo.FindInstitutionUserByEmail(req.Email)
	if err != nil {
		return nil, errors.New("Invalid email or password")
	}

	if institutionUser.Status == "pending" {
		return nil, errors.New("Your account is still under review. Please wait for admin approval.")
	}

	if institutionUser.Status == "rejected" {
		return nil, errors.New("Your registration has been rejected. Please contact support for more information.")
	}

	if institutionUser.Password == nil {
		return nil, errors.New("Your account has not been fully set up. Please contact support.")
	}

	if err := institutionUser.CheckPassword(req.Password); err != nil {
		return nil, errors.New("Invalid email or password")
	}

	token, err := utils.GenerateToken(institutionUser.ID, institutionUser.Email, institutionUser.Role, 0)
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	prefsCompleted := institutionUser.Preferences != nil && institutionUser.Preferences.OnboardingCompleted

	return &LoginResponse{
		User:                 institutionUser,
		Token:                token,
		PreferencesCompleted: prefsCompleted,
	}, nil
}

func (s *Service) InstitutionGoogleLoginOrRegister(googleID, email, name string) (*InstitutionUser, string, error) {
	_, err := s.repo.FindInstitutionUserByEmailOrGoogleID(email, googleID)
	if err != nil {
		_, userErr := s.repo.FindUserByEmail(email)
		_, provErr := s.repo.FindScholarshipProviderUserByEmail(email)
		if userErr == nil || provErr == nil {
			return nil, "", errors.New("This email is already registered for another account type.")
		}
	}

	instUser, err := s.repo.FindInstitutionUserByEmailOrGoogleID(email, googleID)
	if err != nil {
		instUser = &InstitutionUser{
			InstitutionName:    name,
			RegistrationNumber: "GOOGLE-" + googleID,
			Email:              email,
			GoogleID:           &googleID,
			Role:               "institution",
		}
		if err := s.repo.CreateInstitutionUser(instUser); err != nil {
			return nil, "", errors.New("Failed to create institution account: " + err.Error())
		}
	} else {
		if instUser.GoogleID == nil || *instUser.GoogleID == "" {
			instUser.GoogleID = &googleID
			s.repo.db.Save(instUser)
		}
	}

	token, err := utils.GenerateTokenWithClaims(utils.TokenOptions{
		UserID:    instUser.ID,
		Email:     instUser.Email,
		Role:      instUser.Role,
		FirstName: name,
	})
	if err != nil {
		return nil, "", errors.New("Failed to generate token")
	}

	return instUser, token, nil
}

func (s *Service) ScholarshipProviderRegister(req ScholarshipProviderRegisterRequest) (*RegisterResponse, error) {
	if s.emailExistsAcrossTypes(req.Email) {
		return nil, errors.New("An account with this email already exists")
	}

	_, err := s.repo.FindScholarshipProviderUserByRegistrationNumber(req.RegistrationNumber)
	if err == nil {
		return nil, errors.New("Scholarship provider with this registration number already exists")
	}

	providerUser := ScholarshipProviderUser{
		ProviderName:       req.ProviderName,
		RegistrationNumber: req.RegistrationNumber,
		Email:              req.Email,
		ContactNumber:      req.ContactNumber,
		PANNumber:          req.PANNumber,
		WebsiteURL:         req.WebsiteURL,
		Role:               "scholarship_provider",
		Status:             "pending",
	}

	otp, err := utils.GenerateOTP()
	if err != nil {
		return nil, errors.New("Failed to generate OTP")
	}

	utils.StoreOTP(req.Email, otp, providerUser)

	return &RegisterResponse{
		Email:       providerUser.Email,
		RequiresOTP: true,
	}, nil
}

func (s *Service) ListPendingScholarshipProviders() ([]ScholarshipProviderUser, error) {
	return s.repo.FindScholarshipProvidersByStatus("pending")
}

func (s *Service) ListVerifiedScholarshipProviders() ([]ScholarshipProviderUser, error) {
	return s.repo.FindScholarshipProvidersByStatus("approved")
}

func (s *Service) ApproveScholarshipProvider(providerID uint) error {
	provider, err := s.repo.FindScholarshipProviderUserByID(providerID)
	if err != nil {
		return errors.New("Provider not found")
	}

	password, err := utils.GenerateRandomPassword(12)
	if err != nil {
		return errors.New("Failed to generate password")
	}

	if err := provider.HashPassword(password); err != nil {
		return errors.New("Failed to hash password")
	}

	provider.Status = "approved"
	if err := s.repo.UpdateScholarshipProviderUser(provider); err != nil {
		return errors.New("Failed to update provider")
	}

	if emailErr := utils.SendApprovalEmail(provider.Email, provider.ProviderName, password); emailErr != nil {
		log.Printf("Warning: failed to send approval email to %s: %v", provider.Email, emailErr)
	}

	return nil
}

func (s *Service) RejectScholarshipProvider(providerID uint) error {
	provider, err := s.repo.FindScholarshipProviderUserByID(providerID)
	if err != nil {
		return errors.New("Provider not found")
	}

	if err := s.repo.DeleteScholarshipProviderUser(providerID); err != nil {
		return errors.New("Failed to remove provider")
	}

	if emailErr := utils.SendRejectionEmail(provider.Email, provider.ProviderName); emailErr != nil {
		log.Printf("Warning: failed to send rejection email to %s: %v", provider.Email, emailErr)
	}

	return nil
}

func (s *Service) CreateInstitution(req CreateInstitutionRequest) (*InstitutionUser, error) {
	slug := strings.NewReplacer(" ", "_", ".", "_", "-", "_").Replace(strings.ToLower(req.InstitutionName))
	email := fmt.Sprintf("%s@institution.edu.np", slug)

	password, err := utils.GenerateRandomPassword(12)
	if err != nil {
		return nil, errors.New("Failed to generate password")
	}

	profileData := map[string]interface{}{
		"videos":          req.Videos,
		"overview_data":   req.OverviewData,
		"leadership_data": req.LeadershipData,
		"courses_data":    req.CoursesData,
		"programs_data":   req.ProgramsData,
		"facilities_data": req.FacilitiesData,
		"alumni_data":     req.AlumniData,
		"gallery_data":    req.GalleryData,
		"downloads_data":  req.DownloadsData,
	}

	profileJSON, err := json.Marshal(profileData)
	if err != nil {
		return nil, errors.New("Failed to marshal profile data")
	}
	profileStr := string(profileJSON)

	regNumber := fmt.Sprintf("ADMIN-%d", time.Now().UnixMilli())

	institutionUser := InstitutionUser{
		InstitutionName:    req.InstitutionName,
		RegistrationNumber: regNumber,
		Email:              email,
		Role:               "institution",
		Status:             "approved",
		Level:              req.Level,
		Affiliation:        req.Affiliation,
		UniversityID:       &req.UniversityID,
		Verified:           false,
		Claimed:            false,
		District:           req.Location,
		WebsiteURL:         req.Website,
		LogoURL:            req.LogoURL,
		BannerURL:          req.BannerURL,
		About:              req.About,
		Vision:             req.Vision,
		Mission:            req.Mission,
		ProfileData:        &profileStr,
	}

	if err := institutionUser.HashPassword(password); err != nil {
		return nil, errors.New("Failed to hash password")
	}

	if err := s.repo.CreateInstitutionUser(&institutionUser); err != nil {
		return nil, err
	}

	settings := institution.InstitutionSettings{
		InstitutionID: institutionUser.ID,
		PublicProfile: true,
		EmailNotifs:   true,
	}
	if err := s.repo.CreateInstitutionSettings(&settings); err != nil {
		return nil, err
	}

	return &institutionUser, nil
}

func (s *Service) GetInstitution(id uint) (*InstitutionDetailResponse, error) {
	user, err := s.repo.FindInstitutionUserByID(id)
	if err != nil {
		return nil, errors.New("institution not found")
	}

	var profileData map[string]interface{}
	if user.ProfileData != nil && *user.ProfileData != "" {
		if err := json.Unmarshal([]byte(*user.ProfileData), &profileData); err != nil {
			profileData = nil
		}
	}

	logoURL := user.LogoURL
	bannerURL := user.BannerURL
	if strings.HasPrefix(logoURL, "data:") {
		logoURL = ""
	}
	if strings.HasPrefix(bannerURL, "data:") {
		bannerURL = ""
	}

	return &InstitutionDetailResponse{
		ID:                 user.ID,
		InstitutionName:    user.InstitutionName,
		Email:              user.Email,
		RegistrationNumber: user.RegistrationNumber,
		Status:             user.Status,
		Claimed:            user.Claimed,
		Verified:           user.Verified,
		Featured:           user.Featured,
		District:           user.District,
		WebsiteURL:         user.WebsiteURL,
		LogoURL:            logoURL,
		BannerURL:          bannerURL,
		About:              user.About,
		Vision:             user.Vision,
		Mission:            user.Mission,
		Level:              user.Level,
		Affiliation:        user.Affiliation,
		UniversityID:       user.UniversityID,
		IsSponsored:        user.IsSponsored,
		Latitude:           user.Latitude,
		Longitude:          user.Longitude,
		ProfileData:        profileData,
	}, nil
}

func (s *Service) UpdateInstitution(id uint, req UpdateInstitutionRequest) error {
	user, err := s.repo.FindInstitutionUserByID(id)
	if err != nil {
		return errors.New("institution not found")
	}

	if req.InstitutionName != "" {
		user.InstitutionName = req.InstitutionName
	}
	if req.Location != "" {
		user.District = req.Location
	}
	if req.Website != "" {
		user.WebsiteURL = req.Website
	}
	if req.Level != "" {
		user.Level = req.Level
	}
	if req.Affiliation != "" {
		user.Affiliation = req.Affiliation
	}
	if req.UniversityID != nil {
		user.UniversityID = req.UniversityID
	}
	if req.IsSponsored != nil {
		user.IsSponsored = *req.IsSponsored
	}
	if req.About != "" {
		user.About = req.About
	}
	if req.Vision != "" {
		user.Vision = req.Vision
	}
	if req.Mission != "" {
		user.Mission = req.Mission
	}
	if req.LogoURL != "" {
		user.LogoURL = req.LogoURL
	}
	if req.BannerURL != "" {
		user.BannerURL = req.BannerURL
	}

	if req.Latitude != nil {
		user.Latitude = req.Latitude
	}
	if req.Longitude != nil {
		user.Longitude = req.Longitude
	}

	if req.ProfileData != nil {
		profileJSON, err := json.Marshal(req.ProfileData)
		if err == nil {
			profileStr := string(profileJSON)
			user.ProfileData = &profileStr
		}
	}

	return s.repo.UpdateInstitutionUser(user)
}

func (s *Service) RecordInstitutionPayment(institutionID uint, paymentDate time.Time, paidForDays int, amount float64, remarks string) error {
	expireDate := paymentDate.AddDate(0, 0, paidForDays)

	defaultRemarks := fmt.Sprintf("Paid for %d days from %s", paidForDays, paymentDate.Format("Jan 2, 2006"))
	if remarks == "" {
		remarks = defaultRemarks
	}

	sub := &InstitutionSubscription{
		InstitutionID:     institutionID,
		Status:            "paid",
		StartDate:         &paymentDate,
		ExpireDate:        &expireDate,
		LastPaymentDate:   &paymentDate,
		LastPaymentAmount: amount,
		Remarks:           remarks,
	}

	return s.repo.CreateOrUpdateSubscription(sub)
}

func (s *Service) ToggleInstitutionFeatured(institutionID uint) error {
	institution, err := s.repo.FindInstitutionUserByID(institutionID)
	if err != nil {
		return errors.New("Institution not found")
	}

	institution.Featured = !institution.Featured

	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return errors.New("Failed to toggle featured status")
	}

	return nil
}

func (s *Service) VerifyInstitution(institutionID uint) error {
	institution, err := s.repo.FindInstitutionUserByID(institutionID)
	if err != nil {
		return errors.New("Institution not found")
	}

	institution.Verified = !institution.Verified
	if institution.Verified {
		now := time.Now()
		institution.VerifiedAt = &now
	}

	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return errors.New("Failed to update verification status")
	}

	return nil
}

func (s *Service) SuspendInstitution(institutionID uint) error {
	institution, err := s.repo.FindInstitutionUserByID(institutionID)
	if err != nil {
		return errors.New("Institution not found")
	}

	institution.Status = "suspended"
	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return errors.New("Failed to suspend institution")
	}

	return nil
}

func (s *Service) ApproveClaimRequest(institutionID uint) error {
	institution, err := s.repo.FindInstitutionUserByID(institutionID)
	if err != nil {
		return errors.New("Institution not found")
	}
	if institution.Claimed {
		return errors.New("Institution is already claimed")
	}

	if institution.CollegeID > 0 {
		if existing, _ := s.repo.FindClaimedInstitutionByCollegeID(institution.CollegeID); existing != nil && existing.ID != institution.ID {
			return errors.New("This college has already been claimed by another institution")
		}
	}

	password, err := utils.GenerateRandomPassword(12)
	if err != nil {
		return errors.New("Failed to generate password")
	}

	if err := institution.HashPassword(password); err != nil {
		return errors.New("Failed to hash password")
	}

	if institution.CollegeID > 0 {
		if college, cerr := s.repo.FindCollegeByID(institution.CollegeID); cerr == nil {
			if institution.InstitutionName == "" || institution.InstitutionName == college.Name {
				institution.InstitutionName = college.Name
			}
			if institution.District == "" {
				institution.District = college.Location
			}
			if institution.WebsiteURL == "" {
				institution.WebsiteURL = college.Website
			}
			if institution.LogoURL == "" {
				institution.LogoURL = college.ImageURL
			}
			if institution.About == "" {
				institution.About = college.Description
			}
			if institution.Affiliation == "" {
				institution.Affiliation = college.Affiliation
			}
			if institution.OrganizationType == "" {
				institution.OrganizationType = college.CollegeType
			}
			if !institution.Verified {
				institution.Verified = college.Verified
			}
			if institution.ContactEmail == "" {
				institution.ContactEmail = college.Email
			}
			if institution.ContactPhone == "" {
				institution.ContactPhone = college.Phone
			}
		}
	}

	institution.Claimed = true
	institution.Status = "approved"
	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return errors.New("Failed to update institution")
	}

	if emailErr := utils.SendApprovalEmail(institution.Email, institution.InstitutionName, password); emailErr != nil {
		log.Printf("Warning: failed to send claim approval email to %s: %v", institution.Email, emailErr)
	}

	if institution.CollegeID > 0 {
		_ = s.repo.UpdateCollegeClaimed(institution.CollegeID, true)

		otherClaims, err := s.repo.FindInstitutionUsersByStatusAndCollegeID("pending", institution.CollegeID)
		if err == nil {
			for _, claim := range otherClaims {
				if claim.ID == institutionID {
					continue
				}
				claim.Status = "rejected"
				claim.RejectionReason = "Already claimed"
				_ = s.repo.UpdateInstitutionUser(&claim)
				_ = utils.SendRejectionEmail(claim.Email, claim.InstitutionName)
			}
		}
	}

	return nil
}

func (s *Service) DeleteInstitution(institutionID uint) error {
	return s.repo.DeleteInstitutionUser(institutionID)
}

func (s *Service) ClaimRegister(req ClaimRegisterRequest) (*RegisterResponse, error) {
	if exists, _ := s.repo.FindInstitutionUserByEmail(req.Email); exists != nil {
		return nil, errors.New("Email already registered")
	}
	if exists, _ := s.repo.FindInstitutionUserByRegistrationNumber(req.RegistrationNumber); exists != nil {
		return nil, errors.New("Registration number already exists")
	}
	if req.CollegeID > 0 {
		if claimed, _ := s.repo.FindClaimedInstitutionByCollegeID(req.CollegeID); claimed != nil {
			return nil, errors.New("This college has already been claimed")
		}
	}

	institutionUser := InstitutionUser{
		InstitutionName:          req.InstitutionName,
		RegistrationNumber:       req.RegistrationNumber,
		Email:                    req.Email,
		Role:                     "institution",
		Status:                   "pending",
		Claimed:                  false,
		CollegeID:                req.CollegeID,
		ContactNumber:            req.ContactNumber,
		Province:                 req.Province,
		District:                 req.District,
		LocalBody:                req.LocalBody,
		OrganizationType:         req.OrganizationType,
		PANNumber:                req.PANNumber,
		WebsiteURL:               req.WebsiteURL,
		ContactPerson:            req.ContactPerson,
		ContactPersonDesignation: req.ContactPersonDesignation,
		ContactPersonPhone:       req.ContactPersonPhone,
	}

	password, err := utils.GenerateRandomPassword(12)
	if err != nil {
		return nil, errors.New("Failed to generate password")
	}
	if err := institutionUser.HashPassword(password); err != nil {
		return nil, errors.New("Failed to hash password")
	}

	otp, err := utils.GenerateOTP()
	if err != nil {
		return nil, errors.New("Failed to generate OTP")
	}
	utils.StoreOTP(req.Email, otp, institutionUser)

	return &RegisterResponse{Email: req.Email, RequiresOTP: true}, nil
}

func (s *Service) RejectClaimRequest(claimID uint, reason string) error {
	institution, err := s.repo.FindInstitutionUserByID(claimID)
	if err != nil {
		return errors.New("Claim request not found")
	}

	institution.Status = "rejected"
	institution.RejectionReason = reason
	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return errors.New("Failed to reject claim request")
	}

	if emailErr := utils.SendRejectionEmail(institution.Email, institution.InstitutionName); emailErr != nil {
		log.Printf("Warning: failed to send rejection email to %s: %v", institution.Email, emailErr)
	}

	return nil
}

func (s *Service) ListPendingInstitutions() ([]InstitutionUser, error) {
	return s.repo.FindInstitutionUsersByStatus("pending")
}

func (s *Service) ListPendingInstitutionsFiltered(status, reqType string) ([]InstitutionUser, error) {
	if reqType == "registration" {
		return s.repo.FindInstitutionUsersByStatusAndCollegeID(status, 0)
	} else if reqType == "claim" {
		return s.repo.FindInstitutionUsersByStatusAndCollegeID(status, 0, ">")
	}
	return s.repo.FindInstitutionUsersByStatus(status)
}

func (s *Service) ListVerifiedInstitutions() ([]InstitutionUser, error) {
	return s.repo.FindInstitutionUsersByStatus("approved")
}

func (s *Service) ListVerifiedInstitutionsFiltered(filter InstitutionFilter) ([]InstitutionUser, map[string]int64, error) {
	return s.repo.FindInstitutionUsersFiltered("approved", filter)
}

func (s *Service) ListRejectedInstitutions() ([]InstitutionUser, error) {
	return s.repo.FindInstitutionUsersByStatus("rejected")
}

func (s *Service) GetDashboardStats() (*SuperadminDashboardStats, error) {
	return s.repo.CountDashboardStats()
}

func (s *Service) ListAllUsers(search string, page, limit int) ([]User, int64, error) {
	return s.repo.FindAllUsers(search, page, limit)
}

func (s *Service) SuspendUser(userID uint) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("user not found")
	}
	if user.Role != "student" {
		return errors.New("can only suspend student users")
	}
	return s.repo.UpdateUserStatus(userID, "suspended")
}

func (s *Service) ReinstateUser(userID uint) error {
	if _, err := s.repo.FindUserByID(userID); err != nil {
		return errors.New("user not found")
	}
	return s.repo.UpdateUserStatus(userID, "active")
}

func (s *Service) GetUserDetail(userID uint) (*User, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *Service) ApproveInstitution(institutionID uint) error {
	institution, err := s.repo.FindInstitutionUserByID(institutionID)
	if err != nil {
		return errors.New("Institution not found")
	}

	password, err := utils.GenerateRandomPassword(12)
	if err != nil {
		return errors.New("Failed to generate password")
	}

	if err := institution.HashPassword(password); err != nil {
		return errors.New("Failed to hash password")
	}

	institution.Status = "approved"
	institution.Claimed = true
	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return errors.New("Failed to update institution")
	}

	if emailErr := utils.SendApprovalEmail(institution.Email, institution.InstitutionName, password); emailErr != nil {
		log.Printf("Warning: failed to send approval email to %s: %v", institution.Email, emailErr)
	}

	return nil
}

func (s *Service) RejectInstitution(institutionID uint) error {
	institution, err := s.repo.FindInstitutionUserByID(institutionID)
	if err != nil {
		return errors.New("Institution not found")
	}

	institution.Status = "rejected"
	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return errors.New("Failed to update institution")
	}

	if emailErr := utils.SendRejectionEmail(institution.Email, institution.InstitutionName); emailErr != nil {
		log.Printf("Warning: failed to send rejection email to %s: %v", institution.Email, emailErr)
	}

	return nil
}

func (s *Service) UpdateInstitutionProfileAccess(institutionID uint, access map[string]bool) error {
	institution, err := s.repo.FindInstitutionUserByID(institutionID)
	if err != nil {
		return errors.New("Institution not found")
	}

	data, err := json.Marshal(access)
	if err != nil {
		return errors.New("Failed to serialize profile access")
	}

	str := string(data)
	institution.ProfileAccess = &str
	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return errors.New("Failed to update profile access")
	}

	return nil
}

func (s *Service) GetInstitutionProfileAccess(institutionID uint) (map[string]bool, error) {
	institution, err := s.repo.FindInstitutionUserByID(institutionID)
	if err != nil {
		return nil, errors.New("Institution not found")
	}

	access := make(map[string]bool)
	if institution.ProfileAccess != nil && *institution.ProfileAccess != "" {
		if err := json.Unmarshal([]byte(*institution.ProfileAccess), &access); err != nil {
			return nil, errors.New("Failed to parse profile access")
		}
	}

	return access, nil
}

func (s *Service) ScholarshipProviderLogin(req ScholarshipProviderLoginRequest) (*LoginResponse, error) {
	providerUser, err := s.repo.FindScholarshipProviderUserByEmail(req.Email)
	if err != nil {
		return nil, errors.New("Invalid email or password")
	}

	if providerUser.Status == "pending" {
		return nil, errors.New("Your account is still under review. Please wait for admin approval.")
	}

	if providerUser.Status == "rejected" {
		return nil, errors.New("Your registration has been rejected. Please contact support for more information.")
	}

	if providerUser.Password == nil {
		return nil, errors.New("Your account has not been fully set up. Please contact support.")
	}

	if err := providerUser.CheckPassword(req.Password); err != nil {
		return nil, errors.New("Invalid email or password")
	}

	token, err := utils.GenerateToken(providerUser.ID, providerUser.Email, providerUser.Role, providerUser.ID)
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	return &LoginResponse{
		User:  providerUser,
		Token: token,
	}, nil
}

func (s *Service) ScholarshipProviderGoogleLoginOrRegister(googleID, email, name string) (*ScholarshipProviderUser, string, error) {
	_, err := s.repo.FindScholarshipProviderUserByEmailOrGoogleID(email, googleID)
	if err != nil {
		_, userErr := s.repo.FindUserByEmail(email)
		_, instErr := s.repo.FindInstitutionUserByEmail(email)
		if userErr == nil || instErr == nil {
			return nil, "", errors.New("This email is already registered for another account type.")
		}
	}

	providerUser, err := s.repo.FindScholarshipProviderUserByEmailOrGoogleID(email, googleID)
	if err != nil {
		providerUser = &ScholarshipProviderUser{
			ProviderName:       name,
			RegistrationNumber: "GOOGLE-" + googleID,
			Email:              email,
			GoogleID:           &googleID,
			Role:               "scholarship_provider",
		}
		if err := s.repo.CreateScholarshipProviderUser(providerUser); err != nil {
			return nil, "", errors.New("Failed to create scholarship provider account: " + err.Error())
		}
	} else {
		if providerUser.GoogleID == nil || *providerUser.GoogleID == "" {
			providerUser.GoogleID = &googleID
			s.repo.db.Save(providerUser)
		}
	}

	token, err := utils.GenerateTokenWithClaims(utils.TokenOptions{
		UserID:     providerUser.ID,
		Email:      providerUser.Email,
		Role:       providerUser.Role,
		ProviderID: providerUser.ID,
		FirstName:  name,
	})
	if err != nil {
		return nil, "", errors.New("Failed to generate token")
	}

	return providerUser, token, nil
}

func (s *Service) SuperadminRegister(req SuperadminRegisterRequest) (*LoginResponse, error) {
	// Secret Access Code Validation
	if req.AccessCode != "SUPER2026" {
		return nil, errors.New("Invalid administrative access code")
	}

	_, err := s.repo.FindUserByEmail(req.Email)
	if err == nil {
		return nil, errors.New("Administrator with this email already exists")
	}

	user := User{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      "superadmin",
	}

	if err := user.HashPassword(req.Password); err != nil {
		return nil, errors.New("Failed to hash credentials")
	}

	if err := s.repo.CreateUser(&user); err != nil {
		return nil, errors.New("Failed to create superadmin account")
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, 0)
	if err != nil {
		return nil, errors.New("Failed to generate secure token")
	}

	return &LoginResponse{
		User:  user,
		Token: token,
	}, nil
}

func (s *Service) SuperadminLogin(req SuperadminLoginRequest) (*LoginResponse, error) {
	user, err := s.repo.FindUserByEmail(req.Email)
	if err != nil {
		return nil, errors.New("Invalid administrative credentials")
	}

	if user.Role != "superadmin" && user.Role != "super_admin" {
		return nil, errors.New("Access denied: Not a superadmin")
	}

	if err := user.CheckPassword(req.Password); err != nil {
		return nil, errors.New("Invalid administrative credentials")
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, 0)
	if err != nil {
		return nil, errors.New("Failed to generate secure token")
	}

	return &LoginResponse{
		User:  user,
		Token: token,
	}, nil
}

func (s *Service) ChangePassword(userID uint, currentPassword, newPassword string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("User not found")
	}

	if user.CheckPassword(currentPassword) != nil {
		return errors.New("invalid credentials")
	}

	if err := user.HashPassword(newPassword); err != nil {
		return errors.New("Failed to hash password")
	}

	return s.repo.SaveUser(user)
}

func (s *Service) GetEducationEntries(userID uint) ([]EducationEntryResponse, error) {
	entries, err := s.repo.FindEducationEntriesByUserID(userID)
	if err != nil {
		return nil, err
	}

	responses := make([]EducationEntryResponse, len(entries))
	for i, e := range entries {
		responses[i] = EducationEntryResponse{
			ID:              e.ID,
			Level:           e.Level,
			InstitutionName: e.InstitutionName,
			BoardUniversity: e.BoardUniversity,
			Country:         e.Country,
			Stream:          e.Stream,
			StartYear:       e.StartYear,
			EndYear:         e.EndYear,
			GradingSystem:   e.GradingSystem,
			Grade:           e.Grade,
		}
	}
	return responses, nil
}

func (s *Service) CreateEducationEntry(userID uint, req EducationEntryRequest) (*EducationEntryResponse, error) {
	entry := &EducationEntry{
		UserID:          userID,
		Level:           req.Level,
		InstitutionName: req.InstitutionName,
		BoardUniversity: req.BoardUniversity,
		Country:         req.Country,
		Stream:          req.Stream,
		StartYear:       req.StartYear,
		EndYear:         req.EndYear,
		GradingSystem:   req.GradingSystem,
		Grade:           req.Grade,
	}

	if err := s.repo.CreateEducationEntry(entry); err != nil {
		return nil, err
	}

	return &EducationEntryResponse{
		ID:              entry.ID,
		Level:           entry.Level,
		InstitutionName: entry.InstitutionName,
		BoardUniversity: entry.BoardUniversity,
		Country:         entry.Country,
		Stream:          entry.Stream,
		StartYear:       entry.StartYear,
		EndYear:         entry.EndYear,
		GradingSystem:   entry.GradingSystem,
		Grade:           entry.Grade,
	}, nil
}

func (s *Service) UpdateEducationEntry(entryID, userID uint, req EducationEntryRequest) (*EducationEntryResponse, error) {
	entry, err := s.repo.FindEducationEntryByID(entryID, userID)
	if err != nil {
		return nil, errors.New("not found")
	}

	entry.Level = req.Level
	entry.InstitutionName = req.InstitutionName
	entry.BoardUniversity = req.BoardUniversity
	entry.Country = req.Country
	entry.Stream = req.Stream
	entry.StartYear = req.StartYear
	entry.EndYear = req.EndYear
	entry.GradingSystem = req.GradingSystem
	entry.Grade = req.Grade

	if err := s.repo.SaveEducationEntry(entry); err != nil {
		return nil, err
	}

	return &EducationEntryResponse{
		ID:              entry.ID,
		Level:           entry.Level,
		InstitutionName: entry.InstitutionName,
		BoardUniversity: entry.BoardUniversity,
		Country:         entry.Country,
		Stream:          entry.Stream,
		StartYear:       entry.StartYear,
		EndYear:         entry.EndYear,
		GradingSystem:   entry.GradingSystem,
		Grade:           entry.Grade,
	}, nil
}

func (s *Service) DeleteEducationEntry(entryID, userID uint) error {
	return s.repo.DeleteEducationEntry(entryID, userID)
}

func (s *Service) GetUserSessions(userID uint) ([]UserSession, error) {
	sessions, err := s.repo.FindUserSessionsByUserID(userID)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	result := make([]UserSession, 0)
	for i, session := range sessions {
		fingerprint := session.IPAddress + "|" + session.DeviceName + "|" + session.DeviceType
		if seen[fingerprint] {
			continue
		}
		seen[fingerprint] = true
		session.IsCurrent = (i == 0)
		result = append(result, session)
	}
	return result, nil
}

func (s *Service) RevokeSession(sessionID, userID uint) error {
	session, err := s.repo.FindUserSessionByID(sessionID, userID)
	if err != nil {
		return errors.New("session not found")
	}
	return s.repo.DeleteUserSession(session.ID, userID)
}

func (s *Service) RevokeAllSessions(userID uint) error {
	return s.repo.DeleteUserSessionsExcept(userID, 0)
}

func (s *Service) GenerateTOTPSecret(userID uint) (*TOTPGenerateResponse, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "StudSphere",
		AccountName: user.Email,
	})
	if err != nil {
		return nil, errors.New("Failed to generate TOTP secret")
	}

	user.TOTPSecret = key.Secret()
	if err := s.repo.SaveUser(user); err != nil {
		return nil, errors.New("Failed to save TOTP secret")
	}

	return &TOTPGenerateResponse{
		Secret:  key.Secret(),
		QRURI:   key.URL(),
		Account: user.Email,
	}, nil
}

func (s *Service) EnableTOTP(userID uint, code string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("User not found")
	}

	if user.TOTPSecret == "" {
		return errors.New("TOTP not initialized. Generate a secret first.")
	}

	if !totp.Validate(code, user.TOTPSecret) {
		return errors.New("Invalid TOTP code")
	}

	user.TOTPEnabled = true
	user.TOTPVerified = true
	return s.repo.SaveUser(user)
}

func (s *Service) DisableTOTP(userID uint, password, code string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("User not found")
	}

	// If user has a password, verify it. Google users (no password) skip this check.
	if user.Password != nil {
		if err := user.CheckPassword(password); err != nil {
			return errors.New("Invalid password")
		}
	}

	if !totp.Validate(code, user.TOTPSecret) {
		return errors.New("Invalid TOTP code")
	}

	user.TOTPEnabled = false
	user.TOTPVerified = false
	user.TOTPSecret = ""
	return s.repo.SaveUser(user)
}

func (s *Service) DeactivateAccount(userID uint) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("User not found")
	}
	user.Status = "deactivated"
	return s.repo.SaveUser(user)
}

func (s *Service) QueueDeletion(userID uint) (*time.Time, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}
	now := time.Now()
	deletionDate := now.AddDate(0, 0, 14)
	user.ScheduledDeletionAt = &deletionDate
	if err := s.repo.SaveUser(user); err != nil {
		return nil, err
	}
	return &deletionDate, nil
}

func (s *Service) CancelDeletion(userID uint) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("User not found")
	}
	user.ScheduledDeletionAt = nil
	return s.repo.SaveUser(user)
}

func (s *Service) GetDeletionStatus(userID uint) (*DeletionStatusResponse, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}
	if user.ScheduledDeletionAt == nil {
		return &DeletionStatusResponse{}, nil
	}
	remaining := int(time.Until(*user.ScheduledDeletionAt).Hours() / 24)
	if remaining < 0 {
		remaining = 0
	}
	dateStr := user.ScheduledDeletionAt.Format("January 2, 2006")
	return &DeletionStatusResponse{
		ScheduledDeletionAt: &dateStr,
		DaysRemaining:       remaining,
	}, nil
}

func (s *Service) VerifyLoginTOTP(tempToken, code string) (*LoginResponse, error) {
	claims, err := utils.ValidateToken(tempToken)
	if err != nil {
		return nil, errors.New("Invalid or expired TOTP challenge")
	}

	user, err := s.repo.FindUserByID(claims.UserID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	if !user.TOTPEnabled {
		return nil, errors.New("TOTP is not enabled for this account")
	}

	if !totp.Validate(code, user.TOTPSecret) {
		return nil, errors.New("Invalid TOTP code")
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, 0)
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	s.CreateOrUpdateSession(user.ID, "", "", "")

	return &LoginResponse{
		User:  user,
		Token: token,
	}, nil
}

func (s *Service) GetProfileDocuments(userID uint) ([]ProfileDocument, error) {
	docs, err := s.repo.FindProfileDocumentsByUserID(userID)
	if err != nil {
		return nil, err
	}
	if docs == nil {
		docs = []ProfileDocument{}
	}
	return docs, nil
}

func (s *Service) UploadProfileDocument(userID uint, file *multipart.FileHeader, docType string) (*ProfileDocument, error) {
	folder := "documents"
	url, err := utils.SaveUploadedDocument(file, folder)
	if err != nil {
		url, err = utils.SaveUploadedImage(file, folder)
		if err != nil {
			return nil, errors.New("Failed to upload file: " + err.Error())
		}
	}

	mimeType := file.Header.Get("Content-Type")

	doc := &ProfileDocument{
		UserID:   userID,
		FileName: file.Filename,
		FileSize: file.Size,
		Type:     docType,
		MimeType: mimeType,
		URL:      url,
	}

	if err := s.repo.CreateProfileDocument(doc); err != nil {
		return nil, errors.New("Failed to save document record")
	}

	return doc, nil
}

func (s *Service) DeleteProfileDocument(docID, userID uint) error {
	doc, err := s.repo.FindProfileDocumentByID(docID, userID)
	if err != nil {
		return errors.New("document not found")
	}
	return s.repo.DeleteProfileDocument(doc.ID, userID)
}

func (s *Service) ResetPassword(email, otp, newPassword string) error {
	valid, otpType, _ := utils.VerifyOTP(email, otp)
	if !valid {
		return errors.New("Invalid or expired OTP")
	}

	if otpType != "password_reset" {
		return errors.New("Invalid OTP type")
	}

	user, userErr := s.repo.FindUserByEmail(email)
	if userErr == nil {
		if err := user.HashPassword(newPassword); err != nil {
			return errors.New("Failed to hash password")
		}
		return s.repo.SaveUser(user)
	}

	providerUser, providerErr := s.repo.FindScholarshipProviderUserByEmail(email)
	if providerErr == nil {
		if err := providerUser.HashPassword(newPassword); err != nil {
			return errors.New("Failed to hash password")
		}
		return s.repo.UpdateScholarshipProviderUser(providerUser)
	}

	instUser, instErr := s.repo.FindInstitutionUserByEmail(email)
	if instErr != nil {
		return errors.New("User not found")
	}

	if err := instUser.HashPassword(newPassword); err != nil {
		return errors.New("Failed to hash password")
	}

	return s.repo.UpdateInstitutionUser(instUser)
}

type ipGeoResponse struct {
	City    string `json:"city"`
	Region  string `json:"regionName"`
	Country string `json:"country"`
	Query   string `json:"query"`
	Status  string `json:"status"`
}

func lookupLocation(ip string) (string, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=city,regionName,country,status,query", ip)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var geo ipGeoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil {
		return "", err
	}
	if geo.Status != "success" {
		return "", fmt.Errorf("ip-api lookup failed for %s", ip)
	}

	parts := []string{}
	if geo.City != "" {
		parts = append(parts, geo.City)
	}
	if geo.Region != "" {
		parts = append(parts, geo.Region)
	}
	if geo.Country != "" {
		parts = append(parts, geo.Country)
	}
	return strings.Join(parts, ", "), nil
}

func isPrivateIP(ip string) bool {
	// Check common private ranges without net package dependency
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "172.16.") || strings.HasPrefix(ip, "172.17.") ||
		strings.HasPrefix(ip, "172.18.") || strings.HasPrefix(ip, "172.19.") ||
		strings.HasPrefix(ip, "172.20.") || strings.HasPrefix(ip, "172.21.") ||
		strings.HasPrefix(ip, "172.22.") || strings.HasPrefix(ip, "172.23.") ||
		strings.HasPrefix(ip, "172.24.") || strings.HasPrefix(ip, "172.25.") ||
		strings.HasPrefix(ip, "172.26.") || strings.HasPrefix(ip, "172.27.") ||
		strings.HasPrefix(ip, "172.28.") || strings.HasPrefix(ip, "172.29.") ||
		strings.HasPrefix(ip, "172.30.") || strings.HasPrefix(ip, "172.31.") ||
		ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return true
	}
	return false
}

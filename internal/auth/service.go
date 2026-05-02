package auth

import (
	"errors"
	"log"
	"time"

	"studsphere/backend/internal/shared/utils"
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

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	return &LoginResponse{
		User:  user,
		Token: token,
	}, nil
}

func (s *Service) SendOTP(email string, otpType string) error {
	// Default to verification type
	if otpType == "" {
		otpType = "verification"
	}

	if otpType == "password_reset" {
		_, userErr := s.repo.FindUserByEmail(email)
		_, providerErr := s.repo.FindScholarshipProviderUserByEmail(email)
		if userErr != nil && providerErr != nil {
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

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	return &LoginResponse{
		User:  user,
		Token: token,
	}, nil
}

func (s *Service) GoogleLoginOrRegister(googleID, email, givenName, familyName string) (string, error) {
	user, err := s.repo.FindUserByEmail(email)
	if err != nil {
		_, instErr := s.repo.FindInstitutionUserByEmail(email)
		_, provErr := s.repo.FindScholarshipProviderUserByEmail(email)
		if instErr == nil || provErr == nil {
			return "", errors.New("An account with this email already exists using a different login method")
		}

		user = &User{
			Email:     email,
			FirstName: givenName,
			LastName:  familyName,
			GoogleID:  &googleID,
			Role:      "student",
		}
		if err := s.repo.CreateUser(user); err != nil {
			return "", errors.New("Failed to create user: " + err.Error())
		}
	} else {
		if user.GoogleID == nil || *user.GoogleID == "" {
			user.GoogleID = &googleID
			s.repo.SaveUser(user)
		}
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return "", errors.New("Failed to generate token")
	}

	return token, nil
}

func (s *Service) GetProfile(userID uint) (*ProfileResponse, error) {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return nil, errors.New("User not found")
	}

	return &ProfileResponse{
		ID:          user.ID,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Phone:       user.Phone,
		DateOfBirth: user.DateOfBirth,
		Gender:      user.Gender,
		Nationality: user.Nationality,
		Address:     user.Address,
		Bio:         user.Bio,
		Role:        user.Role,
		GoogleID:    user.GoogleID,
		Preferences: user.Preferences,
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
	if req.Phone != "" {
		user.Phone = req.Phone
	}
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

	if err := s.repo.SaveUser(user); err != nil {
		return nil, errors.New("Failed to update profile")
	}

	return &ProfileResponse{
		ID:          user.ID,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Phone:       user.Phone,
		DateOfBirth: user.DateOfBirth,
		Gender:      user.Gender,
		Nationality: user.Nationality,
		Address:     user.Address,
		Bio:         user.Bio,
		Role:        user.Role,
		GoogleID:    user.GoogleID,
		Preferences: user.Preferences,
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

func (s *Service) InstitutionRegister(req InstitutionRegisterRequest) (*LoginResponse, error) {
	if s.emailExistsAcrossTypes(req.Email) {
		return nil, errors.New("An account with this email already exists")
	}

	_, err := s.repo.FindInstitutionUserByRegistrationNumber(req.RegistrationNumber)
	if err == nil {
		return nil, errors.New("Institution with this registration number already exists")
	}

	institutionUser := InstitutionUser{
		InstitutionName:    req.InstitutionName,
		RegistrationNumber: req.RegistrationNumber,
		Email:              req.Email,
		Role:               "institution",
	}

	if err := institutionUser.HashPassword(req.Password); err != nil {
		return nil, errors.New("Failed to hash password")
	}

	if err := s.repo.CreateInstitutionUser(&institutionUser); err != nil {
		return nil, errors.New("Failed to create institution account")
	}

	token, err := utils.GenerateToken(institutionUser.ID, institutionUser.Email, institutionUser.Role)
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	return &LoginResponse{
		User:  institutionUser,
		Token: token,
	}, nil
}

func (s *Service) InstitutionLogin(req InstitutionLoginRequest) (*LoginResponse, error) {
	institutionUser, err := s.repo.FindInstitutionUserByEmail(req.Email)
	if err != nil {
		return nil, errors.New("Invalid email or password")
	}

	if err := institutionUser.CheckPassword(req.Password); err != nil {
		return nil, errors.New("Invalid email or password")
	}

	token, err := utils.GenerateToken(institutionUser.ID, institutionUser.Email, institutionUser.Role)
	if err != nil {
		return nil, errors.New("Failed to generate token")
	}

	return &LoginResponse{
		User:  institutionUser,
		Token: token,
	}, nil
}

func (s *Service) InstitutionGoogleLoginOrRegister(googleID, email, name string) (*InstitutionUser, string, error) {
	_, err := s.repo.FindInstitutionUserByEmailOrGoogleID(email, googleID)
	if err != nil {
		_, userErr := s.repo.FindUserByEmail(email)
		_, provErr := s.repo.FindScholarshipProviderUserByEmail(email)
		if userErr == nil || provErr == nil {
			return nil, "", errors.New("An account with this email already exists using a different login method")
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

	token, err := utils.GenerateToken(instUser.ID, instUser.Email, instUser.Role)
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

	token, err := utils.GenerateToken(providerUser.ID, providerUser.Email, providerUser.Role)
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
			return nil, "", errors.New("An account with this email already exists using a different login method")
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

	token, err := utils.GenerateToken(providerUser.ID, providerUser.Email, providerUser.Role)
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

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
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

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
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
	if providerErr != nil {
		return errors.New("User not found")
	}

	if err := providerUser.HashPassword(newPassword); err != nil {
		return errors.New("Failed to hash password")
	}

	return s.repo.UpdateScholarshipProviderUser(providerUser)
}

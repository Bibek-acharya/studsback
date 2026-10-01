package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/institution"
	"studsphere/backend/internal/notification"
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

// nullableString maps "" to NULL so optional unique columns don't collide
// on empty strings (Postgres unique indexes allow multiple NULLs).
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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

// approvalPendingEmail renders the transactional approval_pending email
// (subject + body) from the registry templates. Sent directly via
// emailqueue — pipeline-free per doc 06 (no delivery row).
func approvalPendingEmail(name, kind string) (string, string) {
	def := notification.Registry[notification.EventAccountApprovalPending]
	data := map[string]any{"name": name, "kind": kind}
	subject, err := notification.ResolveTemplate(def.TitleTpl, data)
	if err != nil {
		log.Printf("Warning: failed to render approval_pending subject for %s: %v", name, err)
	}
	body, err := notification.ResolveTemplate(def.BodyTpl, data)
	if err != nil {
		log.Printf("Warning: failed to render approval_pending body for %s: %v", name, err)
	}
	return subject, body
}

// studentRole is the ONLY authorization role the two public self-registration
// paths may mint. Both are reachable without any credential, so neither is
// allowed to take a role from the request: the session token is minted from the
// persisted row and every middleware.RequireRole in the deployment reads the
// claim out of it. The privileged roles come from SuperadminRegister (behind an
// access code), InstitutionRegister and ScholarshipProviderRegister, each of
// which sets its own role server-side.
const studentRole = "student"

func (s *Service) Register(req RegisterRequest) (*RegisterResponse, error) {
	if s.emailExistsAcrossTypes(req.Email) {
		return nil, errors.New("An account with this email already exists")
	}

	// ── the authorization role is SERVER-OWNED ─────────────────────────────────
	//
	// RegisterRequest binds a `role` field out of the request body, and that
	// field is the onboarding PERSONA the student picked while signing up. It
	// belongs on Preferences.Role and nowhere else.
	//
	// It used to be persisted straight onto User.Role, which made every
	// RequireRole in the deployment self-service: the session token is minted
	// from this row (Login, VerifyOTP, GoogleCallback) and middleware.RequireRole
	// reads the claim straight out of it, so an anonymous caller could POST
	// /api/v1/auth/register with {"role":"superadmin"}, verify the OTP and be
	// holding a superadmin JWT. That walked past every admin group — including
	// GET /api/v1/admin/mock-tests/:id, which returns a paper's full answer key
	// with no coin check, and the study-resource draft listing — for the price
	// of a free account.
	//
	// The privileged roles have their own sign-ups that set their own role
	// behind an access code or an approval flow: SuperadminRegister,
	// InstitutionRegister and ScholarshipProviderRegister. There is no
	// legitimate reason for the public student path to accept one.
	user := User{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      studentRole,
	}

	// The persona the client chose is display data and is never consulted for
	// authorization. It is preserved INDEPENDENTLY of the education level,
	// because education_level has no `binding:"required"` and a request may
	// legitimately omit it: nesting the persona inside that guard silently
	// dropped it for exactly the registrations that were thinnest, which is how
	// a security fix quietly becomes a data-loss fix. An absent persona keeps
	// the historical "student" default rather than becoming an empty string.
	persona := req.Role
	if persona == "" {
		persona = studentRole
	}
	prefs := &Preferences{Role: persona}
	if req.EducationLevel != "" {
		now := time.Now()
		prefs.Preferences = map[string]interface{}{
			"education_level": req.EducationLevel,
		}
		// CompletedAt means onboarding finished, so it belongs to the branch
		// that actually carries the onboarding data and not to the row itself.
		prefs.CompletedAt = &now
	}
	user.Preferences = prefs

	if err := user.HashPassword(req.Password); err != nil {
		return nil, errors.New("Failed to hash password")
	}

	otp, err := utils.GenerateOTP()
	if err != nil {
		return nil, errors.New("Failed to generate OTP")
	}

	// The referral code is staged WITH the pending account, not applied here.
	//
	// This branch of the flow does not create the account — Register deliberately
	// only stages it, and the row is written by VerifyOTP once the email is proven.
	// There is no id to attribute until then, and attributing a row that does not
	// exist yet would mean either writing a referral for an account that later
	// fails its OTP, or holding a transaction open across an email round trip.
	//
	// So the code rides in the OTP store until VerifyOTP consumes the entry. That
	// makes the OTP store load-bearing for referral attribution, which is why
	// VerifyOTP reads the whole staged entry rather than just the account — see
	// SendOTP for the path that would otherwise drop it.
	utils.StoreOTPWithReferral(req.Email, otp, "", user, req.ReferralCode)

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
		} else if notifierInstance != nil {
			email := ""
			if u, ferr := s.repo.FindUserByID(userID); ferr == nil {
				email = u.Email
			}
			_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventAccountNewDeviceLogin,
				Recipients: []notification.Ref{{Type: "user", ID: userID}},
				Data:       map[string]any{"email": email},
				DedupeKey:  fmt.Sprintf("new_device:%d", userID),
			})
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

	// The WHOLE staged entry, not just the account. SendOTP is on the mainline
	// registration path — Register deliberately does not mail the OTP, the
	// frontend calls /send-otp once the student clicks "verify account" — so a
	// re-store that carried back only (type, data) would drop the referral code
	// for precisely the students who took the ordinary route. That is an
	// under-counted referral on the most common path in the product, caused by a
	// line that looks like a harmless re-stage.
	//
	// GetOTPData is still used below for the `data == nil` guard that decides
	// whether this is a registration at all.
	otpType, data, referral, staged := utils.GetOTPStaged(email)
	if !staged {
		// Nothing pending: a plain verification for an account that already exists,
		// or a password reset. Nothing to carry forward.
		otpType = ""
	}
	utils.StoreOTPWithReferral(email, otp, otpType, data, referral)

	if emailErr := utils.SendOTPEmail(email, otp); emailErr != nil {
		log.Printf("Warning: failed to send OTP email to %s: %v", email, emailErr)
		log.Printf("DEV OTP for %s: %s", email, otp)
	}

	return nil
}

// VerifyOTP is user-creation path 1-3 of 6: it is where an OTP registration
// becomes a row, and it is THREE paths rather than one because the staged account
// may be a student, an institution or a scholarship provider. See internal/auth/
// referral.go for the full enumeration.
func (s *Service) VerifyOTP(email, otp string) (*LoginResponse, error) {
	// VerifyOTPWithReferral, not VerifyOTP: the referral code the invitee arrived
	// with is captured on RegisterRequest and lives in the OTP store for the ten
	// minutes between the two calls, and this is the only point at which the
	// request that carried it is still reachable through the store. Verified
	// together with the code rather than read afterwards, because this call
	// CONSUMES the entry — a follow-up read would find nothing.
	valid, otpType, data, referral := utils.VerifyOTPWithReferral(email, otp)
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

		// ── user-creation path 3 of 6 ────────────────────────────────────────────
		// A row in scholarship_provider_users, NOT in users. See internal/auth/
		// referral.go for why the table has to be named.
		s.applyAttribution(context.Background(), ReferralSubject{
			Kind: ReferralSubjectProvider,
			ID:   providerUser.ID,
			Code: referral,
			Path: "verify_otp_provider",
		})

		if notifierInstance != nil {
			_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventAccountWelcome,
				Recipients: []notification.Ref{{Type: "provider", ID: providerUser.ID}},
				Data:       map[string]any{"first_name": providerUser.ProviderName},
			})
		}

		if notifierInstance != nil {
			audience, _ := notifierInstance.ForRoles(context.Background(), "superadmin", "admin")
			if len(audience) > 0 {
				_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
					EventKey:   notification.EventSystemProviderPending,
					Recipients: audience,
					Data:       map[string]any{"name": providerUser.ProviderName},
				})
			}
		}

		subject, html := approvalPendingEmail(providerUser.ProviderName, "provider")
		if emailErr := emailqueue.EnqueueGenericEmail(providerUser.Email, subject, html); emailErr != nil {
			log.Printf("Warning: failed to enqueue approval_pending email to %s: %v", providerUser.Email, emailErr)
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

		// ── user-creation path 2 of 6 ────────────────────────────────────────────
		// A row in institution_users, NOT in users.
		s.applyAttribution(context.Background(), ReferralSubject{
			Kind: ReferralSubjectInstitution,
			ID:   institutionUser.ID,
			Code: referral,
			Path: "verify_otp_institution",
		})

		settings := institution.InstitutionSettings{
			InstitutionID: institutionUser.ID,
			PublicProfile: institutionUser.CollegeID > 0,
			EmailNotifs:   true,
		}
		_ = s.repo.CreateInstitutionSettings(&settings)

		if notifierInstance != nil {
			_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventAccountWelcome,
				Recipients: []notification.Ref{{Type: "institution", ID: institutionUser.ID}},
				Data:       map[string]any{"first_name": institutionUser.InstitutionName},
			})
		}

		if institutionUser.CollegeID == 0 && notifierInstance != nil {
			audience, _ := notifierInstance.ForRoles(context.Background(), "superadmin", "admin")
			if len(audience) > 0 {
				_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
					EventKey:   notification.EventSystemInstitutionPending,
					Recipients: audience,
					Data:       map[string]any{"name": institutionUser.InstitutionName},
				})
			}
		}

		subject, html := approvalPendingEmail(institutionUser.InstitutionName, "institution")
		if emailErr := emailqueue.EnqueueGenericEmail(institutionUser.Email, subject, html); emailErr != nil {
			log.Printf("Warning: failed to enqueue approval_pending email to %s: %v", institutionUser.Email, emailErr)
		}

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

	// Write path 8 of 8. email and preferences.onboarding_completed both arrive
	// pre-set on a registration that carried an education level, so this path can
	// cross a threshold on the very save that creates the account. It is the one
	// write path that fires before the student has ever authenticated.
	s.awardProfile(context.Background(), user.ID)

	// ── user-creation path 1 of 4: referral attribution ───────────────────────
	//
	// The referral code arrives on RegisterRequest, is carried through the OTP
	// store by VerifyOTPWithReferral above, and is applied HERE — after
	// CreateUser, because attribution needs the new account's id, and the id only
	// exists once the row does.
	//
	// Under the Student's Referrer subject, because this is the branch that
	// creates an auth.User. The two branches above create rows in
	// institution_users and scholarship_provider_users, and they attribute
	// against their own subjects — which is the whole reason ReferredKind exists
	// (see internal/coins/referral_model.go).
	s.applyAttribution(context.Background(), ReferralSubject{
		Kind: ReferralSubjectUser,
		ID:   user.ID,
		Code: referral,
		Path: "verify_otp",
	})

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountWelcome,
			Recipients: []notification.Ref{{Type: "user", ID: user.ID}},
			Data:       map[string]any{"first_name": user.FirstName},
		})
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

// GoogleLoginOrRegister is user-creation path 4 of 6. See internal/auth/referral.go
// for the enumeration.
//
// referralCode arrives as an argument because this method runs from a Google
// redirect callback: the request that carried the invite code is the one that
// STARTED the OAuth dance and it was answered with a 302 a minute and a half ago.
// The code has to be persisted client-side across the redirect (the /r/[code]
// capture route) and handed back here.
func (s *Service) GoogleLoginOrRegister(googleID, email, givenName, familyName, picture, referralCode string) (*googleLoginResult, error) {
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
			Role:      studentRole,
		}
		if err := s.repo.CreateUser(user); err != nil {
			return nil, errors.New("Failed to create user: " + err.Error())
		}

		// ── user-creation path 4 of 6 ────────────────────────────────────────────
		//
		// Inside the creation branch and not after it. An EXISTING user arriving
		// with a code is the case the UNIQUE (referred_kind, referred_user_id)
		// constraint exists to refuse — an established account re-entering an
		// invite code to farm both sides is fraud mechanism 3 in
		// 05-economy-and-fraud.md §3.2 — and calling applyAttribution outside this
		// branch would send every Google sign-in of an already-referred student at
		// the coin system on every login. Attribute at creation, once, or not at
		// all.
		s.applyAttribution(context.Background(), ReferralSubject{
			Kind: ReferralSubjectUser,
			ID:   user.ID,
			Code: referralCode,
			Path: "google_login",
		})
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
			// Write path 3 of 8. image_url is one of the twelve completion checks,
			// and this is the only path that sets it WITHOUT going through
			// UpdateProfile — a Google sign-in that downloads and stores a
			// profile picture moves a completion field on its own.
			//
			// Without this call a student who signed up with Google and let the
			// picture download land would score higher on their profile page than
			// one who uploaded a photo through the form, and would be paid less
			// for the same profile. That is the incentive-to-use-one-form bug the
			// award must not have.
			s.awardProfile(context.Background(), user.ID)
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

	// Write path 1 of 8. This is the main one, and the only one the
	// implementation plan named. It is also the path the profile picture upload
	// handler funnels through, so a student who completes their profile entirely
	// by uploading a photo and using this form is paid exactly like one who used
	// every field. See profile_award.go for the full enumeration.
	s.awardProfile(context.Background(), userID)

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

	// Write path 4 of 8. preferences.onboarding_completed is a completion check,
	// and UpdatePreferences (repository.go:47) is a DIFFERENT repository method
	// from SaveUser — this is the path that proves a SaveUser hook would have
	// been incomplete. A student who finished onboarding here and touched nothing
	// else moves this field and no other.
	s.awardProfile(context.Background(), userID)

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

func (s *Service) GetInstitutionPreferences(userID uint) (*Preferences, error) {
	institution, err := s.repo.FindInstitutionUserByID(userID)
	if err != nil {
		return nil, errors.New("Institution not found")
	}

	return institution.Preferences, nil
}

func (s *Service) InstitutionRegister(req InstitutionRegisterRequest) (*RegisterResponse, error) {
	if s.emailExistsAcrossTypes(req.Email) {
		return nil, errors.New("An account with this email already exists")
	}

	_, err := s.repo.FindInstitutionUserByRegistrationNumber(req.RegistrationNumber)
	if req.RegistrationNumber != "" && err == nil {
		return nil, errors.New("Institution with this registration number already exists")
	}

	institutionUser := InstitutionUser{
		InstitutionName:          req.InstitutionName,
		RegistrationNumber:       nullableString(req.RegistrationNumber),
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

	// Staged with the invite code for the same reason Register stages it: this
	// function does not create the row, VerifyOTP does. See Register's comment.
	utils.StoreOTPWithReferral(req.Email, otp, "", institutionUser, req.ReferralCode)

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

// InstitutionGoogleLoginOrRegister is user-creation path 5 of 6, and it writes to
// institution_users rather than users. See internal/auth/referral.go.
func (s *Service) InstitutionGoogleLoginOrRegister(googleID, email, name, referralCode string) (*InstitutionUser, string, error) {
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
			RegistrationNumber: nullableString("GOOGLE-" + googleID),
			Email:              email,
			GoogleID:           &googleID,
			Role:               "institution",
		}
		if err := s.repo.CreateInstitutionUser(instUser); err != nil {
			return nil, "", errors.New("Failed to create institution account: " + err.Error())
		}

		// ── user-creation path 5 of 6 ────────────────────────────────────────────
		// Creation branch only, for the reason given at path 4.
		s.applyAttribution(context.Background(), ReferralSubject{
			Kind: ReferralSubjectInstitution,
			ID:   instUser.ID,
			Code: referralCode,
			Path: "google_institution",
		})
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

	// Staged with the invite code, as above.
	utils.StoreOTPWithReferral(req.Email, otp, "", providerUser, req.ReferralCode)

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

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountApproved,
			Recipients: []notification.Ref{{Type: "provider", ID: provider.ID}},
		})
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

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountRejected,
			Recipients: []notification.Ref{{Type: "provider", ID: provider.ID}},
		})
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
	email := fmt.Sprintf("%s_%d@institution.edu.np", slug, time.Now().UnixMilli())

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
		"faqs_data":       req.FaqsData,
		"brochure_data":   req.BrochureData,
	}

	profileJSON, err := json.Marshal(profileData)
	if err != nil {
		return nil, errors.New("Failed to marshal profile data")
	}
	profileStr := string(profileJSON)

	var uniAffiliations []byte
	if req.UniversityAffiliations != nil {
		uniAffiliations, _ = json.Marshal(req.UniversityAffiliations)
	}

	regNumber := req.RegistrationNumber
	if regNumber == "" {
		regNumber = fmt.Sprintf("ADMIN-%d", time.Now().UnixMilli())
	}

	institutionUser := InstitutionUser{
		InstitutionName:          req.InstitutionName,
		RegistrationNumber:       &regNumber,
		Email:                    email,
		Role:                     "institution",
		Status:                   "approved",
		Level:                    req.Level,
		Affiliation:              req.Affiliation,
		OrganizationType:         req.OrganizationType,
		UniversityID:             &req.UniversityID,
		NonUniversityAffiliation: req.NonUniversityAffiliation,
		UniversityAffiliations:   uniAffiliations,
		Verified:                 false,
		Claimed:                  false,
		ProfileStatus:            "published",
		District:                 req.Location,
		WebsiteURL:               req.Website,
		LogoURL:                  req.LogoURL,
		BannerURL:                req.BannerURL,
		CardImageURL:             req.CardImageURL,
		About:                    req.About,
		Vision:                   req.Vision,
		Mission:                  req.Mission,
		ContactEmail:             req.ContactEmail,
		ContactPhone:             req.ContactPhone,
		MapURL:                   req.MapURL,
		FacebookURL:              req.FacebookURL,
		InstagramURL:             req.InstagramURL,
		TiktokURL:                req.TiktokURL,
		YoutubeURL:               req.YoutubeURL,
		LinkedinURL:              req.LinkedinURL,
		ProfileData:              &profileStr,
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

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountApproved,
			Recipients: []notification.Ref{{Type: "institution", ID: institutionUser.ID}},
		})
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

	// UniversityAffiliations is stored as jsonb bytes; decode so the JSON
	// response carries the array as-is rather than a base64 string.
	var uniAffiliations interface{}
	if len(user.UniversityAffiliations) > 0 {
		_ = json.Unmarshal(user.UniversityAffiliations, &uniAffiliations)
	}

	return &InstitutionDetailResponse{
		ID:                       user.ID,
		InstitutionName:          user.InstitutionName,
		Email:                    user.Email,
		RegistrationNumber:       derefString(user.RegistrationNumber),
		Status:                   user.Status,
		Claimed:                  user.Claimed,
		Verified:                 user.Verified,
		Featured:                 user.Featured,
		District:                 user.District,
		WebsiteURL:               user.WebsiteURL,
		LogoURL:                  logoURL,
		BannerURL:                bannerURL,
		About:                    user.About,
		Vision:                   user.Vision,
		Mission:                  user.Mission,
		Level:                    user.Level,
		Affiliation:              user.Affiliation,
		OrganizationType:         user.OrganizationType,
		NonUniversityAffiliation: user.NonUniversityAffiliation,
		UniversityAffiliations:   uniAffiliations,
		ContactEmail:             user.ContactEmail,
		ContactPhone:             user.ContactPhone,
		MapURL:                   user.MapURL,
		FacebookURL:              user.FacebookURL,
		InstagramURL:             user.InstagramURL,
		TiktokURL:                user.TiktokURL,
		YoutubeURL:               user.YoutubeURL,
		LinkedinURL:              user.LinkedinURL,
		CardImageURL:             user.CardImageURL,
		UniversityID:             user.UniversityID,
		IsSponsored:              user.IsSponsored,
		Latitude:                 user.Latitude,
		Longitude:                user.Longitude,
		ProfileData:              profileData,
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
	if req.RegistrationNumber != "" {
		user.RegistrationNumber = &req.RegistrationNumber
	}
	if req.Email != "" {
		user.Email = req.Email
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
	if req.OrganizationType != "" {
		user.OrganizationType = req.OrganizationType
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
	if req.CardImageURL != "" {
		user.CardImageURL = req.CardImageURL
	}
	if req.ContactEmail != "" {
		user.ContactEmail = req.ContactEmail
	}
	if req.ContactPhone != "" {
		user.ContactPhone = req.ContactPhone
	}
	if req.MapURL != "" {
		user.MapURL = req.MapURL
	}
	if req.FacebookURL != "" {
		user.FacebookURL = req.FacebookURL
	}
	if req.InstagramURL != "" {
		user.InstagramURL = req.InstagramURL
	}
	if req.TiktokURL != "" {
		user.TiktokURL = req.TiktokURL
	}
	if req.YoutubeURL != "" {
		user.YoutubeURL = req.YoutubeURL
	}
	if req.LinkedinURL != "" {
		user.LinkedinURL = req.LinkedinURL
	}
	if req.NonUniversityAffiliation != nil {
		user.NonUniversityAffiliation = *req.NonUniversityAffiliation
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

	if err := s.repo.CreateOrUpdateSubscription(sub); err != nil {
		return err
	}

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventPaymentSubscriptionRecorded,
			Recipients: []notification.Ref{{Type: "institution", ID: institutionID}},
			Data:       map[string]any{"plan": remarks},
		})
	}
	return nil
}

func (s *Service) ToggleInstitutionFeatured(institutionID uint) error {
	institution, err := s.repo.FindInstitutionUserByID(institutionID)
	if err != nil {
		return errors.New("Institution not found")
	}

	// Unclaimed institutions cannot gain featured status; legacy unclaimed
	// rows that are already featured may still be toggled OFF.
	if !institution.Claimed && !institution.Featured {
		return ErrUnclaimedFeatured
	}

	institution.Featured = !institution.Featured

	if err := s.repo.UpdateInstitutionUser(institution); err != nil {
		return errors.New("Failed to toggle featured status")
	}

	return nil
}

// ErrUnclaimedFeatured is returned when trying to feature an unclaimed institution.
var ErrUnclaimedFeatured = errors.New("only claimed institutions can be featured")

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

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountApproved,
			Recipients: []notification.Ref{{Type: "institution", ID: institution.ID}},
		})
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
	if req.RegistrationNumber != "" {
		if exists, _ := s.repo.FindInstitutionUserByRegistrationNumber(req.RegistrationNumber); exists != nil {
			return nil, errors.New("Registration number already exists")
		}
	}
	if req.CollegeID > 0 {
		if claimed, _ := s.repo.FindClaimedInstitutionByCollegeID(req.CollegeID); claimed != nil {
			return nil, errors.New("This college has already been claimed")
		}
	}

	institutionUser := InstitutionUser{
		InstitutionName:          req.InstitutionName,
		RegistrationNumber:       nullableString(req.RegistrationNumber),
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
	// Staged with the invite code. This is an unauthenticated public signup route,
	// so it is a real user-creation path — see ClaimRegisterRequest.ReferralCode.
	utils.StoreOTPWithReferral(req.Email, otp, "", institutionUser, req.ReferralCode)

	if notifierInstance != nil {
		audience, _ := notifierInstance.ForRoles(context.Background(), "superadmin", "admin")
		if len(audience) > 0 {
			collegeName := fmt.Sprintf("college #%d", req.CollegeID)
			if c, err := s.repo.FindCollegeByID(req.CollegeID); err == nil && c.Name != "" {
				collegeName = c.Name
			}
			_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventSystemClaimSubmitted,
				Recipients: audience,
				Data:       map[string]any{"college": collegeName, "email": req.Email},
			})
		}
	}

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

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountRejected,
			Recipients: []notification.Ref{{Type: "institution", ID: institution.ID}},
		})
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

func (s *Service) SearchAllInstitutions(filter InstitutionFilter) ([]InstitutionUser, error) {
	users, _, err := s.repo.FindInstitutionUsersFiltered("", filter)
	return users, err
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
	if err := s.repo.UpdateUserStatus(userID, "suspended"); err != nil {
		return err
	}

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountSuspended,
			Recipients: []notification.Ref{{Type: "user", ID: userID}},
		})
	}
	return nil
}

func (s *Service) ReinstateUser(userID uint) error {
	if _, err := s.repo.FindUserByID(userID); err != nil {
		return errors.New("user not found")
	}
	if err := s.repo.UpdateUserStatus(userID, "active"); err != nil {
		return err
	}

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountReinstated,
			Recipients: []notification.Ref{{Type: "user", ID: userID}},
		})
	}
	return nil
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

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountApproved,
			Recipients: []notification.Ref{{Type: "institution", ID: institution.ID}},
		})
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

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountRejected,
			Recipients: []notification.Ref{{Type: "institution", ID: institution.ID}},
		})
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

// ScholarshipProviderGoogleLoginOrRegister is user-creation path 6 of 6, and it
// writes to scholarship_provider_users. See internal/auth/referral.go.
func (s *Service) ScholarshipProviderGoogleLoginOrRegister(googleID, email, name, referralCode string) (*ScholarshipProviderUser, string, error) {
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

		// ── user-creation path 6 of 6 ────────────────────────────────────────────
		// Creation branch only, for the reason given at path 4.
		s.applyAttribution(context.Background(), ReferralSubject{
			Kind: ReferralSubjectProvider,
			ID:   providerUser.ID,
			Code: referralCode,
			Path: "google_provider",
		})
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

	// Write path 5 of 8. "has an education entry" is the twelfth completion
	// check, and it lives in a different table entirely — no user row is touched
	// here, which is the second reason a SaveUser hook would have missed it.
	s.awardProfile(context.Background(), userID)

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

	// Write path 6 of 8. An update cannot change whether an entry EXISTS, so it
	// cannot newly satisfy the education check — but it is called anyway, and
	// deliberately: the award re-reads completion and claims only unclaimed steps,
	// so a redundant call is a no-op, while a MISSING call here would be one more
	// place a student could be short-changed after a backfill or a data fix.
	s.awardProfile(context.Background(), userID)

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
	if err := s.repo.DeleteEducationEntry(entryID, userID); err != nil {
		return err
	}

	// Write path 7 of 8, and the one that proves the award must tolerate a DECREASE.
	//
	// Deleting a student's last education entry drops completion by one twelfth.
	// Nothing is awarded here — no new threshold is crossed in the upward
	// direction — and that is the correct outcome, not an oversight. Awards are
	// never revoked and never re-paid: the ladder's claim rows are permanent, so a
	// student who drops to 20% and climbs back to 100% collects nothing the second
	// time. Calling the award on the downward path is what makes that true rather
	// than accidental, and it costs one claim insert that conflicts.
	s.awardProfile(context.Background(), userID)
	return nil
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
	if err := s.repo.SaveUser(user); err != nil {
		return err
	}

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountTotpChanged,
			Recipients: []notification.Ref{{Type: "user", ID: userID}},
		})
	}
	return nil
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
	if err := s.repo.SaveUser(user); err != nil {
		return err
	}

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountTotpChanged,
			Recipients: []notification.Ref{{Type: "user", ID: userID}},
		})
	}
	return nil
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

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountDeletionScheduled,
			Recipients: []notification.Ref{{Type: "user", ID: userID}},
		})
	}
	return &deletionDate, nil
}

func (s *Service) CancelDeletion(userID uint) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errors.New("User not found")
	}
	user.ScheduledDeletionAt = nil
	if err := s.repo.SaveUser(user); err != nil {
		return err
	}

	if notifierInstance != nil {
		_ = notifierInstance.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventAccountDeletionCancelled,
			Recipients: []notification.Ref{{Type: "user", ID: userID}},
		})
	}
	return nil
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

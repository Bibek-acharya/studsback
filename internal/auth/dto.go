package auth

type RegisterRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required,min=6"`
	FirstName      string `json:"first_name" binding:"required"`
	LastName       string `json:"last_name" binding:"required"`
	Role           string `json:"role"`
	EducationLevel string `json:"education_level"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type SendOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type VerifyOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required"`
}

type SavePreferencesRequest struct {
	PreferenceRole string                 `json:"preference_role" binding:"required"`
	PreferenceFlow string                 `json:"preference_flow" binding:"required"`
	Preferences    map[string]interface{} `json:"preferences" binding:"required"`
}

type UpdateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type RegisterResponse struct {
	Email       string `json:"email"`
	RequiresOTP bool   `json:"requires_otp"`
}

type LoginResponse struct {
	User  interface{} `json:"user"`
	Token string      `json:"token"`
}

type ProfileResponse struct {
	ID          uint         `json:"id"`
	Email       string       `json:"email"`
	FirstName   string       `json:"first_name"`
	LastName    string       `json:"last_name"`
	Role        string       `json:"role"`
	GoogleID    *string      `json:"google_id"`
	Preferences *Preferences `json:"preferences,omitempty"`
}

type PreferencesResponse struct {
	User User `json:"user"`
}

type InstitutionRegisterRequest struct {
	InstitutionName    string `json:"institution_name" binding:"required"`
	RegistrationNumber string `json:"registration_number" binding:"required"`
	Email              string `json:"email" binding:"required,email"`
	Password           string `json:"password" binding:"required,min=6"`
}

type InstitutionLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ScholarshipProviderRegisterRequest struct {
	ProviderName       string `json:"provider_name" binding:"required"`
	RegistrationNumber string `json:"registration_number" binding:"required"`
	Email              string `json:"email" binding:"required,email"`
	Password           string `json:"password" binding:"required,min=6"`
}

type ScholarshipProviderLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type SuperadminRegisterRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=6"`
	FirstName  string `json:"first_name" binding:"required"`
	LastName   string `json:"last_name" binding:"required"`
	AccessCode string `json:"access_code" binding:"required"`
}

type SuperadminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

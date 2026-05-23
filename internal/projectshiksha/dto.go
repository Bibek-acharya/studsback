package projectshiksha

// CreateApplicationRequest represents the request to create a new application
type CreateApplicationRequest struct {
	// Personal Details
	FullName         string `json:"full_name" binding:"required"`
	Gender           string `json:"gender" binding:"required"`
	DOBBS            string `json:"dob_bs" binding:"required"`
	DOBAD            string `json:"dob_ad" binding:"required"`
	Age              int    `json:"age" binding:"required"`
	Phone            string `json:"phone" binding:"required"`
	Email            string `json:"email"`
	SEESchoolType    string `json:"see_school_type" binding:"required"`
	OtherSchoolType  string `json:"other_school_type"`
	SchoolName       string `json:"school_name" binding:"required"`
	
	// Address
	PermProvince     string `json:"perm_province" binding:"required"`
	PermDistrict     string `json:"perm_district" binding:"required"`
	PermMunicipality string `json:"perm_municipality" binding:"required"`
	PermWard         int    `json:"perm_ward" binding:"required"`
	PermTole         string `json:"perm_tole"`
	TempProvince     string `json:"temp_province" binding:"required"`
	TempDistrict     string `json:"temp_district" binding:"required"`
	TempMunicipality string `json:"temp_municipality" binding:"required"`
	TempWard         int    `json:"temp_ward" binding:"required"`
	TempTole         string `json:"temp_tole"`
	
	// Family
	GuardianName     string `json:"guardian_name" binding:"required"`
	GuardianPhone    string `json:"guardian_phone" binding:"required"`
	GuardianEmail    string `json:"guardian_email"`
	FatherOccupation string `json:"father_occupation"`
	MotherOccupation string `json:"mother_occupation"`
	FamilyIncome     int    `json:"family_income"`
	FamilyMembers    int    `json:"family_members"`
}

// UpdateApplicationRequest represents the request to update an application
type UpdateApplicationRequest struct {
	Status string `json:"status" binding:"required"` // submitted, under_review, accepted, rejected
}

// PaymentRequest represents the payment submission
type PaymentRequest struct {
	ApplicationID uint    `json:"application_id" binding:"required"`
	PaymentMethod string  `json:"payment_method" binding:"required"` // esewa, khalti, bank
	Amount        float64 `json:"amount" binding:"required"`
	TransactionID string  `json:"transaction_id"`
}

// VerifyPaymentRequest represents the request to verify a bank payment
type VerifyPaymentRequest struct {
	PaymentID uint   `json:"payment_id" binding:"required"`
	Status    string `json:"status" binding:"required"` // verified, rejected
}

// ApplicationResponse represents the application response
type ApplicationResponse struct {
	ID               uint    `json:"id"`
	FullName         string  `json:"full_name"`
	Phone            string  `json:"phone"`
	Email            string  `json:"email"`
	PhotoURL         string  `json:"photo_url"`
	SEESchoolType    string  `json:"see_school_type"`
	SchoolName       string  `json:"school_name"`
	PermDistrict     string  `json:"perm_district"`
	GuardianName     string  `json:"guardian_name"`
	GuardianPhone    string  `json:"guardian_phone"`
	FamilyIncome     int     `json:"family_income"`
	PaymentStatus    string  `json:"payment_status"`
	PaymentMethod    string  `json:"payment_method"`
	RollNumber       string  `json:"roll_number"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
}

// PaymentResponse represents the payment response
type PaymentResponse struct {
	ID            uint    `json:"id"`
	ApplicationID uint    `json:"application_id"`
	PaymentMethod string  `json:"payment_method"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	TransactionID string  `json:"transaction_id"`
	ScreenshotURL string  `json:"screenshot_url"`
	CreatedAt     string  `json:"created_at"`
}

// AdmitCardResponse represents the admit card data
type AdmitCardResponse struct {
	RollNumber   string `json:"roll_number"`
	FullName     string `json:"full_name"`
	PhotoURL     string `json:"photo_url"`
	ExamDate     string `json:"exam_date"`
	ExamTime     string `json:"exam_time"`
	ExamCenter   string `json:"exam_center"`
}

// ApplicationListResponse represents a paginated list of applications
type ApplicationListResponse struct {
	Applications []ApplicationResponse `json:"applications"`
	Total        int64                 `json:"total"`
	Page         int                   `json:"page"`
	Limit        int                   `json:"limit"`
}

// eSewa Payment Types

type EsewaInitiateRequest struct {
	ApplicationID uint    `json:"application_id" binding:"required"`
	Amount        float64 `json:"amount" binding:"required"`
}

type EsewaInitiateResponse struct {
	Amount          string `json:"amount"`
	TaxAmount       string `json:"tax_amount"`
	TotalAmount     string `json:"total_amount"`
	TransactionUUID string `json:"transaction_uuid"`
	ProductCode     string `json:"product_code"`
	Signature       string `json:"signature"`
	SuccessURL      string `json:"success_url"`
	FailureURL      string `json:"failure_url"`
	GatewayURL      string `json:"gateway_url"`
}

type EsewaVerifyRequest struct {
	ApplicationID   uint   `json:"application_id" binding:"required"`
	TransactionUUID string `json:"transaction_uuid" binding:"required"`
	TotalAmount     string `json:"total_amount" binding:"required"`
	RefID           string `json:"ref_id" binding:"required"`
}

// StatsResponse represents application statistics
type StatsResponse struct {
	TotalApplications   int64 `json:"total_applications"`
	PendingPayments     int64 `json:"pending_payments"`
	CompletedPayments   int64 `json:"completed_payments"`
	UnderReview         int64 `json:"under_review"`
	Accepted            int64 `json:"accepted"`
	Rejected            int64 `json:"rejected"`
}

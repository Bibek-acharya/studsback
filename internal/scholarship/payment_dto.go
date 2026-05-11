package scholarship

type PaymentRequest struct {
	Method         string  `json:"method" binding:"required"`
	Amount         float64 `json:"amount" binding:"required"`
	ApplicationID  uint    `json:"application_id"`
	TransactionID  string  `json:"transaction_id"`
}

type PaymentResponse struct {
	ID              uint    `json:"id"`
	ApplicationID  uint    `json:"application_id"`
	ScholarshipID  uint    `json:"scholarship_id"`
	Method         string  `json:"method"`
	Amount         float64 `json:"amount"`
	Status         string  `json:"status"`
	ReceiptURL     string  `json:"receipt_url,omitempty"`
	TransactionID string  `json:"transaction_id,omitempty"`
	PaidAt         string  `json:"paid_at,omitempty"`
}

type BankReceiptUploadRequest struct {
	ReceiptImage string `json:"receipt_image" binding:"required"`
}

type ApprovePaymentRequest struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}

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
	ProductCode     string `json:"product_code" binding:"required"`
	Status          string `json:"status" binding:"required"`
	TransactionCode string `json:"transaction_code"`
	RefID           string `json:"ref_id"`
}

// EsewaCallbackResponse maps eSewa's callback data parameter
type EsewaCallbackResponse struct {
	TransactionCode   string `json:"transaction_code"`
	Status            string `json:"status"`
	TotalAmount       string `json:"total_amount"`
	TransactionUUID   string `json:"transaction_uuid"`
	ProductCode       string `json:"product_code"`
	SignedFieldNames  string `json:"signed_field_names"`
	Signature         string `json:"signature"`
}
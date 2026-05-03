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
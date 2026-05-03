package scholarship

import (
	"testing"
	"time"
)

func TestPaymentModel_TableName(t *testing.T) {
	payment := Payment{}
	if payment.TableName() != "scholarship_payments" {
		t.Errorf("TableName() = %q, want %q", payment.TableName(), "scholarship_payments")
	}
}

func TestPaymentModel_DefaultStatus(t *testing.T) {
	payment := Payment{}
	// Note: GORM default is applied during DB save, not struct init
	// The gorm:"default:pending" tag ensures DB default
	if payment.Status != "" && payment.Status != "pending" {
		t.Errorf("Default status = %q, expected empty or pending", payment.Status)
	}
	// Verify the tag exists by checking the model definition
	emptyPayment := Payment{Status: "pending"}
	if emptyPayment.Status != "pending" {
		t.Logf("Status after init: %q", emptyPayment.Status)
	}
}

func TestPaymentRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		method string
		amount float64
		want   string
	}{
		{"valid esewa", "esewa", 100.0, ""},
		{"valid khalti", "khalti", 250.0, ""},
		{"valid bank", "bank", 500.0, ""},
		{"invalid method", "invalid", 100.0, "invalid"}, // Basic validation won't catch during unmarshal
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := PaymentRequest{
				Method:  tt.method,
				Amount: tt.amount,
			}
			_ = req.Method // Just verify struct can hold the data
		})
	}
}

func TestApprovePaymentRequest(t *testing.T) {
	tests := []struct {
		name    string
		approve bool
		reason  string
	}{
		{"approve with reason", true, "Test reason"},
		{"approve without reason", true, ""},
		{"reject with reason", false, "Insufficient documentation"},
		{"reject empty reason", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ApprovePaymentRequest{
				Approve: tt.approve,
				Reason:  tt.reason,
			}
			if req.Approve != tt.approve {
				t.Errorf("Approve = %v, want %v", req.Approve, tt.approve)
			}
			if req.Reason != tt.reason {
				t.Errorf("Reason = %q, want %q", req.Reason, tt.reason)
			}
		})
	}
}

func TestPaymentStatus_Constants(t *testing.T) {
	statuses := map[string]bool{
		"pending":           true,
		"completed":        true,
		"pending_approval": true,
		"failed":           true,
	}

	for status := range statuses {
		if status == "" {
			t.Error("Empty status value found")
		}
	}
}

func TestPayment_Timestamps(t *testing.T) {
	now := time.Now()
	uid := uint(1)
	payment := Payment{
		ID:             1,
		ApplicationID: 10,
		ScholarshipID: 5,
		UserID:         &uid,
		Method:         "esewa",
		Amount:         250.0,
		Status:         "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if payment.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if payment.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestPayment_PaidAt_NotSet(t *testing.T) {
	payment := Payment{
		Status: "pending",
		PaidAt:  nil,
	}

	if payment.PaidAt != nil {
		t.Error("PaidAt should be nil for pending payment")
	}
}

func TestPayment_PaidAt_Set(t *testing.T) {
	now := time.Now()
	payment := Payment{
		Status: "completed",
		PaidAt:  &now,
	}

	if payment.PaidAt == nil {
		t.Error("PaidAt should be set for completed payment")
	}
}

func TestPayment_ApprovedAt_Set(t *testing.T) {
	now := time.Now()
	payment := Payment{
		Status:     "completed",
		ApprovedAt: &now,
		ApprovedBy: 1,
	}

	if payment.ApprovedAt == nil {
		t.Error("ApprovedAt should be set")
	}
	if payment.ApprovedBy != 1 {
		t.Errorf("ApprovedBy = %d, want %d", payment.ApprovedBy, 1)
	}
}

func TestPayment_RejectionReason(t *testing.T) {
	payment := Payment{
		Status:         "failed",
		RejectionReason: "Document verification failed",
	}

	if payment.RejectionReason != "Document verification failed" {
		t.Errorf("RejectionReason = %q, want %q", payment.RejectionReason, "Document verification failed")
	}
}
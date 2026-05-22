package scholarship

import (
	"time"
)

type Payment struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	ApplicationID  uint      `json:"application_id" gorm:"not null"`
	ScholarshipID  uint      `json:"scholarship_id" gorm:"not null"`
	UserID         *uint     `json:"user_id,omitempty" gorm:"index"`
	Method         string    `json:"method" gorm:"not null"`
	Amount         float64   `json:"amount" gorm:"not null"`
	Status         string    `json:"status" gorm:"default:pending"`
	ReceiptURL     string    `json:"receipt_url"`
	TransactionID string    `json:"transaction_id"`
	PaidAt         *time.Time `json:"paid_at"`
	ApprovedAt     *time.Time `json:"approved_at"`
	ApprovedBy     uint      `json:"approved_by" gorm:"index"`
	RejectionReason string    `json:"rejection_reason"`
	DisputeStatus  string    `json:"dispute_status" gorm:"default:pending"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Application  ScholarshipApplication `json:"-" gorm:"foreignKey:ApplicationID"`
	Scholarship Scholarship            `json:"-" gorm:"foreignKey:ScholarshipID"`
	User       User                   `json:"-" gorm:"foreignKey:UserID"`
}

func (Payment) TableName() string {
	return "scholarship_payments"
}
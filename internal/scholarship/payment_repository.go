package scholarship

import (
	"fmt"

	"gorm.io/gorm"
)

type PaymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(payment *Payment) error {
	if err := r.db.Create(payment).Error; err != nil {
		return fmt.Errorf("failed to create payment: %w", err)
	}
	return nil
}

func (r *PaymentRepository) FindByID(id uint) (*Payment, error) {
	var payment Payment
	err := r.db.Preload("Application").Preload("Scholarship").Preload("User").First(&payment, id).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find payment by id %d: %w", id, err)
	}
	return &payment, nil
}

func (r *PaymentRepository) FindByApplicationID(appID uint) (*Payment, error) {
	var payment Payment
	err := r.db.Where("application_id = ?", appID).First(&payment).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find payment by application id %d: %w", appID, err)
	}
	return &payment, nil
}

func (r *PaymentRepository) Update(payment *Payment) error {
	if err := r.db.Save(payment).Error; err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}
	return nil
}

func (r *PaymentRepository) FindByTransactionID(txnID string) (*Payment, error) {
	var payment Payment
	err := r.db.Where("transaction_id = ?", txnID).First(&payment).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find payment by transaction id %s: %w", txnID, err)
	}
	return &payment, nil
}

func (r *PaymentRepository) FindByScholarshipID(scholarshipID uint) ([]Payment, error) {
	var payments []Payment
	err := r.db.Where("scholarship_id = ?", scholarshipID).Find(&payments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find payments by scholarship id %d: %w", scholarshipID, err)
	}
	return payments, nil
}

func (r *PaymentRepository) FindPendingEsewa() ([]Payment, error) {
	var payments []Payment
	err := r.db.Where("method = ? AND status = ?", "esewa", "pending").Find(&payments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find pending eSewa payments: %w", err)
	}
	return payments, nil
}

func (r *PaymentRepository) FindCompletedEsewa() ([]Payment, error) {
	var payments []Payment
	err := r.db.Where("method = ? AND status = ?", "esewa", "completed").
		Find(&payments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find completed eSewa payments: %w", err)
	}
	return payments, nil
}

func (r *PaymentRepository) FindPendingApplicationsWithRollNumber() ([]ScholarshipApplication, error) {
	var apps []ScholarshipApplication
	err := r.db.Where("status = ?", "pending").
		Where("roll_number IS NOT NULL AND roll_number != ''").
		Find(&apps).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find pending applications: %w", err)
	}
	return apps, nil
}
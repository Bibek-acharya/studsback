package projectshiksha

import (
	"gorm.io/gorm"
)

// Repository handles database operations for Project Shiksha
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new repository instance
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateApplication creates a new scholarship application
func (r *Repository) CreateApplication(app *ShikshaApplication) error {
	return r.db.Create(app).Error
}

// GetApplicationByID retrieves an application by ID
func (r *Repository) GetApplicationByID(id uint) (*ShikshaApplication, error) {
	var app ShikshaApplication
	err := r.db.First(&app, id).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// GetApplicationByPhone retrieves an application by phone number
func (r *Repository) GetApplicationByPhone(phone string) (*ShikshaApplication, error) {
	var app ShikshaApplication
	err := r.db.Where("phone = ?", phone).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// GetApplicationByRollNumber retrieves an application by roll number
func (r *Repository) GetApplicationByRollNumber(rollNumber string) (*ShikshaApplication, error) {
	var app ShikshaApplication
	err := r.db.Where("roll_number = ?", rollNumber).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// UpdateApplication updates an application
func (r *Repository) UpdateApplication(app *ShikshaApplication) error {
	return r.db.Save(app).Error
}

// UpdateApplicationStatus updates only the status field
func (r *Repository) UpdateApplicationStatus(id uint, status string) error {
	return r.db.Model(&ShikshaApplication{}).Where("id = ?", id).Update("status", status).Error
}

// UpdatePaymentStatus updates the payment status
func (r *Repository) UpdatePaymentStatus(id uint, status string) error {
	return r.db.Model(&ShikshaApplication{}).Where("id = ?", id).Update("payment_status", status).Error
}

// UpdateRollNumber updates the roll number for an application
func (r *Repository) UpdateRollNumber(id uint, rollNumber string) error {
	return r.db.Model(&ShikshaApplication{}).Where("id = ?", id).Update("roll_number", rollNumber).Error
}

// ListApplications retrieves a paginated list of applications
func (r *Repository) ListApplications(page, limit int, status, paymentStatus string) ([]ShikshaApplication, int64, error) {
	var apps []ShikshaApplication
	var total int64

	query := r.db.Model(&ShikshaApplication{})
	
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if paymentStatus != "" {
		query = query.Where("payment_status = ?", paymentStatus)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&apps).Error; err != nil {
		return nil, 0, err
	}

	return apps, total, nil
}

// DeleteApplication deletes an application by ID
func (r *Repository) DeleteApplication(id uint) error {
	return r.db.Delete(&ShikshaApplication{}, id).Error
}

// GetStats retrieves application statistics
func (r *Repository) GetStats() (map[string]int64, error) {
	stats := make(map[string]int64)
	
	var total int64
	r.db.Model(&ShikshaApplication{}).Count(&total)
	stats["total"] = total

	var pendingPayments int64
	r.db.Model(&ShikshaApplication{}).Where("payment_status = ?", "pending").Count(&pendingPayments)
	stats["pending_payments"] = pendingPayments

	var completedPayments int64
	r.db.Model(&ShikshaApplication{}).Where("payment_status = ?", "completed").Count(&completedPayments)
	stats["completed_payments"] = completedPayments

	var underReview int64
	r.db.Model(&ShikshaApplication{}).Where("status = ?", "under_review").Count(&underReview)
	stats["under_review"] = underReview

	var accepted int64
	r.db.Model(&ShikshaApplication{}).Where("status = ?", "accepted").Count(&accepted)
	stats["accepted"] = accepted

	var rejected int64
	r.db.Model(&ShikshaApplication{}).Where("status = ?", "rejected").Count(&rejected)
	stats["rejected"] = rejected

	return stats, nil
}

// Payment Repository Methods

// CreatePayment creates a new payment record
func (r *Repository) CreatePayment(payment *ShikshaPayment) error {
	return r.db.Create(payment).Error
}

// GetPaymentByID retrieves a payment by ID
func (r *Repository) GetPaymentByID(id uint) (*ShikshaPayment, error) {
	var payment ShikshaPayment
	err := r.db.First(&payment, id).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// GetPaymentByApplicationID retrieves payment by application ID
func (r *Repository) GetPaymentByApplicationID(appID uint) (*ShikshaPayment, error) {
	var payment ShikshaPayment
	err := r.db.Where("application_id = ?", appID).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// GetPaymentByTransactionID retrieves payment by transaction ID
func (r *Repository) GetPaymentByTransactionID(txnID string) (*ShikshaPayment, error) {
	var payment ShikshaPayment
	err := r.db.Where("transaction_id = ?", txnID).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// UpdatePayment updates a payment
func (r *Repository) UpdatePayment(payment *ShikshaPayment) error {
	return r.db.Save(payment).Error
}

// ListPayments retrieves payments for an application
func (r *Repository) ListPayments(applicationID uint) ([]ShikshaPayment, error) {
	var payments []ShikshaPayment
	err := r.db.Where("application_id = ?", applicationID).Order("created_at DESC").Find(&payments).Error
	return payments, err
}

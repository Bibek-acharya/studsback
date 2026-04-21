package projectshiksha

import (
	"fmt"
	"math/rand"
	"time"
)

// Service handles business logic for Project Shiksha
type Service struct {
	repo *Repository
}

// NewService creates a new service instance
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateApplication creates a new scholarship application
func (s *Service) CreateApplication(req CreateApplicationRequest) (*ShikshaApplication, error) {
	// Check if phone number already exists
	existing, _ := s.repo.GetApplicationByPhone(req.Phone)
	if existing != nil {
		return nil, fmt.Errorf("application with this phone number already exists")
	}

	app := &ShikshaApplication{
		FullName:         req.FullName,
		Gender:           req.Gender,
		DOBBS:            req.DOBBS,
		DOBAD:            req.DOBAD,
		Age:              req.Age,
		Phone:            req.Phone,
		Email:            req.Email,
		SEESchoolType:    req.SEESchoolType,
		OtherSchoolType:  req.OtherSchoolType,
		SchoolName:       req.SchoolName,
		PermProvince:     req.PermProvince,
		PermDistrict:     req.PermDistrict,
		PermMunicipality: req.PermMunicipality,
		PermWard:         req.PermWard,
		PermTole:         req.PermTole,
		TempProvince:     req.TempProvince,
		TempDistrict:     req.TempDistrict,
		TempMunicipality: req.TempMunicipality,
		TempWard:         req.TempWard,
		TempTole:         req.TempTole,
		GuardianName:     req.GuardianName,
		GuardianPhone:    req.GuardianPhone,
		GuardianEmail:    req.GuardianEmail,
		FatherOccupation: req.FatherOccupation,
		MotherOccupation: req.MotherOccupation,
		FamilyIncome:     req.FamilyIncome,
		FamilyMembers:    req.FamilyMembers,
		PaymentStatus:    "pending",
		Status:           "submitted",
	}

	if err := s.repo.CreateApplication(app); err != nil {
		return nil, err
	}

	return app, nil
}

// GetApplication retrieves an application by ID
func (s *Service) GetApplication(id uint) (*ShikshaApplication, error) {
	return s.repo.GetApplicationByID(id)
}

// GetApplicationByRollNumber retrieves an application by roll number
func (s *Service) GetApplicationByRollNumber(rollNumber string) (*ShikshaApplication, error) {
	return s.repo.GetApplicationByRollNumber(rollNumber)
}

// ListApplications retrieves a paginated list of applications
func (s *Service) ListApplications(page, limit int, status, paymentStatus string) ([]ShikshaApplication, int64, error) {
	return s.repo.ListApplications(page, limit, status, paymentStatus)
}

// UpdateApplicationStatus updates the status of an application
func (s *Service) UpdateApplicationStatus(id uint, status string) error {
	validStatuses := map[string]bool{
		"submitted":     true,
		"under_review":  true,
		"accepted":      true,
		"rejected":      true,
	}
	
	if !validStatuses[status] {
		return fmt.Errorf("invalid status")
	}
	
	return s.repo.UpdateApplicationStatus(id, status)
}

// ProcessPayment processes a payment for an application
func (s *Service) ProcessPayment(appID uint, method string, amount float64, transactionID string) (*ShikshaPayment, error) {
	app, err := s.repo.GetApplicationByID(appID)
	if err != nil {
		return nil, fmt.Errorf("application not found")
	}

	// Create payment record
	payment := &ShikshaPayment{
		ApplicationID: appID,
		PaymentMethod: method,
		Amount:        amount,
		Status:        "pending",
		TransactionID: transactionID,
	}

	if err := s.repo.CreatePayment(payment); err != nil {
		return nil, err
	}

	// Update application payment info
	app.PaymentMethod = method
	app.PaymentAmount = amount
	
	// For eSewa and Khalti, mark as completed immediately (in real scenario, this would be after webhook)
	// For bank transfer, keep pending until verification
	if method == "esewa" || method == "khalti" {
		app.PaymentStatus = "completed"
		payment.Status = "completed"
		now := time.Now()
		app.PaymentVerifiedAt = &now
		
		// Generate roll number on successful payment
		if app.RollNumber == "" {
			rollNumber := s.generateRollNumber()
			app.RollNumber = rollNumber
			s.repo.UpdateRollNumber(appID, rollNumber)
		}
	} else if method == "bank" {
		app.PaymentStatus = "pending"
	}

	if err := s.repo.UpdateApplication(app); err != nil {
		return nil, err
	}

	if payment.Status != "pending" {
		s.repo.UpdatePayment(payment)
	}

	return payment, nil
}

// VerifyBankPayment verifies a bank payment
func (s *Service) VerifyBankPayment(paymentID uint, verified bool, verifiedBy uint) error {
	payment, err := s.repo.GetPaymentByID(paymentID)
	if err != nil {
		return fmt.Errorf("payment not found")
	}

	app, err := s.repo.GetApplicationByID(payment.ApplicationID)
	if err != nil {
		return fmt.Errorf("application not found")
	}

	now := time.Now()
	
	if verified {
		payment.Status = "verified"
		app.PaymentStatus = "completed"
		app.PaymentVerifiedAt = &now
		
		// Generate roll number on successful verification
		if app.RollNumber == "" {
			rollNumber := s.generateRollNumber()
			app.RollNumber = rollNumber
			s.repo.UpdateRollNumber(app.ID, rollNumber)
		}
	} else {
		payment.Status = "rejected"
		app.PaymentStatus = "failed"
	}

	payment.VerifiedAt = &now
	payment.VerifiedBy = &verifiedBy

	if err := s.repo.UpdatePayment(payment); err != nil {
		return err
	}

	return s.repo.UpdateApplication(app)
}

// UploadPaymentScreenshot handles bank payment screenshot upload
func (s *Service) UploadPaymentScreenshot(paymentID uint, screenshotURL string) error {
	payment, err := s.repo.GetPaymentByID(paymentID)
	if err != nil {
		return fmt.Errorf("payment not found")
	}

	payment.ScreenshotURL = screenshotURL
	return s.repo.UpdatePayment(payment)
}

// GetStats retrieves application statistics
func (s *Service) GetStats() (StatsResponse, error) {
	stats, err := s.repo.GetStats()
	if err != nil {
		return StatsResponse{}, err
	}

	return StatsResponse{
		TotalApplications: stats["total"],
		PendingPayments:   stats["pending_payments"],
		CompletedPayments: stats["completed_payments"],
		UnderReview:       stats["under_review"],
		Accepted:          stats["accepted"],
		Rejected:          stats["rejected"],
	}, nil
}

// GetAdmitCard retrieves admit card information
func (s *Service) GetAdmitCard(rollNumber string) (*AdmitCardResponse, error) {
	app, err := s.repo.GetApplicationByRollNumber(rollNumber)
	if err != nil {
		return nil, fmt.Errorf("application not found")
	}

	if app.PaymentStatus != "completed" {
		return nil, fmt.Errorf("payment not completed")
	}

	return &AdmitCardResponse{
		RollNumber: app.RollNumber,
		FullName:   app.FullName,
		PhotoURL:   app.PhotoURL,
		ExamDate:   "15th Baisakh, 2082",
		ExamTime:   "8:00 AM - 11:00 AM",
		ExamCenter: "Kathmandu District Main Center",
	}, nil
}

// generateRollNumber generates a unique roll number
func (s *Service) generateRollNumber() string {
	// Format: PS-XXXX where XXXX is a random 4-digit number
	rand.Seed(time.Now().UnixNano())
	number := rand.Intn(9000) + 1000
	return fmt.Sprintf("PS-%d", number)
}

// UpdateDocumentURLs updates the document URLs for an application
func (s *Service) UpdateDocumentURLs(appID uint, photoURL, marksheetURL, citizenshipURL string) error {
	app, err := s.repo.GetApplicationByID(appID)
	if err != nil {
		return err
	}

	if photoURL != "" {
		app.PhotoURL = photoURL
	}
	if marksheetURL != "" {
		app.SEEMarksheetURL = marksheetURL
	}
	if citizenshipURL != "" {
		app.CitizenshipURL = citizenshipURL
	}

	return s.repo.UpdateApplication(app)
}

// DeleteApplication deletes an application
func (s *Service) DeleteApplication(id uint) error {
	return s.repo.DeleteApplication(id)
}

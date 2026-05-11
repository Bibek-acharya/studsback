package scholarship

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"studsphere/backend/internal/emailqueue"
)

type Service struct {
	repo       *Repository
	providerDB *gorm.DB
}

type PaymentService struct {
	repo            *PaymentRepository
	scholarshipRepo *Repository
}

func NewPaymentService(db *gorm.DB) *PaymentService {
	return &PaymentService{
		repo:            NewPaymentRepository(db),
		scholarshipRepo: NewRepository(db),
	}
}

func NewService(repo *Repository, providerDB *gorm.DB) *Service {
	return &Service{repo: repo, providerDB: providerDB}
}

func (s *Service) GetScholarships(search, categoryFilter, typeFilter, locationFilter, levelFilter, statusFilter, sortBy, order string) ([]Scholarship, []CategoryResponse, error) {
	scholarships, err := s.repo.FindAll(search, typeFilter, locationFilter, levelFilter, sortBy, order)
	if err != nil {
		return nil, nil, err
	}

	categoryRows, err := s.repo.FindAllCategoryRows()
	if err != nil {
		return nil, nil, err
	}

	categoryDefs := categoryDefinitions()
	categoryCounts := make(map[string]int, len(categoryDefs))
	for _, def := range categoryDefs {
		categoryCounts[def.ID] = 0
	}

	for _, row := range categoryRows {
		status := deriveScholarshipStatus(row.Deadline)
		if statusFilter != "" && statusFilter != status {
			continue
		}
		if statusFilter == "" && !isOpenStatus(status) {
			continue
		}

		if categoryID := mapFieldsToCategoryID(row.ScholarshipType, row.FundingType); categoryID != "" {
			categoryCounts[categoryID]++
		}
	}

	filtered := make([]Scholarship, 0, len(scholarships))
	for _, scholarship := range scholarships {
		status := deriveScholarshipStatus(scholarship.Deadline)
		if statusFilter != "" && statusFilter != status {
			continue
		}

		if categoryFilter != "" {
			requestedCategoryID := normalizeCategoryID(categoryFilter)
			if requestedCategoryID != "" {
				if mapScholarshipToCategoryID(scholarship) != requestedCategoryID {
					continue
				}
			} else {
				categoryText := strings.ToLower(toScholarshipCategory(scholarship))
				if !strings.Contains(categoryText, strings.ToLower(categoryFilter)) {
					continue
				}
			}
		}

		filtered = append(filtered, scholarship)
	}

	categories := make([]CategoryResponse, 0, len(categoryDefs))
	for _, def := range categoryDefs {
		count := categoryCounts[def.ID]
		categories = append(categories, CategoryResponse{
			ID:       def.ID,
			Name:     def.Name,
			Title:    def.Title,
			Count:    count,
			Subtitle: countString(count),
			Desc:     def.Desc,
			Icon:     def.Icon,
			Color:    def.Color,
		})
	}

	return filtered, categories, nil
}

func (s *Service) GetScholarshipByID(id uint) (*Scholarship, error) {
	if id > 10000 {
		providerID := id - 10000
		ps, err := s.repo.FindProviderScholarshipByID(providerID)
		if err != nil {
			return nil, err
		}
		providerName := "Provider"
		imageURL := ""
		if ps.ImageURL != nil {
			imageURL = *ps.ImageURL
		}
		bannerURL := ""
		if ps.BannerBackgroundImageURL != nil {
			bannerURL = *ps.BannerBackgroundImageURL
		}
		return &Scholarship{
			ID:                       id,
			Title:                    ps.Title,
			Provider:                 providerName,
			Location:                 ps.Location,
			Value:                    ps.Value,
			Deadline:                 ps.Deadline,
			DegreeLevel:              ps.DegreeLevel,
			FundingType:              ps.FundingType,
			ScholarshipType:          ps.ScholarshipType,
			Description:              ps.Description,
			ImageURL:                 imageURL,
			BannerBackgroundImageURL: bannerURL,
		}, nil
	}
	return s.repo.FindByID(id)
}

func (s *Service) GetSimilarScholarships(id uint) ([]Scholarship, error) {
	current, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	similar, err := s.repo.FindSimilar(current)
	if err != nil {
		return nil, err
	}

	if len(similar) == 0 {
		similar, err = s.repo.FindFallbackSimilar(current)
		if err != nil {
			return nil, err
		}
	}

	return similar, nil
}

func (s *Service) resolveScholarshipForApplication(scholarshipID uint) (*Scholarship, error) {
	return resolveScholarshipForApplication(s.repo, scholarshipID)
}

type scholarshipApplicationResolver interface {
	FindByProviderScholarshipID(providerScholarshipID uint) (*Scholarship, error)
	FindProviderScholarshipByID(id uint) (*ProviderScholarship, error)
	FindByID(id uint) (*Scholarship, error)
	Create(scholarship *Scholarship) error
}

func resolveScholarshipForApplication(repo scholarshipApplicationResolver, scholarshipID uint) (*Scholarship, error) {
	if scholarshipID <= 10000 {
		return repo.FindByID(scholarshipID)
	}

	providerScholarshipID := scholarshipID - 10000
	scholarship, err := repo.FindByProviderScholarshipID(providerScholarshipID)
	if err == nil {
		if scholarship.ProviderScholarshipID == nil {
			scholarship.ProviderScholarshipID = &providerScholarshipID
		}
		return scholarship, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	providerScholarship, err := repo.FindProviderScholarshipByID(providerScholarshipID)
	if err != nil {
		return nil, err
	}

	scholarship = providerScholarshipToScholarship(providerScholarship)
	scholarship.ProviderScholarshipID = &providerScholarshipID

	if err := repo.Create(scholarship); err != nil {
		return nil, err
	}

	return scholarship, nil
}

func providerScholarshipToScholarship(ps *ProviderScholarship) *Scholarship {
	providerName := "Provider"
	imageURL := ""
	bannerURL := ""
	if ps.ImageURL != nil {
		imageURL = *ps.ImageURL
	}
	if ps.BannerBackgroundImageURL != nil {
		bannerURL = *ps.BannerBackgroundImageURL
	}

	return &Scholarship{
		Title:                    ps.Title,
		Provider:                 providerName,
		ProviderID:               ps.ProviderID,
		Location:                 ps.Location,
		Value:                    ps.Value,
		Deadline:                 ps.Deadline,
		TotalSeats:               ps.TotalSeats,
		AmountPerStudent:         ps.AmountPerStudent,
		ApplicationStartDate:     ps.ApplicationStartDate,
		ResultPublicationDate:    ps.ResultPublicationDate,
		DegreeLevel:              ps.DegreeLevel,
		FundingType:              ps.FundingType,
		ScholarshipType:          ps.ScholarshipType,
		Description:              ps.Description,
		ImageURL:                 imageURL,
		BannerBackgroundImageURL: bannerURL,
		Status:                   ps.Status,
		FieldOfStudy:             ps.FieldOfStudy,
		SelectionProcess:         ps.SelectionProcessSteps,
		EligibilityCriteria:      ps.BasicEligibilityCriteria,
		RequiredDocuments:        ps.RequiredDocuments,
		Timeline:                 ps.Timeline,
		Benefits:                 ps.Benefits,
		FAQs:                     ps.FAQs,
		PaymentConfig:            ps.PaymentConfig,
		ExamCenters:              ps.ExamCenters,
		ExamCentersNew:           ps.ExamCentersNew,

		ProviderName:         ps.ProviderName,
		FundingTypeOther:     ps.FundingTypeOther,
		ScholarshipTypeOther: ps.ScholarshipTypeOther,
		EducationLevel:       ps.EducationLevel,
		EducationLevelOther:  ps.EducationLevelOther,
		ApplyLink:            ps.ApplyLink,
		CoverageArea:         ps.CoverageArea,
		ContactEmail:         ps.ContactEmail,
		PrimaryPhone:         ps.PrimaryPhone,
		SecondaryPhone:       ps.SecondaryPhone,
		WebsiteUrl:           ps.WebsiteUrl,
		OfficeAddress:        ps.OfficeAddress,
		MapUrl:               ps.MapUrl,
		AboutParagraph1:      ps.AboutParagraph1,
		VideoTutorials:       ps.VideoTutorials,
		JourneyTimeline:      ps.JourneyTimeline,
		ScholarshipSectionTitle: ps.ScholarshipSectionTitle,
		ScholarshipSubtitle:     ps.ScholarshipSubtitle,
		ScholarshipDescription1: ps.ScholarshipDescription1,
		ScholarshipDescription2: ps.ScholarshipDescription2,
		ScholarshipTypes:        ps.ScholarshipTypes,
		ScholarshipTypesNew:     ps.ScholarshipTypesNew,
		SelectionRubric:         ps.SelectionRubric,
		SelectionRubricNew:      ps.SelectionRubricNew,
		EligibilitySectionTitle: ps.EligibilitySectionTitle,
		EligibilitySubtitle:     ps.EligibilitySubtitle,
		BasicEligibilityCriteria: ps.BasicEligibilityCriteria,
		FullyFundedCriteria:      ps.FullyFundedCriteria,
		PartiallyFundedCriteria:  ps.PartiallyFundedCriteria,
		SelectionProcessSteps:    ps.SelectionProcessSteps,
		FAQsNew:                 ps.FAQsNew,
		GalleryImages:           ps.GalleryImages,
		GalleryImagesNew:        ps.GalleryImagesNew,
		PartnerGroups:           ps.PartnerGroups,
		PartnerMessages:         ps.PartnerMessages,
		Downloads:               ps.Downloads,
		ExamDate:                ps.ExamDate,
		ExamTime:                ps.ExamTime,
	}
}

func (s *Service) CreateDraftApplication(scholarshipID uint, userID uint) (*ScholarshipApplication, error) {
	scholarship, err := s.resolveScholarshipForApplication(scholarshipID)
	if err != nil {
		return nil, errors.New("scholarship not found")
	}

	application := &ScholarshipApplication{
		ScholarshipID: scholarship.ID,
		UserID:        &userID,
		Status:        ApplicationStatusPendingPayment,
	}

	if err := s.repo.ApplicationCreate(application); err != nil {
		return nil, errors.New("failed to create application")
	}

	if scholarship.ProviderScholarshipID != nil {
		_ = s.repo.CreateProviderApplication(*scholarship.ProviderScholarshipID, application)
	}

	return application, nil
}

func (s *Service) ApplyScholarship(scholarshipID uint, userID *uint, req ScholarshipApplicationRequest) (*ScholarshipApplication, error) {
	scholarship, err := s.resolveScholarshipForApplication(scholarshipID)
	if err != nil {
		return nil, errors.New("scholarship not found")
	}

	if userID != nil && s.repo.ApplicationExists(scholarship.ID, *userID) {
		return nil, errors.New("you have already applied for this scholarship")
	}

	var dobAD time.Time
	if req.DateOfBirthAD != "" {
		dobAD, _ = time.Parse("2006-01-02", req.DateOfBirthAD)
	}

	if req.SEEGPA != "" {
		gpa, err := strconv.ParseFloat(req.SEEGPA, 64)
		if err != nil || gpa < 0 || gpa > 4 {
			return nil, errors.New("SEE GPA must be between 0 and 4")
		}
	}

	application := &ScholarshipApplication{
		ScholarshipID:  scholarship.ID,
		UserID:         userID,
		FullName:       req.FullName,
		Gender:         req.Gender,
		Ethnicity:      req.Ethnicity,
		EthnicityOther: req.EthnicityOther,
		DateOfBirthBS:  req.DateOfBirthBS,
		DateOfBirthAD:  dobAD,
		Age:            req.Age,
		PhoneNumber:    req.PhoneNumber,
		Email:          req.Email,
		PhotoURL:       req.PhotoURL,

		SEEGPA:             req.SEEGPA,
		SchoolType:         req.SchoolType,
		SchoolName:         req.SchoolName,
		SchoolProvince:     req.SchoolProvince,
		SchoolDistrict:     req.SchoolDistrict,
		SchoolMunicipality: req.SchoolMunicipality,
		SchoolTole:         req.SchoolTole,

		PermanentProvince:     req.PermanentProvince,
		PermanentDistrict:     req.PermanentDistrict,
		PermanentMunicipality: req.PermanentMunicipality,
		PermanentWard:         req.PermanentWard,
		PermanentTole:         req.PermanentTole,

		TemporaryProvince:     req.TemporaryProvince,
		TemporaryDistrict:     req.TemporaryDistrict,
		TemporaryMunicipality: req.TemporaryMunicipality,
		TemporaryWard:         req.TemporaryWard,
		TemporaryTole:         req.TemporaryTole,

		GuardianName:          req.GuardianName,
		GuardianPhone:         req.GuardianPhone,
		GuardianEmail:         req.GuardianEmail,
		FatherOccupation:      req.FatherOccupation,
		FatherOccupationOther: req.FatherOccupationOther,
		MotherOccupation:      req.MotherOccupation,
		MotherOccupationOther: req.MotherOccupationOther,
		FamilyMonthlyIncome:   req.FamilyMonthlyIncome,
		FamilyMembersCount:    req.FamilyMembersCount,

		Stream:     req.Stream,
		ExamCenter: req.ExamCenter,
 
		PersonalStatement: req.PersonalStatement,
		Documents: func() []byte {
			if len(req.Documents) == 0 {
				return []byte("[]")
			}
			data, _ := json.Marshal(req.Documents)
			return data
		}(),

		Status: func() string {
			if req.RequiresPayment {
				return ApplicationStatusPendingPayment
			}
			return "pending"
		}(),
	}

	if err := s.repo.ApplicationCreate(application); err != nil {
		return nil, errors.New("failed to submit application")
	}

	rollNumber := fmt.Sprintf("PS-%d-%04d", application.ID, rand.Intn(9000)+1000)
	if err := s.repo.UpdateApplicationRollNumber(application.ID, rollNumber); err == nil {
		application.RollNumber = rollNumber
	}

	if scholarship.ProviderScholarshipID != nil {
		if err := s.repo.CreateProviderApplication(*scholarship.ProviderScholarshipID, application); err != nil {
			return nil, errors.New("failed to submit application")
		}

		ps, _ := s.repo.FindProviderScholarshipByID(*scholarship.ProviderScholarshipID)
		if ps != nil {
			now := time.Now()
			s.providerDB.Table("provider_notifications").Create(map[string]interface{}{
				"provider_id": ps.ProviderID,
				"title":       "New Application Received",
				"message":     fmt.Sprintf("%s submitted an application for your scholarship: %s", application.FullName, ps.Title),
				"type":        "application",
				"link":        "applications",
				"read":        false,
				"created_at":  now,
				"updated_at":  now,
			})
		}
	}

	if !req.RequiresPayment {
		go s.sendAdmitCard(application, scholarship)
	}

	return application, nil
}

func (s *Service) GetMyApplications(userID uint) ([]ScholarshipApplication, error) {
	return s.repo.ApplicationFindByUserID(userID)
}

func (s *Service) sendAdmitCard(app *ScholarshipApplication, scholarship *Scholarship) {
	dobStr := ""
	if !app.DateOfBirthAD.IsZero() {
		dobStr = app.DateOfBirthAD.Format("02-Jan-2006")
	} else if app.DateOfBirthBS != "" {
		dobStr = app.DateOfBirthBS
	}

	cardData := AdmitCardData{
		CandidateName:    app.FullName,
		DateOfBirth:      dobStr,
		Gender:           app.Gender,
		RollNumber:       app.RollNumber,
		ExamCentre:       app.ExamCenter,
		Stream:           app.Stream,
		PhotoURL:         PhotoToBase64(app.PhotoURL),
		ScholarshipTitle: scholarship.Title,
		Provider:         scholarship.Provider,
		ExamDate:         scholarship.ExamDate,
		ExamTime:         scholarship.ExamTime,
		SubjectName:      scholarship.ScholarshipType,
	}

	pdfBytes, err := GenerateAdmitCardPDF(cardData)
	if err != nil {
		fmt.Printf("Warning: PDF generation failed: %v — sending plain email\n", err)
		subject := fmt.Sprintf("Admit Card - %s", scholarship.Title)
		body := fmt.Sprintf(`Dear %s,

Congratulations! Your application for %s has been confirmed.

Roll Number: %s
Status: CONFIRMED

Please keep this email for your records.

Best regards,
%s Team`, app.FullName, scholarship.Title, app.RollNumber, scholarship.Provider)

		if emailqueue.Queue != nil {
			_ = emailqueue.EnqueueGenericEmail(app.Email, subject, body)
		}
		return
	}

	_ = emailqueue.SendAdmitCardEmail(app.Email, app.FullName, scholarship.Title, pdfBytes)
}

func (s *Service) GetApplication(id uint, userID uint) (*ScholarshipApplication, error) {
	app, err := s.repo.ApplicationFindByID(id)
	if err != nil {
		return nil, err
	}

	if app.UserID == nil || *app.UserID != userID {
		return nil, errors.New("you can only view your own applications")
	}

	return app, nil
}

func (s *Service) GetApplicationByUserAndScholarshipID(scholarshipID uint, userID uint) (*ScholarshipApplication, error) {
	return s.repo.ApplicationFindByUserAndScholarshipID(scholarshipID, userID)
}

func (s *Service) GetApplicationForPayment(appID uint, scholarshipID uint) (*ScholarshipApplication, error) {
	app, err := s.repo.ApplicationFindByID(appID)
	if err != nil {
		return nil, err
	}
	if app.ScholarshipID != scholarshipID {
		return nil, errors.New("application does not belong to this scholarship")
	}
	return app, nil
}

func (s *Service) UpdateApplication(id uint, userID uint, req UpdateScholarshipApplicationRequest) (*ScholarshipApplication, error) {
	app, err := s.repo.ApplicationFindByID(id)
	if err != nil {
		return nil, errors.New("application not found")
	}

	if app.UserID == nil || *app.UserID != userID {
		return nil, errors.New("you can only update your own applications")
	}

	if req.FullName != nil {
		app.FullName = *req.FullName
	}
	if req.Gender != nil {
		app.Gender = *req.Gender
	}
	if req.Ethnicity != nil {
		app.Ethnicity = *req.Ethnicity
	}
	if req.EthnicityOther != nil {
		app.EthnicityOther = *req.EthnicityOther
	}
	if req.DateOfBirthBS != nil {
		app.DateOfBirthBS = *req.DateOfBirthBS
	}
	if req.DateOfBirthAD != nil {
		if dob, err := time.Parse("2006-01-02", *req.DateOfBirthAD); err == nil {
			app.DateOfBirthAD = dob
		}
	}
	if req.Age != nil {
		app.Age = *req.Age
	}
	if req.PhoneNumber != nil {
		app.PhoneNumber = *req.PhoneNumber
	}
	if req.Email != nil {
		app.Email = *req.Email
	}
	if req.PhotoURL != nil {
		app.PhotoURL = *req.PhotoURL
	}
	if req.SEEGPA != nil {
		app.SEEGPA = *req.SEEGPA
	}
	if req.SchoolType != nil {
		app.SchoolType = *req.SchoolType
	}
	if req.SchoolName != nil {
		app.SchoolName = *req.SchoolName
	}
	if req.SchoolProvince != nil {
		app.SchoolProvince = *req.SchoolProvince
	}
	if req.SchoolDistrict != nil {
		app.SchoolDistrict = *req.SchoolDistrict
	}
	if req.SchoolMunicipality != nil {
		app.SchoolMunicipality = *req.SchoolMunicipality
	}
	if req.SchoolTole != nil {
		app.SchoolTole = *req.SchoolTole
	}
	if req.PermanentProvince != nil {
		app.PermanentProvince = *req.PermanentProvince
	}
	if req.PermanentDistrict != nil {
		app.PermanentDistrict = *req.PermanentDistrict
	}
	if req.PermanentMunicipality != nil {
		app.PermanentMunicipality = *req.PermanentMunicipality
	}
	if req.PermanentWard != nil {
		app.PermanentWard = *req.PermanentWard
	}
	if req.PermanentTole != nil {
		app.PermanentTole = *req.PermanentTole
	}
	if req.TemporaryProvince != nil {
		app.TemporaryProvince = *req.TemporaryProvince
	}
	if req.TemporaryDistrict != nil {
		app.TemporaryDistrict = *req.TemporaryDistrict
	}
	if req.TemporaryMunicipality != nil {
		app.TemporaryMunicipality = *req.TemporaryMunicipality
	}
	if req.TemporaryWard != nil {
		app.TemporaryWard = *req.TemporaryWard
	}
	if req.TemporaryTole != nil {
		app.TemporaryTole = *req.TemporaryTole
	}
	if req.GuardianName != nil {
		app.GuardianName = *req.GuardianName
	}
	if req.GuardianPhone != nil {
		app.GuardianPhone = *req.GuardianPhone
	}
	if req.GuardianEmail != nil {
		app.GuardianEmail = *req.GuardianEmail
	}
	if req.FatherOccupation != nil {
		app.FatherOccupation = *req.FatherOccupation
	}
	if req.FatherOccupationOther != nil {
		app.FatherOccupationOther = *req.FatherOccupationOther
	}
	if req.MotherOccupation != nil {
		app.MotherOccupation = *req.MotherOccupation
	}
	if req.MotherOccupationOther != nil {
		app.MotherOccupationOther = *req.MotherOccupationOther
	}
	if req.FamilyMonthlyIncome != nil {
		app.FamilyMonthlyIncome = *req.FamilyMonthlyIncome
	}
	if req.FamilyMembersCount != nil {
		app.FamilyMembersCount = *req.FamilyMembersCount
	}
	if req.Stream != nil {
		app.Stream = *req.Stream
	}
	if req.ExamCenter != nil {
		app.ExamCenter = *req.ExamCenter
	}

	if err := s.repo.ApplicationSave(app); err != nil {
		return nil, errors.New("failed to update application")
	}

	return app, nil
}

func (s *Service) DeleteApplication(id uint, userID uint) error {
	app, err := s.repo.ApplicationFindByID(id)
	if err != nil {
		return errors.New("application not found")
	}

	if app.UserID == nil || *app.UserID != userID {
		return errors.New("you can only delete your own applications")
	}

	return s.repo.ApplicationDelete(id)
}

func (s *Service) GetAllApplications(status string) ([]ScholarshipApplication, error) {
	return s.repo.ApplicationFindAll(status)
}

func (s *Service) GetApplicationsByScholarship(scholarshipID string, status string) ([]ScholarshipApplication, error) {
	return s.repo.ApplicationFindByScholarshipID(scholarshipID, status)
}

func (s *Service) UpdateApplicationStatus(id uint, status string) (*ScholarshipApplication, error) {
	app, err := s.repo.ApplicationFindByID(id)
	if err != nil {
		return nil, errors.New("application not found")
	}

	app.Status = status

	if err := s.repo.ApplicationSave(app); err != nil {
		return nil, errors.New("failed to update application status")
	}

	return app, nil
}

func (s *Service) AdminCreateScholarship(req CreateScholarshipRequest) (*Scholarship, error) {
	var deadline time.Time
	if req.Deadline != "" {
		var err error
		deadline, err = time.Parse("2006-01-02", req.Deadline)
		if err != nil {
			return nil, errors.New("invalid deadline format (expected YYYY-MM-DD)")
		}
	}

	fieldOfStudy, _ := json.Marshal(req.FieldOfStudy)

	scholarship := &Scholarship{
		Title:           req.Title,
		Provider:        req.Provider,
		Location:        req.Location,
		Value:           req.Value,
		Deadline:        deadline,
		DegreeLevel:     req.DegreeLevel,
		FundingType:     req.FundingType,
		ScholarshipType: req.ScholarshipType,
		Description:     req.Description,
		ImageURL:        req.ImageURL,
		FieldOfStudy:    fieldOfStudy,
	}

	if err := s.repo.Create(scholarship); err != nil {
		return nil, errors.New("failed to create scholarship")
	}

	return scholarship, nil
}

func (s *Service) AdminUpdateScholarship(id uint, req CreateScholarshipRequest) (*Scholarship, error) {
	scholarship, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("scholarship not found")
	}

	if req.Title != "" {
		scholarship.Title = req.Title
	}
	if req.Provider != "" {
		scholarship.Provider = req.Provider
	}
	if req.Location != "" {
		scholarship.Location = req.Location
	}
	if req.Value != "" {
		scholarship.Value = req.Value
	}
	if req.Deadline != "" {
		if deadline, err := time.Parse("2006-01-02", req.Deadline); err == nil {
			scholarship.Deadline = deadline
		}
	}
	if req.DegreeLevel != "" {
		scholarship.DegreeLevel = req.DegreeLevel
	}
	if req.FundingType != "" {
		scholarship.FundingType = req.FundingType
	}
	if req.ScholarshipType != "" {
		scholarship.ScholarshipType = req.ScholarshipType
	}
	if req.Description != "" {
		scholarship.Description = req.Description
	}
	if req.ImageURL != "" {
		scholarship.ImageURL = req.ImageURL
	}
	if len(req.FieldOfStudy) > 0 {
		if data, err := json.Marshal(req.FieldOfStudy); err == nil {
			scholarship.FieldOfStudy = data
		}
	}

	if err := s.repo.Save(scholarship); err != nil {
		return nil, errors.New("failed to update scholarship")
	}

	return scholarship, nil
}

func (s *Service) AdminDeleteScholarship(id uint) error {
	_, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("scholarship not found")
	}

	return s.repo.Delete(id)
}

func (s *PaymentService) CreatePayment(appID uint, scholarshipID uint, userID *uint, req PaymentRequest) (*Payment, error) {
	payment := &Payment{
		ApplicationID: appID,
		ScholarshipID: scholarshipID,
		UserID:        userID,
		Method:        req.Method,
		Amount:        req.Amount,
		Status:        "pending",
	}
	if err := s.repo.Create(payment); err != nil {
		return nil, err
	}
	return payment, nil
}

func (s *PaymentService) ProcessSuccessfulPayment(paymentID uint, transactionID string) error {
	payment, err := s.repo.FindByID(paymentID)
	if err != nil {
		return err
	}
	payment.Status = "completed"
	payment.TransactionID = transactionID
	now := time.Now()
	payment.PaidAt = &now

	if err := s.repo.Update(payment); err != nil {
		return err
	}

	app, _ := s.scholarshipRepo.ApplicationFindByID(payment.ApplicationID)
	if app != nil && app.Status != "confirmed" {
		app.Status = "pending"
		s.scholarshipRepo.ApplicationSave(app)
	}

	return s.sendAdmitCard(payment)
}

func (s *PaymentService) UploadBankReceipt(paymentID uint, receiptURL string) error {
	payment, err := s.repo.FindByID(paymentID)
	if err != nil {
		return err
	}
	payment.Status = "pending_approval"
	payment.ReceiptURL = receiptURL
	return s.repo.Update(payment)
}

func (s *PaymentService) ApproveBankPayment(paymentID uint, approvedBy uint, reason string) error {
	payment, err := s.repo.FindByID(paymentID)
	if err != nil {
		return err
	}
	if reason != "" {
		payment.Status = "failed"
		payment.RejectionReason = reason
	} else {
		payment.Status = "completed"
		now := time.Now()
		payment.PaidAt = &now
		payment.ApprovedBy = approvedBy
		nowApproval := now
		payment.ApprovedAt = &nowApproval

		app, _ := s.scholarshipRepo.ApplicationFindByID(payment.ApplicationID)
		if app != nil && app.Status != "confirmed" {
			app.Status = "pending"
			s.scholarshipRepo.ApplicationSave(app)
		}

		s.sendAdmitCard(payment)
	}
	return s.repo.Update(payment)
}

func (s *PaymentService) sendAdmitCard(payment *Payment) error {
	app, _ := s.scholarshipRepo.ApplicationFindByID(payment.ApplicationID)
	scholarship, _ := s.scholarshipRepo.FindByID(payment.ScholarshipID)

	if app == nil || scholarship == nil {
		return nil
	}

	// Build admit card data from the application
	dobStr := ""
	if !app.DateOfBirthAD.IsZero() {
		dobStr = app.DateOfBirthAD.Format("02-Jan-2006")
	} else if app.DateOfBirthBS != "" {
		dobStr = app.DateOfBirthBS
	}

	cardData := AdmitCardData{
		CandidateName:    app.FullName,
		DateOfBirth:      dobStr,
		Gender:           app.Gender,
		RollNumber:       app.RollNumber,
		ExamCentre:       app.ExamCenter,
		Stream:           app.Stream,
		PhotoURL:         PhotoToBase64(app.PhotoURL),
		ScholarshipTitle: scholarship.Title,
		Provider:         scholarship.Provider,
		ExamDate:         scholarship.ExamDate,
		ExamTime:         scholarship.ExamTime,
		SubjectName:      scholarship.ScholarshipType,
	}

	pdfBytes, err := GenerateAdmitCardPDF(cardData)
	if err != nil {
		// Fallback: send a plain email if PDF generation fails
		fmt.Printf("Warning: PDF generation failed: %v — sending plain email\n", err)
		subject := fmt.Sprintf("Admit Card - %s", scholarship.Title)
		body := fmt.Sprintf(`Dear %s,

Congratulations! Your application for %s has been confirmed.

Roll Number: %s
Status: CONFIRMED

Please keep this email for your records.

Best regards,
%s Team`, app.FullName, scholarship.Title, app.RollNumber, scholarship.Provider)

		if emailqueue.Queue != nil {
			return emailqueue.EnqueueGenericEmail(app.Email, subject, body)
		}
		return nil
	}

	// Send with PDF attachment
	return emailqueue.SendAdmitCardEmail(app.Email, app.FullName, scholarship.Title, pdfBytes)
}

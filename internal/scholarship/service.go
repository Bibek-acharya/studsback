package scholarship

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/logger"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/system"
)

type Service struct {
	repo       *Repository
	providerDB *gorm.DB
	systemSvc  *system.Service
	notifier   notification.Notifier
}

func NewService(repo *Repository, providerDB *gorm.DB, systemSvc *system.Service, notifier notification.Notifier) *Service {
	return &Service{repo: repo, providerDB: providerDB, systemSvc: systemSvc, notifier: notifier}
}

type PaymentService struct {
	repo            *PaymentRepository
	scholarshipRepo *Repository
	notifier        notification.Notifier
}

func NewPaymentService(db *gorm.DB, notifier notification.Notifier) *PaymentService {
	return &PaymentService{
		repo:            NewPaymentRepository(db),
		scholarshipRepo: NewRepository(db),
		notifier:        notifier,
	}
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
			Slug:                     ps.Slug,
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

func (s *Service) GetScholarshipBySlug(slugStr string) (*Scholarship, error) {
	scholarship, err := s.repo.FindBySlug(slugStr)
	if err != nil {
		ps, err2 := s.repo.FindProviderScholarshipBySlug(slugStr)
		if err2 != nil {
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
			ID:                       ps.ID + 10000,
			Slug:                     ps.Slug,
			Title:                    ps.Title,
			Provider:                 providerName,
			ProviderID:               ps.ProviderID,
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
	return scholarship, nil
}

func (s *Service) GetAvailableExamCenters(scholarshipID uint) ([]string, error) {
	var examCenters []DetailField
	realScholarshipID := scholarshipID

	scholarship, err := s.repo.FindByID(scholarshipID)
	if err != nil {
		if scholarshipID > 10000 {
			providerScholarshipID := scholarshipID - 10000
			ps, psErr := s.repo.FindProviderScholarshipByID(providerScholarshipID)
			if psErr != nil {
				return nil, err
			}
			examCenters = parseDetailFieldArray(ps.ExamCentersNew)
			if len(examCenters) == 0 {
				examCenters = parseDetailFieldArray(ps.ExamCenters)
			}
			return s.filterAvailableCenters(examCenters, realScholarshipID)
		}
		return nil, err
	}

	if scholarship.ProviderScholarshipID != nil {
		ps, psErr := s.repo.FindProviderScholarshipByID(*scholarship.ProviderScholarshipID)
		if psErr == nil {
			examCenters = parseDetailFieldArray(ps.ExamCentersNew)
			if len(examCenters) == 0 {
				examCenters = parseDetailFieldArray(ps.ExamCenters)
			}
		}
	}

	if len(examCenters) == 0 {
		examCenters = parseDetailFieldArray(scholarship.ExamCentersNew)
		if len(examCenters) == 0 {
			examCenters = parseDetailFieldArray(scholarship.ExamCenters)
		}
	}

	return s.filterAvailableCenters(examCenters, scholarship.ID)
}

func (s *Service) filterAvailableCenters(examCenters []DetailField, scholarshipID uint) ([]string, error) {
	var available []string
	for _, ec := range examCenters {
		centerName := ec.CenterName
		if centerName == "" {
			continue
		}
		if ec.AllocatedSeats == 0 {
			available = append(available, centerName)
			continue
		}
		count, err := s.repo.CountApplicationsByExamCenter(scholarshipID, centerName)
		if err != nil {
			continue
		}
		if count < int64(ec.AllocatedSeats) {
			available = append(available, centerName)
		}
	}
	return available, nil
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

		ProviderName:             ps.ProviderName,
		FundingTypeOther:         ps.FundingTypeOther,
		ScholarshipTypeOther:     ps.ScholarshipTypeOther,
		EducationLevel:           ps.EducationLevel,
		EducationLevelOther:      ps.EducationLevelOther,
		ApplyLink:                ps.ApplyLink,
		CoverageArea:             ps.CoverageArea,
		ContactEmail:             ps.ContactEmail,
		PrimaryPhone:             ps.PrimaryPhone,
		SecondaryPhone:           ps.SecondaryPhone,
		WebsiteUrl:               ps.WebsiteUrl,
		OfficeAddress:            ps.OfficeAddress,
		MapUrl:                   ps.MapUrl,
		AboutParagraph1:          ps.AboutParagraph1,
		VideoTutorials:           ps.VideoTutorials,
		JourneyTimeline:          ps.JourneyTimeline,
		ScholarshipSectionTitle:  ps.ScholarshipSectionTitle,
		ScholarshipSubtitle:      ps.ScholarshipSubtitle,
		ScholarshipDescription1:  ps.ScholarshipDescription1,
		ScholarshipDescription2:  ps.ScholarshipDescription2,
		ScholarshipTypes:         ps.ScholarshipTypes,
		ScholarshipTypesNew:      ps.ScholarshipTypesNew,
		SelectionRubric:          ps.SelectionRubric,
		SelectionRubricNew:       ps.SelectionRubricNew,
		EligibilitySectionTitle:  ps.EligibilitySectionTitle,
		EligibilitySubtitle:      ps.EligibilitySubtitle,
		BasicEligibilityCriteria: ps.BasicEligibilityCriteria,
		FullyFundedCriteria:      ps.FullyFundedCriteria,
		PartiallyFundedCriteria:  ps.PartiallyFundedCriteria,
		SelectionProcessSteps:    ps.SelectionProcessSteps,
		FAQsNew:                  ps.FAQsNew,
		GalleryImages:            ps.GalleryImages,
		GalleryImagesNew:         ps.GalleryImagesNew,
		PartnerGroups:            ps.PartnerGroups,
		PartnerMessages:          ps.PartnerMessages,
		Downloads:                ps.Downloads,
		ExamDate:                 ps.ExamDate,
		ExamTime:                 ps.ExamTime,
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
		Status:        ApplicationStatusDraft,
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

	if userID != nil {
		if s.repo.ApplicationExists(scholarship.ID, *userID) {
			return nil, errors.New("you have already applied for this scholarship")
		}
	} else if req.Email != "" && s.repo.ApplicationExistsByEmail(scholarship.ID, req.Email) {
		return nil, errors.New("an application with this email already exists")
	}

	if !scholarship.Deadline.IsZero() && scholarship.Deadline.Before(time.Now().Truncate(24*time.Hour)) {
		return nil, errors.New("scholarship application deadline has passed")
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
				return ApplicationStatusDraft
			}
			return "pending"
		}(),
	}

	if err := s.repo.ApplicationCreate(application); err != nil {
		return nil, errors.New("failed to submit application")
	}

	if scholarship.ProviderScholarshipID != nil {
		if err := s.repo.CreateProviderApplication(*scholarship.ProviderScholarshipID, application); err != nil {
			return nil, errors.New("failed to submit application")
		}

		if !req.RequiresPayment {
			ps, _ := s.repo.FindProviderScholarshipByID(*scholarship.ProviderScholarshipID)
			if ps != nil {
				_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
					EventKey:   notification.EventApplicationReceived,
					Recipients: []notification.Ref{{Type: "provider", ID: ps.ProviderID}},
					Data:       map[string]any{"student_name": application.FullName, "program": ps.Title},
				})
			}
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

	if app.RollNumber == "" {
		if seq, err := s.repo.GetNextRollNumber(); err == nil {
			rn := fmt.Sprintf("PS-%05d", seq)
			s.repo.UpdateApplicationRollNumber(app.ID, rn)
			s.repo.UpdateProviderApplicationRollNumber(app.ID, rn)
			app.RollNumber = rn
		}
	}

	payload := emailqueue.AdmitCardPayload{
		Email:            app.Email,
		CandidateName:    app.FullName,
		DateOfBirth:      dobStr,
		Gender:           app.Gender,
		RollNumber:       app.RollNumber,
		ExamCentre:       app.ExamCenter,
		Stream:           app.Stream,
		PhotoURL:         app.PhotoURL,
		ScholarshipTitle: scholarship.Title,
		Provider:         scholarship.Provider,
		ExamDate:         scholarship.ExamDate,
		ExamTime:         scholarship.ExamTime,
		Shift:            "",
		SubjectName:      app.Stream,
	}

	if err := emailqueue.EnqueueSendAdmitCard(payload); err != nil {
		logger.Error("sendAdmitCard: failed to enqueue admit card task",
			"to", app.Email, "roll", app.RollNumber, "error", err)
	}
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

func (s *Service) PurgeOldDraftApplications(before time.Time) (int64, error) {
	return s.repo.ApplicationDeleteOlderThan(before)
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
	deadlineValue := req.Deadline
	if deadlineValue == "" {
		deadlineValue = req.ApplicationEndDate
	}
	var deadline time.Time
	if parsed, ok := parseOptionalScholarshipTime(deadlineValue); ok {
		deadline = parsed
	} else if req.Deadline != "" {
		return nil, errors.New("invalid deadline format (expected YYYY-MM-DD)")
	}
	var startDate time.Time
	if parsed, ok := parseOptionalScholarshipTime(req.ApplicationStartDate); ok {
		startDate = parsed
	}

	fieldOfStudy, _ := json.Marshal(req.FieldOfStudy)

	scholarship := &Scholarship{
		Title:                    req.Title,
		Provider:                 req.Provider,
		Location:                 req.Location,
		Value:                    req.Value,
		Deadline:                 deadline,
		ApplicationStartDate:     startDate,
		DegreeLevel:              req.DegreeLevel,
		FundingType:              req.FundingType,
		ScholarshipType:          req.ScholarshipType,
		Description:              req.Description,
		ImageURL:                 req.BannerBackgroundImageURL,
		BannerBackgroundImageURL: req.BannerBackgroundImageURL,
		FieldOfStudy:             fieldOfStudy,
		Status:                   normalizeScholarshipStatus(req.Status),
		ProviderName:             req.ProviderName,
		FundingTypeOther:         req.FundingTypeOther,
		ScholarshipTypeOther:     req.ScholarshipTypeOther,
		EducationLevel:           req.EducationLevel,
		EducationLevelOther:      req.EducationLevelOther,
		ApplyLink:                req.ApplyLink,
		CoverageArea:             req.CoverageArea,
		ContactEmail:             req.ContactEmail,
		PrimaryPhone:             req.PrimaryPhone,
		SecondaryPhone:           req.SecondaryPhone,
		WebsiteUrl:               req.WebsiteUrl,
		OfficeAddress:            req.OfficeAddress,
		MapUrl:                   req.MapUrl,
		AboutParagraph1:          req.AboutParagraph1,
		ScholarshipSectionTitle:  req.ScholarshipSectionTitle,
		ScholarshipSubtitle:      req.ScholarshipSubtitle,
		ScholarshipDescription1:  req.ScholarshipDescription1,
		ScholarshipDescription2:  req.ScholarshipDescription2,
		EligibilitySectionTitle:  req.EligibilitySectionTitle,
		EligibilitySubtitle:      req.EligibilitySubtitle,
		ExamDate:                 req.ExamDate,
		ExamTime:                 req.ExamTime,
		TotalSeats:               req.TotalSeats,
		VideoTutorials:           toScholarshipJSON(req.VideoTutorials),
		JourneyTimeline:          toScholarshipJSON(req.JourneyTimeline),
		Timeline:                 toScholarshipJSON(req.Timeline),
		ScholarshipTypes:         toScholarshipJSON(req.ScholarshipTypes),
		ScholarshipTypesNew:      toScholarshipJSON(req.ScholarshipTypesNew),
		SelectionRubric:          toScholarshipJSON(req.SelectionRubric),
		SelectionRubricNew:       toScholarshipJSON(req.SelectionRubricNew),
		BasicEligibilityCriteria: toScholarshipJSON(req.BasicEligibilityCriteria),
		FullyFundedCriteria:      toScholarshipJSON(req.FullyFundedCriteria),
		PartiallyFundedCriteria:  toScholarshipJSON(req.PartiallyFundedCriteria),
		SelectionProcessSteps:    toScholarshipJSON(req.SelectionProcessSteps),
		RequiredDocuments:        toScholarshipJSON(req.RequiredDocuments),
		FAQs:                     toScholarshipJSON(req.FAQs),
		FAQsNew:                  toScholarshipJSON(req.FAQsNew),
		GalleryImages:            toScholarshipJSON(req.GalleryImages),
		GalleryImagesNew:         toScholarshipJSON(req.GalleryImagesNew),
		PartnerGroups:            toScholarshipJSON(req.PartnerGroups),
		PartnerMessages:          toScholarshipJSON(req.PartnerMessages),
		ExamCenters:              toScholarshipJSON(req.ExamCenters),
		ExamCentersNew:           toScholarshipJSON(req.ExamCentersNew),
		Downloads:                toScholarshipJSON(req.Downloads),
		Benefits:                 toScholarshipJSON(req.Benefits),
		PaymentConfig:            toScholarshipJSON(req.PaymentConfig),
	}

	if err := s.repo.Create(scholarship); err != nil {
		return nil, errors.New("failed to create scholarship")
	}

	s.systemSvc.CreatePublicNotification(
		"New Scholarship: "+scholarship.Title,
		scholarship.Description,
		"scholarship",
		fmt.Sprintf("/scholarships/%d", scholarship.ID),
		"fa-trophy",
		"text-yellow-600",
		"bg-yellow-100",
	)

	return scholarship, nil
}

func toScholarshipJSON(v any) []byte {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func normalizeScholarshipStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "published", "active":
		return "published"
	case "draft":
		return "draft"
	default:
		if status == "" {
			return "draft"
		}
		return status
	}
}

func parseOptionalScholarshipTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, true
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func (s *Service) AdminUpdateScholarship(id uint, req CreateScholarshipRequest) (*Scholarship, error) {
	scholarship, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("scholarship not found")
	}

	deadlineValue := req.Deadline
	if deadlineValue == "" {
		deadlineValue = req.ApplicationEndDate
	}

	scholarship.Title = req.Title
	scholarship.Provider = req.Provider
	scholarship.Location = req.Location
	scholarship.Value = req.Value
	scholarship.DegreeLevel = req.DegreeLevel
	scholarship.FundingType = req.FundingType
	scholarship.ScholarshipType = req.ScholarshipType
	scholarship.Description = req.Description
	scholarship.ImageURL = req.BannerBackgroundImageURL
	scholarship.BannerBackgroundImageURL = req.BannerBackgroundImageURL
	scholarship.Status = normalizeScholarshipStatus(req.Status)
	scholarship.ProviderName = req.ProviderName
	scholarship.FundingTypeOther = req.FundingTypeOther
	scholarship.ScholarshipTypeOther = req.ScholarshipTypeOther
	scholarship.EducationLevel = req.EducationLevel
	scholarship.EducationLevelOther = req.EducationLevelOther
	scholarship.ApplyLink = req.ApplyLink
	scholarship.CoverageArea = req.CoverageArea
	scholarship.ContactEmail = req.ContactEmail
	scholarship.PrimaryPhone = req.PrimaryPhone
	scholarship.SecondaryPhone = req.SecondaryPhone
	scholarship.WebsiteUrl = req.WebsiteUrl
	scholarship.OfficeAddress = req.OfficeAddress
	scholarship.MapUrl = req.MapUrl
	scholarship.AboutParagraph1 = req.AboutParagraph1
	scholarship.ScholarshipSectionTitle = req.ScholarshipSectionTitle
	scholarship.ScholarshipSubtitle = req.ScholarshipSubtitle
	scholarship.ScholarshipDescription1 = req.ScholarshipDescription1
	scholarship.ScholarshipDescription2 = req.ScholarshipDescription2
	scholarship.EligibilitySectionTitle = req.EligibilitySectionTitle
	scholarship.EligibilitySubtitle = req.EligibilitySubtitle
	scholarship.ExamDate = req.ExamDate
	scholarship.ExamTime = req.ExamTime
	scholarship.TotalSeats = req.TotalSeats
	scholarship.VideoTutorials = toScholarshipJSON(req.VideoTutorials)
	scholarship.JourneyTimeline = toScholarshipJSON(req.JourneyTimeline)
	scholarship.Timeline = toScholarshipJSON(req.Timeline)
	scholarship.ScholarshipTypes = toScholarshipJSON(req.ScholarshipTypes)
	scholarship.ScholarshipTypesNew = toScholarshipJSON(req.ScholarshipTypesNew)
	scholarship.SelectionRubric = toScholarshipJSON(req.SelectionRubric)
	scholarship.SelectionRubricNew = toScholarshipJSON(req.SelectionRubricNew)
	scholarship.BasicEligibilityCriteria = toScholarshipJSON(req.BasicEligibilityCriteria)
	scholarship.FullyFundedCriteria = toScholarshipJSON(req.FullyFundedCriteria)
	scholarship.PartiallyFundedCriteria = toScholarshipJSON(req.PartiallyFundedCriteria)
	scholarship.SelectionProcessSteps = toScholarshipJSON(req.SelectionProcessSteps)
	scholarship.RequiredDocuments = toScholarshipJSON(req.RequiredDocuments)
	scholarship.FAQs = toScholarshipJSON(req.FAQs)
	scholarship.FAQsNew = toScholarshipJSON(req.FAQsNew)
	scholarship.GalleryImages = toScholarshipJSON(req.GalleryImages)
	scholarship.GalleryImagesNew = toScholarshipJSON(req.GalleryImagesNew)
	scholarship.PartnerGroups = toScholarshipJSON(req.PartnerGroups)
	scholarship.PartnerMessages = toScholarshipJSON(req.PartnerMessages)
	scholarship.ExamCenters = toScholarshipJSON(req.ExamCenters)
	scholarship.ExamCentersNew = toScholarshipJSON(req.ExamCentersNew)
	scholarship.Downloads = toScholarshipJSON(req.Downloads)
	scholarship.Benefits = toScholarshipJSON(req.Benefits)
	scholarship.PaymentConfig = toScholarshipJSON(req.PaymentConfig)
	if len(req.FieldOfStudy) > 0 {
		if data, err := json.Marshal(req.FieldOfStudy); err == nil {
			scholarship.FieldOfStudy = data
		}
	}

	if req.ApplicationStartDate != "" {
		if parsed, ok := parseOptionalScholarshipTime(req.ApplicationStartDate); ok {
			scholarship.ApplicationStartDate = parsed
		} else {
			return nil, errors.New("invalid application start date")
		}
	} else {
		scholarship.ApplicationStartDate = time.Time{}
	}

	if deadlineValue != "" {
		if parsed, ok := parseOptionalScholarshipTime(deadlineValue); ok {
			scholarship.Deadline = parsed
		} else {
			return nil, errors.New("invalid application end date")
		}
	} else {
		scholarship.Deadline = time.Time{}
	}

	if err := s.repo.Save(scholarship); err != nil {
		return nil, errors.New("failed to update scholarship")
	}

	return scholarship, nil
}

func (s *Service) AdminDeleteScholarship(id uint) error {
	scholarship, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("scholarship not found")
	}

	s.deleteScholarshipFiles(scholarship)

	if scholarship.ProviderScholarshipID != nil {
		_ = s.providerDB.Unscoped().
			Delete(&ProviderScholarship{}, *scholarship.ProviderScholarshipID).Error
	}

	return s.repo.CascadeDelete(id)
}

func (s *Service) AdminListScholarships() ([]Scholarship, error) {
	return s.repo.FindAllScholarships()
}

func (s *Service) deleteScholarshipFiles(scholarship *Scholarship) {
	extractPath := func(url string) string {
		if strings.HasPrefix(url, "/uploads/") {
			return url[len("/uploads/"):]
		}
		return ""
	}

	tryDelete := func(url string) {
		if path := extractPath(url); path != "" {
			_ = storage.DeleteObject(path)
		}
	}

	tryDelete(scholarship.ImageURL)
	tryDelete(scholarship.BannerBackgroundImageURL)

	for _, url := range extractUploadURLs(scholarship.GalleryImages) {
		tryDelete(url)
	}
	for _, url := range extractUploadURLs(scholarship.GalleryImagesNew) {
		tryDelete(url)
	}
	for _, url := range extractUploadURLs(scholarship.Downloads) {
		tryDelete(url)
	}
}

func extractUploadURLs(data []byte) []string {
	var urls []string
	var items []any
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	for _, item := range items {
		switch v := item.(type) {
		case string:
			if strings.HasPrefix(v, "/uploads/") {
				urls = append(urls, v)
			}
		case map[string]any:
			if url, ok := v["url"].(string); ok && strings.HasPrefix(url, "/uploads/") {
				urls = append(urls, url)
			}
		}
	}
	return urls
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
		s.scholarshipRepo.UpdateProviderApplicationStatus(app.ID, "pending")
	}

	s.createApplicationReceivedNotification(app, payment.ScholarshipID)
	s.notifyPaymentReceived(payment)

	return s.sendAdmitCard(app, payment)
}

func (s *PaymentService) UploadBankReceipt(paymentID uint, receiptURL string) error {
	payment, err := s.repo.FindByID(paymentID)
	if err != nil {
		return err
	}

	// If receipt is a base64 data URL, upload to MinIO
	if strings.HasPrefix(receiptURL, "data:") {
		commaIdx := strings.Index(receiptURL, ",")
		if commaIdx > 0 {
			mimeType := receiptURL[5:commaIdx]
			if semiIdx := strings.Index(mimeType, ";"); semiIdx > 0 {
				mimeType = mimeType[:semiIdx]
			}
			b64Data := receiptURL[commaIdx+1:]
			decoded, decErr := base64.StdEncoding.DecodeString(b64Data)
			if decErr != nil {
				return fmt.Errorf("failed to decode receipt image: %w", decErr)
			}

			ext := ".png"
			switch {
			case strings.Contains(mimeType, "jpeg"), strings.Contains(mimeType, "jpg"):
				ext = ".jpg"
			case strings.Contains(mimeType, "gif"):
				ext = ".gif"
			case strings.Contains(mimeType, "webp"):
				ext = ".webp"
			}

			objectPath := fmt.Sprintf("scholarship/payments/receipt_%d_%d%s", paymentID, time.Now().UnixNano(), ext)
			if upErr := storage.UploadBytes(objectPath, decoded, mimeType); upErr != nil {
				return fmt.Errorf("failed to upload receipt to MinIO: %w", upErr)
			}

			receiptURL = "/uploads/" + objectPath
		}
	}

	payment.Status = "pending_approval"
	payment.ReceiptURL = receiptURL
	if err := s.repo.Update(payment); err != nil {
		return err
	}

	app, err := s.scholarshipRepo.ApplicationFindByID(payment.ApplicationID)
	if err == nil && app.Status == ApplicationStatusDraft {
		app.Status = ApplicationStatusPendingPayment
		if err := s.scholarshipRepo.ApplicationSave(app); err != nil {
			return err
		}
		s.scholarshipRepo.UpdateProviderApplicationStatus(app.ID, "pending")
		s.createApplicationReceivedNotification(app, payment.ScholarshipID)
	}
	s.notifyBankReceiptSubmitted(payment)

	return nil
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
			s.scholarshipRepo.UpdateProviderApplicationStatus(app.ID, "pending")
		}

		s.sendAdmitCard(app, payment)
	}
	if err := s.repo.Update(payment); err != nil {
		return err
	}

	if reason != "" {
		if app, appErr := s.scholarshipRepo.ApplicationFindByID(payment.ApplicationID); appErr == nil && app.UserID != nil {
			_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventScholarshipBankRejected,
				Recipients: []notification.Ref{{Type: "user", ID: *app.UserID}},
				DedupeKey:  fmt.Sprintf("bank_rejected:%d", payment.ID),
				Data:       map[string]any{"reason": reason},
			})
		}
	}
	return nil
}

func (s *PaymentService) InitiateEsewaPayment(appID uint, amount float64) (*EsewaInitiateResponse, error) {
	app, err := s.scholarshipRepo.ApplicationFindByID(appID)
	if err != nil {
		return nil, fmt.Errorf("application not found")
	}

	cfg := config.AppConfig
	frontendURL := strings.TrimRight(cfg.FrontendURL, "/")

	totalAmount := fmt.Sprintf("%.0f", amount)
	taxAmount := "0"
	transactionUUID := fmt.Sprintf("SCH-%d-%d", appID, time.Now().UnixMilli())

	message := fmt.Sprintf("total_amount=%s,transaction_uuid=%s,product_code=%s", totalAmount, transactionUUID, cfg.EsewaMerchantCode)
	h := hmac.New(sha256.New, []byte(cfg.EsewaSecretKey))
	h.Write([]byte(message))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	existingPayment, _ := s.repo.FindByApplicationID(appID)
	if existingPayment != nil {
		existingPayment.TransactionID = transactionUUID
		existingPayment.Status = "pending"
		s.repo.Update(existingPayment)
	} else {
		payment := &Payment{
			ApplicationID: appID,
			ScholarshipID: app.ScholarshipID,
			Method:        "esewa",
			Amount:        amount,
			Status:        "pending",
			TransactionID: transactionUUID,
		}
		if app.UserID != nil {
			payment.UserID = app.UserID
		}
		if err := s.repo.Create(payment); err != nil {
			return nil, err
		}
	}

	return &EsewaInitiateResponse{
		Amount:          fmt.Sprintf("%.0f", amount),
		TaxAmount:       taxAmount,
		TotalAmount:     totalAmount,
		TransactionUUID: transactionUUID,
		ProductCode:     cfg.EsewaMerchantCode,
		Signature:       signature,
		SuccessURL:      fmt.Sprintf("%s/scholarship-pay/%d/success", frontendURL, app.ScholarshipID),
		FailureURL:      fmt.Sprintf("%s/scholarship-pay/%d/failure", frontendURL, app.ScholarshipID),
		GatewayURL:      cfg.EsewaGatewayURL(),
	}, nil
}

// esewaStatusAPIURL is a seam so tests can point the status check at a
// stub server instead of the real eSewa API.
var esewaStatusAPIURL = func() string { return config.AppConfig.EsewaStatusAPIURL() }

// esewaFailureStatuses are eSewa's explicit terminal-failure statuses from
// the epay v2 status API. COMPLETE is success; everything else (PENDING,
// AMBIGUOUS, …) stays in-flight — no payment_failed emission for those.
var esewaFailureStatuses = map[string]bool{
	"FAILED":    true,
	"NOT_FOUND": true,
	"CANCELED":  true,
	"FULL_REV":  true,
	"PART_REV":  true,
}

func (s *PaymentService) VerifyEsewaPayment(req EsewaVerifyRequest) (*Payment, error) {
	cfg := config.AppConfig

	apiURL := fmt.Sprintf("%s?product_code=%s&total_amount=%s&transaction_uuid=%s",
		esewaStatusAPIURL(), cfg.EsewaMerchantCode, req.TotalAmount, req.TransactionUUID)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to verify with eSewa: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read eSewa response: %w", err)
	}

	var esewaResp struct {
		Status          string      `json:"status"`
		RefID           string      `json:"ref_id"`
		TotalAmount     interface{} `json:"total_amount"`
		TransactionUUID string      `json:"transaction_uuid"`
		ProductCode     string      `json:"product_code"`
	}

	if err := json.Unmarshal(body, &esewaResp); err != nil {
		return nil, fmt.Errorf("failed to parse eSewa response: %w", err)
	}

	if esewaResp.Status != "COMPLETE" {
		// Only explicit terminal failures notify — PENDING/AMBIGUOUS are
		// in-flight (the pending-payment poller re-verifies them later).
		if esewaFailureStatuses[esewaResp.Status] {
			s.notifyPaymentFailed(req.TransactionUUID)
		}
		return nil, fmt.Errorf("eSewa payment not completed, status: %s", esewaResp.Status)
	}

	payment, err := s.repo.FindByTransactionID(req.TransactionUUID)
	if err != nil {
		return nil, fmt.Errorf("payment not found for transaction: %s", req.TransactionUUID)
	}

	payment.Status = "completed"
	payment.TransactionID = req.TransactionUUID
	now := time.Now()
	payment.PaidAt = &now

	if err := s.repo.Update(payment); err != nil {
		return nil, err
	}

	app, _ := s.scholarshipRepo.ApplicationFindByID(req.ApplicationID)
	if app != nil {
		if app.Status != "confirmed" {
			app.Status = "pending"
		}
		s.scholarshipRepo.ApplicationSave(app)
		s.scholarshipRepo.UpdateProviderApplicationStatus(app.ID, "pending")

		s.createApplicationReceivedNotification(app, payment.ScholarshipID)
		s.notifyPaymentReceived(payment)

		if err := s.sendAdmitCard(app, payment); err != nil {
			log.Printf("esewa: failed to send admit card: %v", err)
		}
	}

	return payment, nil
}

type VerifySummary struct {
	Total       int    `json:"total"`
	Verified    int    `json:"verified"`
	Failed      int    `json:"failed"`
	NotComplete int    `json:"not_complete"`
	Error       string `json:"error,omitempty"`
}

func (s *PaymentService) VerifyPendingEsewaPayments() *VerifySummary {
	payments, err := s.repo.FindPendingEsewa()
	if err != nil {
		return &VerifySummary{Error: err.Error()}
	}

	summary := &VerifySummary{Total: len(payments)}

	for _, p := range payments {
		cfg := config.AppConfig
		apiURL := fmt.Sprintf("%s?product_code=%s&total_amount=%.0f&transaction_uuid=%s",
			cfg.EsewaStatusAPIURL(), cfg.EsewaMerchantCode, p.Amount, p.TransactionID)

		resp, err := http.Get(apiURL)
		if err != nil {
			summary.Failed++
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(body, &result); err != nil || result.Status != "COMPLETE" {
			summary.NotComplete++
			continue
		}

		now := time.Now()
		p.Status = "completed"
		p.PaidAt = &now
		if err := s.repo.Update(&p); err != nil {
			summary.Failed++
			continue
		}

		app, _ := s.scholarshipRepo.ApplicationFindByID(p.ApplicationID)
		if app != nil {
			if app.Status != "confirmed" {
				app.Status = "pending"
				s.scholarshipRepo.ApplicationSave(app)
			}
			s.scholarshipRepo.UpdateProviderApplicationStatus(app.ID, "pending")
			go func(a *ScholarshipApplication, pay *Payment) {
				if err := s.sendAdmitCard(a, pay); err != nil {
					log.Printf("verify-esewa: failed to send admit card: %v", err)
				}
			}(app, &p)
		}
		s.notifyPaymentReceived(&p)

		summary.Verified++
	}

	return summary
}

type SendAdmitCardSummary struct {
	Total   int `json:"total"`
	Sent    int `json:"sent"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

func (s *PaymentService) SendAdmitCards() *SendAdmitCardSummary {
	summary := &SendAdmitCardSummary{}

	payments, err := s.repo.FindCompletedEsewa()
	if err != nil {
		return summary
	}

	for _, p := range payments {
		summary.Total++
		app, err := s.scholarshipRepo.ApplicationFindByID(p.ApplicationID)
		if err != nil || app == nil {
			summary.Skipped++
			continue
		}
		if app.RollNumber == "" || app.Status != "pending" {
			summary.Skipped++
			continue
		}
		if err := s.sendAdmitCard(app, &p); err != nil {
			log.Printf("send-admits: failed for app %d: %v", app.ID, err)
			summary.Failed++
		} else {
			summary.Sent++
		}
	}

	return summary
}

func (s *PaymentService) sendAdmitCard(app *ScholarshipApplication, payment *Payment) error {
	scholarship, _ := s.scholarshipRepo.FindByID(payment.ScholarshipID)

	if app == nil || scholarship == nil {
		return nil
	}

	dobStr := ""
	if !app.DateOfBirthAD.IsZero() {
		dobStr = app.DateOfBirthAD.Format("02-Jan-2006")
	} else if app.DateOfBirthBS != "" {
		dobStr = app.DateOfBirthBS
	}

	if app.RollNumber == "" {
		if seq, err := s.scholarshipRepo.GetNextRollNumber(); err == nil {
			rn := fmt.Sprintf("PS-%05d", seq)
			s.scholarshipRepo.UpdateApplicationRollNumber(app.ID, rn)
			s.scholarshipRepo.UpdateProviderApplicationRollNumber(app.ID, rn)
			app.RollNumber = rn
		}
	}

	payload := emailqueue.AdmitCardPayload{
		Email:            app.Email,
		CandidateName:    app.FullName,
		DateOfBirth:      dobStr,
		Gender:           app.Gender,
		RollNumber:       app.RollNumber,
		ExamCentre:       app.ExamCenter,
		Stream:           app.Stream,
		PhotoURL:         app.PhotoURL,
		ScholarshipTitle: scholarship.Title,
		Provider:         scholarship.Provider,
		ExamDate:         scholarship.ExamDate,
		ExamTime:         scholarship.ExamTime,
		Shift:            "",
		SubjectName:      app.Stream,
	}

	return emailqueue.EnqueueSendAdmitCard(payload)
}

func (s *PaymentService) createApplicationReceivedNotification(app *ScholarshipApplication, scholarshipID uint) {
	scholarship, err := s.scholarshipRepo.FindByID(scholarshipID)
	if err != nil || scholarship.ProviderScholarshipID == nil {
		return
	}
	ps, err := s.scholarshipRepo.FindProviderScholarshipByID(*scholarship.ProviderScholarshipID)
	if err != nil {
		return
	}
	_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
		EventKey:   notification.EventApplicationReceived,
		Recipients: []notification.Ref{{Type: "provider", ID: ps.ProviderID}},
		Data:       map[string]any{"student_name": app.FullName, "program": ps.Title},
	})
}

// notifyPaymentReceived sends the receipt copy to the student and, when a
// provider pipeline owns the scholarship, to the provider org. Deduped per
// payment: the verify endpoint, the pending-poll and the legacy confirm path
// can all reach the terminal transition for the same payment row.
// Provider Ref targets a scholarship_provider_users row id (M5, doc 12 §3).
func (s *PaymentService) notifyPaymentReceived(payment *Payment) {
	scholarship, err := s.scholarshipRepo.FindByID(payment.ScholarshipID)
	if err != nil || scholarship == nil {
		return
	}
	dedupe := fmt.Sprintf("esewa_paid:%d", payment.ID)
	data := map[string]any{"scholarship": scholarship.Title, "slug": scholarship.Slug}
	if payment.UserID != nil {
		_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventScholarshipPaymentReceived,
			Recipients: []notification.Ref{{Type: "user", ID: *payment.UserID}},
			DedupeKey:  dedupe,
			Data:       data,
		})
	}
	// Provider copy only when the scholarship flows through a provider
	// pipeline (mirror row exists); admin/institution-created scholarships
	// have no provider inbox (D-Q13 analog).
	if scholarship.ProviderScholarshipID != nil {
		ps, err := s.scholarshipRepo.FindProviderScholarshipByID(*scholarship.ProviderScholarshipID)
		if err == nil {
			_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventScholarshipPaymentReceived,
				Recipients: []notification.Ref{{Type: "provider", ID: ps.ProviderID}},
				DedupeKey:  dedupe,
				Data:       data,
			})
		}
	}
}

// notifyBankReceiptSubmitted tells the provider org a bank receipt awaits
// review. In-app only (registry EmailDefault=false); skipped when no provider
// pipeline owns the scholarship.
func (s *PaymentService) notifyBankReceiptSubmitted(payment *Payment) {
	scholarship, err := s.scholarshipRepo.FindByID(payment.ScholarshipID)
	if err != nil || scholarship == nil || scholarship.ProviderScholarshipID == nil {
		return
	}
	ps, err := s.scholarshipRepo.FindProviderScholarshipByID(*scholarship.ProviderScholarshipID)
	if err != nil {
		return
	}
	_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
		EventKey:   notification.EventScholarshipBankReceipt,
		Recipients: []notification.Ref{{Type: "provider", ID: ps.ProviderID}},
		Data:       map[string]any{"scholarship": scholarship.Title},
	})
}

// notifyPaymentFailed tells the student their gateway payment did not go
// through. Deduped per transaction so retries of the verify endpoint don't
// spam the inbox.
func (s *PaymentService) notifyPaymentFailed(transactionUUID string) {
	payment, err := s.repo.FindByTransactionID(transactionUUID)
	if err != nil {
		return
	}
	app, err := s.scholarshipRepo.ApplicationFindByID(payment.ApplicationID)
	if err != nil || app.UserID == nil {
		return
	}
	scholarship, err := s.scholarshipRepo.FindByID(payment.ScholarshipID)
	if err != nil || scholarship == nil {
		return
	}
	_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
		EventKey:   notification.EventScholarshipPaymentFailed,
		Recipients: []notification.Ref{{Type: "user", ID: *app.UserID}},
		DedupeKey:  fmt.Sprintf("esewa_failed:%s", transactionUUID),
		Data:       map[string]any{"scholarship": scholarship.Title, "slug": scholarship.Slug},
	})
}

package scholarship

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetScholarships(search, categoryFilter, typeFilter, locationFilter, levelFilter, statusFilter, sortBy, order string) ([]Scholarship, []CategoryResponse, error) {
	scholarships, err := s.repo.FindAll(search, typeFilter, locationFilter, levelFilter, sortBy, order)
	if err != nil {
		return nil, nil, err
	}

	allScholarships, err := s.repo.FindAllUnfiltered()
	if err != nil {
		return nil, nil, err
	}

	categoryDefs := categoryDefinitions()
	categoryCounts := make(map[string]int, len(categoryDefs))
	for _, def := range categoryDefs {
		categoryCounts[def.ID] = 0
	}

	for _, scholarship := range allScholarships {
		status := deriveScholarshipStatus(scholarship.Deadline)
		if statusFilter != "" && statusFilter != status {
			continue
		}
		if statusFilter == "" && !isOpenStatus(status) {
			continue
		}

		if categoryID := mapScholarshipToCategoryID(scholarship); categoryID != "" {
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

func (s *Service) ApplyScholarship(scholarshipID uint, userID uint, req ScholarshipApplicationRequest) (*ScholarshipApplication, error) {
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		return nil, errors.New("invalid date of birth format (expected YYYY-MM-DD)")
	}

	if _, err := s.repo.FindByID(scholarshipID); err != nil {
		return nil, errors.New("scholarship not found")
	}

	if userID > 0 && s.repo.ApplicationExists(scholarshipID, userID) {
		return nil, errors.New("you have already applied for this scholarship")
	}

	specialCircumstances, _ := json.Marshal(req.SpecialCircumstances)

	application := &ScholarshipApplication{
		ScholarshipID:        scholarshipID,
		UserID:               userID,
		NationalID:           req.NationalID,
		FirstName:            req.FirstName,
		LastName:             req.LastName,
		DateOfBirth:          dob,
		Gender:               req.Gender,
		StreetAddress:        req.StreetAddress,
		City:                 req.City,
		PostCode:             req.PostCode,
		Country:              req.Country,
		PhoneCode:            req.PhoneCode,
		PhoneNumber:          req.PhoneNumber,
		Email:                req.Email,
		LatestInstitution:    req.LatestInstitution,
		LevelCompleted:       req.LevelCompleted,
		GPAPercentage:        req.GPAPercentage,
		AnnualFamilyIncome:   req.AnnualFamilyIncome,
		PrimaryIncomeSource:  req.PrimaryIncomeSource,
		SpecialCircumstances: specialCircumstances,
		PersonalStatement:    req.PersonalStatement,
		Status:               "pending",
	}

	if err := s.repo.ApplicationCreate(application); err != nil {
		return nil, errors.New("failed to submit application")
	}

	return application, nil
}

func (s *Service) GetMyApplications(userID uint) ([]ScholarshipApplication, error) {
	return s.repo.ApplicationFindByUserID(userID)
}

func (s *Service) GetApplication(id uint, userID uint) (*ScholarshipApplication, error) {
	app, err := s.repo.ApplicationFindByID(id)
	if err != nil {
		return nil, err
	}

	if app.UserID != userID {
		return nil, errors.New("you can only view your own applications")
	}

	return app, nil
}

func (s *Service) UpdateApplication(id uint, userID uint, req UpdateScholarshipApplicationRequest) (*ScholarshipApplication, error) {
	app, err := s.repo.ApplicationFindByID(id)
	if err != nil {
		return nil, errors.New("application not found")
	}

	if app.UserID != userID {
		return nil, errors.New("you can only update your own applications")
	}

	if req.NationalID != nil {
		app.NationalID = *req.NationalID
	}
	if req.FirstName != nil {
		app.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		app.LastName = *req.LastName
	}
	if req.DateOfBirth != nil {
		if dob, err := time.Parse("2006-01-02", *req.DateOfBirth); err == nil {
			app.DateOfBirth = dob
		}
	}
	if req.Gender != nil {
		app.Gender = *req.Gender
	}
	if req.StreetAddress != nil {
		app.StreetAddress = *req.StreetAddress
	}
	if req.City != nil {
		app.City = *req.City
	}
	if req.PostCode != nil {
		app.PostCode = *req.PostCode
	}
	if req.Country != nil {
		app.Country = *req.Country
	}
	if req.PhoneCode != nil {
		app.PhoneCode = *req.PhoneCode
	}
	if req.PhoneNumber != nil {
		app.PhoneNumber = *req.PhoneNumber
	}
	if req.Email != nil {
		app.Email = *req.Email
	}
	if req.LatestInstitution != nil {
		app.LatestInstitution = *req.LatestInstitution
	}
	if req.LevelCompleted != nil {
		app.LevelCompleted = *req.LevelCompleted
	}
	if req.GPAPercentage != nil {
		app.GPAPercentage = *req.GPAPercentage
	}
	if req.AnnualFamilyIncome != nil {
		app.AnnualFamilyIncome = *req.AnnualFamilyIncome
	}
	if req.PrimaryIncomeSource != nil {
		app.PrimaryIncomeSource = *req.PrimaryIncomeSource
	}
	if req.PersonalStatement != nil {
		app.PersonalStatement = *req.PersonalStatement
	}
	if len(req.SpecialCircumstances) > 0 {
		if data, err := json.Marshal(req.SpecialCircumstances); err == nil {
			app.SpecialCircumstances = data
		}
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

	if app.UserID != userID {
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

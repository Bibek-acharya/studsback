package admission

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var notifyStudentFunc func(userID uint, title, message, notifType, link string)

func SetNotifyStudentFunc(fn func(userID uint, title, message, notifType, link string)) {
	notifyStudentFunc = fn
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(req CreateAdmissionRequest, userID *uint) (*Admission, error) {
	if !s.repo.CollegeExists(req.CollegeID) {
		return nil, errors.New("college not found")
	}

	admission := &Admission{
		CollegeID:         req.CollegeID,
		ProgramName:       req.ProgramName,
		ProgramLevel:      req.ProgramLevel,
		StudentName:       req.StudentName,
		StudentEmail:      req.StudentEmail,
		StudentPhone:      req.StudentPhone,
		LastQualification: req.LastQualification,
		Institution:       req.Institution,
		GPA:               req.GPA,
		EntranceScore:     req.EntranceScore,
		Statement:         req.Statement,
		Gender:            req.Gender,
		Address:           req.Address,
		City:              req.City,
		Status:            "pending",
		UserID:            userID,
	}

	if req.DateOfBirth != "" {
		if dob, err := time.Parse("2006-01-02", req.DateOfBirth); err == nil {
			admission.DateOfBirth = &dob
		}
	}

	if err := s.repo.Create(admission); err != nil {
		return nil, errors.New("failed to create admission application")
	}

	return admission, nil
}

func (s *Service) GetMyAdmissions(userID uint) ([]Admission, error) {
	return s.repo.FindByUserID(userID)
}

func (s *Service) GetByID(id uint) (*Admission, error) {
	return s.repo.FindByID(id)
}

func (s *Service) Update(id uint, userID uint, userRole string, req UpdateAdmissionRequest) (*Admission, error) {
	admission, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("admission not found")
	}

	if admission.UserID != nil && *admission.UserID != userID {
		if userRole != "admin" && userRole != "super_admin" {
			return nil, errors.New("you can only update your own admissions")
		}
	}

	if req.ProgramName != nil {
		admission.ProgramName = *req.ProgramName
	}
	if req.ProgramLevel != nil {
		admission.ProgramLevel = *req.ProgramLevel
	}
	if req.StudentName != nil {
		admission.StudentName = *req.StudentName
	}
	if req.StudentEmail != nil {
		admission.StudentEmail = *req.StudentEmail
	}
	if req.StudentPhone != nil {
		admission.StudentPhone = *req.StudentPhone
	}
	if req.DateOfBirth != nil {
		if dob, err := time.Parse("2006-01-02", *req.DateOfBirth); err == nil {
			admission.DateOfBirth = &dob
		}
	}
	if req.Gender != nil {
		admission.Gender = *req.Gender
	}
	if req.Address != nil {
		admission.Address = *req.Address
	}
	if req.City != nil {
		admission.City = *req.City
	}
	if req.LastQualification != nil {
		admission.LastQualification = *req.LastQualification
	}
	if req.Institution != nil {
		admission.Institution = *req.Institution
	}
	if req.GPA != nil {
		admission.GPA = *req.GPA
	}
	if req.EntranceScore != nil {
		admission.EntranceScore = *req.EntranceScore
	}
	if req.Statement != nil {
		admission.Statement = *req.Statement
	}

	if err := s.repo.Save(admission); err != nil {
		return nil, errors.New("failed to update admission application")
	}

	return admission, nil
}

func (s *Service) Delete(id uint, userID uint) error {
	admission, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("admission not found")
	}

	if admission.UserID != nil && *admission.UserID != userID {
		return errors.New("you can only delete your own admissions")
	}

	return s.repo.Delete(id)
}

func (s *Service) UpdateStatus(id uint, req UpdateAdmissionStatusRequest, userID uint) (*Admission, error) {
	admission, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("admission not found")
	}

	now := time.Now()
	admission.Status = req.Status
	admission.Notes = req.Notes
	admission.ReviewedBy = &userID
	admission.ReviewedAt = &now

	if err := s.repo.Save(admission); err != nil {
		return nil, errors.New("failed to update admission status")
	}

	if admission.UserID != nil && notifyStudentFunc != nil {
		statusDisplay := strings.ReplaceAll(req.Status, "_", " ")
		notifyStudentFunc(
			*admission.UserID,
			"Application Status Updated",
			fmt.Sprintf("Your application for %s is now: %s", admission.ProgramName, statusDisplay),
			"application",
			"/user/dashboard/applications",
		)
	}

	return admission, nil
}

func (s *Service) GetByCollegeID(collegeID string, status string) ([]Admission, error) {
	return s.repo.FindByCollegeID(collegeID, status)
}

func (s *Service) GetAll(status string, collegeID string) ([]Admission, error) {
	return s.repo.FindAll(status, collegeID)
}

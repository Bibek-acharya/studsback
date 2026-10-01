package admission

import (
	"context"
	"errors"
	"time"

	"studsphere/backend/internal/notification"
)

type Service struct {
	repo     *Repository
	notifier notification.Notifier
}

func NewService(repo *Repository, notifier notification.Notifier) *Service {
	return &Service{repo: repo, notifier: notifier}
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

	if admission.UserID != nil {
		_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventApplicationSubmitted,
			Recipients: []notification.Ref{{Type: "user", ID: *admission.UserID}},
			Data:       map[string]any{"program": admission.ProgramName},
		})
	}

	if instUserID, err := s.repo.FindApprovedInstitutionUserID(admission.CollegeID); err == nil && instUserID != 0 {
		_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventApplicationReceived,
			Recipients: []notification.Ref{{Type: "institution", ID: instUserID}},
			Data:       map[string]any{"student_name": admission.StudentName, "program": admission.ProgramName},
		})
	}

	return admission, nil
}

func (s *Service) GetMyAdmissions(userID uint) ([]Admission, error) {
	return s.repo.FindByUserID(userID)
}

func (s *Service) GetByID(id uint) (*Admission, error) {
	return s.repo.FindByID(id)
}

// GetByIDForApplicant is the applicant-facing read: their own application, or
// ErrAdmissionNotFound.
//
// This is the fix for the ownership hole in the plain GET /admissions/:id. That
// route is mounted on the protected group AND on the admin group, and the handler
// did its ownership comparison in the handler — after loading the row — so the
// check existed in one of two places and the service, which every other caller
// went through, had none. Update below already refused a non-owner in the service;
// GetByID did not, at any layer.
//
// ErrAdmissionNotFound rather than 403: an Admission row's very existence is
// information about a person who applied to a college, so a refusal that
// confirmed the id would leak what the 404 does not. See ErrAdmissionNotFound.
//
// A row with no UserID is not owned by anybody and is therefore admin-only: it is
// an application submitted without an account, and there is no applicant to
// match it against.
func (s *Service) GetByIDForApplicant(id, userID uint) (*Admission, error) {
	admission, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrAdmissionNotFound
	}
	if admission.UserID == nil || *admission.UserID != userID {
		return nil, ErrAdmissionNotFound
	}
	return admission, nil
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

// UpdateStatus moves an application's workflow status and notifies the applicant.
//
// The reviewer id is `userID`, which is the CALLER and the operator recording the
// decision — it is not the applicant. The notification below then uses
// *admission.UserID as the recipient, which is the applicant. Those two are
// different people and conflating them is how the previous version of this method
// came to look like it was checking ownership when it was not.
//
// It checks nothing about who may call it. That is deliberate and is the shape the
// module uses everywhere: the route supplies a platform-admin gate before the
// handler runs (see PlatformAdminRoles), and the handler does not re-derive it.
// Before this was pinned, the route supplied the shared multi-tenant roleMW, which
// any institution or scholarship_provider account satisfied — so any tenant could
// mark any applicant's application approved or rejected, and overwrite the
// reviewer and review-time fields on a row belonging to somebody else.
//
// There is no tenant-scoped variant on purpose. See PlatformAdminRoles: what a
// college should see of its own applicants is a product decision, and this method
// stays admin-only until that is answered deliberately.
func (s *Service) UpdateStatus(id uint, req UpdateAdmissionStatusRequest, userID uint) (*Admission, error) {
	admission, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrAdmissionNotFound
	}

	now := time.Now()
	admission.Status = req.Status
	admission.Notes = req.Notes
	admission.ReviewedBy = &userID
	admission.ReviewedAt = &now

	if err := s.repo.Save(admission); err != nil {
		return nil, errors.New("failed to update admission status")
	}

	if admission.UserID != nil {
		_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
			EventKey:   notification.EventApplicationStatusChanged,
			Actor:      &notification.Ref{Type: "user", ID: userID},
			Recipients: []notification.Ref{{Type: "user", ID: *admission.UserID}},
			Data:       map[string]any{"program": admission.ProgramName, "status": req.Status},
		})
	}

	return admission, nil
}

func (s *Service) GetByCollegeID(collegeID string, status string) ([]Admission, error) {
	return s.repo.FindByCollegeID(collegeID, status)
}

func (s *Service) GetAll(status string, collegeID string) ([]Admission, error) {
	return s.repo.FindAll(status, collegeID)
}

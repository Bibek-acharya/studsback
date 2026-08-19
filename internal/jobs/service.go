package jobs

import (
	"errors"
	"fmt"
	"math"
	"time"

	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/shared/storage"

	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
	db   *gorm.DB
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func NewServiceWithDB(repo *Repository, db *gorm.DB) *Service {
	return &Service{repo: repo, db: db}
}

func (s *Service) CreateJob(req CreateJobRequest) (*Job, error) {
	job := &Job{
		Title:        req.Title,
		Department:   req.Department,
		Description:  req.Description,
		Requirements: req.Requirements,
		Location:     req.Location,
		JobType:      req.JobType,
		PositionsOpen: req.PositionsOpen,
		SalaryRange:  req.SalaryRange,
		Status:       req.Status,
	}

	if req.ApplicationDeadline != nil {
		if t, err := time.Parse("2006-01-02", *req.ApplicationDeadline); err == nil {
			job.ApplicationDeadline = &t
		}
	}

	if err := s.repo.CreateJob(job); err != nil {
		return nil, errors.New("failed to create job")
	}
	return job, nil
}

func (s *Service) GetJobByID(id uint) (*Job, error) {
	job, err := s.repo.FindJobByID(id)
	if err != nil {
		return nil, errors.New("job not found")
	}
	return job, nil
}

func (s *Service) GetPublishedJobByID(id uint) (*Job, error) {
	job, err := s.repo.FindJobByID(id)
	if err != nil {
		return nil, errors.New("job not found")
	}
	if job.Status != "published" {
		return nil, errors.New("job not found")
	}
	return job, nil
}

func (s *Service) UpdateJob(id uint, req UpdateJobRequest) (*Job, error) {
	job, err := s.repo.FindJobByID(id)
	if err != nil {
		return nil, errors.New("job not found")
	}

	if req.Title != nil {
		job.Title = *req.Title
	}
	if req.Department != nil {
		job.Department = *req.Department
	}
	if req.Description != nil {
		job.Description = *req.Description
	}
	if req.Requirements != nil {
		job.Requirements = *req.Requirements
	}
	if req.Location != nil {
		job.Location = *req.Location
	}
	if req.JobType != nil {
		job.JobType = *req.JobType
	}
	if req.PositionsOpen != nil {
		job.PositionsOpen = *req.PositionsOpen
	}
	if req.SalaryRange != nil {
		job.SalaryRange = *req.SalaryRange
	}
	if req.Status != nil {
		job.Status = *req.Status
	}
	if req.ApplicationDeadline != nil {
		if t, err := time.Parse("2006-01-02", *req.ApplicationDeadline); err == nil {
			job.ApplicationDeadline = &t
		}
	}

	if err := s.repo.UpdateJob(job); err != nil {
		return nil, errors.New("failed to update job")
	}
	return job, nil
}

func (s *Service) DeleteJob(id uint) error {
	job, err := s.repo.FindJobByID(id)
	if err != nil {
		return errors.New("job not found")
	}

	apps, err := s.repo.ListApplicationsByJobForFiles(job.ID)
	if err == nil {
		for _, app := range apps {
			if app.ResumeURL != "" {
				storage.DeleteObject(app.ResumeURL)
			}
			if app.CoverLetterURL != "" {
				storage.DeleteObject(app.CoverLetterURL)
			}
		}
	}

	return s.repo.DeleteJob(id)
}

func (s *Service) autoCloseExpiredJobs() {
	now := time.Now()
	s.db.Model(&Job{}).
		Where("status = ? AND application_deadline IS NOT NULL AND application_deadline < ?", "published", now).
		Update("status", "closed")
}

func (s *Service) ListPublishedJobs(department, search string, page, limit int) *PaginatedJobsResponse {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 12
	}

	s.autoCloseExpiredJobs()

	jobs, total, _ := s.repo.ListPublishedJobs(department, search, page, limit)

	jobIDs := make([]uint, len(jobs))
	for i, j := range jobs {
		jobIDs[i] = j.ID
	}
	counts := s.repo.GetJobApplicationCounts(jobIDs)

	resp := make([]JobResponse, len(jobs))
	for i, j := range jobs {
		resp[i] = toJobResponse(&j, counts[j.ID])
	}

	return &PaginatedJobsResponse{
		Jobs:       resp,
		Total:      total,
		Page:       page,
		PerPage:    limit,
		TotalPages: int(math.Ceil(float64(total) / float64(limit))),
	}
}

func (s *Service) ListAllJobs(status, search string, page, limit int) *PaginatedJobsResponse {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	jobs, total, _ := s.repo.ListAllJobs(status, search, page, limit)

	jobIDs := make([]uint, len(jobs))
	for i, j := range jobs {
		jobIDs[i] = j.ID
	}
	counts := s.repo.GetJobApplicationCounts(jobIDs)

	resp := make([]JobResponse, len(jobs))
	for i, j := range jobs {
		resp[i] = toJobResponse(&j, counts[j.ID])
	}

	return &PaginatedJobsResponse{
		Jobs:       resp,
		Total:      total,
		Page:       page,
		PerPage:    limit,
		TotalPages: int(math.Ceil(float64(total) / float64(limit))),
	}
}

func (s *Service) GetDepartments() ([]string, error) {
	return s.repo.GetDepartments()
}

func (s *Service) SubmitApplication(jobID uint, fullName, email, phone, resumeURL, coverLetterURL string) (*JobApplication, error) {
	job, err := s.repo.FindJobByID(jobID)
	if err != nil {
		return nil, errors.New("job not found")
	}
	if job.Status != "published" {
		return nil, errors.New("job is not accepting applications")
	}
	if job.ApplicationDeadline != nil && time.Now().After(*job.ApplicationDeadline) {
		return nil, errors.New("application deadline has passed")
	}
	if s.repo.ApplicationExists(jobID, email) {
		return nil, errors.New("you have already applied to this job")
	}

	app := &JobApplication{
		JobID:          jobID,
		FullName:       fullName,
		Email:          email,
		Phone:          phone,
		ResumeURL:      resumeURL,
		CoverLetterURL: coverLetterURL,
		Status:         "pending",
	}

	if err := s.repo.CreateApplication(app); err != nil {
		return nil, errors.New("failed to submit application")
	}
	return app, nil
}

func (s *Service) ListApplications(jobID uint, status, search string, page, limit int) *PaginatedApplicationsResponse {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	apps, total, _ := s.repo.ListApplicationsByJob(jobID, status, search, page, limit)

	resp := make([]JobApplicationResponse, len(apps))
	for i, a := range apps {
		resp[i] = toApplicationResponse(&a)
	}

	return &PaginatedApplicationsResponse{
		Applications: resp,
		Total:        total,
		Page:         page,
		PerPage:      limit,
		TotalPages:   int(math.Ceil(float64(total) / float64(limit))),
	}
}

func (s *Service) GetApplicationByID(id uint) (*JobApplication, error) {
	app, err := s.repo.FindApplicationByID(id)
	if err != nil {
		return nil, errors.New("application not found")
	}
	return app, nil
}

func (s *Service) UpdateApplicationStatus(id uint, req UpdateApplicantStatusRequest) (*JobApplication, error) {
	app, err := s.repo.FindApplicationByID(id)
	if err != nil {
		return nil, errors.New("application not found")
	}

	app.Status = req.Status
	app.Notes = req.Notes

	if err := s.repo.UpdateApplication(app); err != nil {
		return nil, errors.New("failed to update application status")
	}
	return app, nil
}

func (s *Service) UpdateApplicationNotes(id uint, notes string) (*JobApplication, error) {
	app, err := s.repo.FindApplicationByID(id)
	if err != nil {
		return nil, errors.New("application not found")
	}

	app.Notes = notes

	if err := s.repo.UpdateApplication(app); err != nil {
		return nil, errors.New("failed to update notes")
	}
	return app, nil
}

func (s *Service) SendApplicantEmail(id uint, req SendApplicantEmailRequest) error {
	app, err := s.repo.FindApplicationByID(id)
	if err != nil {
		return fmt.Errorf("application not found")
	}

	if err := emailqueue.EnqueueGenericEmail(app.Email, req.Subject, req.Body); err != nil {
		return fmt.Errorf("failed to enqueue email: %w", err)
	}

	if req.UpdateStatus != "" {
		app.Status = req.UpdateStatus
		if err := s.repo.UpdateApplication(app); err != nil {
			return fmt.Errorf("email sent but failed to update status: %w", err)
		}
	}

	return nil
}

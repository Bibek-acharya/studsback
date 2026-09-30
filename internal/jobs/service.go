package jobs

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/notification"
	"studsphere/backend/internal/shared/storage"

	"gorm.io/gorm"
)

type Service struct {
	repo     *Repository
	db       *gorm.DB
	notifier notification.Notifier
}

func NewService(repo *Repository, notifier notification.Notifier) *Service {
	return &Service{repo: repo, notifier: notifier}
}

func NewServiceWithDB(repo *Repository, db *gorm.DB, notifier notification.Notifier) *Service {
	return &Service{repo: repo, db: db, notifier: notifier}
}

func (s *Service) CreateJob(req CreateJobRequest) (*Job, error) {
	job := &Job{
		Title:         req.Title,
		Department:    req.Department,
		Description:   req.Description,
		Requirements:  req.Requirements,
		Location:      req.Location,
		JobType:       req.JobType,
		PositionsOpen: req.PositionsOpen,
		SalaryRange:   req.SalaryRange,
		Status:        req.Status,
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

// SubmitApplication records a job application.
//
// applicantUserID is the authenticated submitter, or 0 for a guest. The public
// apply form is OptionalAuth, not Auth, so a signed-out applicant still submits
// exactly as before; the id is stamped only when a valid token was presented,
// and it is what later lets that person read their own record. See
// JobApplication.ApplicantUserID for why the submitted email cannot stand in
// for this.
func (s *Service) SubmitApplication(jobID uint, fullName, email, phone, resumeURL, coverLetterURL string, applicantUserID uint) (*JobApplication, error) {
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
	if applicantUserID != 0 {
		app.ApplicantUserID = &applicantUserID
	}

	if err := s.repo.CreateApplication(app); err != nil {
		return nil, errors.New("failed to submit application")
	}

	if s.notifier != nil {
		if audience, _ := s.notifier.ForRoles(context.Background(), "superadmin", "admin"); len(audience) > 0 {
			_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventJobsApplicationReceived,
				Recipients: audience,
				Data:       map[string]any{"name": fullName, "title": job.Title},
			})
		}
	}

	return app, nil
}

// ListApplications returns the applicants to one job. Admin only — see
// authorizeJobRead: the response carries name, email and phone for every
// applicant to the posting.
func (s *Service) ListApplications(v Viewer, jobID uint, status, search string, page, limit int) (*PaginatedApplicationsResponse, error) {
	if err := s.authorizeJobRead(jobID, v); err != nil {
		return nil, err
	}

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
	}, nil
}

// GetApplicationByID returns one application to a viewer entitled to it: a
// platform admin, or the applicant who submitted it. Returns
// ErrApplicationNotFound for anyone else — including for an id that does not
// exist, so the two are indistinguishable from outside.
func (s *Service) GetApplicationByID(v Viewer, id uint) (*JobApplication, error) {
	return s.authorizeApplicationRead(id, v)
}

func (s *Service) UpdateApplicationStatus(v Viewer, id uint, req UpdateApplicantStatusRequest) (*JobApplication, error) {
	app, err := s.authorizeApplicationWrite(id, v)
	if err != nil {
		return nil, err
	}

	app.Status = req.Status
	app.Notes = req.Notes

	if err := s.repo.UpdateApplication(app); err != nil {
		return nil, errors.New("failed to update application status")
	}

	s.notifyApplicant(app)

	return app, nil
}

func (s *Service) UpdateApplicationNotes(v Viewer, id uint, notes string) (*JobApplication, error) {
	app, err := s.authorizeApplicationWrite(id, v)
	if err != nil {
		return nil, err
	}

	app.Notes = notes

	if err := s.repo.UpdateApplication(app); err != nil {
		return nil, errors.New("failed to update notes")
	}
	return app, nil
}

// notifyApplicant emits jobs.status_changed to the applicant's account.
// Applicants are guests identified only by email, so the emission resolves
// their account by email and skips when they have none (anonymous email
// delivery is a P3 seam).
func (s *Service) notifyApplicant(app *JobApplication) {
	if s.notifier == nil {
		return
	}
	uid, err := s.repo.FindUserIDByEmail(app.Email)
	if err != nil || uid == 0 {
		return
	}
	_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
		EventKey:   notification.EventJobsStatusChanged,
		Recipients: []notification.Ref{{Type: "user", ID: uid}},
		Data:       map[string]any{"job_title": app.Job.Title, "status": app.Status},
	})
}

// SendApplicantEmail sends platform email to an applicant, or rides the status
// change through the notification pipeline. Admin only — before the ownership
// check this was the sharpest edge in the module: any role in roleMW could email
// any applicant on the platform by id.
func (s *Service) SendApplicantEmail(v Viewer, id uint, req SendApplicantEmailRequest) error {
	app, err := s.authorizeApplicationWrite(id, v)
	if err != nil {
		return err
	}

	if req.UpdateStatus != "" {
		// The status-change email rides the notification pipeline
		// (jobs.status_changed, EmailDefault) instead of the old manual send
		// here — the manual call would double-email now that the pipeline
		// emails on its own.
		_, err = s.UpdateApplicationStatus(v, id, UpdateApplicantStatusRequest{Status: req.UpdateStatus, Notes: app.Notes})
		return err
	}

	if err := emailqueue.EnqueueGenericEmail(app.Email, req.Subject, req.Body); err != nil {
		return fmt.Errorf("failed to enqueue email: %w", err)
	}

	return nil
}

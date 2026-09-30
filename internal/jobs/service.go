package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
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

	// deleteObject is storage.DeleteObject in production. It is a field only so
	// the delete cascade can be asserted in a test without a live object store;
	// nothing else sets it. The nil fallback below keeps a Service built as a
	// struct literal working.
	deleteObject func(string) error
}

func NewService(repo *Repository, notifier notification.Notifier) *Service {
	return &Service{repo: repo, notifier: notifier, deleteObject: storage.DeleteObject}
}

func NewServiceWithDB(repo *Repository, db *gorm.DB, notifier notification.Notifier) *Service {
	return &Service{repo: repo, db: db, notifier: notifier, deleteObject: storage.DeleteObject}
}

// deleteStoredObject removes one object from the store, returning its error so
// a failed removal can be recorded rather than discarded.
func (s *Service) deleteStoredObject(objectPath string) error {
	if s.deleteObject != nil {
		return s.deleteObject(objectPath)
	}
	return storage.DeleteObject(objectPath)
}

// CreateJob adds a posting to the platform's own catalogue. Platform-admin only:
// see access.go for why the catalogue is first-party and who may change it.
func (s *Service) CreateJob(v Viewer, req CreateJobRequest) (*Job, error) {
	if err := s.authorizeJobAdmin(v, "create"); err != nil {
		return nil, err
	}

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

// GetJobByID returns a posting to the catalogue's admin reader.
//
// Admin only, unlike GetPublishedJobByID below. This is the unfiltered read:
// it answers for drafts and for internal fields the public shape omits, and it
// carries the applicant count. The public route stays public and keeps using
// GetPublishedJobByID, which is status-filtered.
func (s *Service) GetJobByID(v Viewer, id uint) (*Job, error) {
	if err := s.authorizeJobAdmin(v, "read"); err != nil {
		return nil, err
	}

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

// UpdateJob edits a posting. Platform-admin only — access.go.
func (s *Service) UpdateJob(v Viewer, id uint, req UpdateJobRequest) (*Job, error) {
	if err := s.authorizeJobAdmin(v, "update"); err != nil {
		return nil, err
	}

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

// DeleteJob removes a posting, and with it every application to it.
//
// Platform-admin only, and this is the sharpest edge in the module. The cascade
// is what makes it one: it destroys other people's personal documents. Nothing
// here is scoped to the caller, because there is no caller-scoped alternative —
// a posting belongs to the platform, and an applicant has no claim on the
// deletion of the job they applied to.
//
// WHAT CHANGED, because a delete is not something to quietly reshape:
//
// 1. Authorization. This was reachable by any role in the shared roleMW,
// including "institution" and "scholarship_provider". Now admin only, at the
// route and again here.
//
// 2. Order. The database rows were deleted AFTER the files, which meant a
// failed transaction left live applications pointing at documents that had
// already been destroyed — irrecoverable, because MinIO removal is not
// reversible while the rows are only soft-deleted. The transaction now runs
// first and the objects are removed after it commits. The failure mode that
// replaces it is orphaned objects for applications whose rows are already gone,
// which is recoverable and unreachable through the API. The document paths are
// therefore read before the transaction, since a soft-deleted row is no longer
// returned by an ordinary query.
//
// 3. Refusal on an unreadable document list. A failure to read the applications
// used to be ignored and the delete proceeded, deleting the rows and stranding
// every document in the bucket. It now refuses, because a delete that cannot
// complete its own cleanup should not pretend to.
//
// 4. Record. DeleteObject's error was discarded, so a failed removal was
// invisible. Every removed object is now counted and logged, and a failure
// names the object.
//
// NOT CHANGED, and deliberately: the request shape. A single DELETE from an
// admin still destroys every applicant's documents with no confirmation and no
// dry run. Making that cost more — a confirmation parameter, an archive
// endpoint, refusing while applications exist — is a product decision, because
// it changes what the superadmin dashboard's delete button can do, and the
// dashboard lives in another repository. It is recorded as the recommended
// follow-up rather than landed here. Soft-deleted application rows remain
// recoverable by an operator with Unscoped(); the objects do not, which is why
// the log line above matters.
func (s *Service) DeleteJob(v Viewer, id uint) error {
	if err := s.authorizeJobAdmin(v, "delete"); err != nil {
		return err
	}

	job, err := s.repo.FindJobByID(id)
	if err != nil {
		return errors.New("job not found")
	}

	apps, err := s.repo.ListApplicationsByJobForFiles(job.ID)
	if err != nil {
		return errors.New("failed to read the applications attached to this job")
	}

	if err := s.repo.DeleteJob(id); err != nil {
		return err
	}

	removed := 0
	for _, app := range apps {
		for _, objectPath := range []string{app.ResumeURL, app.CoverLetterURL} {
			if objectPath == "" {
				continue
			}
			if err := s.deleteStoredObject(objectPath); err != nil {
				log.Printf("jobs: deleting job %d left %q in the object store: %v", id, objectPath, err)
				continue
			}
			removed++
		}
	}
	log.Printf("jobs: deleted job %d as user_id=%d role=%q; cascaded over %d applications and destroyed %d stored documents",
		id, v.UserID, v.Role, len(apps), removed)

	return nil
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

// ListAllJobs is the unfiltered catalogue listing — drafts included — for the
// superadmin dashboard. Platform-admin only, for the reasons in access.go. The
// public /careers listing is a different method with a status filter.
func (s *Service) ListAllJobs(v Viewer, status, search string, page, limit int) (*PaginatedJobsResponse, error) {
	if err := s.authorizeJobAdmin(v, "list"); err != nil {
		return nil, err
	}

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
	}, nil
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

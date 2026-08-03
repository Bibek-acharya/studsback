package jobs

import "time"

type CreateJobRequest struct {
	Title              string  `json:"title" binding:"required"`
	Department         string  `json:"department" binding:"required"`
	Description        string  `json:"description" binding:"required"`
	Requirements       string  `json:"requirements"`
	Location           string  `json:"location"`
	JobType            string  `json:"job_type" binding:"required,oneof=full-time part-time contract internship"`
	SalaryRange        string  `json:"salary_range"`
	ApplicationDeadline *string `json:"application_deadline"`
	Status             string  `json:"status" binding:"required,oneof=draft published"`
}

type UpdateJobRequest struct {
	Title              *string `json:"title"`
	Department         *string `json:"department"`
	Description        *string `json:"description"`
	Requirements       *string `json:"requirements"`
	Location           *string `json:"location"`
	JobType            *string `json:"job_type" binding:"omitempty,oneof=full-time part-time contract internship"`
	SalaryRange        *string `json:"salary_range"`
	ApplicationDeadline *string `json:"application_deadline"`
	Status             *string `json:"status" binding:"omitempty,oneof=draft published closed"`
}

type UpdateApplicantStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=shortlisted rejected"`
	Notes  string `json:"notes"`
}

type SendApplicantEmailRequest struct {
	Subject      string `json:"subject" binding:"required"`
	Body         string `json:"body" binding:"required"`
	UpdateStatus string `json:"update_status" binding:"omitempty,oneof=shortlisted rejected"`
}

type JobResponse struct {
	ID                 uint    `json:"id"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	Title              string  `json:"title"`
	Department         string  `json:"department"`
	Description        string  `json:"description"`
	Requirements       string  `json:"requirements"`
	Location           string  `json:"location"`
	JobType            string  `json:"job_type"`
	SalaryRange        string  `json:"salary_range"`
	ApplicationDeadline *string `json:"application_deadline,omitempty"`
	Status             string  `json:"status"`
	ApplicationCount   int64   `json:"application_count"`
}

type JobDetailResponse struct {
	JobResponse
}

type JobApplicationResponse struct {
	ID             uint   `json:"id"`
	CreatedAt      string `json:"created_at"`
	JobID          uint   `json:"job_id"`
	JobTitle       string `json:"job_title"`
	FullName       string `json:"full_name"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	HasResume      bool   `json:"has_resume"`
	HasCoverLetter bool   `json:"has_cover_letter"`
	Status         string `json:"status"`
	Notes          string `json:"notes,omitempty"`
}

type PaginatedJobsResponse struct {
	Jobs       []JobResponse `json:"jobs"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalPages int           `json:"total_pages"`
}

type PaginatedApplicationsResponse struct {
	Applications []JobApplicationResponse `json:"applications"`
	Total        int64                    `json:"total"`
	Page         int                      `json:"page"`
	PerPage      int                      `json:"per_page"`
	TotalPages   int                      `json:"total_pages"`
}

func toJobResponse(job *Job, appCount int64) JobResponse {
	resp := JobResponse{
		ID:              job.ID,
		CreatedAt:       job.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       job.UpdatedAt.Format(time.RFC3339),
		Title:           job.Title,
		Department:      job.Department,
		Description:     job.Description,
		Requirements:    job.Requirements,
		Location:        job.Location,
		JobType:         job.JobType,
		SalaryRange:     job.SalaryRange,
		Status:          job.Status,
		ApplicationCount: appCount,
	}
	if job.ApplicationDeadline != nil {
		deadline := job.ApplicationDeadline.Format(time.RFC3339)
		resp.ApplicationDeadline = &deadline
	}
	return resp
}

func toApplicationResponse(app *JobApplication) JobApplicationResponse {
	return JobApplicationResponse{
		ID:             app.ID,
		CreatedAt:      app.CreatedAt.Format(time.RFC3339),
		JobID:          app.JobID,
		JobTitle:       app.Job.Title,
		FullName:       app.FullName,
		Email:          app.Email,
		Phone:          app.Phone,
		HasResume:      app.ResumeURL != "",
		HasCoverLetter: app.CoverLetterURL != "",
		Status:         app.Status,
		Notes:          app.Notes,
	}
}

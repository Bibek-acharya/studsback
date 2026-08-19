package jobs

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

const maxFileSize = 5 << 20 // 5MB

func (h *Handler) CreateJob(c *gin.Context) {
	var req CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	job, err := h.service.CreateJob(req)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 201, "Job created successfully", toJobResponse(job, 0))
}

func (h *Handler) GetJob(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid job ID")
		return
	}

	job, err := h.service.GetJobByID(id)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	appCount, _ := h.service.repo.GetJobApplicationCount(job.ID)
	response.Success(c, 200, "Job retrieved", toJobResponse(job, appCount))
}

func (h *Handler) UpdateJob(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid job ID")
		return
	}

	var req UpdateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	job, err := h.service.UpdateJob(id, req)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Job updated successfully", toJobResponse(job, 0))
}

func (h *Handler) DeleteJob(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid job ID")
		return
	}

	if err := h.service.DeleteJob(id); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Job deleted successfully", nil)
}

func (h *Handler) ListPublishedJobs(c *gin.Context) {
	department := c.Query("department")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))

	result := h.service.ListPublishedJobs(department, search, page, limit)
	response.Success(c, 200, "Jobs retrieved", result)
}

func (h *Handler) ListAllJobs(c *gin.Context) {
	status := c.Query("status")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result := h.service.ListAllJobs(status, search, page, limit)
	response.Success(c, 200, "Jobs retrieved", result)
}

func (h *Handler) GetDepartments(c *gin.Context) {
	depts, err := h.service.GetDepartments()
	if err != nil {
		response.Error(c, 500, "Failed to fetch departments")
		return
	}
	response.Success(c, 200, "Departments retrieved", depts)
}

func (h *Handler) GetPublishedJob(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid job ID")
		return
	}

	job, err := h.service.GetPublishedJobByID(id)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	response.Success(c, 200, "Job retrieved", toJobResponse(job, 0))
}

func (h *Handler) SubmitApplication(c *gin.Context) {
	jobID, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid job ID")
		return
	}

	if err := c.Request.ParseMultipartForm(maxFileSize); err != nil {
		response.Error(c, 400, "File too large (max 5MB)")
		return
	}

	fullName := c.PostForm("full_name")
	email := c.PostForm("email")
	phone := c.PostForm("phone")

	if fullName == "" || email == "" || phone == "" {
		response.Error(c, 400, "full_name, email, and phone are required")
		return
	}

	resumeFile, header, err := c.Request.FormFile("resume")
	if err != nil {
		response.Error(c, 400, "Resume file is required")
		return
	}
	defer resumeFile.Close()

	if err := validatePDF(resumeFile, header); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	jobIDStr := strconv.FormatUint(uint64(jobID), 10)
	resumeFilename := uuid.New().String() + ".pdf"
	resumePath := "resumes/" + jobIDStr + "/" + resumeFilename

	if err := storage.Upload(resumePath, resumeFile, header.Size, "application/pdf"); err != nil {
		response.Error(c, 500, "Failed to upload resume")
		return
	}

	var coverLetterPath string
	clFile, clHeader, err := c.Request.FormFile("cover_letter")
	if err == nil {
		defer clFile.Close()
		if err := validatePDF(clFile, clHeader); err != nil {
			storage.DeleteObject(resumePath)
			response.Error(c, 400, "Cover letter: "+err.Error())
			return
		}
		clFilename := uuid.New().String() + "_cl.pdf"
		coverLetterPath = "resumes/" + jobIDStr + "/" + clFilename
		if err := storage.Upload(coverLetterPath, clFile, clHeader.Size, "application/pdf"); err != nil {
			storage.DeleteObject(resumePath)
			response.Error(c, 500, "Failed to upload cover letter")
			return
		}
	}

	app, err := h.service.SubmitApplication(jobID, fullName, email, phone, resumePath, coverLetterPath)
	if err != nil {
		storage.DeleteObject(resumePath)
		if coverLetterPath != "" {
			storage.DeleteObject(coverLetterPath)
		}
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, 201, "Application submitted successfully", toApplicationResponse(app))
}

func (h *Handler) ListApplications(c *gin.Context) {
	jobID, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid job ID")
		return
	}

	status := c.Query("status")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result := h.service.ListApplications(jobID, status, search, page, limit)
	response.Success(c, 200, "Applications retrieved", result)
}

func (h *Handler) UpdateApplicantStatus(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid application ID")
		return
	}

	var req UpdateApplicantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	app, err := h.service.UpdateApplicationStatus(id, req)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Status updated successfully", toApplicationResponse(app))
}

func (h *Handler) UpdateApplicantNotes(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid application ID")
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	app, err := h.service.UpdateApplicationNotes(id, req.Notes)
	if err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Notes updated successfully", toApplicationResponse(app))
}

func (h *Handler) SendApplicantEmail(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid application ID")
		return
	}

	var req SendApplicantEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	if err := h.service.SendApplicantEmail(id, req); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	response.Success(c, 200, "Email sent successfully", nil)
}

func (h *Handler) ServeResume(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid application ID")
		return
	}

	app, err := h.service.GetApplicationByID(id)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	h.serveFile(c, app.ResumeURL)
}

func (h *Handler) ServeCoverLetter(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		response.Error(c, 400, "Invalid application ID")
		return
	}

	app, err := h.service.GetApplicationByID(id)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}

	if app.CoverLetterURL == "" {
		response.Error(c, 404, "No cover letter uploaded")
		return
	}

	h.serveFile(c, app.CoverLetterURL)
}

func (h *Handler) serveFile(c *gin.Context, filePath string) {
	if filePath == "" {
		c.Status(http.StatusNotFound)
		return
	}

	reader, info, err := storage.Get(filePath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	ct := info.ContentType
	if ct == "" {
		ct = "application/pdf"
	}

	filename := filepath.Base(filePath)
	if c.Query("download") == "true" {
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	} else {
		c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
	}

	c.DataFromReader(http.StatusOK, -1, ct, reader, nil)
}

func validatePDF(file io.ReadSeeker, header *multipart.FileHeader) error {
	if header.Size > maxFileSize {
		return fmt.Errorf("file too large (max 5MB)")
	}

	ext := strings.ToLower(filepath.Base(header.Filename))
	if !strings.HasSuffix(ext, ".pdf") {
		return fmt.Errorf("only PDF files are accepted")
	}

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return fmt.Errorf("unable to read file")
	}
	buf = buf[:n]

	contentType := http.DetectContentType(buf)
	if contentType != "application/pdf" {
		mimeTypes, _, _ := mime.ParseMediaType(header.Header.Get("Content-Type"))
		if mimeTypes != "application/pdf" {
			return fmt.Errorf("only PDF files are accepted")
		}
	}

	_, err = file.Seek(0, io.SeekStart)
	return err
}

func parseID(param string) (uint, error) {
	id, err := strconv.ParseUint(param, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

// Ensure time import is used
var _ = time.Now

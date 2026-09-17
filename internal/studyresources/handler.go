package studyresources

import (
	"context"
	"fmt"
	"io"
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

var allowedExtensions = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".ppt": true, ".pptx": true,
	".xls": true, ".xlsx": true, ".txt": true, ".csv": true,
	".zip": true, ".rar": true, ".7z": true,
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

const maxFileSize = 20 * 1024 * 1024 // 20MB

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListResources(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := ResourceFilters{
		Search: c.Query("q"),
		Type:   c.Query("type"),
		Course: c.Query("course"),
		Year:   c.Query("year"),
	}

	resources, total, err := h.service.GetResources(filters, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch study resources")
		return
	}

	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Study resources fetched successfully", gin.H{
		"page":            page,
		"limit":           limit,
		"total":           total,
		"study_resources": resources,
	})
}

func (h *Handler) AdminListResources(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := ResourceFilters{
		Search: c.Query("q"),
		Type:   c.Query("type"),
		Course: c.Query("course"),
		Year:   c.Query("year"),
	}

	resources, total, err := h.service.GetResources(filters, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch study resources")
		return
	}

	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Study resources fetched successfully", gin.H{
		"page":            page,
		"limit":           limit,
		"total":           total,
		"study_resources": resources,
	})
}

func (h *Handler) GetResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}
	resource, err := h.service.GetResource(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Resource not found")
		return
	}
	response.Success(c, http.StatusOK, "Resource fetched successfully", resource)
}

func (h *Handler) DownloadResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}

	resource, err := h.service.GetResource(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Resource not found")
		return
	}

	// Download counting is best-effort and happens before streaming.
	h.service.IncrementDownloads(uint(id))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	reader, info, err := storage.GetWithContext(ctx, resource.FilePath)
	if err != nil {
		response.Error(c, http.StatusNotFound, "File not found")
		return
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	contentType := info.ContentType
	if contentType == "" {
		contentType = resource.MimeType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	filename := resource.FileName
	if filename == "" {
		filename = filepath.Base(resource.FilePath)
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if resource.FileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(resource.FileSize, 10))
	}
	c.DataFromReader(http.StatusOK, resource.FileSize, contentType, reader, nil)
}

func sanitizeFileName(name, ext string) string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	sanitized := b.String()
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return "file" + ext
	}
	return sanitized + ext
}

func (h *Handler) CreateResource(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != "" && !allowedExtensions[ext] {
		response.Error(c, http.StatusBadRequest, "File type not allowed")
		return
	}
	if ext == "" {
		response.Error(c, http.StatusBadRequest, "File has no extension")
		return
	}
	if fileHeader.Size > maxFileSize {
		response.Error(c, http.StatusBadRequest, "File size exceeds limit of 20MB")
		return
	}

	var req CreateResourceRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Failed to open uploaded file")
		return
	}
	defer src.Close()

	objectPath := fmt.Sprintf("study-resources/%s-%s", uuid.NewString(), sanitizeFileName(fileHeader.Filename, ext))

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := storage.Upload(objectPath, src, fileHeader.Size, contentType); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to upload file")
		return
	}

	userID, _ := c.Get("user_id")
	uploadedBy, _ := userID.(int)

	resource := &StudyResource{
		Title:        req.Title,
		Description:  req.Description,
		ResourceType: req.ResourceType,
		Course:       req.Course,
		Year:         req.Year,
		FileName:     fileHeader.Filename,
		FilePath:     objectPath,
		FileURL:      "/uploads/" + objectPath,
		FileSize:     fileHeader.Size,
		MimeType:     contentType,
		UploadedBy:   uint(uploadedBy),
	}
	if err := h.service.CreateResource(resource); err != nil {
		// Clean up the uploaded object if the DB save fails.
		_ = storage.DeleteObject(objectPath)
		response.Error(c, http.StatusInternalServerError, "Failed to create resource")
		return
	}

	response.Success(c, http.StatusCreated, "Resource created", resource)
}

func (h *Handler) UpdateResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}
	var req UpdateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	resource, err := h.service.UpdateResource(uint(id), req)
	if err != nil {
		response.Error(c, http.StatusNotFound, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Resource updated", resource)
}

func (h *Handler) DeleteResource(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return
	}

	resource, err := h.service.GetResource(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Resource not found")
		return
	}

	// Soft-delete the model first; object cleanup is best-effort afterwards.
	if err := h.service.DeleteResource(uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete resource")
		return
	}

	if resource.FilePath != "" {
		_ = storage.DeleteObject(resource.FilePath)
	}

	response.Success(c, http.StatusOK, "Resource deleted", nil)
}

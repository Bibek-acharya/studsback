package studyresources

import (
	"context"
	"errors"
	"fmt"
	"io"
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

	data := gin.H{
		"page":            page,
		"limit":           limit,
		"total":           total,
		"study_resources": resources,
	}
	// Filter facets: distinct years/courses across ALL resources (not just the
	// current page). Best-effort — a facet query failure must not break listing.
	years, courses, err := h.service.DistinctFacets()
	if err != nil {
		years, courses = []string{}, []string{}
	}
	data["years"] = years
	data["courses"] = courses

	response.Success(c, http.StatusOK, "Study resources fetched successfully", data)
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

// validateUpload checks the multipart file against the shared extension
// whitelist and size limit. It returns the normalized lowercase extension.
func validateUpload(fileHeader *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == "" {
		return "", errors.New("file has no extension")
	}
	if !allowedExtensions[ext] {
		return "", errors.New("file type not allowed")
	}
	if fileHeader.Size > maxFileSize {
		return "", errors.New("file size exceeds limit of 20MB")
	}
	return ext, nil
}

// uploadFile validates the upload, stores it in object storage under
// study-resources/, and returns the storage key. The multipart file header
// must remain unopened by the caller so the multipart form is still readable.
func uploadFile(fileHeader *multipart.FileHeader) (objectPath string, contentType string, err error) {
	ext, err := validateUpload(fileHeader)
	if err != nil {
		return "", "", err
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", "", errors.New("failed to open uploaded file")
	}
	defer src.Close()

	objectPath = fmt.Sprintf("study-resources/%s-%s", uuid.NewString(), sanitizeFileName(fileHeader.Filename, ext))

	contentType = fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := storage.Upload(objectPath, src, fileHeader.Size, contentType); err != nil {
		return "", "", errUploadFailed
	}
	return objectPath, contentType, nil
}

var errUploadFailed = errors.New("failed to upload file")

func (h *Handler) CreateResource(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	var req CreateResourceRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	objectPath, contentType, err := uploadFile(fileHeader)
	if err != nil {
		if errors.Is(err, errUploadFailed) {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.Error(c, http.StatusBadRequest, err.Error())
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

// ReplaceResourceFile handles POST /admin/study-resources/:id/file.
// It uploads a NEW object, saves the updated metadata, and then best-effort
// deletes the OLD object so a failed save never loses the original file.
func (h *Handler) ReplaceResourceFile(c *gin.Context) {
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

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	// Capture the old object key BEFORE it is overwritten.
	oldObjectPath := resource.FilePath

	objectPath, contentType, err := uploadFile(fileHeader)
	if err != nil {
		if errors.Is(err, errUploadFailed) {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	resource.FileName = fileHeader.Filename
	resource.FilePath = objectPath
	resource.FileURL = "/uploads/" + objectPath
	resource.FileSize = fileHeader.Size
	resource.MimeType = contentType

	if err := h.service.UpdateResourceModel(resource); err != nil {
		// Clean up the new object if the DB save fails; the old file remains intact.
		_ = storage.DeleteObject(objectPath)
		response.Error(c, http.StatusInternalServerError, "Failed to replace resource file")
		return
	}

	// Best-effort cleanup of the OLD object, only after a successful save.
	// Guard against deleting the new key if the upload collided (it cannot in
	// practice because of the UUID prefix, but stay defensive).
	if oldObjectPath != "" && oldObjectPath != objectPath {
		_ = storage.DeleteObject(oldObjectPath)
	}

	response.Success(c, http.StatusOK, "Resource file replaced", resource)
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
		_ = storage.DeleteObject(normalizeObjectKey(resource.FilePath))
	}

	response.Success(c, http.StatusOK, "Resource deleted", nil)
}

// normalizeObjectKey maps legacy stored values to a raw object key. The
// canonical FilePath is the bare MinIO key (e.g. "study-resources/x.pdf"), but
// older rows may hold a "/uploads/..." URL-style path — trim that prefix the
// same way scholarship/service.go does.
func normalizeObjectKey(v string) string {
	if strings.HasPrefix(v, "/uploads/") {
		return v[len("/uploads/"):]
	}
	return v
}

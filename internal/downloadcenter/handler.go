package downloadcenter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"studsphere/backend/internal/shared/response"
	"studsphere/backend/internal/shared/storage"
	"studsphere/backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListItems handles GET /api/v1/downloads (published items only).
func (h *Handler) ListItems(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := ItemFilters{
		Category:      c.Query("category"),
		PublishedOnly: true,
	}

	items, total, err := h.service.GetItems(filters, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch download items")
		return
	}

	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Download items fetched successfully", gin.H{
		"page":   page,
		"limit":  limit,
		"total":  total,
		"items":  items,
	})
}

// AdminListItems handles GET /api/v1/superadmin/downloads (all items).
func (h *Handler) AdminListItems(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := ItemFilters{
		Category: c.Query("category"),
	}

	items, total, err := h.service.GetItems(filters, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch download items")
		return
	}

	page, limit = normalizePageLimit(page, limit)
	response.Success(c, http.StatusOK, "Download items fetched successfully", gin.H{
		"page":   page,
		"limit":  limit,
		"total":  total,
		"items":  items,
	})
}

func (h *Handler) GetItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}
	item, err := h.service.GetItem(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Item not found")
		return
	}
	response.Success(c, http.StatusOK, "Item fetched successfully", item)
}

// CreateItem handles POST /api/v1/superadmin/downloads (multipart form with a
// required file).
func (h *Handler) CreateItem(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "File is required")
		return
	}

	var req CreateDownloadItemInput
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	fileURL, err := utils.SaveUploadedDocument(fileHeader, "downloads")
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// FileURL is "/uploads/downloads/<name>"; FilePath is the bare object key
	// used for streaming, derived from the returned public URL.
	objectPath := fileURL
	if len(objectPath) > len("/uploads/") {
		objectPath = objectPath[len("/uploads/"):]
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	item := &DownloadItem{
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		FileURL:     fileURL,
		FilePath:    objectPath,
		FileName:    fileHeader.Filename,
		FileSize:    fileHeader.Size,
		MimeType:    contentType,
		PublishedAt: req.PublishedAt,
		IsPublished: req.IsPublished,
	}
	if err := h.service.CreateItem(item); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create download item")
		return
	}

	response.Success(c, http.StatusCreated, "Download item created", item)
}

// UpdateItem handles PUT /api/v1/superadmin/downloads/:id — metadata update via
// JSON, or multipart with an optional replacement file.
func (h *Handler) UpdateItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}

	item, err := h.service.GetItem(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Item not found")
		return
	}

	// Optional new file upload (multipart). Detect by presence of "file".
	if fileHeader, err := c.FormFile("file"); err == nil {
		fileURL, err := utils.SaveUploadedDocument(fileHeader, "downloads")
		if err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		objectPath := fileURL
		if len(objectPath) > len("/uploads/") {
			objectPath = objectPath[len("/uploads/"):]
		}

		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		item.FileURL = fileURL
		item.FilePath = objectPath
		item.FileName = fileHeader.Filename
		item.FileSize = fileHeader.Size
		item.MimeType = contentType
	}

	var req UpdateDownloadItemInput
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Title != nil {
		item.Title = *req.Title
	}
	if req.Description != nil {
		item.Description = *req.Description
	}
	if req.Category != nil {
		item.Category = *req.Category
	}
	if req.IsPublished != nil {
		item.IsPublished = *req.IsPublished
	}
	if req.PublishedAt != nil {
		item.PublishedAt = req.PublishedAt
	}

	if err := h.service.UpdateItemModel(item); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update download item")
		return
	}

	response.Success(c, http.StatusOK, "Download item updated", item)
}

func (h *Handler) DeleteItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}
	if err := h.service.DeleteItem(uint(id)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete download item")
		return
	}
	response.Success(c, http.StatusOK, "Download item deleted", nil)
}

// DownloadItem handles GET /api/v1/downloads/:id/download — streams the file
// from object storage and increments the download counter (best-effort).
func (h *Handler) DownloadItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid item ID")
		return
	}

	item, err := h.service.GetItem(uint(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, "Item not found")
		return
	}

	// Download counting is best-effort and happens before streaming.
	h.service.IncrementDownloads(uint(id))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	reader, info, err := storage.GetWithContext(ctx, item.FilePath)
	if err != nil {
		response.Error(c, http.StatusNotFound, "File not found")
		return
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	contentType := info.ContentType
	if contentType == "" {
		contentType = item.MimeType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	filename := item.FileName
	if filename == "" {
		filename = filepath.Base(item.FilePath)
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if item.FileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(item.FileSize, 10))
	}
	c.DataFromReader(http.StatusOK, item.FileSize, contentType, reader, nil)
}

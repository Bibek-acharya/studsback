package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"studsphere/backend/internal/messaging/application"
)

type UploadHandler struct {
	uploadService application.UploadService
}

func NewUploadHandler(s application.UploadService) *UploadHandler {
	return &UploadHandler{uploadService: s}
}

func (h *UploadHandler) Upload(c *gin.Context) {
	userID, _ := c.Get("userID")
	userType, _ := c.Get("userType")

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file provided"})
		return
	}

	upload, err := h.uploadService.Upload(file, userType.(string), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"upload_id":     upload.ID,
		"file_name":     upload.FileName,
		"file_size":     upload.FileSize,
		"file_type":     upload.FileType,
		"storage_key":   upload.StorageKey,
		"thumbnail_key": upload.ThumbnailKey,
	})
}

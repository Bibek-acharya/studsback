package application

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"studsphere/backend/internal/messaging/domain"
	"studsphere/backend/internal/messaging/repository"
	"studsphere/backend/internal/shared/storage"
)

var allowedTypes = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".csv": true, ".txt": true,
	".zip": true, ".rar": true, ".7z": true,
}

const maxFileSize = 10 * 1024 * 1024 // 10MB

type UploadService interface {
	Upload(file *multipart.FileHeader, uploaderType string, uploaderID uint) (*domain.PendingUpload, error)
	GetPendingUpload(id uint) (*domain.PendingUpload, error)
	CleanupExpired() (int64, error)
}

type uploadService struct {
	attachmentRepo repository.AttachmentRepository
}

func NewUploadService(ar repository.AttachmentRepository) UploadService {
	return &uploadService{attachmentRepo: ar}
}

func (s *uploadService) Upload(file *multipart.FileHeader, uploaderType string, uploaderID uint) (*domain.PendingUpload, error) {
	if file.Size > maxFileSize {
		return nil, fmt.Errorf("file size exceeds limit of 10MB")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedTypes[ext] {
		return nil, fmt.Errorf("file type %s not allowed", ext)
	}

	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	id := uuid.New().String()
	storageKey := fmt.Sprintf("messages/%s%s", id, ext)

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := storage.Upload(storageKey, src, file.Size, contentType); err != nil {
		return nil, fmt.Errorf("failed to upload to storage: %w", err)
	}

	upload := &domain.PendingUpload{
		UploaderType: uploaderType,
		UploaderID:   uploaderID,
		FileName:     file.Filename,
		FileSize:     file.Size,
		FileType:     contentType,
		StorageKey:   storageKey,
		ThumbnailKey: "",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}

	if err := s.attachmentRepo.CreatePendingUpload(upload); err != nil {
		return nil, fmt.Errorf("failed to create pending upload: %w", err)
	}

	return upload, nil
}

func (s *uploadService) GetPendingUpload(id uint) (*domain.PendingUpload, error) {
	return s.attachmentRepo.GetPendingUpload(id)
}

func (s *uploadService) CleanupExpired() (int64, error) {
	return s.attachmentRepo.CleanupExpired()
}

func isImage(ext string) bool {
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"
}

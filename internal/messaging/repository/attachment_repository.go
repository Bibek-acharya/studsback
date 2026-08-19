package repository

import (
	"studsphere/backend/internal/messaging/domain"

	"gorm.io/gorm"
)

type AttachmentRepository interface {
	CreatePendingUpload(upload *domain.PendingUpload) error
	GetPendingUpload(id uint) (*domain.PendingUpload, error)
	LinkToMessage(uploadID uint, messageID uint) error
	GetByMessageID(messageID uint) ([]domain.Attachment, error)
	CleanupExpired() (int64, error)
}

type attachmentRepo struct {
	db *gorm.DB
}

func NewAttachmentRepository(db *gorm.DB) AttachmentRepository {
	return &attachmentRepo{db: db}
}

func (r *attachmentRepo) CreatePendingUpload(upload *domain.PendingUpload) error {
	return r.db.Create(upload).Error
}

func (r *attachmentRepo) GetPendingUpload(id uint) (*domain.PendingUpload, error) {
	var upload domain.PendingUpload
	err := r.db.First(&upload, id).Error
	if err != nil {
		return nil, err
	}
	return &upload, nil
}

func (r *attachmentRepo) LinkToMessage(uploadID uint, messageID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var upload domain.PendingUpload
		if err := tx.First(&upload, uploadID).Error; err != nil {
			return err
		}

		attachment := domain.Attachment{
			MessageID:    messageID,
			UploaderType: upload.UploaderType,
			UploaderID:   upload.UploaderID,
			FileName:     upload.FileName,
			FileSize:     upload.FileSize,
			FileType:     upload.FileType,
			StorageKey:   upload.StorageKey,
			ThumbnailKey: upload.ThumbnailKey,
		}
		if err := tx.Create(&attachment).Error; err != nil {
			return err
		}

		return tx.Model(&upload).Update("message_id", messageID).Error
	})
}

func (r *attachmentRepo) GetByMessageID(messageID uint) ([]domain.Attachment, error) {
	var attachments []domain.Attachment
	err := r.db.Where("message_id = ?", messageID).Find(&attachments).Error
	return attachments, err
}

func (r *attachmentRepo) CleanupExpired() (int64, error) {
	result := r.db.Where("message_id IS NULL AND expires_at < NOW()").Delete(&domain.PendingUpload{})
	return result.RowsAffected, result.Error
}

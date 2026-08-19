package repository

import (
	"studsphere/backend/internal/messaging/domain"

	"gorm.io/gorm"
)

type ConversationRepository interface {
	Create(conversation *domain.Conversation, participants []domain.Participant) error
	GetByID(id uint) (*domain.Conversation, error)
	GetByStudentAndInstitution(studentID, institutionID uint) (*domain.Conversation, error)
	ListByStudent(studentID uint, limit, offset int) ([]domain.Conversation, error)
	ListByInstitution(institutionID uint, limit, offset int) ([]domain.Conversation, error)
	UpdateLastMessage(id uint, messageID uint, preview string) error
}

type conversationRepo struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &conversationRepo{db: db}
}

func (r *conversationRepo) Create(conversation *domain.Conversation, participants []domain.Participant) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(conversation).Error; err != nil {
			return err
		}
		for i := range participants {
			participants[i].ConversationID = conversation.ID
		}
		return tx.Create(&participants).Error
	})
}

func (r *conversationRepo) GetByID(id uint) (*domain.Conversation, error) {
	var conversation domain.Conversation
	err := r.db.First(&conversation, id).Error
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *conversationRepo) GetByStudentAndInstitution(studentID, institutionID uint) (*domain.Conversation, error) {
	var conversation domain.Conversation
	err := r.db.Where("student_id = ? AND institution_id = ?", studentID, institutionID).First(&conversation).Error
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *conversationRepo) ListByStudent(studentID uint, limit, offset int) ([]domain.Conversation, error) {
	var conversations []domain.Conversation
	err := r.db.Where("student_id = ?", studentID).
		Order("last_message_at DESC NULLS LAST").
		Limit(limit).Offset(offset).
		Find(&conversations).Error
	return conversations, err
}

func (r *conversationRepo) ListByInstitution(institutionID uint, limit, offset int) ([]domain.Conversation, error) {
	var conversations []domain.Conversation
	err := r.db.Where("institution_id = ?", institutionID).
		Order("last_message_at DESC NULLS LAST").
		Limit(limit).Offset(offset).
		Find(&conversations).Error
	return conversations, err
}

func (r *conversationRepo) UpdateLastMessage(id uint, messageID uint, preview string) error {
	return r.db.Model(&domain.Conversation{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_message_id":      messageID,
			"last_message_at":      gorm.Expr("NOW()"),
			"last_message_preview": preview,
		}).Error
}

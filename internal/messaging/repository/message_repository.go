package repository

import (
	"studsphere/backend/internal/messaging/domain"

	"gorm.io/gorm"
)

type MessageRepository interface {
	Create(message *domain.Message) error
	GetByID(id uint) (*domain.Message, error)
	GetByConversationID(conversationID uint, limit, offset int) ([]domain.Message, error)
	UpdateContent(id uint, content string) error
	SoftDelete(id uint) error
	MarkAsRead(conversationID uint, readerType string, lastMessageID uint) (int64, error)
}

type messageRepo struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepo{db: db}
}

func (r *messageRepo) Create(message *domain.Message) error {
	return r.db.Create(message).Error
}

func (r *messageRepo) GetByID(id uint) (*domain.Message, error) {
	var message domain.Message
	err := r.db.First(&message, id).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *messageRepo) GetByConversationID(conversationID uint, limit, offset int) ([]domain.Message, error) {
	var messages []domain.Message
	err := r.db.Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Limit(limit).Offset(offset).
		Find(&messages).Error
	return messages, err
}

func (r *messageRepo) UpdateContent(id uint, content string) error {
	return r.db.Model(&domain.Message{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"content":   content,
			"edited_at": gorm.Expr("NOW()"),
		}).Error
}

func (r *messageRepo) SoftDelete(id uint) error {
	return r.db.Model(&domain.Message{}).Where("id = ?", id).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}

func (r *messageRepo) MarkAsRead(conversationID uint, readerType string, lastMessageID uint) (int64, error) {
	return 0, nil
}

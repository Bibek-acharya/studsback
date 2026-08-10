package repository

import (
	"studsphere/backend/internal/messaging/domain"

	"gorm.io/gorm"
)

type ParticipantRepository interface {
	Create(participant *domain.Participant) error
	GetByConversationAndUser(conversationID uint, participantType string, participantID uint) (*domain.Participant, error)
	GetByConversation(conversationID uint) ([]domain.Participant, error)
	GetByUser(participantType string, participantID uint) ([]domain.Participant, error)
	IncrementUnread(conversationID uint, excludeType string) error
	MarkAsRead(conversationID uint, participantType string, messageID uint) error
}

type participantRepo struct {
	db *gorm.DB
}

func NewParticipantRepository(db *gorm.DB) ParticipantRepository {
	return &participantRepo{db: db}
}

func (r *participantRepo) Create(participant *domain.Participant) error {
	return r.db.Create(participant).Error
}

func (r *participantRepo) GetByConversationAndUser(conversationID uint, participantType string, participantID uint) (*domain.Participant, error) {
	var participant domain.Participant
	err := r.db.Where("conversation_id = ? AND participant_type = ? AND participant_id = ?",
		conversationID, participantType, participantID).First(&participant).Error
	if err != nil {
		return nil, err
	}
	return &participant, nil
}

func (r *participantRepo) GetByConversation(conversationID uint) ([]domain.Participant, error) {
	var participants []domain.Participant
	err := r.db.Where("conversation_id = ?", conversationID).Find(&participants).Error
	return participants, err
}

func (r *participantRepo) GetByUser(participantType string, participantID uint) ([]domain.Participant, error) {
	var participants []domain.Participant
	err := r.db.Where("participant_type = ? AND participant_id = ?", participantType, participantID).
		Find(&participants).Error
	return participants, err
}

func (r *participantRepo) IncrementUnread(conversationID uint, excludeType string) error {
	return r.db.Model(&domain.Participant{}).
		Where("conversation_id = ? AND participant_type != ?", conversationID, excludeType).
		UpdateColumn("unread_count", gorm.Expr("unread_count + 1")).Error
}

func (r *participantRepo) MarkAsRead(conversationID uint, participantType string, messageID uint) error {
	return r.db.Model(&domain.Participant{}).
		Where("conversation_id = ? AND participant_type = ?", conversationID, participantType).
		Updates(map[string]interface{}{
			"last_read_message_id": messageID,
			"last_read_at":         gorm.Expr("NOW()"),
			"unread_count":         0,
		}).Error
}

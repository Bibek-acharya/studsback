package application

import (
	"encoding/json"
	"fmt"
	"studsphere/backend/internal/messaging/domain"
	"studsphere/backend/internal/messaging/repository"
)

type ReadService interface {
	MarkAsRead(conversationID uint, readerType string, readerID uint, lastMessageID uint) error
}

type readService struct {
	participantRepo repository.ParticipantRepository
	messageRepo     repository.MessageRepository
	outboxRepo      repository.OutboxRepository
}

func NewReadService(
	pr repository.ParticipantRepository,
	mr repository.MessageRepository,
	or repository.OutboxRepository,
) ReadService {
	return &readService{
		participantRepo: pr,
		messageRepo:     mr,
		outboxRepo:      or,
	}
}

func (s *readService) MarkAsRead(conversationID uint, readerType string, readerID uint, lastMessageID uint) error {
	_, err := s.messageRepo.MarkAsRead(conversationID, readerType, lastMessageID)
	if err != nil {
		return fmt.Errorf("failed to mark messages as read: %w", err)
	}

	if err := s.participantRepo.MarkAsRead(conversationID, readerType, lastMessageID); err != nil {
		return fmt.Errorf("failed to update participant read state: %w", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"conversation_id":  conversationID,
		"reader_type":      readerType,
		"reader_id":        readerID,
		"last_message_id":  lastMessageID,
	})
	event := &domain.OutboxEvent{
		AggregateType: "conversation",
		AggregateID:   conversationID,
		EventType:     "message.read",
		Payload:       string(payload),
	}
	if err := s.outboxRepo.Create(event); err != nil {
		fmt.Printf("failed to create outbox event: %v\n", err)
	}

	return nil
}

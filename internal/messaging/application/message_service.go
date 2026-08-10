package application

import (
	"encoding/json"
	"fmt"
	"time"
	"studsphere/backend/internal/messaging/domain"
	"studsphere/backend/internal/messaging/repository"
)

type MessageService interface {
	SendMessage(conversationID uint, senderType string, senderID uint, content, clientMessageID string, attachmentIDs []uint) (*domain.Message, error)
	EditMessage(messageID uint, senderType string, senderID uint, content string) error
	DeleteMessage(messageID uint, senderType string, senderID uint) error
	GetByConversationID(conversationID uint, limit, offset int) ([]domain.Message, error)
}

type messageService struct {
	messageRepo      repository.MessageRepository
	participantRepo  repository.ParticipantRepository
	conversationRepo repository.ConversationRepository
	attachmentRepo   repository.AttachmentRepository
	outboxRepo       repository.OutboxRepository
}

func NewMessageService(
	mr repository.MessageRepository,
	pr repository.ParticipantRepository,
	cr repository.ConversationRepository,
	ar repository.AttachmentRepository,
	or repository.OutboxRepository,
) MessageService {
	return &messageService{
		messageRepo:      mr,
		participantRepo:  pr,
		conversationRepo: cr,
		attachmentRepo:   ar,
		outboxRepo:       or,
	}
}

func (s *messageService) SendMessage(conversationID uint, senderType string, senderID uint, content, clientMessageID string, attachmentIDs []uint) (*domain.Message, error) {
	message := &domain.Message{
		ConversationID:  conversationID,
		SenderType:      senderType,
		SenderID:        senderID,
		ClientMessageID: clientMessageID,
		Content:         content,
	}

	if err := s.messageRepo.Create(message); err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	preview := content
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}
	if err := s.conversationRepo.UpdateLastMessage(conversationID, message.ID, preview); err != nil {
		return nil, fmt.Errorf("failed to update conversation: %w", err)
	}

	if err := s.participantRepo.IncrementUnread(conversationID, senderType); err != nil {
		return nil, fmt.Errorf("failed to increment unread: %w", err)
	}

	for _, uploadID := range attachmentIDs {
		if err := s.attachmentRepo.LinkToMessage(uploadID, message.ID); err != nil {
			fmt.Printf("failed to link attachment %d: %v\n", uploadID, err)
		}
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"conversation_id": conversationID,
		"message":         message,
	})
	event := &domain.OutboxEvent{
		AggregateType: "message",
		AggregateID:   message.ID,
		EventType:     "message.created",
		Payload:       string(payload),
	}
	if err := s.outboxRepo.Create(event); err != nil {
		fmt.Printf("failed to create outbox event: %v\n", err)
	}

	return message, nil
}

func (s *messageService) EditMessage(messageID uint, senderType string, senderID uint, content string) error {
	message, err := s.messageRepo.GetByID(messageID)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}

	if message.SenderType != senderType || message.SenderID != senderID {
		return fmt.Errorf("not authorized to edit this message")
	}

	if time.Since(message.CreatedAt) > 15*time.Minute {
		return fmt.Errorf("edit window expired")
	}

	if err := s.messageRepo.UpdateContent(messageID, content); err != nil {
		return fmt.Errorf("failed to edit message: %w", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"conversation_id": message.ConversationID,
		"message_id":      messageID,
		"content":         content,
		"edited_at":       time.Now(),
	})
	event := &domain.OutboxEvent{
		AggregateType: "message",
		AggregateID:   messageID,
		EventType:     "message.edited",
		Payload:       string(payload),
	}
	if err := s.outboxRepo.Create(event); err != nil {
		fmt.Printf("failed to create outbox event: %v\n", err)
	}

	return nil
}

func (s *messageService) DeleteMessage(messageID uint, senderType string, senderID uint) error {
	message, err := s.messageRepo.GetByID(messageID)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}

	if message.SenderType != senderType || message.SenderID != senderID {
		return fmt.Errorf("not authorized to delete this message")
	}

	if err := s.messageRepo.SoftDelete(messageID); err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"conversation_id": message.ConversationID,
		"message_id":      messageID,
	})
	event := &domain.OutboxEvent{
		AggregateType: "message",
		AggregateID:   messageID,
		EventType:     "message.deleted",
		Payload:       string(payload),
	}
	if err := s.outboxRepo.Create(event); err != nil {
		fmt.Printf("failed to create outbox event: %v\n", err)
	}

	return nil
}

func (s *messageService) GetByConversationID(conversationID uint, limit, offset int) ([]domain.Message, error) {
	return s.messageRepo.GetByConversationID(conversationID, limit, offset)
}

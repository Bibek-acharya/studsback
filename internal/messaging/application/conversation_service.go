package application

import (
	"fmt"
	"studsphere/backend/internal/messaging/domain"
	"studsphere/backend/internal/messaging/repository"
)

type ConversationService interface {
	Create(studentID, institutionID uint, content, subject, clientMessageID string, attachmentIDs []uint) (*domain.Conversation, *domain.Message, error)
	GetByID(id uint) (*domain.Conversation, error)
	ListByStudent(studentID uint, limit, offset int) ([]domain.Conversation, error)
	ListByInstitution(institutionID uint, limit, offset int) ([]domain.Conversation, error)
}

type conversationService struct {
	conversationRepo repository.ConversationRepository
	participantRepo  repository.ParticipantRepository
	messageRepo      repository.MessageRepository
	attachmentRepo   repository.AttachmentRepository
}

func NewConversationService(
	cr repository.ConversationRepository,
	pr repository.ParticipantRepository,
	mr repository.MessageRepository,
	ar repository.AttachmentRepository,
) ConversationService {
	return &conversationService{
		conversationRepo: cr,
		participantRepo:  pr,
		messageRepo:      mr,
		attachmentRepo:   ar,
	}
}

func (s *conversationService) Create(studentID, institutionID uint, content, subject, clientMessageID string, attachmentIDs []uint) (*domain.Conversation, *domain.Message, error) {
	existing, _ := s.conversationRepo.GetByStudentAndInstitution(studentID, institutionID)
	if existing != nil {
		return nil, nil, fmt.Errorf("conversation already exists between student %d and institution %d", studentID, institutionID)
	}

	conversation := &domain.Conversation{
		StudentID:     studentID,
		InstitutionID: institutionID,
	}

	participants := []domain.Participant{
		{ParticipantType: "student", ParticipantID: studentID, UnreadCount: 0},
		{ParticipantType: "institution", ParticipantID: institutionID, UnreadCount: 1},
	}

	if err := s.conversationRepo.Create(conversation, participants); err != nil {
		return nil, nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	preview := content
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}

	message := &domain.Message{
		ConversationID:  conversation.ID,
		SenderType:      "student",
		SenderID:        studentID,
		ClientMessageID: clientMessageID,
		Content:         content,
	}

	if err := s.messageRepo.Create(message); err != nil {
		return nil, nil, fmt.Errorf("failed to create message: %w", err)
	}

	if err := s.conversationRepo.UpdateLastMessage(conversation.ID, message.ID, preview); err != nil {
		return nil, nil, fmt.Errorf("failed to update conversation: %w", err)
	}

	for _, uploadID := range attachmentIDs {
		if err := s.attachmentRepo.LinkToMessage(uploadID, message.ID); err != nil {
			fmt.Printf("failed to link attachment %d: %v\n", uploadID, err)
		}
	}

	return conversation, message, nil
}

func (s *conversationService) GetByID(id uint) (*domain.Conversation, error) {
	return s.conversationRepo.GetByID(id)
}

func (s *conversationService) ListByStudent(studentID uint, limit, offset int) ([]domain.Conversation, error) {
	return s.conversationRepo.ListByStudent(studentID, limit, offset)
}

func (s *conversationService) ListByInstitution(institutionID uint, limit, offset int) ([]domain.Conversation, error) {
	return s.conversationRepo.ListByInstitution(institutionID, limit, offset)
}

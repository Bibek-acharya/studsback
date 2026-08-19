package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"studsphere/backend/internal/messaging/repository"
)

type EventPublisher interface {
	Start()
	Stop()
}

type eventPublisher struct {
	nats       *nats.Conn
	outboxRepo repository.OutboxRepository
	stopCh     chan struct{}
}

func NewEventPublisher(nats *nats.Conn, or repository.OutboxRepository) EventPublisher {
	return &eventPublisher{
		nats:       nats,
		outboxRepo: or,
		stopCh:     make(chan struct{}),
	}
}

func (p *eventPublisher) Start() {
	go p.pollLoop()
}

func (p *eventPublisher) Stop() {
	close(p.stopCh)
}

func (p *eventPublisher) pollLoop() {
	for {
		select {
		case <-p.stopCh:
			return
		default:
			p.processEvents()
		}
	}
}

func (p *eventPublisher) processEvents() {
	events, err := p.outboxRepo.GetUnpublished(10)
	if err != nil {
		return
	}

	for _, event := range events {
		subject := p.eventTypeToSubject(event.EventType)
		if subject == "" {
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			p.outboxRepo.IncrementRetry(event.ID)
			continue
		}

		envelope := Event{
			Version: 1,
			Type:    event.EventType,
			Data:    payload,
		}

		data, _ := json.Marshal(envelope)
		if err := p.nats.Publish(subject, data); err != nil {
			p.outboxRepo.IncrementRetry(event.ID)
			continue
		}

		p.outboxRepo.MarkPublished(event.ID)
	}
}

func (p *eventPublisher) eventTypeToSubject(eventType string) string {
	switch eventType {
	case "message.created":
		return SubjectMessageCreated
	case "message.read":
		return SubjectMessageRead
	case "message.delivered":
		return SubjectMessageDelivered
	case "message.edited":
		return SubjectMessageEdited
	case "message.deleted":
		return SubjectMessageDeleted
	case "typing.start":
		return SubjectTypingStart
	case "typing.stop":
		return SubjectTypingStop
	case "presence.changed":
		return SubjectPresenceChanged
	case "conversation.updated":
		return SubjectConversationUpdated
	default:
		return ""
	}
}

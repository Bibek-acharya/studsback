package events

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

type EventHandler func(event Event)

type EventSubscriber interface {
	Subscribe(subject string, handler EventHandler) error
	Stop()
}

type eventSubscriber struct {
	nats *nats.Conn
	subs []*nats.Subscription
}

func NewEventSubscriber(nats *nats.Conn) EventSubscriber {
	return &eventSubscriber{nats: nats}
}

func (s *eventSubscriber) Subscribe(subject string, handler EventHandler) error {
	sub, err := s.nats.Subscribe(subject, func(msg *nats.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			fmt.Printf("failed to unmarshal event: %v\n", err)
			return
		}
		handler(event)
	})
	if err != nil {
		return err
	}
	s.subs = append(s.subs, sub)
	return nil
}

func (s *eventSubscriber) Stop() {
	for _, sub := range s.subs {
		sub.Unsubscribe()
	}
}

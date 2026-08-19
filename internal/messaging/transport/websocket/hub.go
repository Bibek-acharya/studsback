package websocket

import (
	"encoding/json"
	"fmt"
	"sync"

	"studsphere/backend/internal/messaging/events"
	"studsphere/backend/internal/messaging/presence"
	"studsphere/backend/internal/messaging/repository"
)

type Hub struct {
	connections     map[string]map[uint]*Connection
	mu              sync.RWMutex
	register        chan *Connection
	unregister      chan *Connection
	broadcast       chan []byte
	subscriber      events.EventSubscriber
	presence        presence.PresenceService
	participantRepo repository.ParticipantRepository
}

func NewHub(subscriber events.EventSubscriber, ps presence.PresenceService, pr repository.ParticipantRepository) *Hub {
	return &Hub{
		connections:     make(map[string]map[uint]*Connection),
		register:        make(chan *Connection),
		unregister:      make(chan *Connection),
		broadcast:       make(chan []byte),
		subscriber:      subscriber,
		presence:        ps,
		participantRepo: pr,
	}
}

func (h *Hub) SetSubscriber(subscriber events.EventSubscriber) {
	h.subscriber = subscriber
	subscriber.Subscribe(events.SubjectMessageCreated, h.onMessageCreated)
	subscriber.Subscribe(events.SubjectMessageRead, h.onMessageRead)
	subscriber.Subscribe(events.SubjectMessageEdited, h.onMessageEdited)
	subscriber.Subscribe(events.SubjectMessageDeleted, h.onMessageDeleted)
	subscriber.Subscribe(events.SubjectTypingStart, h.onTypingStart)
	subscriber.Subscribe(events.SubjectTypingStop, h.onTypingStop)
	subscriber.Subscribe(events.SubjectPresenceChanged, h.onPresenceChanged)
}

func (h *Hub) Run() {
	if h.subscriber != nil {
		h.subscriber.Subscribe(events.SubjectMessageCreated, h.onMessageCreated)
		h.subscriber.Subscribe(events.SubjectMessageRead, h.onMessageRead)
		h.subscriber.Subscribe(events.SubjectMessageEdited, h.onMessageEdited)
		h.subscriber.Subscribe(events.SubjectMessageDeleted, h.onMessageDeleted)
		h.subscriber.Subscribe(events.SubjectTypingStart, h.onTypingStart)
		h.subscriber.Subscribe(events.SubjectTypingStop, h.onTypingStop)
		h.subscriber.Subscribe(events.SubjectPresenceChanged, h.onPresenceChanged)
	}

	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			if h.connections[conn.userType] == nil {
				h.connections[conn.userType] = make(map[uint]*Connection)
			}
			h.connections[conn.userType][conn.userID] = conn
			h.mu.Unlock()
			h.presence.SetOnline(conn.userType, conn.userID)

		case conn := <-h.unregister:
			h.mu.Lock()
			if conns, ok := h.connections[conn.userType]; ok {
				if c, ok := conns[conn.userID]; ok && c == conn {
					delete(conns, conn.userID)
					conn.Close()
				}
			}
			h.mu.Unlock()
			h.presence.SetOffline(conn.userType, conn.userID)
		}
	}
}

func (h *Hub) SendToUser(userType string, userID uint, msg WSMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if conns, ok := h.connections[userType]; ok {
		if conn, ok := conns[userID]; ok {
			conn.WriteMessage(msg)
		}
	}
}

func (h *Hub) BroadcastToConversation(conversationID uint, eventType string, data map[string]interface{}) {
	wsMsg := WSMessage{
		Version: 1,
		Type:    eventType,
		Data:    data,
	}

	participants, err := h.participantRepo.GetByConversation(conversationID)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, p := range participants {
		if conns, ok := h.connections[p.ParticipantType]; ok {
			if conn, ok := conns[p.ParticipantID]; ok {
				conn.WriteMessage(wsMsg)
			}
		}
	}
}

func (h *Hub) HandleMessage(conn *Connection, msg WSMessage) {
	switch msg.Type {
	case "message.read":
		var data MessageReadData
		if err := mapToStruct(msg.Data, &data); err != nil {
			return
		}
		h.BroadcastToConversation(data.ConversationID, "message.read", map[string]interface{}{
			"conversation_id": float64(data.ConversationID),
			"reader_type":     conn.userType,
			"reader_id":       float64(conn.userID),
			"last_message_id": float64(data.LastMessageID),
		})

	case "typing.start":
		var data TypingData
		if err := mapToStruct(msg.Data, &data); err != nil {
			return
		}
		h.BroadcastToConversation(data.ConversationID, "typing.start", map[string]interface{}{
			"conversation_id": float64(data.ConversationID),
			"user_type":       conn.userType,
			"user_id":         float64(conn.userID),
		})

	case "typing.stop":
		var data TypingData
		if err := mapToStruct(msg.Data, &data); err != nil {
			return
		}
		h.BroadcastToConversation(data.ConversationID, "typing.stop", map[string]interface{}{
			"conversation_id": float64(data.ConversationID),
			"user_type":       conn.userType,
			"user_id":         float64(conn.userID),
		})

	case "ping":
		conn.WriteMessage(WSMessage{Version: 1, Type: "pong"})
	}
}

func (h *Hub) onMessageCreated(event events.Event) {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return
	}
	convID, ok := data["conversation_id"].(float64)
	if !ok {
		return
	}
	h.BroadcastToConversation(uint(convID), event.Type, data)
}

func (h *Hub) onMessageRead(event events.Event) {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return
	}
	convID, ok := data["conversation_id"].(float64)
	if !ok {
		return
	}
	h.BroadcastToConversation(uint(convID), event.Type, data)
}

func (h *Hub) onMessageEdited(event events.Event) {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return
	}
	convID, ok := data["conversation_id"].(float64)
	if !ok {
		return
	}
	h.BroadcastToConversation(uint(convID), event.Type, data)
}

func (h *Hub) onMessageDeleted(event events.Event) {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return
	}
	convID, ok := data["conversation_id"].(float64)
	if !ok {
		return
	}
	h.BroadcastToConversation(uint(convID), event.Type, data)
}

func (h *Hub) onTypingStart(event events.Event) {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return
	}
	convID, ok := data["conversation_id"].(float64)
	if !ok {
		return
	}
	h.BroadcastToConversation(uint(convID), event.Type, data)
}

func (h *Hub) onTypingStop(event events.Event) {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return
	}
	convID, ok := data["conversation_id"].(float64)
	if !ok {
		return
	}
	h.BroadcastToConversation(uint(convID), event.Type, data)
}

func (h *Hub) onPresenceChanged(event events.Event) {
	fmt.Printf("presence.changed event: %v\n", event.Data)
}

func mapToStruct(data interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

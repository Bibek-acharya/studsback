package websocket

import (
	"encoding/json"
	"fmt"
	"sync"

	"studsphere/backend/internal/messaging/events"
	"studsphere/backend/internal/messaging/presence"
)

type Hub struct {
	connections map[string]map[uint]*Connection
	mu          sync.RWMutex
	register    chan *Connection
	unregister  chan *Connection
	broadcast   chan []byte
	subscriber  events.EventSubscriber
	presence    presence.PresenceService
}

func NewHub(subscriber events.EventSubscriber, ps presence.PresenceService) *Hub {
	return &Hub{
		connections: make(map[string]map[uint]*Connection),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		broadcast:   make(chan []byte),
		subscriber:  subscriber,
		presence:    ps,
	}
}

func (h *Hub) Run() {
	h.subscriber.Subscribe(events.SubjectMessageCreated, h.onMessageCreated)
	h.subscriber.Subscribe(events.SubjectMessageRead, h.onMessageRead)
	h.subscriber.Subscribe(events.SubjectMessageEdited, h.onMessageEdited)
	h.subscriber.Subscribe(events.SubjectMessageDeleted, h.onMessageDeleted)
	h.subscriber.Subscribe(events.SubjectTypingStart, h.onTypingStart)
	h.subscriber.Subscribe(events.SubjectTypingStop, h.onTypingStop)
	h.subscriber.Subscribe(events.SubjectPresenceChanged, h.onPresenceChanged)

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

func (h *Hub) HandleMessage(conn *Connection, msg WSMessage) {
	switch msg.Type {
	case "message.read":
		var data MessageReadData
		if err := mapToStruct(msg.Data, &data); err != nil {
			return
		}
		fmt.Printf("read receipt from %s:%d for conversation %d\n", conn.userType, conn.userID, data.ConversationID)

	case "typing.start":
		var data TypingData
		if err := mapToStruct(msg.Data, &data); err != nil {
			return
		}
		fmt.Printf("typing start from %s:%d in conversation %d\n", conn.userType, conn.userID, data.ConversationID)
		h.broadcastToConversation(events.Event{Type: "typing.start", Data: map[string]interface{}{
			"conversation_id": float64(data.ConversationID),
			"user_type":       conn.userType,
			"user_id":         float64(conn.userID),
		}})

	case "typing.stop":
		var data TypingData
		if err := mapToStruct(msg.Data, &data); err != nil {
			return
		}
		fmt.Printf("typing stop from %s:%d in conversation %d\n", conn.userType, conn.userID, data.ConversationID)
		h.broadcastToConversation(events.Event{Type: "typing.stop", Data: map[string]interface{}{
			"conversation_id": float64(data.ConversationID),
			"user_type":       conn.userType,
			"user_id":         float64(conn.userID),
		}})

	case "ping":
		conn.WriteMessage(WSMessage{Version: 1, Type: "pong"})
	}
}

func (h *Hub) onMessageCreated(event events.Event) {
	fmt.Printf("message.created event: %v\n", event.Data)
	h.broadcastToConversation(event)
}

func (h *Hub) onMessageRead(event events.Event) {
	fmt.Printf("message.read event: %v\n", event.Data)
	h.broadcastToConversation(event)
}

func (h *Hub) onMessageEdited(event events.Event) {
	fmt.Printf("message.edited event: %v\n", event.Data)
	h.broadcastToConversation(event)
}

func (h *Hub) onMessageDeleted(event events.Event) {
	fmt.Printf("message.deleted event: %v\n", event.Data)
	h.broadcastToConversation(event)
}

func (h *Hub) onTypingStart(event events.Event) {
	fmt.Printf("typing.start event: %v\n", event.Data)
	h.broadcastToConversation(event)
}

func (h *Hub) onTypingStop(event events.Event) {
	fmt.Printf("typing.stop event: %v\n", event.Data)
	h.broadcastToConversation(event)
}

func (h *Hub) onPresenceChanged(event events.Event) {
	fmt.Printf("presence.changed event: %v\n", event.Data)
}

func (h *Hub) broadcastToConversation(event events.Event) {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return
	}

	convID, ok := data["conversation_id"].(float64)
	if !ok {
		return
	}

	wsMsg := WSMessage{
		Version: 1,
		Type:    event.Type,
		Data:    event.Data,
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conns := range h.connections {
		for _, conn := range conns {
			_ = uint(convID)
			conn.WriteMessage(wsMsg)
		}
	}
}

func mapToStruct(data interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

package websocket

type WSMessage struct {
	Version int         `json:"version"`
	Type    string      `json:"type"`
	Data    interface{} `json:"data,omitempty"`
}

type MessageReadData struct {
	ConversationID uint `json:"conversation_id"`
	LastMessageID  uint `json:"last_message_id"`
}

type TypingData struct {
	ConversationID uint `json:"conversation_id"`
}

type ErrorData struct {
	Message string `json:"message"`
}

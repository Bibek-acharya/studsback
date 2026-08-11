package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"studsphere/backend/internal/messaging/application"
	ws "studsphere/backend/internal/messaging/transport/websocket"
)

type MessageHandler struct {
	messageService application.MessageService
	readService    application.ReadService
	hub            *ws.Hub
}

func NewMessageHandler(ms application.MessageService, rs application.ReadService, hub *ws.Hub) *MessageHandler {
	return &MessageHandler{messageService: ms, readService: rs, hub: hub}
}

func (h *MessageHandler) List(c *gin.Context) {
	conversationID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	messages, err := h.messageService.GetByConversationID(uint(conversationID), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func (h *MessageHandler) Send(c *gin.Context) {
	conversationID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	role := ""
	if userRole != nil {
		role = userRole.(string)
	}

	var req struct {
		Content         string `json:"content"`
		ClientMessageID string `json:"client_message_id" binding:"required"`
		AttachmentIDs   []uint `json:"attachment_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	message, err := h.messageService.SendMessage(
		uint(conversationID), role, userID.(uint),
		req.Content, req.ClientMessageID, req.AttachmentIDs,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.BroadcastToConversation(uint(conversationID), "message.created", map[string]interface{}{
		"conversation_id": float64(conversationID),
		"message":         message,
	})

	c.JSON(http.StatusCreated, message)
}

func (h *MessageHandler) Edit(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("msg_id"), 10, 64)
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	role := ""
	if userRole != nil {
		role = userRole.(string)
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.messageService.EditMessage(uint(messageID), role, userID.(uint), req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "message updated"})
}

func (h *MessageHandler) Delete(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("msg_id"), 10, 64)
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	role := ""
	if userRole != nil {
		role = userRole.(string)
	}

	if err := h.messageService.DeleteMessage(uint(messageID), role, userID.(uint)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "message deleted"})
}

func (h *MessageHandler) MarkRead(c *gin.Context) {
	conversationID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	role := ""
	if userRole != nil {
		role = userRole.(string)
	}

	var req struct {
		LastMessageID uint `json:"last_message_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.readService.MarkAsRead(uint(conversationID), role, userID.(uint), req.LastMessageID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.hub.BroadcastToConversation(uint(conversationID), "message.read", map[string]interface{}{
		"conversation_id": float64(conversationID),
		"reader_type":     role,
		"reader_id":       float64(userID.(uint)),
		"last_message_id": float64(req.LastMessageID),
	})

	c.JSON(http.StatusOK, gin.H{"message": "marked as read"})
}

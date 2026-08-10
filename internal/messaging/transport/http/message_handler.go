package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"studsphere/backend/internal/messaging/application"
)

type MessageHandler struct {
	messageService application.MessageService
	readService    application.ReadService
}

func NewMessageHandler(ms application.MessageService, rs application.ReadService) *MessageHandler {
	return &MessageHandler{messageService: ms, readService: rs}
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
	userID, _ := c.Get("userID")
	userType, _ := c.Get("userType")

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
		uint(conversationID), userType.(string), userID.(uint),
		req.Content, req.ClientMessageID, req.AttachmentIDs,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, message)
}

func (h *MessageHandler) Edit(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("msg_id"), 10, 64)
	userID, _ := c.Get("userID")
	userType, _ := c.Get("userType")

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.messageService.EditMessage(uint(messageID), userType.(string), userID.(uint), req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "message updated"})
}

func (h *MessageHandler) Delete(c *gin.Context) {
	messageID, _ := strconv.ParseUint(c.Param("msg_id"), 10, 64)
	userID, _ := c.Get("userID")
	userType, _ := c.Get("userType")

	if err := h.messageService.DeleteMessage(uint(messageID), userType.(string), userID.(uint)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "message deleted"})
}

func (h *MessageHandler) MarkRead(c *gin.Context) {
	conversationID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID, _ := c.Get("userID")
	userType, _ := c.Get("userType")

	var req struct {
		LastMessageID uint `json:"last_message_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.readService.MarkAsRead(uint(conversationID), userType.(string), userID.(uint), req.LastMessageID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "marked as read"})
}

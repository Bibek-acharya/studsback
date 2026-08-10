package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"studsphere/backend/internal/messaging/application"
	"studsphere/backend/internal/messaging/domain"
)

type ConversationHandler struct {
	service application.ConversationService
}

func NewConversationHandler(s application.ConversationService) *ConversationHandler {
	return &ConversationHandler{service: s}
}

func (h *ConversationHandler) List(c *gin.Context) {
	userID, _ := c.Get("userID")
	userType, _ := c.Get("userType")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var conversations []domain.Conversation
	var err error

	switch userType {
	case "student":
		conversations, err = h.service.ListByStudent(userID.(uint), limit, offset)
	case "institution":
		conversations, err = h.service.ListByInstitution(userID.(uint), limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

func (h *ConversationHandler) GetByID(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	conversation, err := h.service.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}

	c.JSON(http.StatusOK, conversation)
}

func (h *ConversationHandler) Create(c *gin.Context) {
	userID, _ := c.Get("userID")
	userType, _ := c.Get("userType")

	if userType != "student" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only students can create conversations"})
		return
	}

	var req struct {
		InstitutionID   uint   `json:"institution_id" binding:"required"`
		Content         string `json:"content" binding:"required"`
		Subject         string `json:"subject"`
		ClientMessageID string `json:"client_message_id" binding:"required"`
		AttachmentIDs   []uint `json:"attachment_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conversation, message, err := h.service.Create(
		userID.(uint), req.InstitutionID, req.Content, req.Subject, req.ClientMessageID, req.AttachmentIDs,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"conversation": conversation,
		"message":      message,
	})
}

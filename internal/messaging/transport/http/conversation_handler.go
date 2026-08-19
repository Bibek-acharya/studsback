package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"studsphere/backend/internal/messaging/application"
	"studsphere/backend/internal/messaging/domain"
)

type ConversationHandler struct {
	service application.ConversationService
	db      *gorm.DB
}

func NewConversationHandler(s application.ConversationService, db *gorm.DB) *ConversationHandler {
	return &ConversationHandler{service: s, db: db}
}

type ConversationResponse struct {
	domain.Conversation
	StudentName     string `json:"student_name"`
	InstitutionName string `json:"institution_name"`
}

func (h *ConversationHandler) List(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var conversations []domain.Conversation
	var err error

	role := ""
	if userRole != nil {
		role = userRole.(string)
	}

	switch role {
	case "student":
		conversations, err = h.service.ListByStudent(userID.(uint), limit, offset)
	case "institution":
		conversations, err = h.service.ListByInstitution(userID.(uint), limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []ConversationResponse
	for _, conv := range conversations {
		cr := ConversationResponse{Conversation: conv}

		var studentName string
		h.db.Table("users").Where("id = ?", conv.StudentID).Select("first_name || ' ' || last_name").Scan(&studentName)
		cr.StudentName = studentName

		var instName string
		h.db.Table("institution_users").Where("id = ?", conv.InstitutionID).Select("institution_name").Scan(&instName)
		cr.InstitutionName = instName

		result = append(result, cr)
	}

	c.JSON(http.StatusOK, gin.H{"conversations": result})
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
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	role := ""
	if userRole != nil {
		role = userRole.(string)
	}

	if role != "student" && role != "institution" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only students and institutions can create conversations"})
		return
	}

	var req struct {
		InstitutionID   uint   `json:"institution_id"`
		StudentID       uint   `json:"student_id"`
		Content         string `json:"content" binding:"required"`
		Subject         string `json:"subject"`
		ClientMessageID string `json:"client_message_id" binding:"required"`
		AttachmentIDs   []uint `json:"attachment_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	studentID := uint(0)
	institutionID := uint(0)
	senderType := ""

	if role == "student" {
		studentID = userID.(uint)
		institutionID = req.InstitutionID
		senderType = "student"
	} else {
		institutionID = userID.(uint)
		studentID = req.StudentID
		senderType = "institution"
	}

	if studentID == 0 || institutionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing student_id or institution_id"})
		return
	}

	conversation, message, err := h.service.Create(
		studentID, institutionID, senderType, req.Content, req.Subject, req.ClientMessageID, req.AttachmentIDs,
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

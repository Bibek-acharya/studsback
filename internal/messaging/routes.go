package messaging

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"studsphere/backend/internal/messaging/application"
	"studsphere/backend/internal/messaging/events"
	"studsphere/backend/internal/messaging/presence"
	"studsphere/backend/internal/messaging/repository"
	httpHandler "studsphere/backend/internal/messaging/transport/http"
	ws "studsphere/backend/internal/messaging/transport/websocket"
)

func SetupRoutes(router *gin.RouterGroup, db *gorm.DB, redis *redis.Client, nats *nats.Conn) {
	// Repositories
	conversationRepo := repository.NewConversationRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	participantRepo := repository.NewParticipantRepository(db)
	attachmentRepo := repository.NewAttachmentRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)

	// Services
	conversationService := application.NewConversationService(conversationRepo, participantRepo, messageRepo, attachmentRepo)
	messageService := application.NewMessageService(messageRepo, participantRepo, conversationRepo, attachmentRepo, outboxRepo)
	readService := application.NewReadService(participantRepo, messageRepo, outboxRepo)
	_ = application.NewTypingService(redis)
	uploadService := application.NewUploadService(attachmentRepo)

	// Presence
	presenceService := presence.NewPresenceService(redis)

	// Events (optional — only if NATS is available)
	var eventPublisher events.EventPublisher
	var eventSubscriber events.EventSubscriber
	var hub *ws.Hub

	if nats != nil {
		eventPublisher = events.NewEventPublisher(nats, outboxRepo)
		eventSubscriber = events.NewEventSubscriber(nats)
		hub = ws.NewHub(eventSubscriber, presenceService)
		go hub.Run()
		eventPublisher.Start()
	} else {
		log.Println("NATS not connected, real-time events disabled")
	}

	// HTTP Handlers
	conversationHandler := httpHandler.NewConversationHandler(conversationService)
	messageHandler := httpHandler.NewMessageHandler(messageService, readService)
	uploadHandler := httpHandler.NewUploadHandler(uploadService)

	// Routes
	messaging := router.Group("/conversations")
	{
		messaging.GET("", conversationHandler.List)
		messaging.GET("/:id", conversationHandler.GetByID)
		messaging.POST("", conversationHandler.Create)
		messaging.GET("/:id/messages", messageHandler.List)
		messaging.POST("/:id/messages", messageHandler.Send)
		messaging.PUT("/:id/messages/:msg_id", messageHandler.Edit)
		messaging.DELETE("/:id/messages/:msg_id", messageHandler.Delete)
		messaging.POST("/:id/read", messageHandler.MarkRead)
	}

	router.POST("/uploads", uploadHandler.Upload)

	// WebSocket (only if NATS is available)
	if hub != nil {
		wsHandler := ws.NewWSHandler(hub)
		router.GET("/ws", wsHandler.HandleWS)
	}
}

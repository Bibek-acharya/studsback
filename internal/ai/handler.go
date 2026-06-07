package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Chat handles POST /api/v1/ai/chat with a JSON body of { message, session_id, history? }.
// The response is sent as Server-Sent Events so the client can stream tokens
// as they arrive. Format:
//
//	data: {"token":"..."}\n\n
//	data: {"done":true}\n\n
func (h *Handler) Chat(c *gin.Context) {
	if !h.service.IsEnabled() {
		response.Error(c, http.StatusServiceUnavailable, "Sphere AI is not configured on the server")
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Message == "" {
		response.Error(c, http.StatusBadRequest, "message is required")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	if err := h.service.Chat(c.Request.Context(), c.Writer, req); err != nil {
		log.Printf("ai: chat error: %v", err)
		if !errors.Is(err, errClientClosed) {
			errData, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Fprintf(c.Writer, "data: %s\n\n", errData)
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

var errClientClosed = errors.New("client closed connection")

// ListModels returns the models exposed by the LLM server. Helpful for
// debugging the model name (e.g. confirming the exact Ollama tag like
// "llama3.1:8b") and for surfacing what's available to admins.
func (h *Handler) ListModels(c *gin.Context) {
	if !h.service.IsEnabled() {
		response.Error(c, http.StatusServiceUnavailable, "Sphere AI is not configured on the server")
		return
	}
	models, err := h.service.ListModels()
	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "ok", gin.H{
		"models":      models,
		"active_model": config.AppConfig.LLMModel,
		"base_url":    config.AppConfig.LLMBaseURL,
	})
}

package ai

import (
	"context"

	"studsphere/backend/internal/shared/config"
)

// Provider defines the interface for LLM streaming implementations.
type Provider interface {
	// StreamChat sends messages to the LLM and calls onToken for each streamed token.
	// Returns the complete response text.
	StreamChat(ctx context.Context, messages []Message, onToken func(string)) (string, error)
}

// NewProvider creates the configured OpenAI-compatible API provider.
func NewProvider() Provider {
	return &OpenAIProvider{baseURL: config.AppConfig.LLMBaseURL, model: config.AppConfig.LLMModel, apiKey: config.AppConfig.LLMAPIKey}
}

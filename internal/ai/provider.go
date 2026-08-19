package ai

import (
	"context"
	"log"
	"strings"

	"studsphere/backend/internal/shared/config"
)

// Provider defines the interface for LLM streaming implementations.
type Provider interface {
	// StreamChat sends messages to the LLM and calls onToken for each streamed token.
	// Returns the complete response text.
	StreamChat(ctx context.Context, messages []Message, onToken func(string)) (string, error)
}

// NewProvider creates a Provider based on the LLM_PROVIDER env var.
// Supported values: "gemini", "llama", "ollama" (default: "llama")
func NewProvider() Provider {
	provider := strings.ToLower(config.AppConfig.LLMProvider)
	switch provider {
	case "gemini":
		log.Printf("ai: using Gemini provider (model: %s)", config.AppConfig.GeminiModel)
		return &GeminiProvider{
			apiKey: config.AppConfig.GeminiAPIKey,
			model:  config.AppConfig.GeminiModel,
		}
	case "llama", "ollama", "":
		log.Printf("ai: using Ollama provider (model: %s, base: %s)", config.AppConfig.LLMModel, config.AppConfig.LLMBaseURL)
		return &OllamaProvider{
			baseURL: config.AppConfig.LLMBaseURL,
			model:   config.AppConfig.LLMModel,
			apiKey:  config.AppConfig.LLMAPIKey,
		}
	default:
		log.Fatalf("ai: unknown LLM_PROVIDER: %q — supported values: gemini, llama, ollama", provider)
		return nil
	}
}

package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider implements the OpenAI-compatible chat completions API.
// It works with OpenAI, OpenRouter, and other hosted API providers.
type OpenAIProvider struct {
	baseURL string
	model   string
	apiKey  string
}

type openAIStreamRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *OpenAIProvider) StreamChat(ctx context.Context, messages []Message, onToken func(string)) (string, error) {
	baseURL := strings.TrimRight(p.baseURL, "/")
	if baseURL == "" {
		return "", errors.New("LLM_BASE_URL is not set")
	}
	if p.model == "" {
		return "", errors.New("LLM_MODEL is not set")
	}
	if p.apiKey == "" {
		return "", errors.New("LLM_API_KEY or OPENROUTER_API_KEY is not set")
	}

	body, err := json.Marshal(openAIStreamRequest{Model: p.model, Messages: messages, Stream: true, Temperature: 0.1, MaxTokens: 1024})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if strings.Contains(baseURL, "openrouter.ai") {
		req.Header.Set("HTTP-Referer", "https://studsphere.com")
		req.Header.Set("X-Title", "StudSphere")
	}

	log.Printf("ai: calling OpenAI-compatible API at %s with model %s", baseURL, p.model)
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM API at %s is not responding: %w", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API returned %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if raw == "" {
			continue
		}
		if raw == "[DONE]" {
			break
		}
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return full.String(), fmt.Errorf("LLM error: %s", chunk.Error.Message)
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				onToken(choice.Delta.Content)
				full.WriteString(choice.Delta.Content)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("error reading LLM stream: %w", err)
	}
	return full.String(), nil
}

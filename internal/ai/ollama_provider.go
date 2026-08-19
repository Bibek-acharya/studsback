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

// OllamaProvider implements Provider using OpenAI-compatible /chat/completions endpoint.
type OllamaProvider struct {
	baseURL string
	model   string
	apiKey  string
}

type ollamaRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type ollamaChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (p *OllamaProvider) StreamChat(ctx context.Context, messages []Message, onToken func(string)) (string, error) {
	baseURL := strings.TrimRight(p.baseURL, "/")
	if baseURL == "" {
		return "", errors.New("LLM_BASE_URL is not set")
	}
	url := baseURL + "/chat/completions"

	body, err := json.Marshal(ollamaRequest{
		Model:       p.model,
		Messages:    messages,
		Stream:      true,
		Temperature: 0.1,
		MaxTokens:   1024,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	client := &http.Client{Timeout: 180 * time.Second}
	log.Printf("ai: calling LLM at %s with model %s", baseURL, p.model)
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("LLM at %s is not responding: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("LLM API returned %d: %s", resp.StatusCode, truncate(string(respBody), 300))
		return "", errors.New(errMsg)
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

		var chunk ollamaChunk
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return full.String(), fmt.Errorf("LLM error: %s", chunk.Error.Message)
		}
		for _, choice := range chunk.Choices {
			token := choice.Delta.Content
			if token == "" {
				continue
			}
			onToken(token)
			full.WriteString(token)
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("error reading LLM stream: %w", err)
	}
	return full.String(), nil
}

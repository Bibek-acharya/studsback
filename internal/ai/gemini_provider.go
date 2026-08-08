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

// GeminiProvider implements Provider using Google Gemini's streaming API.
type GeminiProvider struct {
	apiKey string
	model  string
}

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type geminiStreamResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (p *GeminiProvider) StreamChat(ctx context.Context, messages []Message, onToken func(string)) (string, error) {
	if p.apiKey == "" {
		return "", errors.New("GEMINI_API_KEY is not set")
	}

	// Build Gemini messages format (exclude system message, prepend as context)
	var contents []geminiContent
	var systemText string
	for _, msg := range messages {
		if msg.Role == "system" {
			systemText = msg.Content
			continue
		}
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, geminiContent{
			Parts: []geminiPart{{Text: msg.Content}},
			Role:  role,
		})
	}

	// Prepend system prompt as user context if present
	if systemText != "" {
		contents = append([]geminiContent{
			{Parts: []geminiPart{{Text: systemText}}, Role: "user"},
			{Parts: []geminiPart{{Text: "I will follow these instructions."}}, Role: "model"},
		}, contents...)
	}

	reqBody := geminiRequest{
		Contents: contents,
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     0.1,
			MaxOutputTokens: 1024,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s",
		p.model, p.apiKey,
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 180 * time.Second}
	log.Printf("ai: calling Gemini API with model %s", p.model)
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("Gemini API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gemini API returned %d: %s", resp.StatusCode, truncate(string(respBody), 300))
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

		var chunk geminiStreamResponse
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			continue
		}

		for _, candidate := range chunk.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text == "" {
					continue
				}
				onToken(part.Text)
				full.WriteString(part.Text)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("error reading Gemini stream: %w", err)
	}
	return full.String(), nil
}

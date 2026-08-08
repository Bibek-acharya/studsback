package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LLMClient provides text generation with Ollama (primary) and Gemini (fallback)
type LLMClient struct {
	ollamaBaseURL string
	ollamaModel   string
	geminiAPIKey  string
	geminiModel   string
	httpClient    *http.Client
}

// NewLLMClient creates a new LLM client from environment variables
func NewLLMClient() *LLMClient {
	return &LLMClient{
		ollamaBaseURL: getEnv("LLM_BASE_URL", "http://localhost:11434"),
		ollamaModel:   getEnv("LLM_MODEL", "llama3.1:8b"),
		geminiAPIKey:  os.Getenv("GEMINI_API_KEY"),
		geminiModel:   getEnv("GEMINI_MODEL", "gemini-2.0-flash-lite"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GenerateSummary generates a text summary using Ollama (primary) or Gemini (fallback)
func (c *LLMClient) GenerateSummary(ctx context.Context, prompt string) (string, error) {
	// Try Ollama first
	result, err := c.generateWithOllama(ctx, prompt)
	if err == nil && result != "" {
		return result, nil
	}

	// Fallback to Gemini if API key is available
	if c.geminiAPIKey != "" {
		result, err = c.generateWithGemini(ctx, prompt)
		if err == nil && result != "" {
			return result, nil
		}
	}

	return "", fmt.Errorf("all LLM providers failed")
}

// OpenAI-compatible request for Ollama /v1 endpoint
type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *LLMClient) generateWithOllama(ctx context.Context, prompt string) (string, error) {
	reqBody := openAIRequest{
		Model: c.ollamaModel,
		Messages: []openAIMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Use OpenAI-compatible /v1/chat/completions endpoint
	url := c.ollamaBaseURL
	if !strings.HasSuffix(url, "/v1") {
		url = url + "/v1"
	}
	url = url + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(respBody))
	}

	var openAIResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", err
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("ollama returned empty response")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (c *LLMClient) generateWithGemini(ctx context.Context, prompt string) (string, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.geminiModel, c.geminiAPIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini error %d: %s", resp.StatusCode, string(respBody))
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned empty response")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// BuildWhatsNewPrompt creates a prompt for generating What's New summary
func BuildWhatsNewPrompt(data map[string]interface{}) string {
	var sections []string

	sections = append(sections, "Generate a professional, engaging 'What's New' summary for an educational institution's admission page. The summary should be 2-3 paragraphs, student-focused, and highlight key admission information.")

	// Extract programs
	if programs, ok := data["programs_data"].([]interface{}); ok && len(programs) > 0 {
		var programNames []string
		for _, p := range programs {
			if pm, ok := p.(map[string]interface{}); ok {
				if title, ok := pm["title"].(string); ok && title != "" {
					programNames = append(programNames, title)
				}
			}
		}
		if len(programNames) > 0 {
			sections = append(sections, fmt.Sprintf("Programs offered: %s", strings.Join(programNames, ", ")))
		}
	}

	// Extract eligibility
	if eligibility, ok := data["eligibility_data"].([]interface{}); ok && len(eligibility) > 0 {
		sections = append(sections, "Eligibility criteria are available for various programs and streams.")
	}

	// Extract scholarships
	if scholarships, ok := data["scholarships_data"].([]interface{}); ok && len(scholarships) > 0 {
		sections = append(sections, fmt.Sprintf("%d scholarship(s) are available.", len(scholarships)))
	}

	// Extract courses
	if courses, ok := data["courses_data"].([]interface{}); ok && len(courses) > 0 {
		sections = append(sections, fmt.Sprintf("%d course(s) with detailed fee structures are listed.", len(courses)))
	}

	// Extract overview
	if overview, ok := data["overview_data"].(map[string]interface{}); ok {
		if heading, ok := overview["overviewHeading"].(string); ok && heading != "" {
			sections = append(sections, fmt.Sprintf("Institution highlight: %s", heading))
		}
	}

	sections = append(sections, "\nWrite the summary using bullet points (-) for each key point. Mention that admissions are open and encourage prospective students to explore the programs. Keep it under 200 words.")

	return strings.Join(sections, "\n")
}
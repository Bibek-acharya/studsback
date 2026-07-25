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
	"sync"
	"time"

	"studsphere/backend/internal/embedding"
	"studsphere/backend/internal/shared/config"

	"gorm.io/gorm"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Message   string    `json:"message"`
	SessionID string    `json:"session_id"`
	History   []Message `json:"history"`
}

type contextResult struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Content     string `json:"content,omitempty"`
	URL         string `json:"url,omitempty"`
}

type Service struct {
	db         *gorm.DB
	sessions   map[string][]Message
	sessionsMu sync.RWMutex
	httpClient *http.Client
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db:         db,
		sessions:   make(map[string][]Message),
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (s *Service) IsEnabled() bool {
	return config.AppConfig.LLMEnabled
}

type ModelInfo struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

func (s *Service) ListModels() ([]ModelInfo, error) {
	if !s.IsEnabled() {
		return nil, errors.New("LLM service is not configured (set LLM_ENABLED=true)")
	}

	baseURL := strings.TrimRight(config.AppConfig.LLMBaseURL, "/")
	if baseURL == "" {
		return nil, errors.New("LLM_BASE_URL is not set")
	}
	url := baseURL + "/models"

	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create models request: %w", err)
	}
	if key := config.AppConfig.LLMAPIKey; key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("could not reach LLM server at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM /models returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var raw struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to parse /models response: %w", err)
	}
	return raw.Data, nil
}

// Chat streams a token-by-token reply to the supplied writer. It performs RAG
// over the platform's content (colleges, scholarships, courses, etc.) and
// forwards the assembled system prompt + history to an OpenAI-compatible
// /chat/completions endpoint. The writer receives SSE-formatted events:
//   - data: {"token":"..."}\n\n for each streamed chunk
//   - data: {"done":true}\n\n when finished
//   - data: {"error":"..."}\n\n on failure
func (s *Service) Chat(parent context.Context, stream io.Writer, req ChatRequest) error {
	if !s.IsEnabled() {
		return errors.New("LLM service is not configured (set LLM_ENABLED=true)")
	}
	if strings.TrimSpace(req.Message) == "" {
		return errors.New("message is required")
	}

	contextItems := s.retrieveContext(req.Message)
	log.Printf("ai: RAG retrieved %d context items for query: %q", len(contextItems), req.Message)
	contextStr := s.buildContext(contextItems)

	if contextStr == "" {
		fallbackMsg := "I couldn't find any relevant information about that on StudSphere. Try searching for colleges, courses, or scholarships directly on the website, or rephrase your question."
		payload, _ := json.Marshal(map[string]string{"token": fallbackMsg})
		fmt.Fprintf(stream, "data: %s\n\n", payload)
		doneEvent, _ := json.Marshal(map[string]bool{"done": true})
		fmt.Fprintf(stream, "data: %s\n\n", doneEvent)
		if flusher, ok := stream.(http.Flusher); ok {
			flusher.Flush()
		}
		log.Printf("ai: no context found — returning fallback without LLM call")
		return nil
	}

	systemMsg := s.buildSystemPrompt(contextStr)

	messages := s.assembleMessages(req, systemMsg)

	reply, err := s.streamCompletion(parent, stream, messages)
	if err != nil {
		return err
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "default"
	}
	if reply != "" {
		s.sessionsMu.Lock()
		s.sessions[sessionID] = append(s.sessions[sessionID],
			Message{Role: "user", Content: req.Message},
			Message{Role: "assistant", Content: reply},
		)
		if len(s.sessions[sessionID]) > 20 {
			s.sessions[sessionID] = s.sessions[sessionID][len(s.sessions[sessionID])-20:]
		}
		s.sessionsMu.Unlock()
	}

	doneEvent, _ := json.Marshal(map[string]bool{"done": true})
	fmt.Fprintf(stream, "data: %s\n\n", doneEvent)
	if flusher, ok := stream.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (s *Service) retrieveContext(query string) []contextResult {
	var items []contextResult
	if embedding.IsEnabled() {
		if vec, err := embedding.GenerateEmbedding(query); err == nil && len(vec) > 0 {
			items = s.vectorSearch(vec)
		}
	}
	if len(items) == 0 {
		items = s.keywordSearch(query)
	}
	return items
}

func (s *Service) vectorSearch(vec []float32) []contextResult {
	var results []contextResult
	vectorStr := embedding.Float32SliceToPgVector(vec)

	queries := []struct {
		table string
		sql   string
	}{
		{"colleges", fmt.Sprintf("SELECT id, COALESCE(name,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'college' as type, COALESCE(name,'') as url FROM colleges WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 10", vectorStr, vectorStr)},
		{"courses", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'course' as type, '' as url FROM courses WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 10", vectorStr, vectorStr)},
		{"scholarships", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'scholarship' as type, '' as url FROM scholarships WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 10", vectorStr, vectorStr)},
		{"exams", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'exam' as type, COALESCE(title,'') as url FROM exams WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 10", vectorStr, vectorStr)},
		{"news", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(excerpt,'') as description, COALESCE(content,'') as content, 'news' as type, '' as url FROM news WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 10", vectorStr, vectorStr)},
		{"events", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'event' as type, '' as url FROM events WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 10", vectorStr, vectorStr)},
		{"blogs", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(excerpt,'') as description, COALESCE(content,'') as content, 'blog' as type, '' as url FROM blogs WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 10", vectorStr, vectorStr)},
		{"site_pages", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(content,'') as description, COALESCE(content,'') as content, 'page' as type, COALESCE(slug,'') as url FROM site_pages WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 10", vectorStr, vectorStr)},
		{"institution_entrances", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'entrance' as type, COALESCE(title,'') as url FROM institution_entrances WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 10", vectorStr, vectorStr)},
	}

	for _, q := range queries {
		var rows []contextResult
		if err := s.db.Raw(q.sql).Scan(&rows).Error; err != nil {
			log.Printf("ai: vector search on %s failed: %v", q.table, err)
			continue
		}
		results = append(results, rows...)
	}
	return results
}

func (s *Service) keywordSearch(q string) []contextResult {
	var results []contextResult
	like := "%" + strings.ToLower(q) + "%"

	type tableQuery struct {
		table    string
		sql      string
		paramLen int
	}
	queries := []tableQuery{
		{"colleges", "SELECT id, name as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'college' as type, COALESCE(name,'') as url FROM colleges WHERE LOWER(COALESCE(name,'')) LIKE LOWER(?) OR LOWER(COALESCE(description,'')) LIKE LOWER(?) OR LOWER(COALESCE(location,'')) LIKE LOWER(?) LIMIT 10", 3},
		{"courses", "SELECT id, title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'course' as type, '' as url FROM courses WHERE LOWER(COALESCE(title,'')) LIKE LOWER(?) OR LOWER(COALESCE(description,'')) LIKE LOWER(?) OR LOWER(COALESCE(field,'')) LIKE LOWER(?) LIMIT 10", 3},
		{"scholarships", "SELECT id, title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'scholarship' as type, '' as url FROM scholarships WHERE LOWER(COALESCE(title,'')) LIKE LOWER(?) OR LOWER(COALESCE(description,'')) LIKE LOWER(?) OR LOWER(COALESCE(provider,'')) LIKE LOWER(?) LIMIT 10", 3},
		{"exams", "SELECT id, title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'exam' as type, COALESCE(title,'') as url FROM exams WHERE LOWER(COALESCE(title,'')) LIKE LOWER(?) OR LOWER(COALESCE(description,'')) LIKE LOWER(?) LIMIT 10", 2},
		{"site_pages", "SELECT id, title, content as description, content as content, 'page' as type, slug as url FROM site_pages WHERE LOWER(title) LIKE ? OR LOWER(content) LIKE ? LIMIT 10", 2},
		{"news", "SELECT id, title, COALESCE(excerpt,'') as description, COALESCE(content,'') as content, 'news' as type, '' as url FROM news WHERE LOWER(COALESCE(title,'')) LIKE LOWER(?) OR LOWER(COALESCE(excerpt,'')) LIKE LOWER(?) OR LOWER(COALESCE(content,'')) LIKE LOWER(?) LIMIT 10", 3},
		{"events", "SELECT id, title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'event' as type, '' as url FROM events WHERE LOWER(COALESCE(title,'')) LIKE LOWER(?) OR LOWER(COALESCE(description,'')) LIKE LOWER(?) LIMIT 10", 2},
		{"institution_entrances", "SELECT id, title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'entrance' as type, COALESCE(title,'') as url FROM institution_entrances WHERE LOWER(COALESCE(title,'')) LIKE LOWER(?) OR LOWER(COALESCE(description,'')) LIKE LOWER(?) OR LOWER(COALESCE(program,'')) LIKE LOWER(?) LIMIT 10", 3},
		{"blogs", "SELECT id, title, COALESCE(excerpt,'') as description, COALESCE(content,'') as content, 'blog' as type, '' as url FROM blogs WHERE LOWER(COALESCE(title,'')) LIKE LOWER(?) OR LOWER(COALESCE(excerpt,'')) LIKE LOWER(?) OR LOWER(COALESCE(content,'')) LIKE LOWER(?) LIMIT 10", 3},
	}

	for _, q := range queries {
		var rows []contextResult
		var err error
		args := []interface{}{like, like, like}
		err = s.db.Raw(q.sql, args[:q.paramLen]...).Scan(&rows).Error
		if err != nil {
			log.Printf("ai: keyword search on %s failed: %v", q.table, err)
			continue
		}
		results = append(results, rows...)
	}
	return results
}

func (s *Service) buildContext(items []contextResult) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		fmt.Fprintf(&b, "[%s] %s\n", item.Type, item.Title)
		content := item.Content
		if content == "" {
			content = item.Description
		}
		runes := []rune(content)
		if len(runes) > 2000 {
			content = string(runes[:2000]) + "..."
		}
		b.WriteString(content)
	}
	return b.String()
}

func (s *Service) buildSystemPrompt(contextStr string) string {
	base := "You are StudSphere AI, a helpful assistant for StudSphere.com — Nepal's " +
		"college discovery, course comparison, and scholarship platform. " +
		"Your job is to help students find the right college, course, exam, or scholarship in Nepal. " +
		"Be friendly, concise, and practical. Use short paragraphs and bullet points when listing options."

	return base + "\n\nRULES (you MUST follow all of them):\n" +
		"1. You have NO knowledge outside the CONTEXT section below. Do not use your training data.\n" +
		"2. For EVERY fact you state, append the source in brackets — e.g. [College: Kathmandu University], [Scholarship: Fulbright], [Course: BSc Nursing].\n" +
		"3. If the CONTEXT does not contain enough information, say \"I don't have enough information about that on StudSphere\" and suggest searching the website.\n" +
		"4. Never invent college names, scholarship amounts, deadlines, contact details, or any other specific data.\n" +
		"5. Never mention that you are following rules or that you received context — just answer naturally.\n" +
		"6. If the user asks about topics outside StudSphere (general knowledge, entertainment, politics), politely say you can only answer questions about colleges, courses, scholarships, exams, and education in Nepal.\n\n" +
		"--- CONTEXT ---\n" + contextStr
}

func (s *Service) assembleMessages(req ChatRequest, systemMsg string) []Message {
	messages := []Message{
		{Role: "system", Content: systemMsg},
	}

	if len(req.History) > 0 {
		messages = append(messages, req.History...)
	} else if req.SessionID != "" {
		s.sessionsMu.RLock()
		if h := s.sessions[req.SessionID]; len(h) > 0 {
			history := make([]Message, len(h))
			copy(history, h)
			messages = append(messages, history...)
		}
		s.sessionsMu.RUnlock()
	}

	messages = append(messages, Message{Role: "user", Content: req.Message})
	return messages
}

type llmRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type llmChunk struct {
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

func (s *Service) streamCompletion(parent context.Context, stream io.Writer, messages []Message) (string, error) {
	cfg := config.AppConfig
	baseURL := strings.TrimRight(cfg.LLMBaseURL, "/")
	if baseURL == "" {
		return "", errors.New("LLM_BASE_URL is not set")
	}
	url := baseURL + "/chat/completions"

	body, err := json.Marshal(llmRequest{
		Model:       cfg.LLMModel,
		Messages:    messages,
		Stream:      true,
		Temperature: 0.3,
		MaxTokens:   1024,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(parent, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.LLMAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.LLMAPIKey)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("LLM API returned %d: %s", resp.StatusCode, truncate(string(respBody), 300))
		if resp.StatusCode == http.StatusNotFound {
			if models, mErr := s.ListModels(); mErr == nil && len(models) > 0 {
				names := make([]string, 0, len(models))
				for _, m := range models {
					names = append(names, m.ID)
				}
				errMsg += fmt.Sprintf(" | Configured LLM_MODEL=%q not found. Available models: %s",
					config.AppConfig.LLMModel, strings.Join(names, ", "))
			}
		}
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

		var chunk llmChunk
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
			payload, _ := json.Marshal(map[string]string{"token": token})
			if _, err := fmt.Fprintf(stream, "data: %s\n\n", payload); err != nil {
				return full.String(), err
			}
			if flusher, ok := stream.(http.Flusher); ok {
				flusher.Flush()
			}
			full.WriteString(token)
		}
	}

	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("error reading LLM stream: %w", err)
	}
	return full.String(), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

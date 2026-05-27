package chat

import (
	"bytes"
	"encoding/json"
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
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

type contextResult struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Content     string `json:"content,omitempty"`
}

type Service struct {
	db          *gorm.DB
	sessions    map[string][]Message
	sessionsMu  sync.RWMutex
	httpClient  *http.Client
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db:        db,
		sessions:  make(map[string][]Message),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (s *Service) Chat(stream io.Writer, req ChatRequest) error {
	if req.Message == "" {
		return fmt.Errorf("message is required")
	}

	// 1. Generate embedding for the query
	vec, err := embedding.GenerateEmbedding(req.Message)
	if err != nil {
		log.Printf("Chat: embedding generation failed: %v", err)
	}

	// 2. Search for relevant content
	var contextItems []contextResult
	if err == nil && len(vec) > 0 {
		contextItems = s.vectorSearch(vec)
	}
	if len(contextItems) == 0 {
		contextItems = s.keywordSearch(req.Message)
	}

	// 3. Build context string
	contextStr := s.buildContext(contextItems)

	// 4. Build system prompt
	var systemMsg string
	if contextStr == "" {
		systemMsg = "You are StudSphere AI, a helpful assistant for StudSphere.com. " +
			"If asked a question, say 'I don't have information about that.' Never make up or guess information. Be concise."
	} else {
		systemMsg = "You are StudSphere AI, a helpful assistant for StudSphere.com — Nepal's " +
			"college and scholarship discovery platform. Answer ONLY from the provided context. " +
			"If the context doesn't contain enough information to answer, say " +
			"'I don't have information about that.' Never make up or guess information. " +
			"Be concise and helpful. Here is the website content to answer from:\n\n" + contextStr
	}

	// 5. Call Gemini API
	return s.callGemini(stream, systemMsg, req)
}

func (s *Service) vectorSearch(vec []float32) []contextResult {
	var results []contextResult
	vectorStr := embedding.Float32SliceToPgVector(vec)

	queries := []struct {
		table string
		sql   string
	}{
		{"colleges", fmt.Sprintf("SELECT id, COALESCE(name,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'college' as type FROM colleges WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 3", vectorStr, vectorStr)},
		{"courses", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'course' as type FROM courses WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 3", vectorStr, vectorStr)},
		{"scholarships", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'scholarship' as type FROM scholarships WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 3", vectorStr, vectorStr)},
		{"news", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(excerpt,'') as description, COALESCE(content,'') as content, 'news' as type FROM news WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 3", vectorStr, vectorStr)},
		{"events", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'event' as type FROM events WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 3", vectorStr, vectorStr)},
		{"exams", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'exam' as type FROM exams WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 3", vectorStr, vectorStr)},
		{"blogs", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(excerpt,'') as description, COALESCE(content,'') as content, 'blog' as type FROM blogs WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 3", vectorStr, vectorStr)},
		{"site_pages", fmt.Sprintf("SELECT id, COALESCE(title,'') as title, COALESCE(content,'') as description, COALESCE(content,'') as content, 'page' as type FROM site_pages WHERE embedding IS NOT NULL AND embedding <=> '%s'::vector < 1.5 ORDER BY embedding <=> '%s'::vector LIMIT 3", vectorStr, vectorStr)},
	}

	for _, q := range queries {
		var rows []contextResult
		if err := s.db.Raw(q.sql).Scan(&rows).Error; err != nil {
			continue
		}
		results = append(results, rows...)
	}

	return results
}

func (s *Service) keywordSearch(q string) []contextResult {
	var results []contextResult
	like := "%" + strings.ToLower(q) + "%"

	var sitePages []contextResult
	s.db.Table("site_pages").
		Select("id, title, content as description, content as content, 'page' as type").
		Where("LOWER(title) LIKE ? OR LOWER(content) LIKE ?", like, like).
		Limit(3).Scan(&sitePages)
	results = append(results, sitePages...)

	var colleges []contextResult
	s.db.Table("colleges").
		Select("id, name as title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'college' as type").
		Where("LOWER(COALESCE(name,'')) LIKE LOWER(?) OR LOWER(COALESCE(description,'')) LIKE LOWER(?)", like, like).
		Limit(3).Scan(&colleges)
	results = append(results, colleges...)

	var scholarships []contextResult
	s.db.Table("scholarships").
		Select("id, title, COALESCE(description,'') as description, COALESCE(description,'') as content, 'scholarship' as type").
		Where("LOWER(COALESCE(title,'')) LIKE LOWER(?) OR LOWER(COALESCE(description,'')) LIKE LOWER(?)", like, like).
		Limit(3).Scan(&scholarships)
	results = append(results, scholarships...)

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
		b.WriteString(fmt.Sprintf("[%s] %s\n", item.Type, item.Title))
		content := item.Content
		if content == "" {
			content = item.Description
		}
		runes := []rune(content)
		if len(runes) > 500 {
			content = string(runes[:500]) + "..."
		}
		b.WriteString(content)
	}
	return b.String()
}

func (s *Service) callGemini(stream io.Writer, systemMsg string, req ChatRequest) error {
	apiKey := config.AppConfig.GeminiAPIKey
	if apiKey == "" {
		return fmt.Errorf("Gemini API key not configured")
	}

	model := config.AppConfig.GeminiModel
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, apiKey)

	systemInstruction := map[string]interface{}{
		"parts": []map[string]string{{"text": systemMsg}},
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	s.sessionsMu.RLock()
	history := make([]Message, len(s.sessions[sessionID]))
	copy(history, s.sessions[sessionID])
	s.sessionsMu.RUnlock()

	var contents []map[string]interface{}
	for _, msg := range history {
		contents = append(contents, map[string]interface{}{
			"role":  msg.Role,
			"parts": []map[string]string{{"text": msg.Content}},
		})
	}

	contents = append(contents, map[string]interface{}{
		"role":  "user",
		"parts": []map[string]string{{"text": req.Message}},
	})

	payload := map[string]interface{}{
		"system_instruction": systemInstruction,
		"contents":           contents,
		"generationConfig": map[string]interface{}{
			"temperature":     0.2,
			"maxOutputTokens": 1024,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("Gemini API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Gemini API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var fullResponse strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for {
		var geminiResp struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
		}
		if err := decoder.Decode(&geminiResp); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		for _, candidate := range geminiResp.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					token, _ := json.Marshal(map[string]string{"token": part.Text})
					fmt.Fprintf(stream, "data: %s\n\n", token)
					fullResponse.WriteString(part.Text)
				}
			}
		}
	}

	done, _ := json.Marshal(map[string]bool{"done": true})
	fmt.Fprintf(stream, "data: %s\n\n", done)

	if fullResponse.Len() > 0 {
		s.sessionsMu.Lock()
		s.sessions[sessionID] = append(s.sessions[sessionID],
			Message{Role: "user", Content: req.Message},
			Message{Role: "model", Content: fullResponse.String()},
		)
		if len(s.sessions[sessionID]) > 6 {
			s.sessions[sessionID] = s.sessions[sessionID][len(s.sessions[sessionID])-6:]
		}
		s.sessionsMu.Unlock()
	}

	return nil
}

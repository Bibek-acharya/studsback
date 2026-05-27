package embedding

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"studsphere/backend/internal/shared/config"

	"gorm.io/gorm"
)

var (
	client     *http.Client
	clientOnce sync.Once
)

func httpClient() *http.Client {
	clientOnce.Do(func() {
		client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
			},
		}
	})
	return client
}

type embeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func IsEnabled() bool {
	return config.AppConfig.EmbeddingEnabled
}

func GenerateEmbedding(text string) ([]float32, error) {
	embeddings, err := GenerateEmbeddingsBatch([]string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, errors.New("no embedding returned")
	}
	return embeddings[0], nil
}

func GenerateEmbeddingsBatch(texts []string) ([][]float32, error) {
	if !IsEnabled() {
		return nil, errors.New("embedding service is not configured")
	}

	dimension := config.AppConfig.VectorDimension

	cleaned := make([]string, len(texts))
	totalChars := 0
	for i, t := range texts {
		cleaned[i] = truncateText(strings.TrimSpace(t), 8000)
		totalChars += len(cleaned[i])
	}

	if totalChars == 0 {
		return make([][]float32, len(texts)), nil
	}

	reqBody := embeddingRequest{
		Input: cleaned,
		Model: config.AppConfig.EmbeddingModel,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimRight(config.AppConfig.EmbeddingBaseURL, "/") + "/embeddings"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.EmbeddingAPIKey)

	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("embedding API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("embedding API error: %s - %s", result.Error.Type, result.Error.Message)
	}

	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index < len(embeddings) {
			vec := make([]float32, dimension)
			for i := 0; i < dimension && i < len(d.Embedding); i++ {
				vec[i] = float32(d.Embedding[i])
			}
			embeddings[d.Index] = vec
		}
	}

	for i, e := range embeddings {
		if e == nil {
			embeddings[i] = make([]float32, dimension)
		}
	}

	return embeddings, nil
}

func BuildEmbeddingText(parts ...string) string {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(strings.TrimSpace(p))
	}
	return b.String()
}

func truncateText(s string, maxChars int) string {
	runes := []rune(s)
	if len(runes) > maxChars {
		return string(runes[:maxChars])
	}
	return s
}

func ReindexAll() error {
	if !IsEnabled() {
		log.Println("Embedding service not enabled, skipping reindex")
		return nil
	}

	db := config.GetDB()
	batchSize := config.AppConfig.EmbeddingBatchSize
	tables := []string{"colleges", "courses", "exams", "scholarships", "news", "events", "blogs"}

	for _, table := range tables {
		log.Printf("Reindexing embeddings for table: %s", table)
		if err := reindexTable(db, table, batchSize); err != nil {
			log.Printf("Error reindexing table %s: %v", table, err)
		}
	}

	log.Println("Embedding reindex complete")
	return nil
}

func reindexTable(db *gorm.DB, table string, batchSize int) error {
	var total int64
	db.Table(table).Where("embedding IS NULL").Count(&total)
	if total == 0 {
		log.Printf("  Table %s: no items need embedding", table)
		return nil
	}

	log.Printf("  Table %s: %d items to embed", table, total)

	offset := 0
	for {
		var rows []map[string]interface{}
		if err := db.Table(table).
			Where("embedding IS NULL").
			Select(buildSelectForTable(table)).
			Limit(batchSize).
			Offset(offset).
			Find(&rows).Error; err != nil {
			return err
		}

		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			text := buildEmbeddingInput(table, row)
			if text == "" {
				continue
			}

			vec, err := GenerateEmbedding(text)
			if err != nil {
				log.Printf("  Warning: failed to embed %s id=%v: %v", table, row["id"], err)
				continue
			}

			vectorStr := Float32SliceToPgVector(vec)
			id := row["id"]
			sql := fmt.Sprintf("UPDATE %s SET embedding = '%s'::vector WHERE id = ?", table, vectorStr)
			if err := db.Exec(sql, id).Error; err != nil {
				log.Printf("  Warning: failed to update embedding for %s id=%v: %v", table, id, err)
			}
		}

		offset += len(rows)
		log.Printf("  Table %s: processed %d/%d", table, offset, total)
	}

	return nil
}

func buildSelectForTable(table string) string {
	switch table {
	case "colleges":
		return "id, name, COALESCE(full_name, '') as full_name, COALESCE(description, '') as description, COALESCE(location, '') as location, COALESCE(affiliation, '') as affiliation, COALESCE(college_type, '') as college_type"
	case "courses":
		return "id, title, COALESCE(short_title, '') as short_title, COALESCE(description, '') as description, COALESCE(field, '') as field, COALESCE(level, '') as level, COALESCE(affiliation, '') as affiliation"
	case "exams":
		return "id, title, COALESCE(description, '') as description, COALESCE(board, '') as board, COALESCE(type, '') as type, COALESCE(university, '') as university"
	case "scholarships":
		return "id, title, COALESCE(description, '') as description, COALESCE(provider, '') as provider, COALESCE(location, '') as location, COALESCE(scholarship_type, '') as scholarship_type"
	case "news":
		return "id, title, COALESCE(excerpt, '') as excerpt, COALESCE(content, '') as content, COALESCE(category, '') as category, COALESCE(source, '') as source"
	case "events":
		return "id, title, COALESCE(description, '') as description, COALESCE(excerpt, '') as excerpt, COALESCE(category, '') as category, COALESCE(location, '') as location"
	case "blogs":
		return "id, title, COALESCE(excerpt, '') as excerpt, COALESCE(content, '') as content, COALESCE(category, '') as category, COALESCE(author, '') as author"
	default:
		return "id, title"
	}
}

func buildEmbeddingInput(table string, row map[string]interface{}) string {
	var parts []string
	switch table {
	case "colleges":
		parts = append(parts,
			getStr(row, "name"),
			getStr(row, "full_name"),
			getStr(row, "description"),
			getStr(row, "location"),
			getStr(row, "affiliation"),
			getStr(row, "college_type"),
		)
	case "courses":
		parts = append(parts,
			getStr(row, "title"),
			getStr(row, "short_title"),
			getStr(row, "description"),
			getStr(row, "field"),
			getStr(row, "level"),
			getStr(row, "affiliation"),
		)
	case "exams":
		parts = append(parts,
			getStr(row, "title"),
			getStr(row, "description"),
			getStr(row, "board"),
			getStr(row, "type"),
			getStr(row, "university"),
		)
	case "scholarships":
		parts = append(parts,
			getStr(row, "title"),
			getStr(row, "description"),
			getStr(row, "provider"),
			getStr(row, "location"),
			getStr(row, "scholarship_type"),
		)
	case "news":
		parts = append(parts,
			getStr(row, "title"),
			getStr(row, "excerpt"),
			getStr(row, "content"),
			getStr(row, "category"),
			getStr(row, "source"),
		)
	case "events":
		parts = append(parts,
			getStr(row, "title"),
			getStr(row, "description"),
			getStr(row, "excerpt"),
			getStr(row, "category"),
			getStr(row, "location"),
		)
	case "blogs":
		parts = append(parts,
			getStr(row, "title"),
			getStr(row, "excerpt"),
			getStr(row, "content"),
			getStr(row, "category"),
			getStr(row, "author"),
		)
	}

	var nonEmpty []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(p))
		}
	}
	return strings.Join(nonEmpty, " | ")
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func Float32SliceToPgVector(v []float32) string {
	var b strings.Builder
	b.WriteString("[")
	for i, val := range v {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%.8f", val))
	}
	b.WriteString("]")
	return b.String()
}



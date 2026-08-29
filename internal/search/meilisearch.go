package search

import (
	"os"

	"studsphere/backend/internal/search/retrieval"

	"github.com/meilisearch/meilisearch-go"
)

type MeiliConfig struct {
	Host        string
	APIKey      string
	IndexPrefix string
}

type MeiliClient struct {
	Client      meilisearch.ServiceManager
	IndexPrefix string
}

func LoadMeiliConfig() MeiliConfig {
	return MeiliConfig{
		Host:        getEnv("MEILI_HOST", "http://localhost:7700"),
		APIKey:      getEnv("MEILI_MASTER_KEY", ""),
		IndexPrefix: getEnv("MEILI_INDEX_PREFIX", "studs_"),
	}
}

func NewMeiliClient(cfg MeiliConfig) *MeiliClient {
	opts := []meilisearch.Option{}
	if cfg.APIKey != "" {
		opts = append(opts, meilisearch.WithAPIKey(cfg.APIKey))
	}
	client := meilisearch.New(cfg.Host, opts...)
	return &MeiliClient{
		Client:      client,
		IndexPrefix: cfg.IndexPrefix,
	}
}

func (c *MeiliClient) IndexName(entity string) string {
	return c.IndexPrefix + entity
}

func (c *MeiliClient) IsHealthy() bool {
	_, err := c.Client.Health()
	return err == nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// serviceManagerAdapter adapts meilisearch.ServiceManager to retrieval.MeilisearchIndexProvider.
type serviceManagerAdapter struct {
	sm meilisearch.ServiceManager
}

func (a *serviceManagerAdapter) Index(uid string) retrieval.IndexClient {
	return a.sm.Index(uid)
}

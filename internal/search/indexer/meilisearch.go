package indexer

import (
	"context"
	"fmt"

	"github.com/meilisearch/meilisearch-go"

	"studsphere/backend/internal/search"
)

type IndexConfig struct {
	Entity          string
	PrimaryKey      string
	SearchableAttrs []string
	FilterableAttrs []string
	SortableAttrs   []string
	RankingRules    []string
	Synonyms        map[string][]string
}

var IndexConfigs = []IndexConfig{
	{
		Entity:          "colleges",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"name", "full_name", "description", "location", "affiliation", "college_type", "programs_list"},
		FilterableAttrs: []string{"college_type", "location", "affiliation", "rating"},
		SortableAttrs:   []string{"rating", "created_at"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
		Synonyms:        map[string][]string{"uni": {"university"}, "clg": {"college"}, "csit": {"computer science"}, "bit": {"information technology"}, "bca": {"computer application"}, "mbbs": {"medicine"}, "mba": {"business administration"}, "tu": {"tribhuvan university"}, "ku": {"kathmandu university"}, "pu": {"pokhara university"}, "pu)": {"purbanchal university"}},
	},
	{
		Entity:          "courses",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"title", "short_title", "description", "field", "level", "affiliation"},
		FilterableAttrs: []string{"field", "level", "affiliation"},
		SortableAttrs:   []string{"created_at"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
	},
	{
		Entity:          "scholarships",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"title", "description", "provider", "location", "scholarship_type"},
		FilterableAttrs: []string{"scholarship_type", "location"},
		SortableAttrs:   []string{"created_at"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
	},
	{
		Entity:          "news",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"title", "excerpt", "content", "category", "source"},
		FilterableAttrs: []string{"category", "source"},
		SortableAttrs:   []string{"created_at"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
	},
	{
		Entity:          "events",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"title", "description", "excerpt", "category", "location"},
		FilterableAttrs: []string{"category", "location"},
		SortableAttrs:   []string{"created_at", "event_date"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
	},
	{
		Entity:          "exams",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"title", "description", "board", "type", "university"},
		FilterableAttrs: []string{"board", "type", "university"},
		SortableAttrs:   []string{"created_at"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
	},
	{
		Entity:          "blogs",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"title", "excerpt", "content", "category", "author"},
		FilterableAttrs: []string{"category", "author"},
		SortableAttrs:   []string{"created_at"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
	},
	{
		Entity:          "universities",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"name", "description", "location", "type"},
		FilterableAttrs: []string{"type", "location", "rating"},
		SortableAttrs:   []string{"rating", "created_at"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
	},
	{
		Entity:          "admission_pages",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"title", "institution_name", "institution_location", "level"},
		FilterableAttrs: []string{"level", "status", "institution_location"},
		SortableAttrs:   []string{"created_at"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
	},
	{
		Entity:          "institutions",
		PrimaryKey:      "id",
		SearchableAttrs: []string{"institution_name", "district", "affiliation", "about", "organization_type"},
		FilterableAttrs: []string{"organization_type", "district", "affiliation", "status"},
		SortableAttrs:   []string{"created_at"},
		RankingRules:    []string{"words", "typo", "proximity", "attribute", "exactness"},
		Synonyms:        map[string][]string{"uni": {"university"}, "clg": {"college"}, "csit": {"computer science"}, "bit": {"information technology"}, "tu": {"tribhuvan university"}, "ku": {"kathmandu university"}, "pu": {"pokhara university"}},
	},
}

type MeiliIndexer struct {
	client *search.MeiliClient
}

func NewMeiliIndexer(client *search.MeiliClient) *MeiliIndexer {
	return &MeiliIndexer{client: client}
}

func (idx *MeiliIndexer) CreateIndexes(ctx context.Context) error {
	for _, cfg := range IndexConfigs {
		indexName := idx.client.IndexName(cfg.Entity)

		task, err := idx.client.Client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        indexName,
			PrimaryKey: cfg.PrimaryKey,
		})
		if err != nil {
			// Index may already exist — continue to update settings
		}

		if task != nil {
			_, _ = idx.client.Client.WaitForTask(task.TaskUID, 0)
		}

		settings := meilisearch.Settings{
			SearchableAttributes: cfg.SearchableAttrs,
			FilterableAttributes: cfg.FilterableAttrs,
			SortableAttributes:   cfg.SortableAttrs,
			RankingRules:         cfg.RankingRules,
			Synonyms:             cfg.Synonyms,
		}

		task, err = idx.client.Client.Index(indexName).UpdateSettings(&settings)
		if err != nil {
			return fmt.Errorf("update settings %s: %w", indexName, err)
		}
		_, _ = idx.client.Client.WaitForTask(task.TaskUID, 0)
	}

	return nil
}

func (idx *MeiliIndexer) Upsert(ctx context.Context, entity string, documents []map[string]interface{}) error {
	indexName := idx.client.IndexName(entity)
	task, err := idx.client.Client.Index(indexName).AddDocuments(documents)
	if err != nil {
		return fmt.Errorf("upsert %s: %w", indexName, err)
	}
	_, err = idx.client.Client.WaitForTask(task.TaskUID, 0)
	return err
}

func (idx *MeiliIndexer) Delete(ctx context.Context, entity string, ids []uint) error {
	indexName := idx.client.IndexName(entity)
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = fmt.Sprintf("%d", id)
	}
	task, err := idx.client.Client.Index(indexName).DeleteDocuments(strIDs)
	if err != nil {
		return fmt.Errorf("delete %s: %w", indexName, err)
	}
	_, err = idx.client.Client.WaitForTask(task.TaskUID, 0)
	return err
}

package indexer

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

type Reindexer struct {
	db      *gorm.DB
	indexer *MeiliIndexer
}

func NewReindexer(db *gorm.DB, indexer *MeiliIndexer) *Reindexer {
	return &Reindexer{db: db, indexer: indexer}
}

func (r *Reindexer) ReindexAll(ctx context.Context) error {
	start := time.Now()
	log.Println("reindex: starting full rebuild")

	if err := r.indexer.CreateIndexes(ctx); err != nil {
		return fmt.Errorf("reindex: create indexes: %w", err)
	}

	total := 0
	for _, table := range syncTables {
		count, err := r.reindexTable(ctx, table)
		if err != nil {
			log.Printf("reindex: table %s failed: %v", table.Name, err)
			continue
		}
		total += count
		log.Printf("reindex: %s indexed %d records", table.Name, count)
	}

	log.Printf("reindex: complete — %d records in %s", total, time.Since(start))
	return nil
}

func (r *Reindexer) reindexTable(ctx context.Context, table syncTable) (int, error) {
	var offset int
	batchSize := 500
	total := 0

	for {
		var rows []map[string]interface{}
		query := fmt.Sprintf(
			"SELECT %s FROM %s WHERE deleted_at IS NULL ORDER BY id ASC LIMIT ? OFFSET ?",
			table.SelectColumns, table.Name,
		)

		if err := r.db.WithContext(ctx).Raw(query, batchSize, offset).Scan(&rows).Error; err != nil {
			return total, err
		}

		if len(rows) == 0 {
			break
		}

		if err := r.indexer.Upsert(ctx, table.Entity, rows); err != nil {
			return total, err
		}

		total += len(rows)
		offset += batchSize

		if len(rows) < batchSize {
			break
		}
	}

	return total, nil
}

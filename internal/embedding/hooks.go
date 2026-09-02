package embedding

import (
	"fmt"
	"log"
	"reflect"
	"sync"

	"studsphere/backend/internal/shared/config"

	"gorm.io/gorm"
)

var embeddingTableSet = map[string]bool{}

func init() {
	for _, t := range embeddingTablesList {
		embeddingTableSet[t] = true
	}
}

type updateJob struct {
	table string
	id    any
}

var (
	queue      = make(chan updateJob, 256)
	queued     sync.Map
	workerOnce sync.Once
)

// EnqueueUpdate schedules an async embedding refresh for one row.
// Dedup collapses bursts of edits to the same row into one pending job.
func EnqueueUpdate(table string, id any) {
	if !IsEnabled() {
		return
	}
	if !embeddingTableSet[table] {
		return
	}
	key := fmt.Sprintf("%s:%v", table, id)
	if _, loaded := queued.LoadOrStore(key, true); loaded {
		return
	}
	select {
	case queue <- updateJob{table: table, id: id}:
	default:
		queued.Delete(key)
	}
}

// StartWorker launches the single embedding worker goroutine.
func StartWorker() {
	workerOnce.Do(startWorker)
}

func startWorker() {
	go func() {
		for job := range queue {
			processUpdate(job)
		}
	}()
}

func processUpdate(job updateJob) {
	defer queued.Delete(fmt.Sprintf("%s:%v", job.table, job.id))
	db := config.GetDB()
	if db == nil {
		return
	}
	if !hasEmbeddingColumn(db, job.table) {
		return
	}
	var rows []map[string]interface{}
	if err := db.Table(job.table).
		Select(buildSelectForTable(job.table)).
		Where("id = ? AND deleted_at IS NULL", job.id).
		Limit(1).
		Find(&rows).Error; err != nil || len(rows) == 0 {
		return
	}
	text := buildEmbeddingInput(job.table, rows[0])
	if text == "" {
		db.Exec(fmt.Sprintf("UPDATE %s SET embedded_at = now() WHERE id = ?", job.table), job.id)
		return
	}
	vec, err := GenerateEmbedding(text)
	if err != nil {
		log.Printf("Embedding worker: failed to embed %s id=%v: %v", job.table, job.id, err)
		return
	}
	db.Exec(fmt.Sprintf("UPDATE %s SET embedding = '%s'::vector, embedded_at = now() WHERE id = ?", job.table, Float32SliceToPgVector(vec)), job.id)
}

// RegisterGORMCallbacks enqueues embedding refreshes after every create/update
// on the embedding tables. Raw-SQL writes (db.Exec) bypass these callbacks, so
// the worker's own updates cannot re-trigger it.
func RegisterGORMCallbacks(db *gorm.DB) {
	enqueue := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Error != nil {
			return
		}
		if !embeddingTableSet[tx.Statement.Schema.Table] {
			return
		}
		field := tx.Statement.Schema.PrioritizedPrimaryField
		if field == nil {
			return
		}
		rv := tx.Statement.ReflectValue
		if !rv.IsValid() {
			return
		}
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Slice {
			for i := 0; i < rv.Len(); i++ {
				if v, zero := field.ValueOf(tx.Statement.Context, rv.Index(i)); !zero {
					EnqueueUpdate(tx.Statement.Schema.Table, v)
				}
			}
			return
		}
		if v, zero := field.ValueOf(tx.Statement.Context, rv); !zero {
			// ponytail: updates via db.Model(&Table{}).Where("id=?").Updates(map) have a zero
			// ID in dest and are skipped here; the stale-row reindex sweep catches them.
			EnqueueUpdate(tx.Statement.Schema.Table, v)
		}
	}
	db.Callback().Create().After("gorm:create").Register("embedding:enqueue", enqueue)
	db.Callback().Update().After("gorm:update").Register("embedding:enqueue", enqueue)
}

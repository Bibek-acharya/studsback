package indexer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SyncCursor struct {
	Table      string
	LastSyncAt time.Time
	LastSyncID uint
	BatchSize  int
}

func (c *SyncCursor) Advance(lastUpdatedAt time.Time, lastID uint) {
	c.LastSyncAt = lastUpdatedAt
	c.LastSyncID = lastID
}

type syncState struct {
	Table       string    `gorm:"column:table_name;primaryKey"`
	LastSyncAt  time.Time `gorm:"column:last_synced_at"`
	LastSyncID  uint      `gorm:"column:last_sync_id"`
}

func (syncState) TableName() string {
	return "sync_state"
}

type SyncWorker struct {
	db        *gorm.DB
	indexer   *MeiliIndexer
	interval  time.Duration
	batchSize int
	cursors   map[string]*SyncCursor
	tables    []syncTable
}

type syncTable struct {
	Name          string
	Entity        string
	SelectColumns string
	StatusFilter  string // SQL WHERE condition for published records (appended to query)
}

var syncTables = []syncTable{
	{"colleges", "colleges", "id, name, full_name, description, location, affiliation, college_type, rating, image_url, featured, verified, website, created_at, updated_at, deleted_at", ""},
	{"courses", "courses", "id, title, short_title, description, field, level, affiliation, status, created_at, updated_at, deleted_at", "status != 'draft'"},
	{"scholarships", "scholarships", "id, title, description, provider, location, scholarship_type, funding_type, status, deadline, created_at, updated_at, deleted_at", "status != 'draft'"},
	{"news", "news", "id, title, excerpt, content, category, source, image, author, published, created_at, updated_at, deleted_at", "published = true"},
	{"events", "events", "id, title, description, excerpt, category, location, date, image, organizer, end_date, status, created_at, updated_at, deleted_at", ""},
	{"exams", "exams", "id, title, description, board, type, university, created_at, updated_at, deleted_at", ""},
	{"blogs", "blogs", "id, title, excerpt, content, category, image, author, published, created_at, updated_at, deleted_at", "published = true"},
	{"universities", "universities", "id, name, description, location, type, rating, logo, cover, verified, programs_count, colleges_count, website, status, created_at, updated_at, deleted_at", "status = 'published'"},
	{"admission_pages", "admission_pages", "id, title, level, status, institution_name, institution_location, institution_link, created_at, updated_at, deleted_at", "status != 'draft'"},
	{"institution_users", "institutions", "id, institution_name, district, affiliation, organization_type, about, logo_url, card_image_url, featured, verified, website_url, status, profile_status, deleted_at, updated_at", "profile_status = 'published' AND status = 'approved'"},
}

func NewSyncWorker(db *gorm.DB, indexer *MeiliIndexer, interval time.Duration, batchSize int) *SyncWorker {
	cursors := make(map[string]*SyncCursor)
	for _, t := range syncTables {
		cursors[t.Name] = &SyncCursor{
			Table:      t.Name,
			LastSyncAt: time.Time{},
			LastSyncID: 0,
			BatchSize:  batchSize,
		}
	}

	return &SyncWorker{
		db:        db,
		indexer:   indexer,
		interval:  interval,
		batchSize: batchSize,
		cursors:   cursors,
		tables:    syncTables,
	}
}

func (w *SyncWorker) Start(ctx context.Context) {
	w.loadCursorsFromDB()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("sync worker started (interval=%s, batch=%d)", w.interval, w.batchSize)

	for {
		select {
		case <-ctx.Done():
			log.Println("sync worker stopped")
			return
		case <-ticker.C:
			w.syncAll(ctx)
		}
	}
}

func (w *SyncWorker) loadCursorsFromDB() {
	var states []syncState
	if err := w.db.Find(&states).Error; err != nil {
		log.Printf("sync: failed to load cursors from DB: %v", err)
		return
	}
	for _, s := range states {
		if cursor, ok := w.cursors[s.Table]; ok {
			cursor.LastSyncAt = s.LastSyncAt
			cursor.LastSyncID = s.LastSyncID
			log.Printf("sync: loaded cursor for %s: (%s, %d)", s.Table, s.LastSyncAt.Format(time.RFC3339), s.LastSyncID)
		}
	}
}

func (w *SyncWorker) saveCursorToDB(table string, cursor *SyncCursor) {
	state := syncState{
		Table:      table,
		LastSyncAt: cursor.LastSyncAt,
		LastSyncID: cursor.LastSyncID,
	}
	if err := w.db.Save(&state).Error; err != nil {
		log.Printf("sync: failed to save cursor for %s: %v", table, err)
	}
}

func (w *SyncWorker) syncAll(ctx context.Context) {
	for _, table := range w.tables {
		w.syncTable(ctx, table)
	}
}

func (w *SyncWorker) syncTable(ctx context.Context, table syncTable) {
	cursor := w.cursors[table.Name]

	for {
		var rows []map[string]interface{}
		query := fmt.Sprintf(
			"SELECT %s FROM %s WHERE (updated_at > ? OR (updated_at = ? AND id > ?)) ORDER BY updated_at ASC, id ASC LIMIT ?",
			table.SelectColumns, table.Name,
		)

		result := w.db.WithContext(ctx).Raw(query, cursor.LastSyncAt, cursor.LastSyncAt, cursor.LastSyncID, cursor.BatchSize).Scan(&rows)
		if result.Error != nil {
			log.Printf("sync: query %s failed: %v", table.Name, result.Error)
			return
		}

		if len(rows) == 0 {
			return
		}

		var toUpsert []map[string]interface{}
		var toDelete []uint

		for _, row := range rows {
			id := toUint(row["id"])
			if isDeleted(row) || !isPublished(row, table.StatusFilter) {
				toDelete = append(toDelete, id)
			} else {
				toUpsert = append(toUpsert, row)
			}
		}

		if len(toUpsert) > 0 {
			if err := w.indexer.Upsert(ctx, table.Entity, toUpsert); err != nil {
				log.Printf("sync: upsert %s failed: %v", table.Name, err)
				return
			}
		}

		if len(toDelete) > 0 {
			if err := w.indexer.Delete(ctx, table.Entity, toDelete); err != nil {
				log.Printf("sync: delete %s failed: %v", table.Name, err)
				return
			}
		}

		lastRow := rows[len(rows)-1]
		cursor.Advance(toTime(lastRow["updated_at"]), toUint(lastRow["id"]))
		w.saveCursorToDB(table.Name, cursor)

		log.Printf("sync: %s batch=%d upsert=%d delete=%d cursor=(%s, %d)",
			table.Name, len(rows), len(toUpsert), len(toDelete),
			cursor.LastSyncAt.Format(time.RFC3339), cursor.LastSyncID)

		if len(rows) < cursor.BatchSize {
			return
		}
	}
}

func toUint(v interface{}) uint {
	switch val := v.(type) {
	case uint:
		return val
	case int64:
		return uint(val)
	case float64:
		return uint(val)
	default:
		return 0
	}
}

func toTime(v interface{}) time.Time {
	switch val := v.(type) {
	case time.Time:
		return val
	default:
		return time.Time{}
	}
}

func isDeleted(row map[string]interface{}) bool {
	if v, ok := row["deleted_at"]; ok && v != nil {
		return true
	}
	return false
}

// isPublished checks if a row should be indexed based on its status fields.
func isPublished(row map[string]interface{}, statusFilter string) bool {
	if statusFilter == "" {
		if v, ok := row["end_date"]; ok && v != nil {
			if t, ok := v.(time.Time); ok && !t.IsZero() && t.Before(time.Now()) {
				return false
			}
		}
		if v, ok := row["status"]; ok {
			if s, ok := v.(string); ok {
				if s == "past" || s == "completed" || s == "cancelled" {
					return false
				}
			}
		}
		return true
	}

	// Boolean published field (news, blogs) — must exist and be true
	if strings.Contains(statusFilter, "published") {
		v, ok := row["published"]
		if !ok || v == nil {
			return false
		}
		if b, ok := v.(bool); ok {
			return b
		}
		return false
	}

	// Check scholarship deadline — skip if deadline has passed
	if v, ok := row["deadline"]; ok && v != nil {
		if t, ok := v.(time.Time); ok && !t.IsZero() && t.Before(time.Now()) {
			return false
		}
	}

	// String status field — must exist and not be draft/pending
	if v, ok := row["status"]; ok {
		if s, ok := v.(string); ok {
			switch s {
			case "draft", "pending", "":
				return false
			}
		}
	} else if strings.Contains(statusFilter, "status") {
		return false
	}

	// Institution profile_status — must exist and be published
	if v, ok := row["profile_status"]; ok {
		if s, ok := v.(string); ok {
			if s == "draft" || s == "" {
				return false
			}
		}
	} else if strings.Contains(statusFilter, "profile_status") {
		return false
	}

	return true
}

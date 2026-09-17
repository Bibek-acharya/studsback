package analytics

import (
	"time"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type bucketRow struct {
	Bucket string
	Key    string
	Count  int64
}

// bucketCounts groups created_at into day/week buckets with a string key column.
func (r *Repository) bucketCounts(table, keyCol string, from, to time.Time, gran, extraWhere string, args ...any) ([]bucketRow, error) {
	trunc := "day"
	if gran == "week" {
		trunc = "week"
	}
	q := `SELECT date_trunc('` + trunc + `', created_at)::date::text AS bucket, ` + keyCol + ` AS key, COUNT(*) AS count FROM ` + table + ` WHERE created_at >= ? AND created_at < ?`
	if extraWhere != "" {
		q += " AND " + extraWhere
	}
	q += ` GROUP BY 1, 2 ORDER BY 1`
	allArgs := append([]any{from, to}, args...)
	var rows []bucketRow
	if err := r.db.Raw(q, allArgs...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pivot(rows []bucketRow) []SeriesPoint {
	byBucket := map[string]map[string]int64{}
	var order []string
	for _, row := range rows {
		if _, seen := byBucket[row.Bucket]; !seen {
			byBucket[row.Bucket] = map[string]int64{}
			order = append(order, row.Bucket)
		}
		byBucket[row.Bucket][row.Key] += row.Count
	}
	out := make([]SeriesPoint, 0, len(order))
	for _, b := range order {
		out = append(out, SeriesPoint{Bucket: b, Values: byBucket[b]})
	}
	return out
}

func mergeSeries(lists ...[]SeriesPoint) []SeriesPoint {
	byBucket := map[string]map[string]int64{}
	var order []string
	for _, list := range lists {
		for _, p := range list {
			if _, seen := byBucket[p.Bucket]; !seen {
				byBucket[p.Bucket] = map[string]int64{}
				order = append(order, p.Bucket)
			}
			for k, v := range p.Values {
				byBucket[p.Bucket][k] += v
			}
		}
	}
	out := make([]SeriesPoint, 0, len(order))
	for _, b := range order {
		out = append(out, SeriesPoint{Bucket: b, Values: byBucket[b]})
	}
	return out
}

func (r *Repository) countWhere(table, where string, args ...any) (int64, error) {
	var n int64
	if err := r.db.Table(table).Where(where, args...).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *Repository) RecordPageVisit(v *PageVisit) error {
	return r.db.Create(v).Error
}

type pageRankRow struct {
	Path   string
	Visits int64
}

func (r *Repository) topPageVisits(from, to time.Time, limit int) ([]pageRankRow, error) {
	var rows []pageRankRow
	if err := r.db.Model(&PageVisit{}).
		Where("created_at >= ? AND created_at < ?", from, to).
		Select("path, COUNT(*) AS visits").
		Group("path").
		Order("visits DESC, path ASC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

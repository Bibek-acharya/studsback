package analytics

type SeriesPoint struct {
	Bucket string           `json:"bucket"`
	Values map[string]int64 `json:"values"`
}

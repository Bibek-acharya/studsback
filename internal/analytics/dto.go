package analytics

type SeriesPoint struct {
	Bucket string           `json:"bucket"`
	Values map[string]int64 `json:"values"`
}

type UserTotals struct {
	Students            int64   `json:"students"`
	Institutions        int64   `json:"institutions"`
	Providers           int64   `json:"providers"`
	PendingInstitutions int64   `json:"pending_institutions"`
	PendingProviders    int64   `json:"pending_providers"`
	Active7d            int64   `json:"active_7d"`
	ActivationPct       float64 `json:"activation_pct"`
}

type UsersAnalytics struct {
	Totals          UserTotals       `json:"totals"`
	StatusBreakdown map[string]int64 `json:"user_status_breakdown"`
	Series          []SeriesPoint    `json:"series"`
}

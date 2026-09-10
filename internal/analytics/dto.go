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

type FunnelTotals struct {
	Admissions              int64 `json:"admissions"`
	ScholarshipApplications int64 `json:"scholarship_applications"`
	Bookings                int64 `json:"bookings"`
}

type FunnelAnalytics struct {
	Totals        FunnelTotals     `json:"totals"`
	ConversionPct float64          `json:"admission_conversion_pct"`
	ByStatus      map[string]int64 `json:"admissions_by_status"`
	Series        []SeriesPoint    `json:"series"`
}

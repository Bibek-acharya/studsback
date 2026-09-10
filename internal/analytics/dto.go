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

type AgeBuckets struct {
	Lt24h int64 `json:"lt_24h"`
	D1_3  int64 `json:"d1_3"`
	Gt3d  int64 `json:"gt_3d"`
}

type ApprovalAging struct {
	Institutions AgeBuckets `json:"institutions"`
	Providers    AgeBuckets `json:"providers"`
}

type RankedItem struct {
	Kind  string `json:"kind"`
	ID    int64  `json:"id"`
	Count int64  `json:"count"`
}

type StaleScholarship struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Deadline string `json:"deadline"`
}

type SupplyTotals struct {
	Colleges              int64 `json:"colleges"`
	ScholarshipsPublished int64 `json:"scholarships_published"`
	Events                int64 `json:"events"`
	Blogs                 int64 `json:"blogs"`
	News                  int64 `json:"news"`
}

type SupplyAnalytics struct {
	Totals            SupplyTotals       `json:"totals"`
	ApprovalAging     ApprovalAging      `json:"approval_aging"`
	TopBookmarked     []RankedItem       `json:"top_bookmarked"`
	TopFollowed       []RankedItem       `json:"top_followed"`
	StaleScholarships []StaleScholarship `json:"stale_scholarships"`
	Series            []SeriesPoint      `json:"series"`
}

type OpsTotals struct {
	ForumReports     int64 `json:"forum_reports"`
	ReviewReports    int64 `json:"review_reports"`
	Feedback         int64 `json:"feedback"`
	Broadcasts       int64 `json:"broadcasts"`
	BroadcastsFailed int64 `json:"broadcasts_failed"`
}

type BroadcastRow struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	Audience  string `json:"audience"`
	CreatedAt string `json:"created_at"`
}

type OpsAnalytics struct {
	Totals            OpsTotals        `json:"totals"`
	InquiriesByStatus map[string]int64 `json:"inquiries_by_status"`
	RecentBroadcasts  []BroadcastRow   `json:"recent_broadcasts"`
	Series            []SeriesPoint    `json:"series"`
}

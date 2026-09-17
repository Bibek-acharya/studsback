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

type PageVisitTotals struct {
	TotalVisits int64 `json:"total_visits"`
	UniquePaths int64 `json:"unique_paths"`
}

type TopPage struct {
	Path   string `json:"path"`
	Visits int64  `json:"visits"`
}

type PageVisitAnalytics struct {
	Totals   PageVisitTotals `json:"totals"`
	Series   []SeriesPoint   `json:"series"`
	TopPages []TopPage       `json:"top_pages"`
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

type ProcessHealth struct {
	UptimeSeconds  int64  `json:"uptime_seconds"`
	Goroutines     int    `json:"goroutines"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes   uint64 `json:"heap_sys_bytes"`
}

type DatabaseHealth struct {
	PoolOpen      int   `json:"pool_open"`
	PoolInUse     int   `json:"pool_in_use"`
	PoolIdle      int   `json:"pool_idle"`
	PoolWaitCount int64 `json:"pool_wait_count"`
	SizeBytes     int64 `json:"size_bytes"`
}

type EmailQueueHealth struct {
	Available bool `json:"available"`
	Pending   int  `json:"pending"`
	Active    int  `json:"active"`
	Failed    int  `json:"failed"`
}

type QueueHealth struct {
	Email         EmailQueueHealth `json:"email"`
	OutboxPending int64            `json:"outbox_pending"`
}

type Health struct {
	Process  ProcessHealth  `json:"process"`
	Database DatabaseHealth `json:"database"`
	Queues   QueueHealth    `json:"queues"`
	API      UsageSnapshot  `json:"api"`
}

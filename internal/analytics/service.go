package analytics

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type Service struct {
	repo  *Repository
	usage *UsageTracker
}

func NewService(db *gorm.DB, usage *UsageTracker) *Service {
	return &Service{repo: NewRepository(db), usage: usage}
}

func (s *Service) Users(from, to time.Time, gran string) (UsersAnalytics, error) {
	var out UsersAnalytics
	var err error
	r := s.repo
	if out.Totals.Students, err = r.countWhere("users", "role = 'student' AND deleted_at IS NULL"); err != nil {
		return out, err
	}
	if out.Totals.Institutions, err = r.countWhere("institution_users", "status = 'approved' AND deleted_at IS NULL"); err != nil {
		return out, err
	}
	if out.Totals.Providers, err = r.countWhere("scholarship_provider_users", "status = 'approved' AND deleted_at IS NULL"); err != nil {
		return out, err
	}
	if out.Totals.PendingInstitutions, err = r.countWhere("institution_users", "status = 'pending' AND deleted_at IS NULL"); err != nil {
		return out, err
	}
	if out.Totals.PendingProviders, err = r.countWhere("scholarship_provider_users", "status = 'pending' AND deleted_at IS NULL"); err != nil {
		return out, err
	}
	var active int64
	if err = r.db.Table("user_sessions").Where("last_active_at >= ?", time.Now().UTC().Add(-7*24*time.Hour)).Distinct("user_id").Count(&active).Error; err != nil {
		return out, err
	}
	out.Totals.Active7d = active
	// Activation: students created in window with an education entry within 7 days.
	var created, activated int64
	if err = r.db.Table("users").Where("role = 'student' AND deleted_at IS NULL AND created_at >= ? AND created_at < ?", from, to).Count(&created).Error; err != nil {
		return out, err
	}
	if err = r.db.Table("users u").Joins("JOIN education_entries e ON e.user_id = u.id AND e.created_at <= u.created_at + interval '7 days'").Where("u.role = 'student' AND u.deleted_at IS NULL AND u.created_at >= ? AND u.created_at < ?", from, to).Distinct("u.id").Count(&activated).Error; err != nil {
		return out, err
	}
	if created > 0 {
		out.Totals.ActivationPct = float64(activated) * 100.0 / float64(created)
	}
	breakdown := map[string]int64{}
	type kv struct {
		Status string
		Count  int64
	}
	var kvs []kv
	if err = r.db.Table("users").Select("status, COUNT(*) AS count").Where("deleted_at IS NULL").Group("status").Scan(&kvs).Error; err != nil {
		return out, err
	}
	for _, kv := range kvs {
		breakdown[kv.Status] = kv.Count
	}
	out.StatusBreakdown = breakdown
	students, err := r.bucketCounts("users", "'students'", from, to, gran, "role = 'student' AND deleted_at IS NULL")
	if err != nil {
		return out, err
	}
	insts, err := r.bucketCounts("institution_users", "'institutions'", from, to, gran, "deleted_at IS NULL")
	if err != nil {
		return out, err
	}
	provs, err := r.bucketCounts("scholarship_provider_users", "'providers'", from, to, gran, "deleted_at IS NULL")
	if err != nil {
		return out, err
	}
	out.Series = mergeSeries(pivot(students), pivot(insts), pivot(provs))
	if out.Series == nil {
		out.Series = []SeriesPoint{}
	}
	return out, nil
}

// positiveStatuses matches terminal-positive funnel states case-insensitively.
// Deliberate heuristic: adjust here if the domain adds new terminal states.
var positiveStatuses = map[string]bool{
	"approved": true, "accepted": true, "admitted": true,
	"confirmed": true, "completed": true,
}

func isPositive(status string) bool {
	return positiveStatuses[strings.ToLower(strings.TrimSpace(status))]
}

func (s *Service) Funnel(from, to time.Time, gran string) (FunnelAnalytics, error) {
	var out FunnelAnalytics
	admissions, err := s.repo.bucketCounts("admissions", "status", from, to, gran, "")
	if err != nil {
		return out, err
	}
	schol, err := s.repo.bucketCounts("scholarship_applications", "status", from, to, gran, "")
	if err != nil {
		return out, err
	}
	bookings, err := s.repo.bucketCounts("counselling_bookings", "status", from, to, gran, "")
	if err != nil {
		return out, err
	}
	var total, positive int64
	byStatus := map[string]int64{}
	for _, row := range admissions {
		out.Totals.Admissions += row.Count
		byStatus[row.Key] += row.Count
		total += row.Count
		if isPositive(row.Key) {
			positive += row.Count
		}
	}
	for _, row := range schol {
		out.Totals.ScholarshipApplications += row.Count
	}
	for _, row := range bookings {
		out.Totals.Bookings += row.Count
	}
	if total > 0 {
		out.ConversionPct = float64(positive) * 100.0 / float64(total)
	}
	out.ByStatus = byStatus
	// Prefix keys per domain so series values stay unambiguous in one chart.
	out.Series = mergeSeries(pivot(prefixed(admissions, "admissions:")),
		pivot(prefixed(schol, "scholarship:")), pivot(prefixed(bookings, "booking:")))
	if out.Series == nil {
		out.Series = []SeriesPoint{}
	}
	return out, nil
}

func prefixed(rows []bucketRow, prefix string) []bucketRow {
	out := make([]bucketRow, len(rows))
	for i, row := range rows {
		row.Key = prefix + row.Key
		out[i] = row
	}
	return out
}

func (s *Service) Supply(from, to time.Time, gran string) (SupplyAnalytics, error) {
	var out SupplyAnalytics
	var err error
	r := s.repo
	if out.Totals.Colleges, err = r.countWhere("colleges", ""); err != nil {
		return out, err
	}
	if out.Totals.ScholarshipsPublished, err = r.countWhere("scholarships", "status = 'published'"); err != nil {
		return out, err
	}
	if out.Totals.Events, err = r.countWhere("events", ""); err != nil {
		return out, err
	}
	if out.Totals.Blogs, err = r.countWhere("blogs", ""); err != nil {
		return out, err
	}
	if out.Totals.News, err = r.countWhere("news", ""); err != nil {
		return out, err
	}
	if out.ApprovalAging.Institutions, err = s.ageBuckets("institution_users"); err != nil {
		return out, err
	}
	if out.ApprovalAging.Providers, err = s.ageBuckets("scholarship_provider_users"); err != nil {
		return out, err
	}
	type ranked struct {
		Kind  string
		ID    int64
		Count int64
	}
	var bm []ranked
	if err = r.db.Table("bookmarks").Select("item_type AS kind, item_id AS id, COUNT(*) AS count").Group("item_type, item_id").Order("count DESC").Limit(10).Scan(&bm).Error; err != nil {
		return out, err
	}
	for _, b := range bm {
		out.TopBookmarked = append(out.TopBookmarked, RankedItem{Kind: b.Kind, ID: b.ID, Count: b.Count})
	}
	var fw []ranked
	if err = r.db.Table("user_follows").Select("target_type AS kind, target_id AS id, COUNT(*) AS count").Group("target_type, target_id").Order("count DESC").Limit(10).Scan(&fw).Error; err != nil {
		return out, err
	}
	for _, f := range fw {
		out.TopFollowed = append(out.TopFollowed, RankedItem{Kind: f.Kind, ID: f.ID, Count: f.Count})
	}
	type stale struct {
		ID       int64
		Title    string
		Deadline time.Time
	}
	var stales []stale
	if err = r.db.Table("scholarships").Select("id, title, deadline").Where("status = 'published' AND deadline < ?", time.Now().UTC()).Order("deadline DESC").Limit(20).Scan(&stales).Error; err != nil {
		return out, err
	}
	for _, s := range stales {
		out.StaleScholarships = append(out.StaleScholarships, StaleScholarship{ID: s.ID, Title: s.Title, Deadline: s.Deadline.UTC().Format(time.RFC3339)})
	}
	for _, tbl := range []struct {
		table, key string
		where      string
	}{
		{"colleges", "colleges", ""},
		{"scholarships", "scholarships", "status = 'published'"},
		{"events", "events", ""},
		{"blogs", "blogs", ""},
		{"news", "news", ""},
	} {
		rows, err := r.bucketCounts(tbl.table, "'"+tbl.key+"'", from, to, gran, tbl.where)
		if err != nil {
			return out, err
		}
		out.Series = mergeSeries(out.Series, pivot(rows))
	}
	if out.TopBookmarked == nil {
		out.TopBookmarked = []RankedItem{}
	}
	if out.TopFollowed == nil {
		out.TopFollowed = []RankedItem{}
	}
	if out.StaleScholarships == nil {
		out.StaleScholarships = []StaleScholarship{}
	}
	if out.Series == nil {
		out.Series = []SeriesPoint{}
	}
	return out, nil
}

func (s *Service) Ops(from, to time.Time, gran string) (OpsAnalytics, error) {
	var out OpsAnalytics
	var err error
	r := s.repo
	if out.Totals.ForumReports, err = r.countWhere("forum_reports", ""); err != nil {
		return out, err
	}
	if out.Totals.ReviewReports, err = r.countWhere("review_reports", ""); err != nil {
		return out, err
	}
	if out.Totals.Feedback, err = r.countWhere("feedback", ""); err != nil {
		return out, err
	}
	if out.Totals.Broadcasts, err = r.countWhere("notification_broadcasts", ""); err != nil {
		return out, err
	}
	if out.Totals.BroadcastsFailed, err = r.countWhere("notification_broadcasts", "status = 'failed'"); err != nil {
		return out, err
	}
	out.InquiriesByStatus = map[string]int64{}
	type kv struct {
		Status string
		Count  int64
	}
	var kvs []kv
	if err = r.db.Table("contact_inquiries").Select("status, COUNT(*) AS count").Group("status").Scan(&kvs).Error; err != nil {
		return out, err
	}
	for _, kv := range kvs {
		out.InquiriesByStatus[kv.Status] = kv.Count
	}
	type bc struct {
		ID        int64
		Status    string
		Audience  string
		CreatedAt time.Time
	}
	var bcs []bc
	if err = r.db.Table("notification_broadcasts").Select("id, status, audience, created_at").Order("created_at DESC").Limit(10).Scan(&bcs).Error; err != nil {
		return out, err
	}
	for _, b := range bcs {
		out.RecentBroadcasts = append(out.RecentBroadcasts, BroadcastRow{ID: b.ID, Status: b.Status, Audience: b.Audience, CreatedAt: b.CreatedAt.UTC().Format(time.RFC3339)})
	}
	for _, tbl := range []struct{ table, key string }{
		{"forum_reports", "forum_reports"},
		{"review_reports", "review_reports"},
		{"contact_inquiries", "inquiries"},
		{"feedback", "feedback"},
		{"account_notifications", "notifications"},
	} {
		rows, err := r.bucketCounts(tbl.table, "'"+tbl.key+"'", from, to, gran, "")
		if err != nil {
			return out, err
		}
		out.Series = mergeSeries(out.Series, pivot(rows))
	}
	notifByCat, err := r.bucketCounts("account_notifications", "category", from, to, gran, "")
	if err != nil {
		return out, err
	}
	out.Series = mergeSeries(out.Series, pivot(prefixed(notifByCat, "notif:")))
	if out.RecentBroadcasts == nil {
		out.RecentBroadcasts = []BroadcastRow{}
	}
	if out.Series == nil {
		out.Series = []SeriesPoint{}
	}
	return out, nil
}

func (s *Service) ageBuckets(table string) (AgeBuckets, error) {
	var out AgeBuckets
	now := time.Now().UTC()
	count := func(where string, args ...any) (int64, error) {
		var n int64
		if err := s.repo.db.Table(table).Where(where, args...).Count(&n).Error; err != nil {
			return 0, err
		}
		return n, nil
	}
	var err error
	pending := "status = 'pending' AND deleted_at IS NULL"
	if out.Lt24h, err = count(pending+" AND created_at >= ?", now.Add(-24*time.Hour)); err != nil {
		return out, err
	}
	if out.D1_3, err = count(pending+" AND created_at >= ? AND created_at < ?", now.Add(-72*time.Hour), now.Add(-24*time.Hour)); err != nil {
		return out, err
	}
	if out.Gt3d, err = count(pending+" AND created_at < ?", now.Add(-72*time.Hour)); err != nil {
		return out, err
	}
	return out, nil
}

package analytics

import (
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

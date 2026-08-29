package ranking

import (
	"sort"

	"studsphere/backend/internal/search/retrieval"
)

type EntityBoost struct {
	FreshnessBoost float64
}

type BusinessBoostConfig struct {
	College     EntityBoost
	Course      EntityBoost
	Scholarship EntityBoost
	News        EntityBoost
	Event       EntityBoost
	Exam        EntityBoost
	Blog        EntityBoost
	SitePage    EntityBoost
}

type BusinessBooster struct {
	config BusinessBoostConfig
}

func NewBusinessBooster(cfg BusinessBoostConfig) *BusinessBooster {
	return &BusinessBooster{config: cfg}
}

func (b *BusinessBooster) Boost(candidates []retrieval.Candidate) []retrieval.Candidate {
	for i := range candidates {
		boost := b.getBoost(candidates[i].Type)
		candidates[i].Score += boost
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates
}

func (b *BusinessBooster) getBoost(entity retrieval.EntityType) float64 {
	boosts := map[retrieval.EntityType]float64{
		retrieval.EntityCollege:     b.config.College.FreshnessBoost,
		retrieval.EntityCourse:      b.config.Course.FreshnessBoost,
		retrieval.EntityScholarship: b.config.Scholarship.FreshnessBoost,
		retrieval.EntityNews:        b.config.News.FreshnessBoost,
		retrieval.EntityEvent:       b.config.Event.FreshnessBoost,
		retrieval.EntityExam:        b.config.Exam.FreshnessBoost,
		retrieval.EntityBlog:        b.config.Blog.FreshnessBoost,
		retrieval.EntitySitePage:    b.config.SitePage.FreshnessBoost,
	}
	return boosts[entity]
}

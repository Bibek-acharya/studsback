package analytics

import "gorm.io/gorm"

type Service struct {
	repo  *Repository
	usage *UsageTracker
}

func NewService(db *gorm.DB, usage *UsageTracker) *Service {
	return &Service{repo: NewRepository(db), usage: usage}
}

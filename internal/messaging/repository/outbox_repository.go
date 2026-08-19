package repository

import (
	"studsphere/backend/internal/messaging/domain"

	"gorm.io/gorm"
)

type OutboxRepository interface {
	Create(event *domain.OutboxEvent) error
	GetUnpublished(limit int) ([]domain.OutboxEvent, error)
	MarkPublished(id uint) error
	IncrementRetry(id uint) error
}

type outboxRepo struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepo{db: db}
}

func (r *outboxRepo) Create(event *domain.OutboxEvent) error {
	return r.db.Create(event).Error
}

func (r *outboxRepo) GetUnpublished(limit int) ([]domain.OutboxEvent, error) {
	var events []domain.OutboxEvent
	err := r.db.Where("published = false AND retry_count < max_retries").
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (r *outboxRepo) MarkPublished(id uint) error {
	return r.db.Model(&domain.OutboxEvent{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"published":    true,
			"published_at": gorm.Expr("NOW()"),
		}).Error
}

func (r *outboxRepo) IncrementRetry(id uint) error {
	return r.db.Model(&domain.OutboxEvent{}).Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

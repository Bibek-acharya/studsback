package downloadcenter

import (
	"errors"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func normalizePageLimit(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func (s *Service) GetItems(filters ItemFilters, page, limit int) ([]DownloadItem, int64, error) {
	page, limit = normalizePageLimit(page, limit)
	items, total, err := s.repo.FindAll(filters, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []DownloadItem{}
	}
	return items, total, nil
}

func (s *Service) GetItem(id uint) (*DownloadItem, error) {
	return s.repo.FindItemByID(id)
}

func (s *Service) CreateItem(item *DownloadItem) error {
	return s.repo.CreateItem(item)
}

func (s *Service) UpdateItem(id uint, input UpdateDownloadItemInput) (*DownloadItem, error) {
	item, err := s.repo.FindItemByID(id)
	if err != nil {
		return nil, errors.New("download item not found")
	}

	if input.Title != nil {
		item.Title = *input.Title
	}
	if input.Description != nil {
		item.Description = *input.Description
	}
	if input.Category != nil {
		item.Category = *input.Category
	}
	if input.IsPublished != nil {
		item.IsPublished = *input.IsPublished
	}
	if input.PublishedAt != nil {
		item.PublishedAt = input.PublishedAt
	}

	if err := s.repo.UpdateItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateItemModel(item *DownloadItem) error {
	return s.repo.UpdateItem(item)
}

func (s *Service) DeleteItem(id uint) error {
	if _, err := s.repo.FindItemByID(id); err != nil {
		return errors.New("download item not found")
	}
	return s.repo.DeleteItem(id)
}

func (s *Service) IncrementDownloads(id uint) {
	// Download counting is best-effort: a failed increment must not block the
	// file download.
	_ = s.repo.IncrementDownloads(id)
}

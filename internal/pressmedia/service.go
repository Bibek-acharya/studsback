package pressmedia

import (
	"errors"
	"fmt"

	"studsphere/backend/internal/shared/slug"
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

var validCategories = map[string]bool{
	"press_release":  true,
	"news":           true,
	"media_coverage": true,
}

func isValidCategory(category string) bool {
	return validCategories[category]
}

// uniqueSlug returns the first available slug derived from base, appending
// -2, -3, ... on conflict. excludeID lets updates skip the item's own row.
func (s *Service) uniqueSlug(base string, excludeID uint) (string, error) {
	candidate := base
	for i := 2; ; i++ {
		exists, err := s.repo.SlugExists(candidate, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

func (s *Service) GetItems(filters ItemFilters, page, limit int) ([]PressMediaItem, int64, error) {
	page, limit = normalizePageLimit(page, limit)
	filters.Category = sanitizeCategory(filters.Category)
	items, total, err := s.repo.FindAll(filters, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []PressMediaItem{}
	}
	return items, total, nil
}

func (s *Service) GetItem(id uint) (*PressMediaItem, error) {
	return s.repo.FindItemByID(id)
}

func (s *Service) CreateItem(input CreatePressMediaItemInput) (*PressMediaItem, error) {
	input.Category = sanitizeCategory(input.Category)
	if !isValidCategory(input.Category) {
		return nil, errors.New("category must be one of: press_release, news, media_coverage")
	}

	base := input.Slug
	if base == "" {
		base = input.Title
	}
	slugValue, err := s.uniqueSlug(slug.Generate(base), 0)
	if err != nil {
		return nil, err
	}

	item := &PressMediaItem{
		Title:       input.Title,
		Slug:        slugValue,
		Category:    input.Category,
		Summary:     input.Summary,
		Content:     input.Content,
		ImageURL:    input.ImageURL,
		FileURL:     input.FileURL,
		ExternalURL: input.ExternalURL,
		PublishedAt: input.PublishedAt,
		IsPublished: input.IsPublished,
	}
	if err := s.repo.CreateItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateItem(id uint, input UpdatePressMediaItemInput) (*PressMediaItem, error) {
	item, err := s.repo.FindItemByID(id)
	if err != nil {
		return nil, errors.New("press media item not found")
	}

	if input.Title != nil {
		item.Title = *input.Title
	}
	if input.Slug != nil {
		slugValue := slug.Generate(*input.Slug)
		unique, err := s.uniqueSlug(slugValue, item.ID)
		if err != nil {
			return nil, err
		}
		item.Slug = unique
	}
	if input.Category != nil {
		category := sanitizeCategory(*input.Category)
		if !isValidCategory(category) {
			return nil, errors.New("category must be one of: press_release, news, media_coverage")
		}
		item.Category = category
	}
	if input.Summary != nil {
		item.Summary = *input.Summary
	}
	if input.Content != nil {
		item.Content = *input.Content
	}
	if input.ImageURL != nil {
		item.ImageURL = *input.ImageURL
	}
	if input.FileURL != nil {
		item.FileURL = *input.FileURL
	}
	if input.ExternalURL != nil {
		item.ExternalURL = *input.ExternalURL
	}
	if input.PublishedAt != nil {
		item.PublishedAt = input.PublishedAt
	}
	if input.IsPublished != nil {
		item.IsPublished = *input.IsPublished
	}

	if err := s.repo.UpdateItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateItemModel(item *PressMediaItem) error {
	return s.repo.UpdateItem(item)
}

func (s *Service) DeleteItem(id uint) error {
	if _, err := s.repo.FindItemByID(id); err != nil {
		return errors.New("press media item not found")
	}
	return s.repo.DeleteItem(id)
}

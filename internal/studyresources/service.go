package studyresources

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

func (s *Service) GetResources(filters ResourceFilters, page, limit int) ([]StudyResource, int64, error) {
	page, limit = normalizePageLimit(page, limit)
	resources, total, err := s.repo.FindAll(filters, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if resources == nil {
		resources = []StudyResource{}
	}
	return resources, total, nil
}

func (s *Service) GetResource(id uint) (*StudyResource, error) {
	return s.repo.FindResourceByID(id)
}

func (s *Service) CreateResource(resource *StudyResource) error {
	return s.repo.CreateResource(resource)
}

func (s *Service) UpdateResource(id uint, req UpdateResourceRequest) (*StudyResource, error) {
	resource, err := s.repo.FindResourceByID(id)
	if err != nil {
		return nil, errors.New("resource not found")
	}
	if req.Title != nil {
		resource.Title = *req.Title
	}
	if req.ResourceType != nil {
		resource.ResourceType = *req.ResourceType
	}
	if req.Course != nil {
		resource.Course = *req.Course
	}
	if req.Year != nil {
		resource.Year = *req.Year
	}
	if req.Description != nil {
		resource.Description = *req.Description
	}
	if err := s.repo.UpdateResource(resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *Service) DeleteResource(id uint) error {
	if _, err := s.repo.FindResourceByID(id); err != nil {
		return errors.New("resource not found")
	}
	return s.repo.DeleteResource(id)
}

func (s *Service) IncrementDownloads(id uint) {
	// Download counting is best-effort: a failed increment must not block the
	// file download.
	_ = s.repo.IncrementDownloads(id)
}

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

// GetPublishedResource returns a resource only when it is published. It backs
// the new public stream endpoint so drafts stay unreachable.
func (s *Service) GetPublishedResource(id uint) (*StudyResource, error) {
	resource, err := s.repo.FindResourceByID(id)
	if err != nil {
		return nil, err
	}
	if !resource.IsPublished {
		return nil, errors.New("resource not found")
	}
	return resource, nil
}

// ErrNotAVideoResource marks a published resource that is not a video lecture.
var ErrNotAVideoResource = errors.New("resource is not a video lecture")

// GetPlayableVideoResource returns a resource only when it is actually
// playable: published, a video lecture, and backed by a stored object. It
// backs both the playback-token endpoint and the stream gate, so a draft, a
// document or a row without an object can never start a stream.
func (s *Service) GetPlayableVideoResource(id uint) (*StudyResource, error) {
	resource, err := s.repo.FindResourceByID(id)
	if err != nil {
		return nil, errors.New("resource not found")
	}
	if !resource.IsPublished {
		return nil, errors.New("resource not found")
	}
	if !IsVideoType(resource.ResourceType) {
		return nil, ErrNotAVideoResource
	}
	if normalizeObjectKey(resource.FilePath) == "" {
		return nil, errors.New("resource not found")
	}
	return resource, nil
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
		// The caller already normalized and validated the type.
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
	if req.DurationSeconds != nil {
		resource.DurationSeconds = *req.DurationSeconds
	}
	if req.IsPublished != nil {
		resource.IsPublished = *req.IsPublished
	}
	if err := s.repo.UpdateResource(resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *Service) UpdateResourceModel(resource *StudyResource) error {
	return s.repo.UpdateResource(resource)
}

// DistinctFacets returns the distinct non-empty, normalized years and course
// names across the PUBLISHED study resources. Years are sorted descending;
// courses alphabetically. These power the list-page filter facets, so drafts
// must not leak through them.
func (s *Service) DistinctFacets() ([]string, []string, error) {
	years, err := s.repo.DistinctPublishedValues("year", "year DESC")
	if err != nil {
		return nil, nil, err
	}
	courses, err := s.repo.DistinctPublishedValues("course", "course ASC")
	if err != nil {
		return nil, nil, err
	}
	if years == nil {
		years = []string{}
	}
	if courses == nil {
		courses = []string{}
	}
	return years, courses, nil
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

// IncrementViews counts a video-lecture stream request. Best-effort, exactly
// like IncrementDownloads: analytics must never break playback.
func (s *Service) IncrementViews(id uint) {
	_ = s.repo.IncrementViews(id)
}

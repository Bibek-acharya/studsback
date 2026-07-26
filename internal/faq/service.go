package faq

import "errors"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetCategories() ([]FAQCategory, error) {
	return s.repo.FindAllCategories()
}

func (s *Service) GetCategory(id uint) (*FAQCategory, error) {
	return s.repo.FindCategoryByID(id)
}

func (s *Service) CreateCategory(req CreateCategoryRequest) (*FAQCategory, error) {
	cat := &FAQCategory{
		Name:        req.Name,
		Description: req.Description,
	}
	if err := s.repo.CreateCategory(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *Service) UpdateCategory(id uint, req UpdateCategoryRequest) (*FAQCategory, error) {
	cat, err := s.repo.FindCategoryByID(id)
	if err != nil {
		return nil, errors.New("category not found")
	}
	if req.Name != nil {
		cat.Name = *req.Name
	}
	if req.Description != nil {
		cat.Description = *req.Description
	}
	if req.Order != nil {
		cat.Order = *req.Order
	}
	if err := s.repo.UpdateCategory(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *Service) DeleteCategory(id uint) error {
	return s.repo.DeleteCategory(id)
}

func (s *Service) CreateItem(req CreateItemRequest) (*FAQItem, error) {
	item := &FAQItem{
		CategoryID: req.CategoryID,
		Question:   req.Question,
		Answer:     req.Answer,
	}
	if err := s.repo.CreateItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateItem(id uint, req UpdateItemRequest) (*FAQItem, error) {
	item, err := s.repo.FindItemByID(id)
	if err != nil {
		return nil, errors.New("item not found")
	}
	if req.Question != nil {
		item.Question = *req.Question
	}
	if req.Answer != nil {
		item.Answer = *req.Answer
	}
	if req.Order != nil {
		item.Order = *req.Order
	}
	if err := s.repo.UpdateItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteItem(id uint) error {
	return s.repo.DeleteItem(id)
}

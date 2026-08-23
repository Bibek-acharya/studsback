package college

import "errors"

func (s *Service) CompareColleges(college1ID, college2ID uint) (*CollegeComparisonResponse, error) {
	college1, err := s.repo.FindByID(college1ID)
	if err != nil {
		return nil, errors.New("first college not found")
	}
	college2, err := s.repo.FindByID(college2ID)
	if err != nil {
		return nil, errors.New("second college not found")
	}

	first, err := s.repo.BuildComparisonCollege(*college1)
	if err != nil {
		return nil, errors.New("failed to build first college comparison")
	}
	second, err := s.repo.BuildComparisonCollege(*college2)
	if err != nil {
		return nil, errors.New("failed to build second college comparison")
	}
	return &CollegeComparisonResponse{College1: first, College2: second}, nil
}

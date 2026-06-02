package feedback

import (
	"unicode/utf8"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SubmitFeedback(userID uint, req CreateFeedbackRequest) (*Feedback, error) {
	feedback := &Feedback{
		UserID:      userID,
		Rating:      req.Rating,
		Experience:  req.Experience,
		Designation: req.Designation,
		Email:       req.Email,
	}
	if err := s.repo.Create(feedback); err != nil {
		return nil, err
	}
	return feedback, nil
}

func (s *Service) SubmitTestimonial(req CreateTestimonialRequest) (*Feedback, error) {
	f := &Feedback{
		Rating:      req.Rating,
		Experience:  req.Review,
		Designation: req.Designation,
	}
	if err := s.repo.Create(f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Service) buildResponse(feedbacks []Feedback) ([]FeedbackResponse, error) {
	userIDs := make([]uint, 0, len(feedbacks))
	for _, f := range feedbacks {
		if f.UserID > 0 {
			userIDs = append(userIDs, f.UserID)
		}
	}

	profiles, err := s.repo.GetUserProfiles(userIDs)
	if err != nil {
		return nil, err
	}

	responses := make([]FeedbackResponse, 0, len(feedbacks))
	for _, f := range feedbacks {
		experience := f.Experience
		if utf8.RuneCountInString(experience) > 300 {
			runes := []rune(experience)
			experience = string(runes[:300]) + "..."
		}

		resp := FeedbackResponse{
			ID:         f.ID,
			UserID:     f.UserID,
			Rating:     f.Rating,
			Experience: experience,
			Email:      f.Email,
			CreatedAt:  f.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}

		if f.UserID > 0 {
			if profile, ok := profiles[f.UserID]; ok {
				resp.UserName = profile.FirstName
				resp.ImageURL = profile.ImageURL
			}
			resp.Role = f.Designation
		} else {
			// Public testimonial (user_id = 0) — stored via public endpoint
			resp.Role = f.Designation
		}

		if resp.Role == "" {
			resp.Role = "StudSphere User"
		}
		if resp.UserName == "" {
			resp.UserName = "Anonymous"
		}

		responses = append(responses, resp)
	}
	return responses, nil
}

func (s *Service) ListFeedback() ([]FeedbackResponse, error) {
	feedbacks, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	return s.buildResponse(feedbacks)
}

func (s *Service) DeleteFeedback(id uint) error {
	return s.repo.Delete(id)
}

func (s *Service) ListPublicFeedback(limit int) ([]FeedbackResponse, error) {
	feedbacks, err := s.repo.FindPublic(limit)
	if err != nil {
		return nil, err
	}
	return s.buildResponse(feedbacks)
}

func (s *Service) HasUserSubmitted(userID uint) (bool, error) {
	return s.repo.HasUserSubmitted(userID)
}

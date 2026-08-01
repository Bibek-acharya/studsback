package review

import (
	"encoding/json"
	"errors"
	"math"
	"time"

	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SubmitReview(userID uint, req CreateReviewRequest) (*ReviewResponse, error) {
	ratingsJSON, err := json.Marshal(req.Ratings)
	if err != nil {
		return nil, errors.New("failed to serialize ratings")
	}

	review := &Review{
		UserID:            userID,
		CollegeID:         req.CollegeID,
		CollegeName:       req.CollegeName,
		StudentType:       req.StudentType,
		Course:            req.Course,
		Level:             req.Level,
		BatchYear:         req.BatchYear,
		Ratings:           ratingsJSON,
		Pros:              req.Pros,
		Cons:              req.Cons,
		SummaryTitle:      req.SummaryTitle,
		YearlyFee:         req.YearlyFee,
		Scholarship:       req.Scholarship,
		InternshipOutcome: req.InternshipOutcome,
		Email:             req.Email,
		IsPublished:       true,
	}

	if err := s.repo.Create(review); err != nil {
		return nil, err
	}

	return toReviewResponse(review), nil
}

func (s *Service) GetUserReviews(userID uint, page, limit int) (*PaginatedReviewsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	reviews, total, err := s.repo.FindByUser(userID, page, limit)
	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	responses := make([]ReviewResponse, len(reviews))
	for i, r := range reviews {
		responses[i] = *toReviewResponse(&r)
	}

	return &PaginatedReviewsResponse{
		Reviews: responses,
		Meta: Meta{
			Total:      int(total),
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *Service) GetCollegeReviews(collegeID uint, page, limit int) (*CollegeReviewsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	reviews, total, err := s.repo.FindByCollege(collegeID, page, limit)
	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	responses := make([]ReviewResponse, len(reviews))
	for i, r := range reviews {
		responses[i] = *toReviewResponse(&r)
	}

	var overallRating float64
	categoryAverages := make(map[string]float64)
	categoryCounts := make(map[string]int)

	allReviews, err := s.repo.FindAllByCollege(collegeID)
	if err == nil {
		var totalRating float64
		for _, r := range allReviews {
			ratings := make(map[string]float64)
			if err := json.Unmarshal(r.Ratings, &ratings); err == nil {
				for cat, val := range ratings {
					categoryAverages[cat] += val
					categoryCounts[cat]++
				}
				for _, val := range ratings {
					totalRating += val
				}
			}
		}
		if len(allReviews) > 0 {
			ratingCount := 0
			for _, r := range allReviews {
				ratings := make(map[string]float64)
				if json.Unmarshal(r.Ratings, &ratings) == nil {
					ratingCount += len(ratings)
				}
			}
			if ratingCount > 0 {
				overallRating = math.Round(totalRating/float64(len(allReviews))*10) / 10
			}
		}
		for cat := range categoryAverages {
			if categoryCounts[cat] > 0 {
				categoryAverages[cat] = math.Round(categoryAverages[cat]/float64(categoryCounts[cat])*10) / 10
			}
		}
	}

	return &CollegeReviewsResponse{
		Reviews:          responses,
		OverallRating:    overallRating,
		ReviewCount:      int(total),
		CategoryAverages: categoryAverages,
		Meta: Meta{
			Total:      int(total),
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *Service) UpdateReview(reviewID, userID uint, req UpdateReviewRequest) (*ReviewResponse, error) {
	review, err := s.repo.FindByIDAndUser(reviewID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("review not found")
		}
		return nil, err
	}

	if req.Pros != nil {
		review.Pros = *req.Pros
	}
	if req.Cons != nil {
		review.Cons = *req.Cons
	}
	if req.SummaryTitle != nil {
		review.SummaryTitle = *req.SummaryTitle
	}
	if req.Ratings != nil {
		ratingsJSON, err := json.Marshal(*req.Ratings)
		if err != nil {
			return nil, errors.New("failed to serialize ratings")
		}
		review.Ratings = ratingsJSON
	}

	if err := s.repo.Save(review); err != nil {
		return nil, err
	}

	return toReviewResponse(review), nil
}

func (s *Service) DeleteReview(reviewID, userID uint) error {
	return s.repo.Delete(reviewID, userID)
}

func (s *Service) MarkHelpful(reviewID, userID uint) (int, error) {
	review, err := s.repo.FindByID(reviewID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("review not found")
		}
		return 0, err
	}

	alreadyMarked, err := s.repo.HasMarkedHelpful(reviewID, userID)
	if err != nil {
		return 0, err
	}
	if alreadyMarked {
		return review.HelpfulCount, nil
	}

	if err := s.repo.MarkHelpful(reviewID, userID); err != nil {
		return 0, err
	}

	if err := s.repo.IncrementHelpfulCount(reviewID); err != nil {
		return 0, err
	}

	return review.HelpfulCount + 1, nil
}

func (s *Service) ReportReview(reviewID, userID uint, reason string) error {
	_, err := s.repo.FindByID(reviewID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("review not found")
		}
		return err
	}

	report := &ReviewReport{
		ReviewID: reviewID,
		UserID:   userID,
		Reason:   reason,
	}

	return s.repo.CreateReport(report)
}

func (s *Service) SubmitUniversityReview(userID uint, req CreateUniversityReviewRequest) (*ReviewResponse, error) {
	existing, err := s.repo.FindByUserAndUniversity(userID, req.UniversityID)
	if err == nil && existing != nil {
		return nil, errors.New("you have already reviewed this university")
	}

	ratings := map[string]float64{"overall": req.Rating}
	ratingsJSON, err := json.Marshal(ratings)
	if err != nil {
		return nil, errors.New("failed to serialize ratings")
	}

	review := &Review{
		UserID:       userID,
		UniversityID: req.UniversityID,
		StudentType:  "current",
		Course:       "University",
		Level:        "Bachelor",
		BatchYear:    2024,
		Ratings:      ratingsJSON,
		SummaryTitle: "University Review",
		Pros:         req.Pros,
		Cons:         req.Cons,
		Email:        "",
		IsPublished:  true,
	}

	if err := s.repo.Create(review); err != nil {
		return nil, err
	}

	s.updateUniversityRating(req.UniversityID)

	return toReviewResponse(review), nil
}

func (s *Service) GetUniversityReviews(universityID uint, page, limit int) (*UniversityReviewsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	reviews, total, err := s.repo.FindByUniversity(universityID, page, limit)
	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	responses := make([]ReviewResponse, len(reviews))
	for i, r := range reviews {
		responses[i] = *toReviewResponse(&r)
	}

	var overallRating float64
	distribution := map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0}

	allReviews, err := s.repo.FindAllByUniversity(universityID)
	if err == nil {
		var totalRating float64
		for _, r := range allReviews {
			ratings := make(map[string]float64)
			if err := json.Unmarshal(r.Ratings, &ratings); err == nil {
				if val, ok := ratings["overall"]; ok {
					totalRating += val
					star := int(math.Round(val))
					if star >= 1 && star <= 5 {
						distribution[star]++
					}
				}
			}
		}
		if len(allReviews) > 0 {
			overallRating = math.Round(totalRating/float64(len(allReviews))*10) / 10
		}
	}

	return &UniversityReviewsResponse{
		Reviews:       responses,
		OverallRating: overallRating,
		ReviewCount:   int(total),
		Distribution:  distribution,
		Meta: Meta{
			Total:      int(total),
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *Service) GetUserUniversityReview(userID, universityID uint) (*ReviewResponse, error) {
	review, err := s.repo.FindByUserAndUniversity(userID, universityID)
	if err != nil {
		return nil, err
	}
	return toReviewResponse(review), nil
}

func (s *Service) updateUniversityRating(universityID uint) {
	allReviews, err := s.repo.FindAllByUniversity(universityID)
	if err != nil {
		return
	}

	var totalRating float64
	for _, r := range allReviews {
		ratings := make(map[string]float64)
		if json.Unmarshal(r.Ratings, &ratings) == nil {
			if val, ok := ratings["overall"]; ok {
				totalRating += val
			}
		}
	}

	var avgRating float64
	if len(allReviews) > 0 {
		avgRating = math.Round(totalRating/float64(len(allReviews))*10) / 10
	}

	s.repo.UpdateUniversityRating(universityID, avgRating, len(allReviews))
}

func toReviewResponse(r *Review) *ReviewResponse {
	ratings := make(map[string]float64)
	if len(r.Ratings) > 0 {
		json.Unmarshal(r.Ratings, &ratings)
	}

	return &ReviewResponse{
		ID:                r.ID,
		CollegeID:         r.CollegeID,
		UniversityID:      r.UniversityID,
		CollegeName:       r.CollegeName,
		UserID:            r.UserID,
		StudentType:       r.StudentType,
		Course:            r.Course,
		Level:             r.Level,
		BatchYear:         r.BatchYear,
		Ratings:           ratings,
		Pros:              r.Pros,
		Cons:              r.Cons,
		SummaryTitle:      r.SummaryTitle,
		YearlyFee:         r.YearlyFee,
		Scholarship:       r.Scholarship,
		InternshipOutcome: r.InternshipOutcome,
		Email:             r.Email,
		IsVerified:        r.IsVerified,
		IsPublished:       r.IsPublished,
		HelpfulCount:      r.HelpfulCount,
		CreatedAt:         r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         r.UpdatedAt.Format(time.RFC3339),
	}
}

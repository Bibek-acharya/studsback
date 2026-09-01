package review

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
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
		InstitutionID:     req.InstitutionID,
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
		if isDuplicateReviewError(err) {
			return nil, errors.New("you have already reviewed this university")
		}
		return nil, err
	}

	if review.CollegeID > 0 {
		_ = s.repo.UpdateCollegeRating(review.CollegeID)
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

func (s *Service) GetCollegeReviews(collegeID, instID, userID uint, page, limit int) (*CollegeReviewsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	allReviews := make([]Review, 0)
	var total int64

	if collegeID > 0 {
		r, t, err := s.repo.FindByCollege(collegeID, page, limit)
		if err != nil {
			return nil, err
		}
		allReviews = append(allReviews, r...)
		total = t
	}

	if instID > 0 {
		r, t, err := s.repo.FindByInstitution(instID, page, limit)
		if err != nil {
			return nil, err
		}
		seen := make(map[uint]bool)
		for _, existing := range allReviews {
			seen[existing.ID] = true
		}
		for _, rev := range r {
			if !seen[rev.ID] {
				allReviews = append(allReviews, rev)
			}
		}
		total += t
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	responses := make([]ReviewResponse, len(allReviews))
	for i, r := range allReviews {
		responses[i] = *toReviewResponse(&r)
	}

	reviewIDs := make([]uint, len(allReviews))
	for i, r := range allReviews {
		reviewIDs[i] = r.ID
	}
	voteCounts, err := s.repo.VoteCountsForReviews(reviewIDs)
	if err != nil {
		return nil, err
	}
	userVotes, err := s.repo.UserVotesForReviews(userID, reviewIDs)
	if err != nil {
		return nil, err
	}
	for i := range responses {
		responses[i].HelpfulUpvotes = voteCounts[responses[i].ID][0]
		responses[i].HelpfulDownvotes = voteCounts[responses[i].ID][1]
		responses[i].MyVote = userVotes[responses[i].ID]
	}

	var overallRating float64
	categoryAverages := make(map[string]float64)
	categoryCounts := make(map[string]int)

	forReviewCalc := allReviews
	if collegeID > 0 {
		extra, _ := s.repo.FindAllByCollege(collegeID)
		forReviewCalc = append(forReviewCalc, extra...)
	}
	if instID > 0 {
		extra, _ := s.repo.FindAllByInstitution(instID)
		forReviewCalc = append(forReviewCalc, extra...)
	}

	seenForCalc := make(map[uint]bool)
	var uniqueReviews []Review
	for _, r := range forReviewCalc {
		if !seenForCalc[r.ID] {
			seenForCalc[r.ID] = true
			uniqueReviews = append(uniqueReviews, r)
		}
	}

	var totalRating float64
	var ratingValues int
	for _, r := range uniqueReviews {
		ratings := make(map[string]float64)
		if err := json.Unmarshal(r.Ratings, &ratings); err == nil {
			for cat, val := range ratings {
				categoryAverages[cat] += val
				categoryCounts[cat]++
			}
			for _, val := range ratings {
				totalRating += val
				ratingValues++
			}
		}
	}
	if ratingValues > 0 {
		overallRating = math.Round(totalRating/float64(ratingValues)*10) / 10
	}
	for cat := range categoryAverages {
		if categoryCounts[cat] > 0 {
			categoryAverages[cat] = math.Round(categoryAverages[cat]/float64(categoryCounts[cat])*10) / 10
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

func (s *Service) GetInstitutionReviews(instID uint, page, limit int) (*PaginatedReviewsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	reviews, total, err := s.repo.FindByInstitution(instID, page, limit)
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

func (s *Service) VoteReview(reviewID, userID uint, vote string) (up, down int, myVote string, err error) {
	if vote != "up" && vote != "down" {
		return 0, 0, "", errors.New("invalid vote")
	}
	if _, err := s.repo.FindByID(reviewID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, "", errors.New("review not found")
		}
		return 0, 0, "", err
	}

	myVote, err = s.repo.ToggleVote(reviewID, userID, vote)
	if err != nil {
		return 0, 0, "", err
	}

	counts, err := s.repo.VoteCountsForReviews([]uint{reviewID})
	if err != nil {
		return 0, 0, "", err
	}
	return counts[reviewID][0], counts[reviewID][1], myVote, nil
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
		SummaryTitle: req.Pros,
		Pros:         req.Pros,
		Cons:         req.Cons,
		Email:        "",
		IsPublished:  true,
	}

	if err := s.repo.Create(review); err != nil {
		return nil, err
	}
	review, err = s.repo.FindByID(review.ID)
	if err != nil {
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

func (s *Service) UpdateUniversityReview(userID, universityID uint, req UpdateUniversityReviewRequest) (*ReviewResponse, error) {
	if req.Rating == nil && req.Pros == nil && req.Cons == nil {
		return nil, errors.New("At least one review field is required")
	}

	review, err := s.repo.FindByUserAndUniversity(userID, universityID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("review not found")
		}
		return nil, err
	}

	updates := make(map[string]interface{})
	if req.Rating != nil {
		ratings := make(map[string]float64)
		if len(review.Ratings) > 0 {
			if err := json.Unmarshal(review.Ratings, &ratings); err != nil {
				return nil, errors.New("failed to deserialize ratings")
			}
		}
		ratings["overall"] = *req.Rating
		ratingsJSON, err := json.Marshal(ratings)
		if err != nil {
			return nil, errors.New("failed to serialize ratings")
		}
		updates["ratings"] = ratingsJSON
	}
	if req.Pros != nil {
		updates["pros"] = *req.Pros
	}
	if req.Cons != nil {
		updates["cons"] = *req.Cons
	}

	if err := s.repo.UpdateUniversityFields(review, updates); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("review not found")
		}
		return nil, err
	}

	s.updateUniversityRating(universityID)
	updated, err := s.repo.FindByUserAndUniversity(userID, universityID)
	if err != nil {
		return nil, err
	}
	return toReviewResponse(updated), nil
}

func isDuplicateReviewError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "duplicate key") ||
		strings.Contains(lower, "unique constraint") ||
		strings.Contains(lower, "unique violation")
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

// Admin methods for managing reviews
func (s *Service) AdminDeleteReview(reviewID uint) error {
	return s.repo.AdminDeleteReview(reviewID)
}

func (s *Service) AdminGetAllReviews(page, limit int) (*PaginatedReviewsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	reviews, total, err := s.repo.AdminGetAllReviews(page, limit)
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

func (s *Service) AdminGetUniversityReviews(universityID uint, page, limit int) (*PaginatedReviewsResponse, error) {
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

// Date report methods
func (s *Service) CreateDateReport(req CreateDateReportRequest, fileURL string) (*DateReportResponse, error) {
	report := &DateReport{
		UniversityID: req.UniversityID,
		Contact:      req.Contact,
		Feedback:     req.Feedback,
		FileURL:      fileURL,
		Status:       "pending",
	}

	if err := s.repo.CreateDateReport(report); err != nil {
		return nil, err
	}

	uniName, _ := s.repo.GetUniversityByID(req.UniversityID)

	return &DateReportResponse{
		ID:             report.ID,
		UniversityID:   report.UniversityID,
		UniversityName: uniName,
		Contact:        report.Contact,
		Feedback:       report.Feedback,
		FileURL:        report.FileURL,
		Status:         report.Status,
		CreatedAt:      report.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Service) GetAllDateReports(page, limit int) ([]DateReportResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	reports, total, err := s.repo.GetAllDateReports(page, limit)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]DateReportResponse, len(reports))
	for i, r := range reports {
		uniName, _ := s.repo.GetUniversityByID(r.UniversityID)
		responses[i] = DateReportResponse{
			ID:             r.ID,
			UniversityID:   r.UniversityID,
			UniversityName: uniName,
			Contact:        r.Contact,
			Feedback:       r.Feedback,
			FileURL:        r.FileURL,
			Status:         r.Status,
			CreatedAt:      r.CreatedAt.Format(time.RFC3339),
		}
	}

	return responses, total, nil
}

func (s *Service) UpdateDateReportStatus(id uint, status string) error {
	return s.repo.UpdateDateReportStatus(id, status)
}

func (s *Service) DeleteDateReport(id uint) error {
	return s.repo.DeleteDateReport(id)
}

func toReviewResponse(r *Review) *ReviewResponse {
	ratings := make(map[string]float64)
	if len(r.Ratings) > 0 {
		json.Unmarshal(r.Ratings, &ratings)
	}

	// Get the overall rating
	var rating float64
	if val, ok := ratings["overall"]; ok {
		rating = val
	} else if len(ratings) > 0 {
		// Calculate average if no overall rating
		var sum float64
		for _, v := range ratings {
			sum += v
		}
		rating = sum / float64(len(ratings))
	}

	firstName := strings.TrimSpace(r.User.FirstName)
	lastName := strings.TrimSpace(r.User.LastName)
	nameParts := make([]string, 0, 2)
	initials := make([]byte, 0, 2)
	for _, name := range []string{firstName, lastName} {
		if name == "" {
			continue
		}
		nameParts = append(nameParts, name)
		initials = append(initials, strings.ToUpper(name[:1])[0])
	}
	if len(initials) == 0 {
		initials = []byte{'U'}
	}

	return &ReviewResponse{
		ID:                r.ID,
		CollegeID:         r.CollegeID,
		UniversityID:      r.UniversityID,
		InstitutionID:     r.InstitutionID,
		CollegeName:       r.CollegeName,
		UserID:            r.UserID,
		UserName:          strings.Join(nameParts, " "),
		UserInitials:      string(initials),
		UserProfileImage:  r.User.ImageURL,
		StudentType:       r.StudentType,
		Course:            r.Course,
		Level:             r.Level,
		BatchYear:         r.BatchYear,
		Ratings:           ratings,
		Rating:            rating,
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

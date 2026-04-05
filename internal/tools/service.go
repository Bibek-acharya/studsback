package tools

import (
	"encoding/json"
	"sort"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetScholarshipRecommendations(req ScholarshipFinderRequest) ([]ScholarshipRecommendation, error) {
	scholarships, err := s.repo.LoadScholarships()
	if err != nil {
		return nil, err
	}

	scored := make([]scoredScholarship, 0, len(scholarships))
	for _, scholarship := range scholarships {
		score := 0
		reason := []string{}

		if containsFold(scholarship.DegreeLevel, req.EducationLevel) {
			score += 4
			reason = append(reason, "Matches your education level")
		}

		if req.TargetCountry == "Nepal" || req.TargetCountry == "Inside Nepal" {
			if scholarship.Location == "" || containsAnyFold(scholarship.Location, []string{"Nepal", "Kathmandu", "Pokhara", "Lalitpur", "Biratnagar"}) {
				score += 3
				reason = append(reason, "Available for Nepal-based applicants")
			}
		}

		if req.NeedType != "" {
			if containsAnyFold(req.NeedType, []string{"full", "maximum", "need"}) && containsAnyFold(scholarship.FundingType, []string{"full", "fully"}) {
				score += 3
				reason = append(reason, "High funding coverage")
			} else if containsAnyFold(req.NeedType, []string{"partial", "cost"}) && containsAnyFold(scholarship.FundingType, []string{"partial", "tuition"}) {
				score += 2
				reason = append(reason, "Matches your budget preference")
			}
		}

		fieldMatches := fieldMatchCount(req.Skills, scholarship.FieldOfStudy)
		if fieldMatches > 0 {
			score += fieldMatches
			reason = append(reason, "Aligned with your interests/skills")
		}

		scored = append(scored, scoredScholarship{Item: scholarship, Score: score, Reason: reason})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	results := make([]ScholarshipRecommendation, 0)
	for i, item := range scored {
		if i >= 16 {
			break
		}
		results = append(results, ScholarshipRecommendation{
			ID:          item.Item.ID,
			Title:       item.Item.Title,
			Provider:    item.Item.Provider,
			Location:    item.Item.Location,
			Value:       item.Item.Value,
			Deadline:    item.Item.Deadline.Format("2006-01-02"),
			DegreeLevel: item.Item.DegreeLevel,
			FundingType: item.Item.FundingType,
			ScholarType: item.Item.ScholarshipType,
			Description: item.Item.Description,
			ImageURL:    item.Item.ImageURL,
			MatchScore:  item.Score,
			Reasons:     item.Reason,
		})
	}

	return results, nil
}

func (s *Service) GetCollegeRecommendations(req CollegeRecommenderRequest) ([]CollegeRecommendation, error) {
	colleges, err := s.repo.LoadColleges()
	if err != nil {
		return nil, err
	}

	scored := make([]scoredCollege, 0, len(colleges))
	for _, college := range colleges {
		score := 0
		reason := []string{}

		studentType := normalizeStudentType(req.StudentType)
		switch studentType {
		case "academic":
			score += college.AcademicFitScore
			reason = append(reason, "Strong academic environment for toppers")
		case "campus":
			score += college.CampusLifeScore
			reason = append(reason, "Great campus life and student activities")
		case "career":
			score += college.CareerFitScore
			reason = append(reason, "Career-focused with placement outcomes")
		case "balanced":
			score += college.BalancedFitScore
			reason = append(reason, "Balanced academics, activities, and growth")
		}

		if profileTagMatches(college.ProfileTags, studentType) {
			score += 2
			reason = append(reason, "Matches your student profile")
		}

		if req.PreferredLocation != "" && containsFold(college.Location, req.PreferredLocation) {
			score += 4
			reason = append(reason, "Matches your preferred location")
		}

		if req.CollegeType != "" && containsFold(college.CollegeType, req.CollegeType) {
			score += 3
			reason = append(reason, "Matches preferred college type")
		}

		if req.BudgetPreference != "" {
			if containsAnyFold(req.BudgetPreference, []string{"low", "affordable"}) && containsAnyFold(college.CollegeType, []string{"public", "government"}) {
				score += 3
				reason = append(reason, "Affordable/public-friendly option")
			} else if containsAnyFold(req.BudgetPreference, []string{"quality", "premium"}) && containsAnyFold(college.CollegeType, []string{"private"}) {
				score += 2
				reason = append(reason, "Premium learning environment")
			}
		}

		if req.ProgramInterest != "" {
			if containsFold(string(college.FeaturedPrograms), req.ProgramInterest) || containsFold(string(college.Courses), req.ProgramInterest) {
				score += 4
				reason = append(reason, "Offers your preferred program")
			}
		}

		if req.NeedScholarship && len(college.Scholarships) > 0 {
			score += 2
			reason = append(reason, "Scholarship support available")
		}

		if college.Rating >= 4.0 {
			score += 1
			reason = append(reason, "Strong student rating")
		}

		scored = append(scored, scoredCollege{Item: college, Score: score, Reason: reason})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	results := make([]CollegeRecommendation, 0)
	for i, item := range scored {
		if i >= 12 {
			break
		}
		results = append(results, CollegeRecommendation{
			ID:          item.Item.ID,
			Name:        item.Item.Name,
			Location:    item.Item.Location,
			Affiliation: item.Item.Affiliation,
			Type:        item.Item.CollegeType,
			Rating:      item.Item.Rating,
			Reviews:     item.Item.Reviews,
			ImageURL:    item.Item.ImageURL,
			Website:     item.Item.Website,
			MatchScore:  item.Score,
			Reasons:     item.Reason,
		})
	}

	return results, nil
}

func normalizeStudentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "academic", "academic topper":
		return "academic"
	case "campus", "campus life lover":
		return "campus"
	case "career", "career-focused planner", "career focused planner":
		return "career"
	case "balanced", "balanced explorer":
		return "balanced"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func profileTagMatches(raw []byte, studentType string) bool {
	if len(raw) == 0 || studentType == "" {
		return false
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return false
	}
	for _, tag := range tags {
		if normalizeStudentType(tag) == studentType {
			return true
		}
	}
	return false
}

func containsFold(source, q string) bool {
	if source == "" || q == "" {
		return false
	}
	return strings.Contains(strings.ToLower(source), strings.ToLower(q))
}

func containsAnyFold(source string, queries []string) bool {
	for _, q := range queries {
		if containsFold(source, q) {
			return true
		}
	}
	return false
}

func fieldMatchCount(skills []string, fieldJSON []byte) int {
	if len(skills) == 0 || len(fieldJSON) == 0 {
		return 0
	}

	var fields []string
	if err := json.Unmarshal(fieldJSON, &fields); err != nil {
		return 0
	}

	count := 0
	for _, skill := range skills {
		for _, field := range fields {
			if containsFold(field, skill) {
				count++
				break
			}
		}
	}
	return count
}

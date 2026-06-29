package college

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const maxCollegeRecommendations = 30

var preferredFieldKeywords = map[string][]string{
	"science":         {"science", "bsc", "engineering", "be ", "bit", "csit", "b.sc", "physics", "chemistry", "biology", "math", "mathematics"},
	"management":      {"management", "bbs", "bba", "bbm", "bhm", "bba-tt", "bba-f", "commerce", "finance", "accounting", "marketing", "business"},
	"humanities":      {"humanities", "law", "social", "arts", "ba ", "history", "sociology", "anthropology", "political", "philosophy"},
	"medical":         {"medical", "nursing", "pharmacy", "mbbs", "bds", "bns", "bpharm", "health", "pcl", "ha "},
	"it":              {"it", "computer", "csit", "bit", "bca", "computing", "software", "data science", "ai", "artificial intelligence"},
	"hotel":           {"hotel", "tourism", "hospitality", "bhm", "hm "},
	"education":       {"education", "b.ed", "bed", "teaching", "teacher"},
	"others":          {},
}

var budgetRanges = map[string]int{
	"Under NPR. 10,000 / year":         10000,
	"NPR. 10,000 - NPR. 30,000 / year": 30000,
	"NPR. 30,000 - NPR. 50,000 / year": 50000,
	"Over NPR. 50,000 / year":          1000000,
}

type CollegeProfileData struct {
	EducationEntries []CollegeEducationEntry
	Preferences      *CollegePreferences
	BookmarkedFields []string
}

type CollegeEducationEntry struct {
	Level  string
	Stream string
	Grade  string
}

type CollegePreferences struct {
	Preferences map[string]interface{}
}

type scoredCollege struct {
	College   College
	Score     int
	Reasons   []string
	Breakdown CollegeRecommendationBreakdown
}

func (s *Service) RecommendColleges(req CollegeRecommenderRequest, userID *uint) ([]CollegeRecommendationResult, error) {
	colleges, err := s.repo.FindAllForRecommendation(500)
	if err != nil {
		return nil, err
	}

	var profileData *CollegeProfileData
	if userID != nil {
		pd, err := s.repo.GetCollegeProfileForRecommendation(*userID)
		if err == nil {
			profileData = pd
		}
	}

	scored := make([]scoredCollege, 0, len(colleges))
	for _, c := range colleges {
		score, reasons, breakdown := scoreCollege(c, req, profileData)
		scored = append(scored, scoredCollege{
			College:   c,
			Score:     score,
			Reasons:   reasons,
			Breakdown: breakdown,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].College.Rating > scored[j].College.Rating
	})

	limit := maxCollegeRecommendations
	if len(scored) < limit {
		limit = len(scored)
	}

	results := make([]CollegeRecommendationResult, 0, limit)
	for i := 0; i < limit; i++ {
		sc := scored[i]
		results = append(results, toCollegeRecommendation(sc.College, sc.Score, sc.Reasons, sc.Breakdown))
	}

	return results, nil
}

func scoreCollege(c College, req CollegeRecommenderRequest, profile *CollegeProfileData) (int, []string, CollegeRecommendationBreakdown) {
	breakdown := CollegeRecommendationBreakdown{}
	reasons := make([]string, 0, 8)

	dimValues := []struct {
		score   int
		reasons []string
		target  *int
	}{
		{0, nil, &breakdown.StudentType},
		{0, nil, &breakdown.PreferredField},
		{0, nil, &breakdown.Location},
		{0, nil, &breakdown.Budget},
		{0, nil, &breakdown.FinancialAid},
		{0, nil, &breakdown.AcademicsVsCampus},
		{0, nil, &breakdown.Activities},
		{0, nil, &breakdown.Facilities},
		{0, nil, &breakdown.Reputation},
	}

	s, r := scoreStudentTypeFit(c, req.StudentType)
	dimValues[0].score = s
	dimValues[0].reasons = r
	reasons = append(reasons, r...)

	s, r = scorePreferredField(c, req.PreferredField)
	dimValues[1].score = s
	dimValues[1].reasons = r
	reasons = append(reasons, r...)

	s, r = scoreLocation(c, req.Province, req.District, req.Setting)
	dimValues[2].score = s
	dimValues[2].reasons = r
	reasons = append(reasons, r...)

	s, r = scoreBudget(c, req.YearlyBudget, req.TuitionFactor)
	dimValues[3].score = s
	dimValues[3].reasons = r
	reasons = append(reasons, r...)

	s, r = scoreFinancialAid(c, req.FinancialSupport)
	dimValues[4].score = s
	dimValues[4].reasons = r
	reasons = append(reasons, r...)

	s, r = scoreAcademicsVsCampus(c, req.AcademicsVsCampus)
	dimValues[5].score = s
	dimValues[5].reasons = r
	reasons = append(reasons, r...)

	s, r = scoreActivities(c, req.ActivitiesImportance)
	dimValues[6].score = s
	dimValues[6].reasons = r
	reasons = append(reasons, r...)

	s, r = scoreFacilities(c, req.FacilityChoice)
	dimValues[7].score = s
	dimValues[7].reasons = r
	reasons = append(reasons, r...)

	s, r = scoreReputation(c, req.ReputationImportance)
	dimValues[8].score = s
	dimValues[8].reasons = r
	reasons = append(reasons, r...)

	raw := make([]float64, 0, 11)
	for i := range dimValues {
		raw = append(raw, float64(dimValues[i].score))
		*dimValues[i].target = dimValues[i].score
	}

	s, _ = scoreDistanceFromHome(c, req.DistanceFromHome)
	breakdown.DistanceFromHome = s
	raw = append(raw, float64(s))

	s, _ = scoreClassSize(c, req.ClassSize)
	breakdown.ClassSize = s
	raw = append(raw, float64(s))

	hasProfile := profile != nil && len(profile.EducationEntries) > 0
	if hasProfile {
		ps := scoreCollegeProfileCompatibility(c, profile)
		breakdown.ProfileCompatibility = ps
		raw = append(raw, float64(ps))
	}

	norm := normalizePercentile(raw)
	weights := getCollegeDimensionWeights(hasProfile)

	var totalScore float64
	for i := range norm {
		totalScore += norm[i] * weights[i]
	}

	return int(math.Round(totalScore * 100)), reasons, breakdown
}

func scoreStudentTypeFit(c College, studentType string) (int, []string) {
	if studentType == "" {
		return 5, nil
	}

	var fitScore int
	var label string
	switch studentType {
	case "academic":
		fitScore = c.AcademicFitScore
		label = "Strong academics"
	case "campus_life":
		fitScore = c.CampusLifeScore
		label = "Active campus life"
	case "career":
		fitScore = c.CareerFitScore
		label = "Career-focused"
	case "balanced":
		fitScore = c.BalancedFitScore
		label = "Balanced experience"
	default:
		fitScore = 5
	}

	points := int(float64(fitScore) * 3.0)
	if points > 30 {
		points = 30
	}
	if points < 0 {
		points = 0
	}

	var reasons []string
	if fitScore >= 7 {
		reasons = append(reasons, label)
	}
	return points, reasons
}

func scorePreferredField(c College, field string) (int, []string) {
	if field == "" {
		return 5, nil
	}

	lowerField := strings.ToLower(field)
	var matched bool
	for key, kws := range preferredFieldKeywords {
		if len(kws) == 0 {
			continue
		}
		if !strings.HasPrefix(lowerField, key) {
			continue
		}

		combined := strings.ToLower(string(c.FeaturedPrograms) + " " +
			string(c.Courses) + " " +
			string(c.ProgramsList) + " " +
			c.Name + " " +
			c.Description)

		for _, kw := range kws {
			if strings.Contains(combined, kw) {
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}

	if matched {
		return 20, []string{"Offers programs in your field"}
	}
	return 5, nil
}

func scoreLocation(c College, province, district, setting string) (int, []string) {
	if province == "" || province == "No preference" {
		if c.Popular || c.Featured {
			return 10, []string{"Popular choice"}
		}
		return 5, nil
	}

	score := 0
	loc := strings.ToLower(c.Location)
	provLower := strings.ToLower(province)

	if strings.Contains(loc, provLower) {
		score += 8
	}

	if district != "" && strings.Contains(loc, strings.ToLower(district)) {
		score += 5
	}

	if setting != "" {
		settingLower := strings.ToLower(setting)
		combined := loc + " " + c.Description
		if strings.Contains(settingLower, "urban") {
			if strings.Contains(combined, "kathmandu") || strings.Contains(combined, "lalitpur") ||
				strings.Contains(combined, "bhaktapur") || strings.Contains(combined, "pokhara") ||
				strings.Contains(combined, "biratnagar") {
				score += 2
			}
		} else if strings.Contains(settingLower, "rural") {
			if !strings.Contains(combined, "kathmandu") && !strings.Contains(combined, "lalitpur") {
				score += 2
			}
		}
	}

	if score > 15 {
		score = 15
	}

	var reasons []string
	if score >= 8 {
		reasons = append(reasons, "Located in your preferred area")
	}
	return score, reasons
}

func scoreBudget(c College, budgetStr, tuitionFactor string) (int, []string) {
	if budgetStr == "" {
		return 5, nil
	}

	maxBudget, ok := budgetRanges[budgetStr]
	if !ok {
		return 5, nil
	}

	minFee, found := extractCollegeMinFee(c)
	if !found {
		if tuitionFactor == "Yes, low cost is very important" {
			return 5, nil
		}
		return 8, nil
	}

	score := 0
	if minFee <= maxBudget {
		score = 12
	} else if minFee <= maxBudget*15/10 {
		score = 7
	} else if minFee <= maxBudget*2 {
		score = 3
	}

	if tuitionFactor == "Yes, low cost is very important" {
		if c.CollegeType == "Public / Govt" || c.CollegeType == "Constituent" {
			score += 3
		}
	}

	if score > 15 {
		score = 15
	}

	var reasons []string
	if score >= 10 {
		reasons = append(reasons, "Fits your budget")
	}
	return score, reasons
}

func scoreFinancialAid(c College, financialSupport string) (int, []string) {
	if financialSupport != "yes" {
		return 0, nil
	}

	hasScholarship := len(c.Scholarships) > 0
	combined := strings.ToLower(string(c.Scholarships) + " " +
		c.Description + " " +
		string(c.About) + " " +
		string(c.Admissions))

	hasKeyword := strings.Contains(combined, "scholarship") ||
		strings.Contains(combined, "financial aid") ||
		strings.Contains(combined, "fee waiver") ||
		strings.Contains(combined, "discount")

	if hasScholarship || hasKeyword {
		return 5, []string{"Offers scholarships"}
	}
	return 0, nil
}

func scoreAcademicsVsCampus(c College, pref string) (int, []string) {
	if pref == "" {
		return 2, nil
	}

	var score int
	var reasons []string
	switch pref {
	case "Academics matter more than fun":
		score = int(float64(c.AcademicFitScore) * 0.5)
		if c.AcademicFitScore >= 7 {
			reasons = append(reasons, "Strong academic environment")
		}
	case "Campus life matters more":
		score = int(float64(c.CampusLifeScore) * 0.5)
		if c.CampusLifeScore >= 7 {
			reasons = append(reasons, "Vibrant campus life")
		}
	case "Both equally important":
		avg := (c.AcademicFitScore + c.CampusLifeScore) / 2
		score = int(float64(avg) * 0.5)
		if avg >= 7 {
			reasons = append(reasons, "Great balance of academics and life")
		}
	}

	if score > 5 {
		score = 5
	}
	return score, reasons
}

func scoreActivities(c College, importance string) (int, []string) {
	if importance == "" {
		return 1, nil
	}

	combined := strings.ToLower(string(c.Amenities) + " " +
		c.Description + " " +
		string(c.About))

	hasActivities := strings.Contains(combined, "sport") ||
		strings.Contains(combined, "club") ||
		strings.Contains(combined, "event") ||
		strings.Contains(combined, "festival") ||
		strings.Contains(combined, "activity") ||
		strings.Contains(combined, "gym") ||
		strings.Contains(combined, "cultural")

	score := 0
	switch importance {
	case "Yes":
		if hasActivities {
			score = 3
		} else {
			score = 1
		}
	case "Somewhat":
		score = 2
	case "No":
		score = 1
	}

	var reasons []string
	if score == 3 {
		reasons = append(reasons, "Active extracurricular scene")
	}
	return score, reasons
}

func scoreFacilities(c College, facilityChoice string) (int, []string) {
	if facilityChoice == "" {
		return 1, nil
	}

	choices := strings.Split(facilityChoice, ", ")
	if len(choices) == 0 {
		return 1, nil
	}

	combined := strings.ToLower(string(c.Amenities) + " " +
		c.Description + " " +
		string(c.About))

	matches := 0
	for _, choice := range choices {
		choiceLower := strings.ToLower(choice)
		var found bool
		switch {
		case strings.Contains(choiceLower, "lab"):
			found = strings.Contains(combined, "lab") || strings.Contains(combined, "laboratory")
		case strings.Contains(choiceLower, "hostel"):
			found = strings.Contains(combined, "hostel") || strings.Contains(combined, "accommodation")
		case strings.Contains(choiceLower, "library"):
			found = strings.Contains(combined, "library")
		case strings.Contains(choiceLower, "cafeteria"):
			found = strings.Contains(combined, "cafeteria") || strings.Contains(combined, "canteen") || strings.Contains(combined, "mess")
		default:
			found = true
		}
		if found {
			matches++
		}
	}

	score := matches
	if score > 5 {
		score = 5
	}

	var reasons []string
	if score >= 3 {
		reasons = append(reasons, "Has facilities you wanted")
	}
	return score, reasons
}

func scoreReputation(c College, importance string) (int, []string) {
	if importance == "" {
		return 2, nil
	}

	rating := c.Rating
	if rating == 0 {
		rating = 3.5
	}

	var score int
	switch importance {
	case "Yes":
		if rating >= 4.5 {
			score = 5
		} else if rating >= 4.0 {
			score = 4
		} else if rating >= 3.5 {
			score = 3
		} else {
			score = 2
		}
	case "Somewhat":
		if rating >= 4.0 {
			score = 4
		} else {
			score = 3
		}
	case "No":
		score = 2
	}

	var reasons []string
	if score >= 4 {
		reasons = append(reasons, "Highly rated")
	}
	return score, reasons
}

func toCollegeRecommendation(c College, score int, reasons []string, breakdown CollegeRecommendationBreakdown) CollegeRecommendationResult {
	if len(reasons) > 4 {
		reasons = reasons[:4]
	}
	if len(reasons) == 0 {
		reasons = []string{"Matches your profile"}
	}

	tuition := extractTuitionString(c)

	return CollegeRecommendationResult{
		ID:         c.ID,
		Name:       c.Name,
		Location:   c.Location,
		Type:       c.CollegeType,
		Tuiton:     tuition,
		MatchScore: score,
		Reasons:    reasons,
		Breakdown:  breakdown,
	}
}

func normalizePercentile(scores []float64) []float64 {
	if len(scores) == 0 {
		return nil
	}
	min, max := scores[0], scores[0]
	for _, s := range scores {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}
	result := make([]float64, len(scores))
	if max == min {
		for i := range result {
			result[i] = 0.5
		}
		return result
	}
	for i, s := range scores {
		result[i] = (s - min) / (max - min)
	}
	return result
}

func getCollegeDimensionWeights(hasProfile bool) []float64 {
	if hasProfile {
		return []float64{0.15, 0.12, 0.12, 0.10, 0.05, 0.05, 0.05, 0.05, 0.03, 0.03, 0.03, 0.12}
	}
	return []float64{0.18, 0.14, 0.14, 0.12, 0.06, 0.06, 0.06, 0.06, 0.06, 0.04, 0.04}
}

var distanceScores = map[string]int{
	"Less than 10 km":  5,
	"10–20 km":         4,
	"20–50 km":         3,
	"More than 50 km":  2,
	"No preference":    3,
}

func scoreDistanceFromHome(c College, distance string) (int, []string) {
	if distance == "" {
		return 2, nil
	}
	score, ok := distanceScores[distance]
	if !ok {
		return 2, nil
	}
	if score >= 4 {
		return score, []string{"Convenient location from home"}
	}
	return score, nil
}

var classSizeScores = map[string]int{
	"Small classes (< 30)":   5,
	"Medium classes (30–50)": 4,
	"Large classes (50+)":    2,
	"No preference":          3,
}

func scoreClassSize(c College, classSize string) (int, []string) {
	if classSize == "" {
		return 2, nil
	}
	score, ok := classSizeScores[classSize]
	if !ok {
		return 2, nil
	}
	if score >= 4 {
		return score, []string{"Preferred class size"}
	}
	return score, nil
}

func scoreCollegeProfileCompatibility(c College, profile *CollegeProfileData) int {
	if profile == nil || len(profile.EducationEntries) == 0 {
		return 0
	}
	score := 0
	combined := strings.ToLower(c.Name + " " + c.Description + " " + string(c.FeaturedPrograms) + " " + string(c.Courses) + " " + string(c.ProgramsList))

	for _, e := range profile.EducationEntries {
		stream := strings.ToLower(e.Stream)
		if strings.Contains(combined, stream) {
			score += 3
		}
	}

	if profile.Preferences != nil && profile.Preferences.Preferences != nil {
		if fields, ok := profile.Preferences.Preferences["fields"].([]interface{}); ok {
			for _, f := range fields {
				fs := strings.ToLower(fmt.Sprintf("%v", f))
				if strings.Contains(combined, fs) {
					score += 2
				}
			}
		}
	}

	for _, bf := range profile.BookmarkedFields {
		bfLower := strings.ToLower(bf)
		if strings.Contains(combined, bfLower) {
			score += 1
		}
	}

	if score > 20 {
		score = 20
	}
	return score
}

func extractTuitionString(c College) string {
	minFee, found := extractCollegeMinFee(c)
	if !found {
		if c.Students != "" {
			return c.Students
		}
		return "Contact college"
	}

	return fmt.Sprintf("NPR %s/year", formatNPR(int(minFee)))
}

func formatNPR(amount int) string {
	if amount < 1000 {
		return fmt.Sprintf("%d", amount)
	}

	str := fmt.Sprintf("%d", amount)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%2 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}



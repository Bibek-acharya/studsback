package scholarship

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
)

const maxRecommendations = 20

var educationLevelMap = map[string][]string{
	"high_school":    {"+2", "10+2", "high school", "slc", "see", "higher secondary", "secondary"},
	"diploma":        {"diploma", "certificate", "ctevt", "tslc"},
	"undergraduate":  {"bachelor", "bachelors", "undergraduate", "bachelor's", "bachelor degree"},
	"postgraduate":   {"master", "masters", "phd", "postgraduate", "doctoral", "master's", "doctorate"},
}

var fieldOfStudyMap = map[string][]string{
	"cs":         {"computer science", "it", "information technology", "cs", "software", "computing", "data science", "programming", "artificial intelligence", "machine learning", "cybersecurity"},
	"business":   {"business", "management", "commerce", "finance", "economics", "accounting", "marketing", "entrepreneurship", "banking", "supply chain", "bba", "mba"},
	"engineering": {"engineering", "civil", "mechanical", "electrical", "electronics", "architecture", "chemical", "aerospace", "biomedical", "environmental", "industrial"},
	"medicine":   {"medicine", "medical", "health", "nursing", "pharmacy", "dentistry", "public health", "veterinary", "surgery", "anatomy", "physiology", "mbbs", "bds"},
	"arts":       {"arts", "humanities", "social science", "literature", "history", "philosophy", "fine arts", "music", "design", "linguistics", "sociology", "psychology", "anthropology", "education", "teaching"},
	"law":        {"law", "legal", "political science", "criminal justice", "international relations", "llb", "ba llb"},
}

var providerTypeMap = map[string]string{
	"Government":          "Government",
	"NGO":                 "NGO",
	"INGO":                "INGO",
	"University":          "University",
	"College":             "College",
	"Private Organization": "Private Organization",
	"private_organization": "Private Organization",
}

type scoredScholarship struct {
	Scholarship  Scholarship
	Score        int
	ProviderType string
}

func (s *Service) RecommendScholarships(req ScholarshipRecommendRequest) ([]RecommendResult, error) {
	platformScholarships, err := s.repo.FindAllForRecommendation()
	if err != nil {
		return nil, err
	}

	providerScholarships, err := s.repo.FindProviderScholarshipsForRecommendation()
	if err != nil {
		return nil, err
	}

	var scored []scoredScholarship

	for _, sch := range platformScholarships {
		score, providerType := scoreScholarship(sch, req)
		scored = append(scored, scoredScholarship{
			Scholarship:  sch,
			Score:        score,
			ProviderType: providerType,
		})
	}

	for _, ps := range providerScholarships {
		sch, providerType := providerScholarshipToRecommendScholarship(ps)
		score, pt := scoreScholarship(sch, req)
		if pt != "" {
			providerType = pt
		}
		scored = append(scored, scoredScholarship{
			Scholarship:  sch,
			Score:        score,
			ProviderType: providerType,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	limit := maxRecommendations
	if len(scored) < limit {
		limit = len(scored)
	}

	results := make([]RecommendResult, 0, limit)
	for i := 0; i < limit; i++ {
		sc := scored[i]
		results = append(results, toRecommendResult(sc.Scholarship, sc.Score, sc.ProviderType))
	}

	return results, nil
}

func scoreScholarship(s Scholarship, req ScholarshipRecommendRequest) (int, string) {
	score := 0

	score += scoreEducationLevel(s, req.EducationLevel)
	score += scoreFieldOfStudy(s, req.FieldOfStudy)
	score += scoreLocation(s, req.Province, req.District)
	score += scoreFinancialFit(s, req.Income)
	score += scoreStudyLocation(s, req.StudyLocation)
	score += scoreCategoryGender(s, req.Category, req.Gender)
	score += scoreGPAMatch(s, req.AcademicScoreType, req.AcademicScore)
	score += scoreWillingness(s, req.WillingEssay, req.WillingInterview, req.WillingGpa)

	providerType := determineProviderType(s.Provider, s.FundingType, s.ScholarshipType)

	return score, providerType
}

func scoreEducationLevel(s Scholarship, userLevel string) int {
	if userLevel == "" {
		return 0
	}

	mapped, ok := educationLevelMap[userLevel]
	if !ok {
		return 0
	}

	schLevels := strings.ToLower(s.DegreeLevel)
	for _, term := range mapped {
		if strings.Contains(schLevels, term) {
			return 30
		}
	}

	if s.EducationLevel != "" {
		eduLevel := strings.ToLower(s.EducationLevel)
		for _, term := range mapped {
			if strings.Contains(eduLevel, term) {
				return 30
			}
		}
	}

	return 0
}

func scoreFieldOfStudy(s Scholarship, userField string) int {
	if userField == "" {
		return 0
	}

	mapped, ok := fieldOfStudyMap[userField]
	if !ok {
		return 5
	}

	fieldOfStudyData := parseStringArray(s.FieldOfStudy)
	for _, f := range fieldOfStudyData {
		fLower := strings.ToLower(f)
		for _, term := range mapped {
			if strings.Contains(fLower, term) {
				return 25
			}
		}
	}

	desc := strings.ToLower(s.Description)
	for _, term := range mapped {
		if strings.Contains(desc, term) {
			return 15
		}
	}

	title := strings.ToLower(s.Title)
	for _, term := range mapped {
		if strings.Contains(title, term) {
			return 10
		}
	}

	return 0
}

func scoreLocation(s Scholarship, province, district string) int {
	if province == "" {
		return 5
	}

	loc := strings.ToLower(s.Location)
	coverage := strings.ToLower(s.CoverageArea)
	combined := loc + " " + coverage

	score := 0

	if strings.Contains(combined, strings.ToLower(province)) {
		score += 8
	}
	if district != "" && strings.Contains(combined, strings.ToLower(district)) {
		score += 7
	}

	if province != "" {
		excluded := parseStringArray(s.ExcludedRegions)
		for _, ex := range excluded {
			if strings.Contains(strings.ToLower(ex), strings.ToLower(province)) {
				score -= 5
				break
			}
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 15 {
		score = 15
	}

	return score
}

func scoreFinancialFit(s Scholarship, income string) int {
	scholarshipType := strings.ToLower(s.ScholarshipType)
	fundingType := strings.ToLower(s.FundingType)
	combined := scholarshipType + " " + fundingType

	isNeedBased := strings.Contains(combined, "need") || strings.Contains(combined, "need-based") ||
		strings.Contains(combined, "need_based") || strings.Contains(combined, "fully funded") ||
		strings.Contains(combined, "scholarship for underprivileged")
	isMeritBased := strings.Contains(combined, "merit") || strings.Contains(combined, "merit-based") ||
		strings.Contains(combined, "merit_based") || strings.Contains(combined, "excellence")

	switch income {
	case "below_2":
		if isNeedBased {
			return 10
		}
		if isMeritBased {
			return 5
		}
		return 3
	case "2_to_5":
		if isNeedBased || isMeritBased {
			return 8
		}
		return 5
	case "5_to_10":
		if isMeritBased {
			return 10
		}
		if isNeedBased {
			return 5
		}
		return 5
	case "above_10":
		if isMeritBased {
			return 10
		}
		return 5
	}

	return 5
}

func scoreStudyLocation(s Scholarship, studyLocation string) int {
	loc := strings.ToLower(s.Location)
	isNepal := strings.Contains(loc, "nepal") ||
		strings.Contains(loc, "kathmandu") ||
		strings.Contains(loc, "pokhara") ||
		strings.Contains(loc, "lalitpur") ||
		strings.Contains(loc, "bhaktapur") ||
		strings.Contains(loc, "chitwan") ||
		strings.Contains(loc, "biratnagar") ||
		strings.Contains(loc, "butwal") ||
		strings.Contains(loc, "dharan") ||
		strings.Contains(loc, "janakpur") ||
		strings.Contains(loc, "nepalgunj") ||
		strings.Contains(loc, "hetauda")

	switch studyLocation {
	case "inside":
		if isNepal {
			return 10
		}
		return 0
	case "abroad":
		if !isNepal {
			return 10
		}
		return 0
	case "both":
		return 10
	}

	return 5
}

func scoreCategoryGender(s Scholarship, category, gender string) int {
	score := 0

	prov := strings.ToLower(s.Provider)
	title := strings.ToLower(s.Title)
	desc := strings.ToLower(s.Description)
	combined := prov + " " + title + " " + desc

	categoryMap := map[string]string{
		"dalit":    "dalit",
		"janajati": "janajati",
		"madhesi":  "madhesi",
		"muslim":   "muslim",
		"disabled": "disabled",
	}

	if mapped, ok := categoryMap[category]; ok {
		if strings.Contains(combined, mapped) {
			score += 3
		}
	}

	if gender == "female" {
		if strings.Contains(combined, "women") || strings.Contains(combined, "female") ||
			strings.Contains(combined, "girl") || strings.Contains(combined, "mother") {
			score += 2
		}
	}

	if score > 5 {
		score = 5
	}

	return score
}

func scoreGPAMatch(s Scholarship, scoreType, score string) int {
	if scoreType == "" || score == "" {
		return 2
	}

	userScore, err := strconv.ParseFloat(score, 64)
	if err != nil {
		return 2
	}

	var userGPA float64
	if scoreType == "percentage" {
		if userScore >= 80 {
			userGPA = 4.0
		} else if userScore >= 60 {
			userGPA = 3.0
		} else if userScore >= 40 {
			userGPA = 2.0
		} else {
			userGPA = 1.0
		}
	} else {
		userGPA = userScore
	}

	minGPA := extractMinGPA(s.ScholarshipType, s.FundingType, s.Description, s.BasicEligibilityCriteria, s.EligibilityCriteria)

	if minGPA <= 0 {
		return 3
	}

	if userGPA >= minGPA {
		return 5
	}
	if userGPA >= minGPA-0.5 {
		return 2
	}

	return 0
}

func extractMinGPA(scholarshipType, fundingType, description string, basicEligibility, eligibility []byte) float64 {
	combined := strings.ToLower(scholarshipType + " " + fundingType + " " + description)

	if strings.Contains(combined, "3.5") {
		return 3.5
	}
	if strings.Contains(combined, "3.0") {
		return 3.0
	}
	if strings.Contains(combined, "2.5") {
		return 2.5
	}
	if strings.Contains(combined, "2.0") {
		return 2.0
	}

	for _, data := range [][]byte{basicEligibility, eligibility} {
		if len(data) == 0 {
			continue
		}
		var fields []DetailField
		if err := json.Unmarshal(data, &fields); err != nil {
			continue
		}
		for _, f := range fields {
			text := strings.ToLower(f.Title + " " + f.Description + " " + f.Criterion + " " + f.Eligibility)
			if strings.Contains(text, "3.5") {
				return 3.5
			}
			if strings.Contains(text, "3.0") {
				return 3.0
			}
			if strings.Contains(text, "2.5") {
				return 2.5
			}
			if strings.Contains(text, "2.0") {
				return 2.0
			}
		}
	}

	return 0
}

func scoreWillingness(s Scholarship, willingEssay, willingInterview, willingGpa string) int {
	score := 0

	if willingEssay == "yes" {
		score += 1
	}
	if willingInterview == "yes" {
		score += 1
	}
	if willingGpa == "yes" {
		score += 1
	}

	return score
}

func determineProviderType(provider, fundingType, scholarshipType string) string {
	combined := strings.ToLower(provider + " " + fundingType + " " + scholarshipType)

	for key, value := range providerTypeMap {
		if strings.Contains(combined, strings.ToLower(key)) {
			return value
		}
	}

	if strings.Contains(combined, "government") || strings.Contains(combined, "ministry") {
		return "Government"
	}
	if strings.Contains(combined, "ngo") || strings.Contains(combined, "foundation") {
		return "NGO"
	}
	if strings.Contains(combined, "ingo") {
		return "INGO"
	}
	if strings.Contains(combined, "university") {
		return "University"
	}
	if strings.Contains(combined, "college") || strings.Contains(combined, "campus") {
		return "College"
	}
	if strings.Contains(combined, "private") || strings.Contains(combined, "corporate") {
		return "Private Organization"
	}

	return "Other"
}

func determineCoverage(s Scholarship) string {
	fundingType := strings.ToLower(s.FundingType)
	scholarshipType := strings.ToLower(s.ScholarshipType)
	combined := fundingType + " " + scholarshipType

	if strings.Contains(combined, "full") || strings.Contains(combined, "fully funded") {
		return "Full"
	}
	if strings.Contains(combined, "tuition") {
		return "Tuition Only"
	}
	if strings.Contains(combined, "75") || strings.Contains(combined, "75%") {
		return "75%"
	}
	if strings.Contains(combined, "50") || strings.Contains(combined, "50%") {
		return "50%"
	}
	if strings.Contains(combined, "25") || strings.Contains(combined, "25%") {
		return "25%"
	}
	if strings.Contains(combined, "partial") {
		return "Partial"
	}

	return "Partial"
}

func determineTagColorClass(s Scholarship) string {
	status := deriveScholarshipStatus(s.Deadline)
	switch status {
	case "OPEN":
		return "bg-green-100 text-green-700"
	case "CLOSING SOON":
		return "bg-orange-100 text-orange-700"
	case "CLOSED":
		return "bg-red-100 text-red-700"
	}

	fundingType := strings.ToLower(s.FundingType)
	if strings.Contains(fundingType, "full") || strings.Contains(fundingType, "fully funded") {
		return "bg-emerald-100 text-emerald-700"
	}
	if strings.Contains(fundingType, "merit") {
		return "bg-blue-100 text-blue-700"
	}
	if strings.Contains(fundingType, "need") {
		return "bg-purple-100 text-purple-700"
	}

	return "bg-slate-100 text-slate-600"
}

func providerScholarshipToRecommendScholarship(ps ProviderScholarship) (Scholarship, string) {
	imageURL := ""
	if ps.ImageURL != nil {
		imageURL = *ps.ImageURL
	}

	providerName := ps.ProviderName
	if providerName == "" {
		providerName = "Provider"
	}

	desc := ps.Description
	if desc == "" {
		desc = ps.AboutParagraph1
	}

	if ps.ScholarshipDescription1 != "" {
		if desc != "" {
			desc += " "
		}
		desc += ps.ScholarshipDescription1
	}

	providerType := determineProviderType(providerName, ps.FundingType, ps.ScholarshipType)

	return Scholarship{
		ID:              ps.ID + 10000,
		Title:           ps.Title,
		Provider:        providerName,
		Location:        ps.Location,
		Value:           ps.Value,
		Deadline:        ps.Deadline,
		DegreeLevel:     ps.DegreeLevel,
		FundingType:     ps.FundingType,
		ScholarshipType: ps.ScholarshipType,
		Description:     desc,
		ImageURL:        imageURL,
		FieldOfStudy:    ps.FieldOfStudy,
		ExcludedRegions: nil,
		EducationLevel:  ps.EducationLevel,
		CoverageArea:    ps.CoverageArea,
		EligibilityCriteria: ps.BasicEligibilityCriteria,
		BasicEligibilityCriteria: ps.BasicEligibilityCriteria,
		Status: ps.Status,
	}, providerType
}

func toRecommendResult(s Scholarship, score int, providerType string) RecommendResult {
	tagClass := determineTagColorClass(s)

	shortDesc := s.Description
	if len(shortDesc) > 200 {
		shortDesc = shortDesc[:200]
		if idx := strings.LastIndex(shortDesc, " "); idx > 0 {
			shortDesc = shortDesc[:idx]
		}
		shortDesc += "..."
	}
	if shortDesc == "" {
		shortDesc = s.Title
	}

	return RecommendResult{
		ID:             s.ID,
		Slug:           s.Slug,
		Title:          s.Title,
		Provider:       s.Provider,
		ProviderType:   providerType,
		Coverage:       determineCoverage(s),
		Deadline:       formatDeadline(s.Deadline),
		Description:    shortDesc,
		DegreeLevel:    s.DegreeLevel,
		FundingType:    s.FundingType,
		ScholarshipType: s.ScholarshipType,
		ImageURL:       s.ImageURL,
		TagColorClass:  tagClass,
		Score:          score,
	}
}

func scoreScholarshipMath(s Scholarship, req ScholarshipRecommendRequest) int {
	total := scoreEducationLevel(s, req.EducationLevel) +
		scoreFieldOfStudy(s, req.FieldOfStudy) +
		scoreLocation(s, req.Province, req.District) +
		scoreFinancialFit(s, req.Income) +
		scoreStudyLocation(s, req.StudyLocation) +
		scoreCategoryGender(s, req.Category, req.Gender) +
		scoreGPAMatch(s, req.AcademicScoreType, req.AcademicScore) +
		scoreWillingness(s, req.WillingEssay, req.WillingInterview, req.WillingGpa)

	return int(math.Min(float64(total), 100))
}

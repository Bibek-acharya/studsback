package scholarship

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
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

var fieldAliases = map[string][]string{
	"cs":         {"computer science", "it", "information technology", "computing", "software", "programming", "data science"},
	"business":   {"management", "commerce", "finance", "economics", "accounting"},
	"engineering": {"civil", "mechanical", "electrical", "electronics"},
	"medicine":   {"medical", "health", "nursing", "pharmacy", "dentistry"},
	"arts":       {"humanities", "social science", "literature", "history", "philosophy", "fine arts", "music"},
	"law":        {"legal", "political science", "criminal justice"},
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
	Breakdown    RecommendResultBreakdown
	ProviderType string
}

type EducationEntryData struct {
	Level           string
	Stream          string
	Grade           string
	GradingSystem   string
	InstitutionName string
}

type PreferencesData struct {
	Preferences map[string]interface{} `json:"preferences"`
}

type ProfileData struct {
	EducationEntries []EducationEntryData
	Preferences      *PreferencesData
	BookmarkedFields []string
}

func (s *Service) RecommendScholarships(req ScholarshipRecommendRequest, userID *uint) ([]RecommendResult, error) {
	var profileData *ProfileData
	if userID != nil {
		pd, err := s.repo.GetUserProfileForRecommendation(*userID)
		if err == nil {
			profileData = pd
		}
	}

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
		score, breakdown, providerType := scoreScholarship(sch, req, profileData)
		scored = append(scored, scoredScholarship{
			Scholarship:  sch,
			Score:        score,
			Breakdown:    breakdown,
			ProviderType: providerType,
		})
	}

	for _, ps := range providerScholarships {
		sch, providerType := providerScholarshipToRecommendScholarship(ps)
		score, breakdown, pt := scoreScholarship(sch, req, profileData)
		if pt != "" {
			providerType = pt
		}
		scored = append(scored, scoredScholarship{
			Scholarship:  sch,
			Score:        score,
			Breakdown:    breakdown,
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
		results = append(results, toRecommendResult(sc.Scholarship, sc.Score, sc.Breakdown, sc.ProviderType))
	}

	return results, nil
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

func scoreScholarship(s Scholarship, req ScholarshipRecommendRequest, profile *ProfileData) (int, RecommendResultBreakdown, string) {
	breakdown := RecommendResultBreakdown{}

	dimValues := []struct {
		score  int
		target *int
	}{
		{scoreEducationLevel(s, req.EducationLevel), &breakdown.EducationLevel},
		{scoreFieldOfStudy(s, req.FieldOfStudy), &breakdown.FieldOfStudy},
		{scoreLocation(s, req.Province, req.District), &breakdown.Location},
		{scoreFinancialFit(s, req.Income), &breakdown.FinancialFit},
		{scoreStudyLocation(s, req.StudyLocation), &breakdown.StudyLocation},
		{scoreCategoryGender(s, req.Category, req.Gender), &breakdown.CategoryGender},
		{scoreGPAMatch(s, req.AcademicScoreType, req.AcademicScore), &breakdown.GPAMatch},
		{scoreWillingness(s, req.WillingEssay, req.WillingInterview, req.WillingGpa), &breakdown.Willingness},
		{scoreTalents(s, req.Talents), &breakdown.Talents},
		{scoreAchievements(s, req.Achievements), &breakdown.Achievements},
	}

	raw := make([]float64, len(dimValues))
	for i, d := range dimValues {
		raw[i] = float64(d.score)
		*d.target = d.score
	}

	hasProfile := profile != nil && len(profile.EducationEntries) > 0
	weights := []float64{0.15, 0.15, 0.10, 0.10, 0.05, 0.05, 0.10, 0.05, 0.05, 0.05}

	if hasProfile {
		profileScore := scoreProfileCompatibility(s, profile.EducationEntries, profile.Preferences, profile.BookmarkedFields)
		breakdown.ProfileCompatibility = profileScore
		raw = append(raw, float64(profileScore))
		weights = []float64{0.12, 0.12, 0.08, 0.08, 0.04, 0.04, 0.08, 0.03, 0.04, 0.04, 0.17}
	}

	norm := normalizePercentile(raw)
	var totalScore float64
	for i := range norm {
		totalScore += norm[i] * weights[i]
	}

	providerType := determineProviderType(s.Provider, s.FundingType, s.ScholarshipType)
	return int(math.Round(totalScore * 100)), breakdown, providerType
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
		if fuzzyMatch(schLevels, term) {
			return 30
		}
	}

	if s.EducationLevel != "" {
		eduLevel := strings.ToLower(s.EducationLevel)
		for _, term := range mapped {
			if fuzzyMatch(eduLevel, term) {
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
			if fuzzyMatch(fLower, term) {
				return 25
			}
		}
	}

	desc := strings.ToLower(s.Description)
	for _, term := range mapped {
		if fuzzyMatch(desc, term) {
			return 15
		}
	}

	title := strings.ToLower(s.Title)
	for _, term := range mapped {
		if fuzzyMatch(title, term) {
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

	if fuzzyMatch(combined, province) {
		score += 8
	}
	if district != "" && fuzzyMatch(combined, district) {
		score += 7
	}

	if province != "" {
		excluded := parseStringArray(s.ExcludedRegions)
		for _, ex := range excluded {
			if fuzzyMatch(strings.ToLower(ex), province) {
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

	isNeedBased := fuzzyMatch(combined, "need") || fuzzyMatch(combined, "need-based") ||
		fuzzyMatch(combined, "need_based") || fuzzyMatch(combined, "fully funded") ||
		fuzzyMatch(combined, "scholarship for underprivileged")
	isMeritBased := fuzzyMatch(combined, "merit") || fuzzyMatch(combined, "merit-based") ||
		fuzzyMatch(combined, "merit_based") || fuzzyMatch(combined, "excellence")

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
	isNepal := fuzzyMatch(loc, "nepal") ||
		fuzzyMatch(loc, "kathmandu") ||
		fuzzyMatch(loc, "pokhara") ||
		fuzzyMatch(loc, "lalitpur") ||
		fuzzyMatch(loc, "bhaktapur") ||
		fuzzyMatch(loc, "chitwan") ||
		fuzzyMatch(loc, "biratnagar") ||
		fuzzyMatch(loc, "butwal") ||
		fuzzyMatch(loc, "dharan") ||
		fuzzyMatch(loc, "janakpur") ||
		fuzzyMatch(loc, "nepalgunj") ||
		fuzzyMatch(loc, "hetauda")

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
		if fuzzyMatch(combined, mapped) {
			score += 3
		}
	}

	if gender == "female" {
		if fuzzyMatch(combined, "women") || fuzzyMatch(combined, "female") ||
			fuzzyMatch(combined, "girl") || fuzzyMatch(combined, "mother") {
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

func scoreTalents(s Scholarship, talents []string) int {
	if len(talents) == 0 {
		return 0
	}
	text := strings.ToLower(s.Description + " " + s.Title)
	score := 0
	talentKeywords := map[string][]string{
		"programming":    {"programming", "coding", "software", "developer", "tech"},
		"public_speaking": {"public speaking", "debate", "orator", "presentation"},
		"arts":           {"arts", "creative", "writing", "design", "music", "performing"},
		"athletics":      {"sports", "athletics", "sportsperson", "physical"},
	}
	for range talents {
		for _, kw := range talentKeywords {
			for _, word := range kw {
				if strings.Contains(text, word) {
					score += 1
					break
				}
			}
		}
	}
	if score > 5 {
		score = 5
	}
	return score
}

func scoreAchievements(s Scholarship, achievements []string) int {
	if len(achievements) == 0 {
		return 0
	}
	text := strings.ToLower(s.Description + " " + s.Title)
	score := 0
	achievementKeywords := map[string][]string{
		"academic_excellence": {"academic", "excellence", "scholar", "top", "rank"},
		"national_sports":     {"national", "sports", "athlete", "tournament"},
		"leadership":          {"leadership", "captain", "president", "head"},
		"olympiad":            {"olympiad", "science", "math", "competition"},
	}
	for range achievements {
		for _, kw := range achievementKeywords {
			for _, word := range kw {
				if strings.Contains(text, word) {
					score += 1
					break
				}
			}
		}
	}
	if score > 5 {
		score = 5
	}
	return score
}

func scoreInvolvement(s Scholarship, involvement []string) int {
	if len(involvement) == 0 {
		return 0
	}
	text := strings.ToLower(s.Description + " " + s.Title)
	score := 0
	for _, inv := range involvement {
		if strings.Contains(text, strings.ToLower(inv)) {
			score += 1
		}
	}
	if score > 3 {
		score = 3
	}
	return score
}

func scoreProfileCompatibility(s Scholarship, entries []EducationEntryData, prefs *PreferencesData, bookmarkedFields []string) int {
	score := 0

	fos := parseStringArray(s.FieldOfStudy)
	fosText := strings.ToLower(strings.Join(fos, " "))

	for _, e := range entries {
		stream := strings.ToLower(e.Stream)
		if fosText != "" && (strings.Contains(fosText, stream) || strings.Contains(stream, fosText)) {
			score += 4
		}
		if e.Grade != "" {
			grade, err := strconv.ParseFloat(e.Grade, 64)
			if err == nil {
				minGpa := extractMinGPAFromText(s.Description + " " + string(s.BasicEligibilityCriteria))
				if minGpa > 0 && grade >= minGpa {
					score += 3
				}
			}
		}
	}

	if prefs != nil && prefs.Preferences != nil {
		if fields, ok := prefs.Preferences["fields"].([]interface{}); ok {
			for _, f := range fields {
				fs := strings.ToLower(fmt.Sprintf("%v", f))
				if fosText != "" && strings.Contains(fosText, fs) {
					score += 2
				}
			}
		}
	}

	for _, bf := range bookmarkedFields {
		bfLower := strings.ToLower(bf)
		if fosText != "" && strings.Contains(fosText, bfLower) {
			score += 1
		}
	}

	if score > 20 {
		score = 20
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

func toRecommendResult(s Scholarship, score int, breakdown RecommendResultBreakdown, providerType string) RecommendResult {
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
		ID:              s.ID,
		Slug:            s.Slug,
		Title:           s.Title,
		Provider:        s.Provider,
		ProviderType:    providerType,
		Coverage:        determineCoverage(s),
		Deadline:        formatDeadline(s.Deadline),
		Description:     shortDesc,
		DegreeLevel:     s.DegreeLevel,
		FundingType:     s.FundingType,
		ScholarshipType: s.ScholarshipType,
		ImageURL:        s.ImageURL,
		TagColorClass:   tagClass,
		Score:           score,
		Breakdown:       breakdown,
	}
}

func fuzzyMatch(text, keyword string) bool {
	text = strings.ToLower(strings.TrimSpace(text))

	if strings.Contains(text, strings.ToLower(keyword)) {
		return true
	}

	if aliases, ok := fieldAliases[keyword]; ok {
		for _, alias := range aliases {
			if strings.Contains(text, alias) {
				return true
			}
		}
	}

	words := strings.Fields(text)
	stemmed := make([]string, len(words))
	for i, w := range words {
		stemmed[i] = stemWord(w)
	}
	kwStemmed := stemWord(strings.ToLower(keyword))
	for _, s := range stemmed {
		if s == kwStemmed {
			return true
		}
	}

	return false
}

func stemWord(w string) string {
	w = strings.ToLower(w)
	suffixes := []string{"'s", "s", "ing", "ed", "tion", "ment", "al"}
	for _, s := range suffixes {
		if strings.HasSuffix(w, s) && len(w)-len(s) >= 3 {
			return w[:len(w)-len(s)]
		}
	}
	return w
}

func extractMinGPAFromText(text string) float64 {
	text = strings.ToLower(text)
	re := regexp.MustCompile(`\b(\d+\.\d+)\b`)
	matches := re.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		val, _ := strconv.ParseFloat(m[1], 64)
		if val >= 2.0 && val <= 4.0 {
			return val
		}
	}
	return 0
}


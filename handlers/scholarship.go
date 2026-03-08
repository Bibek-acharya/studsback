package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
)

type detailField struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Stage       string `json:"stage,omitempty"`
	Criterion   string `json:"criterion,omitempty"`
	Name        string `json:"name,omitempty"`
	Date        string `json:"date,omitempty"`
	Event       string `json:"event,omitempty"`
	Question    string `json:"question,omitempty"`
	Answer      string `json:"answer,omitempty"`
}

type categoryMeta struct {
	ID    string
	Name  string
	Title string
	Desc  string
	Icon  string
	Color string
}

func scholarshipCategoryDefinitions() []categoryMeta {
	return []categoryMeta{
		{ID: "college", Name: "College Based", Title: "College-Based", Desc: "Direct aid from universities for enrolled students.", Icon: "fa-building-columns", Color: "blue"},
		{ID: "school", Name: "School Based", Title: "School-Based", Desc: "For students excelling in secondary education.", Icon: "fa-graduation-cap", Color: "indigo"},
		{ID: "institutional", Name: "Institutional Merit", Title: "Institutional Merit", Desc: "Awarded to students with outstanding academic achievements.", Icon: "fa-medal", Color: "emerald"},
		{ID: "need", Name: "Institutional Need", Title: "Institutional Need", Desc: "Financial aid for students demonstrating significant financial need.", Icon: "fa-hand-holding-heart", Color: "amber"},
		{ID: "entrance", Name: "Entrance", Title: "Entrance", Desc: "Scholarships for top rankers in IOE, IOM, and exams.", Icon: "fa-pencil", Color: "purple"},
		{ID: "ngo", Name: "NGO / INGO", Title: "NGO / INGO", Desc: "Supported by international and national organizations.", Icon: "fa-globe", Color: "rose"},
		{ID: "departmental", Name: "Departmental", Title: "Departmental", Desc: "Specific to faculties like Engineering, Medicine, and IT.", Icon: "fa-laptop-code", Color: "cyan"},
		{ID: "fee-waiver", Name: "Fee Waiver", Title: "Fee Waiver", Desc: "Full or partial tuition fee waivers for deserving candidates.", Icon: "fa-file-invoice-dollar", Color: "teal"},
		{ID: "research", Name: "Research", Title: "Research", Desc: "Funding support for project and thesis based study.", Icon: "fa-flask", Color: "blue"},
	}
}

func normalizeCategoryID(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(value, "college"):
		return "college"
	case strings.Contains(value, "school"):
		return "school"
	case strings.Contains(value, "institutional need") || strings.Contains(value, "need"):
		return "need"
	case strings.Contains(value, "institution") || strings.Contains(value, "merit"):
		return "institutional"
	case strings.Contains(value, "entrance"):
		return "entrance"
	case strings.Contains(value, "ngo") || strings.Contains(value, "ingo"):
		return "ngo"
	case strings.Contains(value, "department"):
		return "departmental"
	case strings.Contains(value, "waiver") || strings.Contains(value, "tuition") || strings.Contains(value, "fee"):
		return "fee-waiver"
	case strings.Contains(value, "research"):
		return "research"
	default:
		return ""
	}
}

func mapScholarshipToCategoryID(s models.Scholarship) string {
	if id := normalizeCategoryID(s.ScholarshipType); id != "" {
		return id
	}
	if id := normalizeCategoryID(s.FundingType); id != "" {
		return id
	}
	return ""
}

func isOpenStatus(status string) bool {
	return status == "OPEN" || status == "CLOSING SOON"
}

func parseStringArray(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	out := []string{}
	if err := json.Unmarshal(data, &out); err != nil {
		return []string{}
	}
	return out
}

func parseDetailFieldArray(data []byte) []detailField {
	if len(data) == 0 {
		return []detailField{}
	}
	out := []detailField{}
	if err := json.Unmarshal(data, &out); err != nil {
		return []detailField{}
	}
	return out
}

func deriveScholarshipStatus(deadline time.Time) string {
	if deadline.IsZero() {
		return "OPEN"
	}

	now := time.Now()
	if deadline.Before(now) {
		return "CLOSED"
	}
	if deadline.Before(now.AddDate(0, 0, 21)) {
		return "CLOSING SOON"
	}
	return "OPEN"
}

func toScholarshipCategory(s models.Scholarship) string {
	if s.ScholarshipType != "" {
		return s.ScholarshipType
	}
	if s.FundingType != "" {
		return s.FundingType
	}
	return "General"
}

func splitFilterValues(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetScholarships returns a list of all scholarships
func GetScholarships(c *gin.Context) {
	search := c.Query("search")
	categoryFilter := c.Query("category")
	typeFilter := c.Query("type")
	locationFilter := c.Query("location")
	levelFilter := c.Query("level")
	statusFilter := strings.ToUpper(c.Query("status"))
	sortBy := strings.ToLower(c.DefaultQuery("sort", "deadline"))
	order := strings.ToUpper(c.DefaultQuery("order", "ASC"))
	if order != "ASC" && order != "DESC" {
		order = "ASC"
	}

	query := config.GetDB().Model(&models.Scholarship{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("title ILIKE ? OR provider ILIKE ? OR description ILIKE ?", like, like, like)
	}

	if typeFilter != "" {
		like := "%" + typeFilter + "%"
		query = query.Where("funding_type ILIKE ? OR scholarship_type ILIKE ?", like, like)
	}

	if locationFilter != "" {
		locations := splitFilterValues(locationFilter)
		for _, location := range locations {
			query = query.Where("location ILIKE ?", "%"+location+"%")
		}
	}

	if levelFilter != "" {
		levels := splitFilterValues(levelFilter)
		for _, level := range levels {
			query = query.Where("degree_level ILIKE ?", "%"+level+"%")
		}
	}

	switch sortBy {
	case "latest":
		query = query.Order("created_at " + order)
	case "title":
		query = query.Order("title " + order)
	default:
		query = query.Order("deadline " + order)
	}

	var scholarships []models.Scholarship
	if err := query.Find(&scholarships).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch scholarships")
		return
	}

	var allScholarships []models.Scholarship
	if err := config.GetDB().Model(&models.Scholarship{}).Find(&allScholarships).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch scholarship category counts")
		return
	}

	categoryDefs := scholarshipCategoryDefinitions()

	categoryCounts := make(map[string]int, len(categoryDefs))
	for _, def := range categoryDefs {
		categoryCounts[def.ID] = 0
	}

	for _, scholarship := range allScholarships {
		status := deriveScholarshipStatus(scholarship.Deadline)
		if statusFilter != "" && statusFilter != status {
			continue
		}
		if statusFilter == "" && !isOpenStatus(status) {
			continue
		}

		if categoryID := mapScholarshipToCategoryID(scholarship); categoryID != "" {
			categoryCounts[categoryID]++
		}
	}

	filtered := make([]models.Scholarship, 0, len(scholarships))
	for _, scholarship := range scholarships {
		status := deriveScholarshipStatus(scholarship.Deadline)
		if statusFilter != "" && statusFilter != status {
			continue
		}

		if categoryFilter != "" {
			requestedCategoryID := normalizeCategoryID(categoryFilter)
			if requestedCategoryID != "" {
				if mapScholarshipToCategoryID(scholarship) != requestedCategoryID {
					continue
				}
			} else {
				categoryText := strings.ToLower(toScholarshipCategory(scholarship))
				if !strings.Contains(categoryText, strings.ToLower(categoryFilter)) {
					continue
				}
			}
		}

		filtered = append(filtered, scholarship)
	}

	normalized := make([]gin.H, 0, len(filtered))
	for _, scholarship := range filtered {
		logoText := "SC"
		providerParts := strings.Fields(scholarship.Provider)
		if len(providerParts) == 1 && len(providerParts[0]) > 0 {
			if len(providerParts[0]) >= 2 {
				logoText = strings.ToUpper(providerParts[0][:2])
			} else {
				logoText = strings.ToUpper(providerParts[0])
			}
		} else if len(providerParts) >= 2 {
			logoText = strings.ToUpper(string(providerParts[0][0]) + string(providerParts[1][0]))
		}

		normalized = append(normalized, gin.H{
			"id":               scholarship.ID,
			"title":            scholarship.Title,
			"provider":         scholarship.Provider,
			"logoText":         logoText,
			"logoBg":           "bg-blue-600",
			"location":         scholarship.Location,
			"type":             scholarship.FundingType,
			"amount":           scholarship.Value,
			"deadline":         scholarship.Deadline.Format("Jan 02, 2006"),
			"status":           deriveScholarshipStatus(scholarship.Deadline),
			"category":         toScholarshipCategory(scholarship),
			"description":      scholarship.Description,
			"image":            scholarship.ImageURL,
			"eligibility":      scholarship.DegreeLevel,
			"tags":             []string{toScholarshipCategory(scholarship), scholarship.DegreeLevel},
			"scholarship_type": scholarship.ScholarshipType,
			"funding_type":     scholarship.FundingType,
			"degree_level":     scholarship.DegreeLevel,
		})
	}

	categories := make([]gin.H, 0, len(categoryDefs))
	for _, def := range categoryDefs {
		count := categoryCounts[def.ID]
		categories = append(categories, gin.H{
			"id":       def.ID,
			"name":     def.Name,
			"title":    def.Title,
			"count":    count,
			"subtitle": fmt.Sprintf("%d Scholarships Open", count),
			"desc":     def.Desc,
			"icon":     def.Icon,
			"color":    def.Color,
		})
	}

	utils.SuccessResponse(c, 200, "Scholarships retrieved successfully", gin.H{
		"scholarships": normalized,
		"categories":   categories,
	})
}

// GetScholarshipByID returns details of a specific scholarship
func GetScholarshipByID(c *gin.Context) {
	id := c.Param("id")
	var scholarship models.Scholarship
	if err := config.GetDB().First(&scholarship, id).Error; err != nil {
		utils.ErrorResponse(c, 404, "Scholarship not found")
		return
	}

	response := gin.H{
		"id":                   scholarship.ID,
		"title":                scholarship.Title,
		"provider":             scholarship.Provider,
		"location":             scholarship.Location,
		"value":                scholarship.Value,
		"deadline":             scholarship.Deadline.Format("Jan 02, 2006"),
		"degree_level":         scholarship.DegreeLevel,
		"funding_type":         scholarship.FundingType,
		"scholarship_type":     scholarship.ScholarshipType,
		"description":          scholarship.Description,
		"image_url":            scholarship.ImageURL,
		"status":               deriveScholarshipStatus(scholarship.Deadline),
		"field_of_study":       parseStringArray(scholarship.FieldOfStudy),
		"selection_process":    parseDetailFieldArray(scholarship.SelectionProcess),
		"eligibility_criteria": parseDetailFieldArray(scholarship.EligibilityCriteria),
		"excluded_regions":     parseStringArray(scholarship.ExcludedRegions),
		"required_documents":   parseDetailFieldArray(scholarship.RequiredDocuments),
		"timeline":             parseDetailFieldArray(scholarship.Timeline),
		"benefits":             parseDetailFieldArray(scholarship.Benefits),
		"faqs":                 parseDetailFieldArray(scholarship.FAQs),
	}

	utils.SuccessResponse(c, 200, "Scholarship details retrieved successfully", response)
}

// GetSimilarScholarships returns related scholarships for a given scholarship
func GetSimilarScholarships(c *gin.Context) {
	id := c.Param("id")

	var current models.Scholarship
	if err := config.GetDB().First(&current, id).Error; err != nil {
		utils.ErrorResponse(c, 404, "Scholarship not found")
		return
	}

	query := config.GetDB().Model(&models.Scholarship{}).
		Where("id <> ?", current.ID).
		Where(
			"(degree_level ILIKE ? OR funding_type ILIKE ? OR scholarship_type ILIKE ? OR location ILIKE ?)",
			"%"+current.DegreeLevel+"%",
			"%"+current.FundingType+"%",
			"%"+current.ScholarshipType+"%",
			"%"+current.Location+"%",
		).
		Order("deadline ASC").
		Limit(5)

	var scholarships []models.Scholarship
	if err := query.Find(&scholarships).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch similar scholarships")
		return
	}

	// Fallback to latest open scholarships if the similarity query is too strict
	if len(scholarships) == 0 {
		if err := config.GetDB().Model(&models.Scholarship{}).
			Where("id <> ?", current.ID).
			Order("deadline ASC").
			Limit(5).
			Find(&scholarships).Error; err != nil {
			utils.ErrorResponse(c, 500, "Failed to fetch fallback similar scholarships")
			return
		}
	}

	items := make([]gin.H, 0, len(scholarships))
	for _, scholarship := range scholarships {
		items = append(items, gin.H{
			"id":           scholarship.ID,
			"title":        scholarship.Title,
			"provider":     scholarship.Provider,
			"deadline":     scholarship.Deadline.Format("Jan 02, 2006"),
			"status":       deriveScholarshipStatus(scholarship.Deadline),
			"location":     scholarship.Location,
			"funding_type": scholarship.FundingType,
			"degree_level": scholarship.DegreeLevel,
			"image_url":    scholarship.ImageURL,
			"category":     toScholarshipCategory(scholarship),
			"description":  scholarship.Description,
		})
	}

	utils.SuccessResponse(c, 200, "Similar scholarships retrieved successfully", gin.H{
		"scholarships": items,
	})
}

// ApplyScholarship handles the scholarship application submission
func ApplyScholarship(c *gin.Context) {
	scholarshipID := c.Param("id")
	userID, _ := c.Get("user_id")

	var req models.ScholarshipApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	// Parse dates
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		utils.ErrorResponse(c, 400, "Invalid date of birth format (expected YYYY-MM-DD)")
		return
	}

	// Check if scholarship exists
	var scholarship models.Scholarship
	if err := config.GetDB().First(&scholarship, scholarshipID).Error; err != nil {
		utils.ErrorResponse(c, 404, "Scholarship not found")
		return
	}

	// Check if user already applied
	var existingApp models.ScholarshipApplication
	if err := config.GetDB().Where("scholarship_id = ? AND user_id = ?", scholarshipID, userID).First(&existingApp).Error; err == nil {
		utils.ErrorResponse(c, 409, "You have already applied for this scholarship")
		return
	}

	// Marshal complex fields
	specialCircumstances, _ := json.Marshal(req.SpecialCircumstances)

	// Create application
	application := models.ScholarshipApplication{
		ScholarshipID:        scholarship.ID,
		UserID:               userID.(uint),
		NationalID:           req.NationalID,
		FirstName:            req.FirstName,
		LastName:             req.LastName,
		DateOfBirth:          dob,
		Gender:               req.Gender,
		StreetAddress:        req.StreetAddress,
		City:                 req.City,
		PostCode:             req.PostCode,
		Country:              req.Country,
		PhoneCode:            req.PhoneCode,
		PhoneNumber:          req.PhoneNumber,
		Email:                req.Email,
		LatestInstitution:    req.LatestInstitution,
		LevelCompleted:       req.LevelCompleted,
		GPAPercentage:        req.GPAPercentage,
		AnnualFamilyIncome:   req.AnnualFamilyIncome,
		PrimaryIncomeSource:  req.PrimaryIncomeSource,
		SpecialCircumstances: specialCircumstances,
		PersonalStatement:    req.PersonalStatement,
		Status:               "pending",
	}

	if err := config.GetDB().Create(&application).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to submit application")
		return
	}

	utils.SuccessResponse(c, 201, "Application submitted successfully", application)
}

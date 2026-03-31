package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
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

// GetMyScholarshipApplications retrieves all scholarship applications for the authenticated user
func GetMyScholarshipApplications(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var applications []models.ScholarshipApplication
	if err := config.GetDB().Where("user_id = ?", userID).Preload("Scholarship").Order("created_at DESC").Find(&applications).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch applications")
		return
	}

	utils.SuccessResponse(c, 200, "Applications retrieved successfully", applications)
}

// GetScholarshipApplication retrieves a single scholarship application by ID
func GetScholarshipApplication(c *gin.Context) {
	applicationID := c.Param("id")
	parsedID, err := strconv.ParseUint(applicationID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid application ID")
		return
	}

	var application models.ScholarshipApplication
	if err := config.GetDB().Preload("Scholarship").First(&application, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Application not found")
		return
	}

	userID, _ := c.Get("user_id")
	if application.UserID != userID.(uint) {
		utils.ErrorResponse(c, 403, "You can only view your own applications")
		return
	}

	utils.SuccessResponse(c, 200, "Application retrieved successfully", application)
}

// UpdateScholarshipApplication updates an existing scholarship application
func UpdateScholarshipApplication(c *gin.Context) {
	applicationID := c.Param("id")
	parsedID, err := strconv.ParseUint(applicationID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid application ID")
		return
	}

	var application models.ScholarshipApplication
	if err := config.GetDB().First(&application, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Application not found")
		return
	}

	userID, _ := c.Get("user_id")
	if application.UserID != userID.(uint) {
		utils.ErrorResponse(c, 403, "You can only update your own applications")
		return
	}

	var req models.UpdateScholarshipApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	if req.NationalID != nil {
		application.NationalID = *req.NationalID
	}
	if req.FirstName != nil {
		application.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		application.LastName = *req.LastName
	}
	if req.DateOfBirth != nil {
		if dob, err := time.Parse("2006-01-02", *req.DateOfBirth); err == nil {
			application.DateOfBirth = dob
		}
	}
	if req.Gender != nil {
		application.Gender = *req.Gender
	}
	if req.StreetAddress != nil {
		application.StreetAddress = *req.StreetAddress
	}
	if req.City != nil {
		application.City = *req.City
	}
	if req.PostCode != nil {
		application.PostCode = *req.PostCode
	}
	if req.Country != nil {
		application.Country = *req.Country
	}
	if req.PhoneCode != nil {
		application.PhoneCode = *req.PhoneCode
	}
	if req.PhoneNumber != nil {
		application.PhoneNumber = *req.PhoneNumber
	}
	if req.Email != nil {
		application.Email = *req.Email
	}
	if req.LatestInstitution != nil {
		application.LatestInstitution = *req.LatestInstitution
	}
	if req.LevelCompleted != nil {
		application.LevelCompleted = *req.LevelCompleted
	}
	if req.GPAPercentage != nil {
		application.GPAPercentage = *req.GPAPercentage
	}
	if req.AnnualFamilyIncome != nil {
		application.AnnualFamilyIncome = *req.AnnualFamilyIncome
	}
	if req.PrimaryIncomeSource != nil {
		application.PrimaryIncomeSource = *req.PrimaryIncomeSource
	}
	if req.PersonalStatement != nil {
		application.PersonalStatement = *req.PersonalStatement
	}
	if len(req.SpecialCircumstances) > 0 {
		if data, err := json.Marshal(req.SpecialCircumstances); err == nil {
			application.SpecialCircumstances = data
		}
	}

	if err := config.GetDB().Save(&application).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update application")
		return
	}

	utils.SuccessResponse(c, 200, "Application updated successfully", application)
}

// DeleteScholarshipApplication deletes a scholarship application
func DeleteScholarshipApplication(c *gin.Context) {
	applicationID := c.Param("id")
	parsedID, err := strconv.ParseUint(applicationID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid application ID")
		return
	}

	var application models.ScholarshipApplication
	if err := config.GetDB().First(&application, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Application not found")
		return
	}

	userID, _ := c.Get("user_id")
	if application.UserID != userID.(uint) {
		utils.ErrorResponse(c, 403, "You can only delete your own applications")
		return
	}

	if err := config.GetDB().Unscoped().Delete(&application).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to delete application")
		return
	}

	utils.SuccessResponse(c, 200, "Application deleted successfully", nil)
}

// GetAllScholarships retrieves all scholarships (admin only)
func GetAllScholarships(c *gin.Context) {
	var scholarships []models.Scholarship
	if err := config.GetDB().Order("created_at DESC").Find(&scholarships).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch scholarships")
		return
	}

	utils.SuccessResponse(c, 200, "Scholarships retrieved successfully", scholarships)
}

// AdminCreateScholarship creates a new scholarship (admin only)
func AdminCreateScholarship(c *gin.Context) {
	var req models.CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var deadline time.Time
	if req.Deadline != "" {
		var err error
		deadline, err = time.Parse("2006-01-02", req.Deadline)
		if err != nil {
			utils.ErrorResponse(c, 400, "Invalid deadline format (expected YYYY-MM-DD)")
			return
		}
	}

	fieldOfStudy, _ := json.Marshal(req.FieldOfStudy)

	scholarship := models.Scholarship{
		Title:           req.Title,
		Provider:        req.Provider,
		Location:        req.Location,
		Value:           req.Value,
		Deadline:        deadline,
		DegreeLevel:     req.DegreeLevel,
		FundingType:     req.FundingType,
		ScholarshipType: req.ScholarshipType,
		Description:     req.Description,
		ImageURL:        req.ImageURL,
		FieldOfStudy:    fieldOfStudy,
	}

	if err := config.GetDB().Create(&scholarship).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create scholarship")
		return
	}

	utils.SuccessResponse(c, 201, "Scholarship created successfully", scholarship)
}

// AdminUpdateScholarship updates an existing scholarship (admin only)
func AdminUpdateScholarship(c *gin.Context) {
	scholarshipID := c.Param("id")
	parsedID, err := strconv.ParseUint(scholarshipID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid scholarship ID")
		return
	}

	var scholarship models.Scholarship
	if err := config.GetDB().First(&scholarship, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Scholarship not found")
		return
	}

	var req models.CreateScholarshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	if req.Title != "" {
		scholarship.Title = req.Title
	}
	if req.Provider != "" {
		scholarship.Provider = req.Provider
	}
	if req.Location != "" {
		scholarship.Location = req.Location
	}
	if req.Value != "" {
		scholarship.Value = req.Value
	}
	if req.Deadline != "" {
		if deadline, err := time.Parse("2006-01-02", req.Deadline); err == nil {
			scholarship.Deadline = deadline
		}
	}
	if req.DegreeLevel != "" {
		scholarship.DegreeLevel = req.DegreeLevel
	}
	if req.FundingType != "" {
		scholarship.FundingType = req.FundingType
	}
	if req.ScholarshipType != "" {
		scholarship.ScholarshipType = req.ScholarshipType
	}
	if req.Description != "" {
		scholarship.Description = req.Description
	}
	if req.ImageURL != "" {
		scholarship.ImageURL = req.ImageURL
	}
	if len(req.FieldOfStudy) > 0 {
		if data, err := json.Marshal(req.FieldOfStudy); err == nil {
			scholarship.FieldOfStudy = data
		}
	}

	if err := config.GetDB().Save(&scholarship).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update scholarship")
		return
	}

	utils.SuccessResponse(c, 200, "Scholarship updated successfully", scholarship)
}

// AdminDeleteScholarship deletes a scholarship (admin only)
func AdminDeleteScholarship(c *gin.Context) {
	scholarshipID := c.Param("id")
	parsedID, err := strconv.ParseUint(scholarshipID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid scholarship ID")
		return
	}

	var scholarship models.Scholarship
	if err := config.GetDB().First(&scholarship, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Scholarship not found")
		return
	}

	if err := config.GetDB().Unscoped().Delete(&scholarship).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to delete scholarship")
		return
	}

	utils.SuccessResponse(c, 200, "Scholarship deleted successfully", nil)
}

// GetAllScholarshipApplications retrieves all scholarship applications (admin only)
func GetAllScholarshipApplications(c *gin.Context) {
	var applications []models.ScholarshipApplication
	query := config.GetDB().Preload("Scholarship").Preload("User")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&applications).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch applications")
		return
	}

	utils.SuccessResponse(c, 200, "Applications retrieved successfully", applications)
}

// AdminUpdateScholarshipApplicationStatus updates the status of a scholarship application (admin only)
func AdminUpdateScholarshipApplicationStatus(c *gin.Context) {
	applicationID := c.Param("id")
	parsedID, err := strconv.ParseUint(applicationID, 10, 64)
	if err != nil || parsedID == 0 {
		utils.ErrorResponse(c, 400, "Invalid application ID")
		return
	}

	var application models.ScholarshipApplication
	if err := config.GetDB().First(&application, uint(parsedID)).Error; err != nil {
		utils.ErrorResponse(c, 404, "Application not found")
		return
	}

	var req struct {
		Status string `json:"status" binding:"required,oneof=pending under_review approved rejected shortlisted"`
		Notes  string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	application.Status = req.Status

	if err := config.GetDB().Save(&application).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update application status")
		return
	}

	utils.SuccessResponse(c, 200, "Application status updated successfully", application)
}

// GetScholarshipApplicationsByScholarship retrieves all applications for a specific scholarship
func GetScholarshipApplicationsByScholarship(c *gin.Context) {
	scholarshipID := c.Param("scholarshipId")

	var applications []models.ScholarshipApplication
	query := config.GetDB().Where("scholarship_id = ?", scholarshipID).Preload("User")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&applications).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch applications")
		return
	}

	utils.SuccessResponse(c, 200, "Applications retrieved successfully", applications)
}

package handlers

import (
	"encoding/json"
	"strconv"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CollegeResponse struct {
	ID               uint        `json:"id"`
	UniversityID     uint        `json:"university_id"`
	CreatedAt        interface{} `json:"created_at"`
	UpdatedAt        interface{} `json:"updated_at"`
	Name             string      `json:"name"`
	FullName         string      `json:"full_name,omitempty"`
	Location         string      `json:"location"`
	Affiliation      string      `json:"affiliation"`
	CollegeType      string      `json:"type"`
	Verified         bool        `json:"verified"`
	Popular          bool        `json:"popular"`
	Rating           float64     `json:"rating"`
	Reviews          int         `json:"reviews"`
	Programs         int         `json:"programs"`
	Established      string      `json:"established,omitempty"`
	Students         string      `json:"students,omitempty"`
	Description      string      `json:"description,omitempty"`
	Website          string      `json:"website,omitempty"`
	Email            string      `json:"email,omitempty"`
	Phone            string      `json:"phone,omitempty"`
	ImageURL         string      `json:"image_url,omitempty"`
	FeaturedPrograms interface{} `json:"featured_programs,omitempty"`
	Amenities        interface{} `json:"amenities,omitempty"`
	Courses          interface{} `json:"courses,omitempty"`
	Scholarships     interface{} `json:"scholarships,omitempty"`
	Gallery          interface{} `json:"gallery,omitempty"`
	ProgramsList     interface{} `json:"programs_list,omitempty"`
	About            interface{} `json:"about,omitempty"`
	Admissions       interface{} `json:"admissions,omitempty"`
	AdmissionCards   interface{} `json:"admission_cards,omitempty"`
	OfferedPrograms  interface{} `json:"offered_programs,omitempty"`
	Alumni           interface{} `json:"alumni,omitempty"`
	Departments      interface{} `json:"departments,omitempty"`
	CollegeReviews   interface{} `json:"college_reviews,omitempty"`
}

func parseJSONField(data []byte, fallback interface{}) interface{} {
	if len(data) == 0 {
		return fallback
	}

	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fallback
	}

	return parsed
}

func buildCollegeResponse(college models.College) CollegeResponse {
	affiliation := college.Affiliation
	if college.University.ID != 0 && college.University.Name != "" {
		affiliation = college.University.Name
	}

	return CollegeResponse{
		ID:               college.ID,
		UniversityID:     college.UniversityID,
		CreatedAt:        college.CreatedAt,
		UpdatedAt:        college.UpdatedAt,
		Name:             college.Name,
		FullName:         college.FullName,
		Location:         college.Location,
		Affiliation:      affiliation,
		CollegeType:      college.CollegeType,
		Verified:         college.Verified,
		Popular:          college.Popular,
		Rating:           college.Rating,
		Reviews:          college.Reviews,
		Programs:         college.Programs,
		Established:      college.Established,
		Students:         college.Students,
		Description:      college.Description,
		Website:          college.Website,
		Email:            college.Email,
		Phone:            college.Phone,
		ImageURL:         college.ImageURL,
		FeaturedPrograms: parseJSONField(college.FeaturedPrograms, []interface{}{}),
		Amenities:        parseJSONField(college.Amenities, []interface{}{}),
		Courses:          parseJSONField(college.Courses, []interface{}{}),
		Scholarships:     parseJSONField(college.Scholarships, []interface{}{}),
		Gallery:          parseJSONField(college.Gallery, []interface{}{}),
		ProgramsList:     parseJSONField(college.ProgramsList, []interface{}{}),
		About:            parseJSONField(college.About, map[string]interface{}{}),
		Admissions:       parseJSONField(college.Admissions, map[string]interface{}{}),
		AdmissionCards:   parseJSONField(college.AdmissionCards, []interface{}{}),
		OfferedPrograms:  parseJSONField(college.OfferedPrograms, []interface{}{}),
		Alumni:           parseJSONField(college.Alumni, []interface{}{}),
		Departments:      parseJSONField(college.Departments, []interface{}{}),
		CollegeReviews:   parseJSONField(college.CollegeReviews, []interface{}{}),
	}
}

// GetColleges retrieves all colleges with optional filtering and pagination
func GetColleges(c *gin.Context) {
	var colleges []models.College
	baseQuery := config.GetDB().Model(&models.College{})

	if universityID := c.Query("universityId"); universityID != "" {
		baseQuery = baseQuery.Where("university_id = ?", universityID)
	}

	// Filter by location
	if location := c.Query("location"); location != "" {
		baseQuery = baseQuery.Where("location ILIKE ?", "%"+location+"%")
	}

	// Filter by affiliation (university)
	if affiliation := c.Query("affiliation"); affiliation != "" {
		baseQuery = baseQuery.Where("affiliation ILIKE ?", "%"+affiliation+"%")
	}

	// Filter by type (Public/Private)
	if collegeType := c.Query("type"); collegeType != "" {
		baseQuery = baseQuery.Where("college_type = ?", collegeType)
	}

	// Filter by verified status
	if verified := c.Query("verified"); verified == "true" {
		baseQuery = baseQuery.Where("verified = ?", true)
	}

	// Filter by popular status
	if popular := c.Query("popular"); popular == "true" {
		baseQuery = baseQuery.Where("popular = ?", true)
	}

	// Filter by minimum rating
	if minRating := c.Query("minRating"); minRating != "" {
		rating, err := strconv.ParseFloat(minRating, 64)
		if err == nil {
			baseQuery = baseQuery.Where("rating >= ?", rating)
		}
	}

	// Search by name
	if search := c.Query("search"); search != "" {
		baseQuery = baseQuery.Where("name ILIKE ? OR affiliation ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Sorting
	sort := c.DefaultQuery("sort", "rating")
	if sort != "rating" && sort != "name" && sort != "reviews" {
		sort = "rating"
	}

	order := c.DefaultQuery("order", "DESC")
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	// Get total count for pagination
	var total int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to count colleges")
		return
	}

	// Execute query with pagination
	fetchQuery := baseQuery.Order(sort + " " + order)
	if err := fetchQuery.Offset(offset).Limit(pageSize).Preload("University").Find(&colleges).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch colleges")
		return
	}

	collegeResponses := make([]CollegeResponse, 0, len(colleges))
	for _, college := range colleges {
		collegeResponses = append(collegeResponses, buildCollegeResponse(college))
	}

	utils.SuccessResponse(c, 200, "Colleges retrieved successfully", gin.H{
		"colleges": collegeResponses,
		"pagination": gin.H{
			"page":       page,
			"pageSize":   pageSize,
			"total":      total,
			"totalPages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// GetCollegeByID retrieves a single college by ID
func GetCollegeByID(c *gin.Context) {
	collegeID := c.Param("id")

	var college models.College
	if err := config.GetDB().Preload("University").First(&college, collegeID).Error; err != nil {
		utils.ErrorResponse(c, 404, "College not found")
		return
	}

	utils.SuccessResponse(c, 200, "College retrieved successfully", buildCollegeResponse(college))
}

// CreateCollege creates a new college (admin only)
func CreateCollege(c *gin.Context) {
	var req models.CreateCollegeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var university models.University
	if err := config.GetDB().First(&university, req.UniversityID).Error; err != nil {
		utils.ErrorResponse(c, 400, "Invalid university_id. College must be affiliated to an existing university")
		return
	}

	// Convert arrays to JSON for storage
	var featuredPrograms []byte
	var amenities []byte
	var err error

	if len(req.FeaturedPrograms) > 0 {
		featuredPrograms, err = json.Marshal(req.FeaturedPrograms)
		if err != nil {
			utils.ErrorResponse(c, 400, "Invalid featured programs format")
			return
		}
	}

	if len(req.Amenities) > 0 {
		amenities, err = json.Marshal(req.Amenities)
		if err != nil {
			utils.ErrorResponse(c, 400, "Invalid amenities format")
			return
		}
	}

	college := models.College{
		UniversityID:     req.UniversityID,
		Name:             req.Name,
		FullName:         req.FullName,
		Location:         req.Location,
		Affiliation:      university.Name,
		CollegeType:      req.CollegeType,
		Verified:         req.Verified,
		Popular:          req.Popular,
		Rating:           req.Rating,
		Reviews:          req.Reviews,
		Programs:         req.Programs,
		Established:      req.Established,
		Students:         req.Students,
		Description:      req.Description,
		Website:          req.Website,
		Email:            req.Email,
		Phone:            req.Phone,
		ImageURL:         req.ImageURL,
		FeaturedPrograms: featuredPrograms,
		Amenities:        amenities,
	}

	if err := config.GetDB().Create(&college).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to create college")
		return
	}

	if err := config.GetDB().Preload("University").First(&college, college.ID).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch created college")
		return
	}

	utils.SuccessResponse(c, 201, "College created successfully", buildCollegeResponse(college))
}

// UpdateCollege updates an existing college (admin only)
func UpdateCollege(c *gin.Context) {
	collegeID := c.Param("id")

	var college models.College
	if err := config.GetDB().Preload("University").First(&college, collegeID).Error; err != nil {
		utils.ErrorResponse(c, 404, "College not found")
		return
	}

	var req models.UpdateCollegeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	// Update fields if provided
	if req.Name != "" {
		college.Name = req.Name
	}
	if req.FullName != "" {
		college.FullName = req.FullName
	}
	if req.Location != "" {
		college.Location = req.Location
	}
	if req.Affiliation != "" {
		var byName models.University
		if err := config.GetDB().Where("LOWER(name) = LOWER(?)", req.Affiliation).First(&byName).Error; err != nil {
			utils.ErrorResponse(c, 400, "Invalid affiliation. College must be affiliated to an existing university")
			return
		}
		college.UniversityID = byName.ID
		college.Affiliation = byName.Name
	}
	if req.UniversityID != nil {
		var university models.University
		if err := config.GetDB().First(&university, *req.UniversityID).Error; err != nil {
			utils.ErrorResponse(c, 400, "Invalid university_id. College must be affiliated to an existing university")
			return
		}
		college.UniversityID = university.ID
		college.Affiliation = university.Name
	}
	if req.CollegeType != "" {
		college.CollegeType = req.CollegeType
	}
	if req.Verified != nil {
		college.Verified = *req.Verified
	}
	if req.Popular != nil {
		college.Popular = *req.Popular
	}
	if req.Rating != nil {
		college.Rating = *req.Rating
	}
	if req.Reviews != nil {
		college.Reviews = *req.Reviews
	}
	if req.Programs != nil {
		college.Programs = *req.Programs
	}
	if req.Established != "" {
		college.Established = req.Established
	}
	if req.Students != "" {
		college.Students = req.Students
	}
	if req.Description != "" {
		college.Description = req.Description
	}
	if req.Website != "" {
		college.Website = req.Website
	}
	if req.Email != "" {
		college.Email = req.Email
	}
	if req.Phone != "" {
		college.Phone = req.Phone
	}
	if req.ImageURL != "" {
		college.ImageURL = req.ImageURL
	}

	if len(req.FeaturedPrograms) > 0 {
		if data, err := json.Marshal(req.FeaturedPrograms); err == nil {
			college.FeaturedPrograms = data
		}
	}

	if len(req.Amenities) > 0 {
		if data, err := json.Marshal(req.Amenities); err == nil {
			college.Amenities = data
		}
	}

	if err := config.GetDB().Save(&college).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to update college")
		return
	}

	if err := config.GetDB().Preload("University").First(&college, college.ID).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch updated college")
		return
	}

	utils.SuccessResponse(c, 200, "College updated successfully", buildCollegeResponse(college))
}

// DeleteCollege deletes a college (admin only)
func DeleteCollege(c *gin.Context) {
	collegeID := c.Param("id")

	var college models.College
	if err := config.GetDB().First(&college, collegeID).Error; err != nil {
		utils.ErrorResponse(c, 404, "College not found")
		return
	}

	if err := config.GetDB().Delete(&college).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to delete college")
		return
	}

	utils.SuccessResponse(c, 200, "College deleted successfully", nil)
}

package handlers

import (
	"encoding/json"
	"strconv"
	"strings"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
)

type UniversityResponse struct {
	ID              uint            `json:"id"`
	Name            string          `json:"name"`
	Logo            string          `json:"logo"`
	Location        string          `json:"location"`
	Rating          float64         `json:"rating"`
	ReviewCount     int             `json:"review_count"`
	Type            string          `json:"type"`
	Rank            int             `json:"rank"`
	Verified        bool            `json:"verified"`
	IsPopular       bool            `json:"isPopular"`
	ProgramsCount   int             `json:"programsCount"`
	CollegesCount   int             `json:"collegesCount"`
	PopularPrograms []string        `json:"popularPrograms"`
	Description     string          `json:"description,omitempty"`
	Established     string          `json:"established,omitempty"`
	Students        string          `json:"students,omitempty"`
	Chancellor      string          `json:"chancellor"`
	ViceChancellor  string          `json:"vice_chancellor"`
	Founder         string          `json:"founder"`
	Website         string          `json:"website,omitempty"`
	Cover           string          `json:"cover"`
	About           json.RawMessage `json:"about"`
	Contact         json.RawMessage `json:"contact"`
	Quick           json.RawMessage `json:"quick"`
	Overview        json.RawMessage `json:"overview"`
	Leadership      json.RawMessage `json:"leadership"`
	Courses         json.RawMessage `json:"courses"`
	Programs        json.RawMessage `json:"programs"`
	Scholarships    json.RawMessage `json:"scholarships"`
	Events          json.RawMessage `json:"events"`
	News            json.RawMessage `json:"news"`
	Downloads       json.RawMessage `json:"downloads"`
	Gallery         json.RawMessage `json:"gallery"`
	Faculties       json.RawMessage `json:"faculties"`
	Admissions      json.RawMessage `json:"admissions"`
	Reviews         json.RawMessage `json:"reviews"`
}

type UniversityCollegeResponse struct {
	ID           uint    `json:"id"`
	UniversityID uint    `json:"universityId"`
	Name         string  `json:"name"`
	Logo         string  `json:"logo"`
	Rating       float64 `json:"rating"`
	Reviews      int     `json:"reviews"`
	Affiliation  string  `json:"affiliation"`
	Type         string  `json:"type"`
}

type CreateUniversityRequest struct {
	Name         string      `json:"name" binding:"required"`
	Logo         string      `json:"logo"`
	Location     string      `json:"location"`
	Type         string      `json:"type"`
	Rank         int         `json:"rank"`
	Rating       float64     `json:"rating"`
	ReviewCount  int         `json:"review_count"`
	Verified     bool        `json:"verified"`
	Popular      bool        `json:"popular"`
	Description  string      `json:"description"`
	Established    string      `json:"established"`
	Students       string      `json:"students"`
	Chancellor     string      `json:"chancellor"`
	ViceChancellor string      `json:"vice_chancellor"`
	Founder        string      `json:"founder"`
	Website        string      `json:"website"`
	Cover          string      `json:"cover"`
	About          interface{} `json:"about"`
	Contact      interface{} `json:"contact"`
	Quick        interface{} `json:"quick"`
	Overview     interface{} `json:"overview"`
	Leadership   interface{} `json:"leadership"`
	Courses      interface{} `json:"courses"`
	Programs     interface{} `json:"programs"`
	Scholarships interface{} `json:"scholarships"`
	Events       interface{} `json:"events"`
	News         interface{} `json:"news"`
	Downloads    interface{} `json:"downloads"`
	Gallery      interface{} `json:"gallery"`
	Faculties    interface{} `json:"faculties"`
	Admissions   interface{} `json:"admissions"`
	Reviews      interface{} `json:"reviews"`
}

type UpdateUniversityRequest struct {
	Name         *string     `json:"name"`
	Logo         *string     `json:"logo"`
	Location     *string     `json:"location"`
	Type         *string     `json:"type"`
	Rank         *int        `json:"rank"`
	Rating       *float64    `json:"rating"`
	ReviewCount  *int        `json:"review_count"`
	Verified     *bool       `json:"verified"`
	Popular      *bool       `json:"popular"`
	Description  *string     `json:"description"`
	Established    *string     `json:"established"`
	Students       *string     `json:"students"`
	Chancellor     *string     `json:"chancellor"`
	ViceChancellor *string     `json:"vice_chancellor"`
	Founder        *string     `json:"founder"`
	Website        *string     `json:"website"`
	Cover          *string     `json:"cover"`
	About          interface{} `json:"about"`
	Contact      interface{} `json:"contact"`
	Quick        interface{} `json:"quick"`
	Overview     interface{} `json:"overview"`
	Leadership   interface{} `json:"leadership"`
	Courses      interface{} `json:"courses"`
	Programs     interface{} `json:"programs"`
	Scholarships interface{} `json:"scholarships"`
	Events       interface{} `json:"events"`
	News         interface{} `json:"news"`
	Downloads    interface{} `json:"downloads"`
	Gallery      interface{} `json:"gallery"`
	Faculties    interface{} `json:"faculties"`
	Admissions   interface{} `json:"admissions"`
	Reviews      interface{} `json:"reviews"`
}

func toUniversityResponse(uni models.University, colleges []models.College) UniversityResponse {
	programsCount := 0
	collegesCount := len(colleges)
	ratingTotal := 0.0
	ratedCount := 0
	popularPrograms := make([]string, 0)
	seenPrograms := map[string]bool{}

	for _, college := range colleges {
		programsCount += college.Programs
		if college.Rating > 0 {
			ratingTotal += college.Rating
			ratedCount++
		}

		var featured []string
		if err := json.Unmarshal(college.FeaturedPrograms, &featured); err == nil {
			for _, program := range featured {
				name := strings.TrimSpace(program)
				if name == "" || seenPrograms[name] {
					continue
				}
				seenPrograms[name] = true
				popularPrograms = append(popularPrograms, name)
				if len(popularPrograms) >= 4 {
					break
				}
			}
		}
	}

	rating := uni.Rating
	if rating == 0 && ratedCount > 0 {
		rating = ratingTotal / float64(ratedCount)
	}

	return UniversityResponse{
		ID:              uni.ID,
		Name:            uni.Name,
		Logo:            uni.Logo,
		Location:        uni.Location,
		Rating:          rating,
		ReviewCount:     uni.ReviewCount,
		Type:            uni.Type,
		Rank:            uni.Rank,
		Verified:        uni.Verified,
		IsPopular:       uni.Popular,
		ProgramsCount:   programsCount,
		CollegesCount:   collegesCount,
		PopularPrograms: popularPrograms,
		Description:     uni.Description,
		Established:     uni.Established,
		Students:        uni.Students,
		Chancellor:      uni.Chancellor,
		ViceChancellor:  uni.ViceChancellor,
		Founder:         uni.Founder,
		Website:         uni.Website,
		Cover:           uni.Cover,
		About:           uni.About,
		Contact:         uni.Contact,
		Quick:           uni.Quick,
		Overview:        uni.Overview,
		Leadership:      uni.Leadership,
		Courses:         uni.Courses,
		Programs:        uni.Programs,
		Scholarships:    uni.Scholarships,
		Events:          uni.Events,
		News:            uni.News,
		Downloads:       uni.Downloads,
		Gallery:         uni.Gallery,
		Faculties:       uni.Faculties,
		Admissions:      uni.Admissions,
		Reviews:         uni.Reviews,
	}
}

func GetUniversities(c *gin.Context) {
	var universities []models.University
	query := config.GetDB().Model(&models.University{})

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("name ILIKE ? OR location ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if uniType := strings.TrimSpace(c.Query("type")); uniType != "" {
		query = query.Where("type = ?", uniType)
	}

	if popular := c.Query("popular"); popular == "true" {
		query = query.Where("popular = ?", true)
	}

	if err := query.Order("rank ASC").Find(&universities).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch universities")
		return
	}

	responses := make([]UniversityResponse, 0, len(universities))
	for _, uni := range universities {
		var mappingCollegeIDs []uint
		if err := config.GetDB().
			Model(&models.CollegeUniversityCourse{}).
			Where("university_id = ?", uni.ID).
			Distinct("college_id").
			Pluck("college_id", &mappingCollegeIDs).Error; err != nil {
			utils.ErrorResponse(c, 500, "Failed to fetch university college mappings")
			return
		}

		var colleges []models.College
		query := config.GetDB().Model(&models.College{})
		if len(mappingCollegeIDs) > 0 {
			query = query.Where("id IN ?", mappingCollegeIDs)
		} else {
			query = query.Where("university_id = ?", uni.ID)
		}

		if err := query.Find(&colleges).Error; err != nil {
			utils.ErrorResponse(c, 500, "Failed to fetch university colleges")
			return
		}
		responses = append(responses, toUniversityResponse(uni, colleges))
	}

	utils.SuccessResponse(c, 200, "Universities retrieved successfully", gin.H{
		"universities": responses,
	})
}

func GetUniversityByID(c *gin.Context) {
	universityID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorResponse(c, 400, "Invalid university ID")
		return
	}

	var uni models.University
	if err := config.GetDB().First(&uni, universityID).Error; err != nil {
		utils.ErrorResponse(c, 404, "University not found")
		return
	}

	var mappingCollegeIDs []uint
	if err := config.GetDB().
		Model(&models.CollegeUniversityCourse{}).
		Where("university_id = ?", uni.ID).
		Distinct("college_id").
		Pluck("college_id", &mappingCollegeIDs).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch affiliated college mappings")
		return
	}

	var colleges []models.College
	query := config.GetDB().Model(&models.College{})
	if len(mappingCollegeIDs) > 0 {
		query = query.Where("id IN ?", mappingCollegeIDs)
	} else {
		query = query.Where("university_id = ?", uni.ID)
	}

	if err := query.Find(&colleges).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to fetch affiliated colleges")
		return
	}

	collegeResponses := make([]UniversityCollegeResponse, 0, len(colleges))
	for _, college := range colleges {
		collegeResponses = append(collegeResponses, UniversityCollegeResponse{
			ID:           college.ID,
			UniversityID: college.UniversityID,
			Name:         college.Name,
			Logo:         college.ImageURL,
			Rating:       college.Rating,
			Reviews:      college.Reviews,
			Affiliation:  uni.Name,
			Type:         college.CollegeType,
		})
	}

	utils.SuccessResponse(c, 200, "University retrieved successfully", gin.H{
		"university": toUniversityResponse(uni, colleges),
		"colleges":   collegeResponses,
	})
}

func CreateUniversity(c *gin.Context) {
	var req CreateUniversityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		utils.ErrorResponse(c, 400, "name is required")
		return
	}

	uni := models.University{
		Name:        req.Name,
		Logo:        strings.TrimSpace(req.Logo),
		Location:    strings.TrimSpace(req.Location),
		Type:        strings.TrimSpace(req.Type),
		Rank:        req.Rank,
		Rating:      req.Rating,
		ReviewCount: req.ReviewCount,
		Verified:    req.Verified,
		Popular:     req.Popular,
		Description:    strings.TrimSpace(req.Description),
		Established:    strings.TrimSpace(req.Established),
		Students:       strings.TrimSpace(req.Students),
		Chancellor:     strings.TrimSpace(req.Chancellor),
		ViceChancellor: strings.TrimSpace(req.ViceChancellor),
		Founder:        strings.TrimSpace(req.Founder),
		Website:        strings.TrimSpace(req.Website),
		Cover:          strings.TrimSpace(req.Cover),
	}
	
	if req.About != nil {
		if b, err := json.Marshal(req.About); err == nil {
			uni.About = b
		}
	}
	if req.Contact != nil {
		if b, err := json.Marshal(req.Contact); err == nil {
			uni.Contact = b
		}
	}
	if req.Quick != nil {
		if b, err := json.Marshal(req.Quick); err == nil {
			uni.Quick = b
		}
	}
	if req.Overview != nil {
		if b, err := json.Marshal(req.Overview); err == nil {
			uni.Overview = b
		}
	}
	if req.Leadership != nil {
		if b, err := json.Marshal(req.Leadership); err == nil {
			uni.Leadership = b
		}
	}
	if req.Courses != nil {
		if b, err := json.Marshal(req.Courses); err == nil {
			uni.Courses = b
		}
	}
	if req.Programs != nil {
		if b, err := json.Marshal(req.Programs); err == nil {
			uni.Programs = b
		}
	}
	if req.Scholarships != nil {
		if b, err := json.Marshal(req.Scholarships); err == nil {
			uni.Scholarships = b
		}
	}
	if req.Events != nil {
		if b, err := json.Marshal(req.Events); err == nil {
			uni.Events = b
		}
	}
	if req.News != nil {
		if b, err := json.Marshal(req.News); err == nil {
			uni.News = b
		}
	}
	if req.Downloads != nil {
		if b, err := json.Marshal(req.Downloads); err == nil {
			uni.Downloads = b
		}
	}
	if req.Gallery != nil {
		if b, err := json.Marshal(req.Gallery); err == nil {
			uni.Gallery = b
		}
	}
	if req.Faculties != nil {
		if b, err := json.Marshal(req.Faculties); err == nil {
			uni.Faculties = b
		}
	}
	if req.Admissions != nil {
		if b, err := json.Marshal(req.Admissions); err == nil {
			uni.Admissions = b
		}
	}
	if req.Reviews != nil {
		if b, err := json.Marshal(req.Reviews); err == nil {
			uni.Reviews = b
		}
	}

	if err := config.GetDB().Create(&uni).Error; err != nil {
		utils.ErrorResponse(c, 400, "Failed to create university: "+err.Error())
		return
	}

	utils.SuccessResponse(c, 201, "University created successfully", gin.H{
		"university": toUniversityResponse(uni, []models.College{}),
	})
}

func UpdateUniversity(c *gin.Context) {
	universityID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorResponse(c, 400, "Invalid university ID")
		return
	}

	var req UpdateUniversityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, 400, err.Error())
		return
	}

	var uni models.University
	if err := config.GetDB().First(&uni, universityID).Error; err != nil {
		utils.ErrorResponse(c, 404, "University not found")
		return
	}

	if req.Name != nil {
		uni.Name = strings.TrimSpace(*req.Name)
	}
	if req.Logo != nil {
		uni.Logo = strings.TrimSpace(*req.Logo)
	}
	if req.Location != nil {
		uni.Location = strings.TrimSpace(*req.Location)
	}
	if req.Type != nil {
		uni.Type = strings.TrimSpace(*req.Type)
	}
	if req.Rank != nil {
		uni.Rank = *req.Rank
	}
	if req.Rating != nil {
		uni.Rating = *req.Rating
	}
	if req.ReviewCount != nil {
		uni.ReviewCount = *req.ReviewCount
	}
	if req.Verified != nil {
		uni.Verified = *req.Verified
	}
	if req.Popular != nil {
		uni.Popular = *req.Popular
	}
	if req.Description != nil {
		uni.Description = strings.TrimSpace(*req.Description)
	}
	if req.Established != nil {
		uni.Established = strings.TrimSpace(*req.Established)
	}
	if req.Students != nil {
		uni.Students = strings.TrimSpace(*req.Students)
	}
	if req.Chancellor != nil {
		uni.Chancellor = strings.TrimSpace(*req.Chancellor)
	}
	if req.ViceChancellor != nil {
		uni.ViceChancellor = strings.TrimSpace(*req.ViceChancellor)
	}
	if req.Founder != nil {
		uni.Founder = strings.TrimSpace(*req.Founder)
	}
	if req.Website != nil {
		uni.Website = strings.TrimSpace(*req.Website)
	}
	if req.Cover != nil {
		uni.Cover = strings.TrimSpace(*req.Cover)
	}

	if req.About != nil {
		if b, err := json.Marshal(req.About); err == nil {
			uni.About = b
		}
	}
	if req.Contact != nil {
		if b, err := json.Marshal(req.Contact); err == nil {
			uni.Contact = b
		}
	}
	if req.Quick != nil {
		if b, err := json.Marshal(req.Quick); err == nil {
			uni.Quick = b
		}
	}
	if req.Overview != nil {
		if b, err := json.Marshal(req.Overview); err == nil {
			uni.Overview = b
		}
	}
	if req.Leadership != nil {
		if b, err := json.Marshal(req.Leadership); err == nil {
			uni.Leadership = b
		}
	}
	if req.Courses != nil {
		if b, err := json.Marshal(req.Courses); err == nil {
			uni.Courses = b
		}
	}
	if req.Programs != nil {
		if b, err := json.Marshal(req.Programs); err == nil {
			uni.Programs = b
		}
	}
	if req.Scholarships != nil {
		if b, err := json.Marshal(req.Scholarships); err == nil {
			uni.Scholarships = b
		}
	}
	if req.Events != nil {
		if b, err := json.Marshal(req.Events); err == nil {
			uni.Events = b
		}
	}
	if req.News != nil {
		if b, err := json.Marshal(req.News); err == nil {
			uni.News = b
		}
	}
	if req.Downloads != nil {
		if b, err := json.Marshal(req.Downloads); err == nil {
			uni.Downloads = b
		}
	}
	if req.Gallery != nil {
		if b, err := json.Marshal(req.Gallery); err == nil {
			uni.Gallery = b
		}
	}
	if req.Faculties != nil {
		if b, err := json.Marshal(req.Faculties); err == nil {
			uni.Faculties = b
		}
	}
	if req.Admissions != nil {
		if b, err := json.Marshal(req.Admissions); err == nil {
			uni.Admissions = b
		}
	}
	if req.Reviews != nil {
		if b, err := json.Marshal(req.Reviews); err == nil {
			uni.Reviews = b
		}
	}

	if strings.TrimSpace(uni.Name) == "" {
		utils.ErrorResponse(c, 400, "name is required")
		return
	}

	if err := config.GetDB().Save(&uni).Error; err != nil {
		utils.ErrorResponse(c, 400, "Failed to update university: "+err.Error())
		return
	}

	utils.SuccessResponse(c, 200, "University updated successfully", gin.H{
		"university": toUniversityResponse(uni, []models.College{}),
	})
}

func DeleteUniversity(c *gin.Context) {
	universityID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.ErrorResponse(c, 400, "Invalid university ID")
		return
	}

	var uni models.University
	if err := config.GetDB().First(&uni, universityID).Error; err != nil {
		utils.ErrorResponse(c, 404, "University not found")
		return
	}

	if err := config.GetDB().Unscoped().Delete(&uni).Error; err != nil {
		utils.ErrorResponse(c, 500, "Failed to delete university")
		return
	}

	utils.SuccessResponse(c, 200, "University deleted successfully", nil)
}

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
	ID              uint     `json:"id"`
	Name            string   `json:"name"`
	Logo            string   `json:"logo"`
	Location        string   `json:"location"`
	Rating          float64  `json:"rating"`
	Type            string   `json:"type"`
	Rank            int      `json:"rank"`
	IsPopular       bool     `json:"isPopular"`
	ProgramsCount   int      `json:"programsCount"`
	CollegesCount   int      `json:"collegesCount"`
	PopularPrograms []string `json:"popularPrograms"`
	Description     string   `json:"description,omitempty"`
	Established     string   `json:"established,omitempty"`
	Website         string   `json:"website,omitempty"`
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

	rating := 0.0
	if ratedCount > 0 {
		rating = ratingTotal / float64(ratedCount)
	}

	return UniversityResponse{
		ID:              uni.ID,
		Name:            uni.Name,
		Logo:            uni.Logo,
		Location:        uni.Location,
		Rating:          rating,
		Type:            uni.Type,
		Rank:            uni.Rank,
		IsPopular:       uni.Popular,
		ProgramsCount:   programsCount,
		CollegesCount:   collegesCount,
		PopularPrograms: popularPrograms,
		Description:     uni.Description,
		Established:     uni.Established,
		Website:         uni.Website,
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
		var colleges []models.College
		if err := config.GetDB().Where("university_id = ?", uni.ID).Find(&colleges).Error; err != nil {
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

	var colleges []models.College
	if err := config.GetDB().Where("university_id = ?", uni.ID).Find(&colleges).Error; err != nil {
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

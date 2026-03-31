package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
)

type EducationCourseResponse struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	ShortTitle  string   `json:"shortTitle"`
	Colleges    int      `json:"colleges"`
	Affiliation string   `json:"affiliation"`
	Badges      []string `json:"badges"`
	Level       string   `json:"level"`
	Field       string   `json:"field"`
	Duration    string   `json:"duration"`
	EstFee      string   `json:"estFee"`
	Highlights  []string `json:"highlights"`
	CareerPath  string   `json:"careerPath"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	GovtFee     string   `json:"govtFee"`
	PrivateFee  string   `json:"privateFee"`
}

type CourseCurriculumSemester struct {
	Semester int      `json:"semester"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle"`
	Subjects []string `json:"subjects"`
}

type CourseCareerOpportunity struct {
	Title string `json:"title"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

type CourseContactSupport struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type CourseOtherProgram struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Duration string `json:"duration"`
	Faculty  string `json:"faculty"`
}

type EducationCourseDetailsResponse struct {
	Course                EducationCourseResponse    `json:"course"`
	About                 []string                   `json:"about"`
	Mode                  string                     `json:"mode"`
	DegreeLabel           string                     `json:"degreeLabel"`
	Curriculum            []CourseCurriculumSemester `json:"curriculum"`
	AdmissionRequirements []string                   `json:"admissionRequirements"`
	CareerOpportunities   []CourseCareerOpportunity  `json:"careerOpportunities"`
	Universities          []string                   `json:"universities"`
	Contact               CourseContactSupport       `json:"contact"`
	OtherPrograms         []CourseOtherProgram       `json:"otherPrograms"`
	HighlightsUniversity  string                     `json:"highlightsUniversity"`
	HighlightsFaculty     string                     `json:"highlightsFaculty"`
	HighlightsDuration    string                     `json:"highlightsDuration"`
	HighlightsDegreeLevel string                     `json:"highlightsDegreeLevel"`
	OfferingCollegesCount int                        `json:"offeringCollegesCount"`
}

func parseStringArrayField(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}

	var parsed []string
	if err := json.Unmarshal(data, &parsed); err == nil {
		return parsed
	}

	var dynamicParsed []interface{}
	if err := json.Unmarshal(data, &dynamicParsed); err != nil {
		return []string{}
	}

	result := make([]string, 0, len(dynamicParsed))
	for _, item := range dynamicParsed {
		if value, ok := item.(string); ok {
			result = append(result, value)
		}
	}

	return result
}

func buildEducationCourseResponse(course models.Course) EducationCourseResponse {
	return EducationCourseResponse{
		ID:          strconv.FormatUint(uint64(course.ID), 10),
		Title:       course.Title,
		ShortTitle:  course.ShortTitle,
		Colleges:    course.CollegesCount,
		Affiliation: course.Affiliation,
		Badges:      parseStringArrayField(course.Badges),
		Level:       course.Level,
		Field:       course.Field,
		Duration:    course.Duration,
		EstFee:      course.EstFee,
		Highlights:  parseStringArrayField(course.Highlights),
		CareerPath:  course.CareerPath,
		Description: course.Description,
		Location:    course.Location,
		GovtFee:     course.GovtFee,
		PrivateFee:  course.PrivateFee,
	}
}

// Helper to convert interface to []byte for JSONB storage
func ToJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func GetEducationRankings(c *gin.Context) {
	var colleges []models.College
	// In a real scenario, we might have a specific Ranking model or just fetch popularized colleges
	if err := config.GetDB().Preload("University").Order("rating desc").Limit(10).Find(&colleges).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch rankings")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Education rankings retrieved successfully", gin.H{"colleges": colleges})
}

func GetEducationExams(c *gin.Context) {
	var exams []models.Exam
	if err := config.GetDB().Find(&exams).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch exams")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Education exams retrieved successfully", gin.H{"exams": exams})
}

func GetEducationExamByID(c *gin.Context) {
	id := c.Param("id")
	var exam models.Exam
	if err := config.GetDB().Where("id = ? OR slug = ?", id, id).First(&exam).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Exam not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Education exam retrieved successfully", exam)
}

func GetEducationCourses(c *gin.Context) {
	var courses []models.Course
	if err := config.GetDB().Find(&courses).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	courseResponses := make([]EducationCourseResponse, 0, len(courses))
	for _, course := range courses {
		response := buildEducationCourseResponse(course)

		var offeringCount int64
		if err := config.GetDB().
			Model(&models.CollegeUniversityCourse{}).
			Distinct("college_id").
			Where("course_id = ?", course.ID).
			Count(&offeringCount).Error; err == nil && offeringCount > 0 {
			response.Colleges = int(offeringCount)
		}

		courseResponses = append(courseResponses, response)
	}

	utils.SuccessResponse(c, http.StatusOK, "Education courses retrieved successfully", gin.H{"courses": courseResponses})
}

func GetEducationCourseByID(c *gin.Context) {
	id := c.Param("id")
	var course models.Course
	if err := config.GetDB().First(&course, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Course not found")
		return
	}

	response := buildEducationCourseResponse(course)
	var offeringCount int64
	if err := config.GetDB().
		Model(&models.CollegeUniversityCourse{}).
		Distinct("college_id").
		Where("course_id = ?", course.ID).
		Count(&offeringCount).Error; err == nil && offeringCount > 0 {
		response.Colleges = int(offeringCount)
	}

	utils.SuccessResponse(c, http.StatusOK, "Education course retrieved successfully", response)
}

func GetEducationCourseDetailsByID(c *gin.Context) {
	id := c.Param("id")
	var course models.Course
	if err := config.GetDB().First(&course, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Course not found")
		return
	}

	baseCourse := buildEducationCourseResponse(course)
	var offeringCount int64
	if err := config.GetDB().
		Model(&models.CollegeUniversityCourse{}).
		Distinct("college_id").
		Where("course_id = ?", course.ID).
		Count(&offeringCount).Error; err == nil && offeringCount > 0 {
		baseCourse.Colleges = int(offeringCount)
	}

	var relatedCourses []models.Course
	_ = config.GetDB().
		Where("id <> ?", course.ID).
		Where("field = ? OR level = ?", course.Field, course.Level).
		Order("id asc").
		Limit(3).
		Find(&relatedCourses).Error

	if len(relatedCourses) < 3 {
		var fallbackCourses []models.Course
		_ = config.GetDB().
			Where("id <> ?", course.ID).
			Order("id asc").
			Limit(3 - len(relatedCourses)).
			Find(&fallbackCourses).Error
		relatedCourses = append(relatedCourses, fallbackCourses...)
	}

	otherPrograms := make([]CourseOtherProgram, 0, len(relatedCourses))
	for _, related := range relatedCourses {
		otherPrograms = append(otherPrograms, CourseOtherProgram{
			ID:       strconv.FormatUint(uint64(related.ID), 10),
			Title:    related.Title,
			Duration: related.Duration,
			Faculty:  related.Field,
		})
	}

	var mappings []models.CollegeUniversityCourse
	_ = config.GetDB().
		Where("course_id = ?", course.ID).
		Order("college_id asc").
		Find(&mappings).Error

	collegeIDSet := map[uint]bool{}
	collegeIDs := make([]uint, 0)
	for _, mapping := range mappings {
		if !collegeIDSet[mapping.CollegeID] {
			collegeIDSet[mapping.CollegeID] = true
			collegeIDs = append(collegeIDs, mapping.CollegeID)
		}
	}

	var colleges []models.College
	if len(collegeIDs) > 0 {
		_ = config.GetDB().
			Where("id IN ?", collegeIDs).
			Order("rating desc").
			Find(&colleges).Error
	}

	universitiesMap := map[string]bool{}
	universities := make([]string, 0, 4)
	for _, mapping := range mappings {
		var university models.University
		if err := config.GetDB().First(&university, mapping.UniversityID).Error; err != nil {
			continue
		}
		if university.Name == "" {
			continue
		}
		if !universitiesMap[university.Name] {
			universitiesMap[university.Name] = true
			universities = append(universities, university.Name)
		}
	}
	if len(universities) == 0 && course.Affiliation != "" {
		universities = append(universities, course.Affiliation)
	}

	contact := CourseContactSupport{}
	if len(colleges) > 0 {
		contact.Email = colleges[0].Email
		contact.Phone = colleges[0].Phone
	}
	if contact.Email == "" {
		contact.Email = "info@studsphere.com"
	}
	if contact.Phone == "" {
		contact.Phone = "+977-1-0000000"
	}

	curriculum := make([]CourseCurriculumSemester, 0)
	if len(course.Curriculum) > 0 {
		_ = json.Unmarshal(course.Curriculum, &curriculum)
	}
	if len(curriculum) == 0 {
		curriculum = []CourseCurriculumSemester{
			{Semester: 1, Title: "Semester 1", Subtitle: "Core Technical Knowledge", Subjects: []string{"Introduction to Programming", "Applied Mathematics", "Digital Logic"}},
			{Semester: 2, Title: "Semester 2", Subtitle: "Foundation Building", Subjects: []string{"Object Oriented Programming", "Statistics", "Database Fundamentals"}},
		}
	}

	admissionRequirements := parseStringArrayField(course.Admissions)
	if len(admissionRequirements) == 0 {
		admissionRequirements = []string{
			"Mark sheets / Transcripts",
			"ID / Citizenship Proof",
			"Passport-size Photos",
			"Transfer / Character Certificate",
		}
	}

	careers := make([]CourseCareerOpportunity, 0)
	if len(course.Careers) > 0 {
		_ = json.Unmarshal(course.Careers, &careers)
	}
	if len(careers) == 0 {
		careers = []CourseCareerOpportunity{
			{Title: "Data Scientist", Icon: "database", Color: "blue"},
			{Title: "AI Engineer", Icon: "cpu", Color: "emerald"},
			{Title: "Machine Learning Engineer", Icon: "chart", Color: "purple"},
		}
	}

	about := parseStringArrayField(course.About)
	if len(about) == 0 {
		about = []string{
			"The program is designed to bridge the gap between theoretical mathematics and practical engineering.",
			"Graduates are prepared for modern technology roles through project-based learning and labs.",
		}
	}

	response := EducationCourseDetailsResponse{
		Course: baseCourse,
		About:  about,
		Mode: func() string {
			if course.Mode != "" {
				return course.Mode
			}
			return "On-Campus"
		}(),
		DegreeLabel: func() string {
			if course.DegreeLabel != "" {
				return course.DegreeLabel
			}
			return "Bachelor's Degree"
		}(),
		Curriculum:            curriculum,
		AdmissionRequirements: admissionRequirements,
		CareerOpportunities:   careers,
		Universities:          universities,
		Contact:               contact,
		OtherPrograms:         otherPrograms,
		HighlightsUniversity:  course.Affiliation,
		HighlightsFaculty:     course.Field,
		HighlightsDuration:    course.Duration,
		HighlightsDegreeLevel: course.Level,
		OfferingCollegesCount: baseCourse.Colleges,
	}

	utils.SuccessResponse(c, http.StatusOK, "Education course details retrieved successfully", response)
}

func GetEducationAdmissions(c *gin.Context) {
	var colleges []models.College
	// Fetch colleges with admission cards or specific marks
	if err := config.GetDB().Where("verified = ?", true).Limit(6).Find(&colleges).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch admissions")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Education admissions retrieved successfully", gin.H{"colleges": colleges})
}

func GetEducationNews(c *gin.Context) {
	var news []models.News
	if err := config.GetDB().Order("created_at desc").Limit(10).Find(&news).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch news")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "News retrieved successfully", gin.H{"news": news})
}

func GetEducationEvents(c *gin.Context) {
	var events []models.Event
	if err := config.GetDB().Order("date asc").Find(&events).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch events")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Events retrieved successfully", gin.H{"events": events})
}

func GetEducationNewsByID(c *gin.Context) {
	id := c.Param("id")
	var news models.News
	if err := config.GetDB().First(&news, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "News article not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "News article retrieved successfully", news)
}

func GetEducationEventByID(c *gin.Context) {
	id := c.Param("id")
	var event models.Event
	if err := config.GetDB().First(&event, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Event not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Event retrieved successfully", event)
}

func GetEducationBlogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	category := c.Query("category")
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := config.GetDB().Model(&models.Blog{}).Where("published = ?", true)

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		query = query.Where("title ILIKE ? OR excerpt ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var blogs []models.Blog
	if err := query.Order("featured desc, created_at desc").Offset(offset).Limit(limit).Find(&blogs).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to fetch blogs")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Blogs retrieved successfully", gin.H{
		"blogs": blogs,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
			"pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

func GetEducationBlogByID(c *gin.Context) {
	id := c.Param("id")
	var blog models.Blog

	query := config.GetDB().Where("published = ?", true)
	if err := query.First(&blog, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Blog post not found")
		return
	}

	config.GetDB().Model(&blog).Update("views", blog.Views+1)

	var relatedBlogs []models.Blog
	config.GetDB().Where("published = ? AND id <> ? AND category = ?", true, blog.ID, blog.Category).
		Order("created_at desc").Limit(3).Find(&relatedBlogs)

	utils.SuccessResponse(c, http.StatusOK, "Blog post retrieved successfully", gin.H{
		"blog":    blog,
		"related": relatedBlogs,
	})
}

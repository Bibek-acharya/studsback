package education

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"studsphere/backend/internal/shared/utils"

	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
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

func buildExamResponse(exam Exam) ExamResponse {
	return ExamResponse{
		ID:           exam.ID,
		Slug:         exam.Slug,
		Title:        exam.Title,
		Board:        exam.Board,
		Badges:       parseStringArrayField(exam.Badges),
		Level:        exam.Level,
		Type:         exam.Type,
		ExamDate:     exam.ExamDate,
		FormDeadline: exam.FormDeadline,
		Fee:          exam.Fee,
		Highlights:   parseStringArrayField(exam.Highlights),
		Description:  exam.Description,
		Status:       exam.Status,
		ImageUrl:     exam.ImageUrl,
		University:   exam.University,
		Faculty:      exam.Faculty,
		NepaliDate:   exam.NepaliDate,
		Overview:     exam.Overview,
		Weightage:    exam.Weightage,
		Timeline:     exam.Timeline,
		Notices:      exam.Notices,
		Faqs:         exam.Faqs,
	}
}

func buildCourseResponse(course Course, colleges int) CourseResponse {
	return CourseResponse{
		ID:          strconv.FormatUint(uint64(course.ID), 10),
		Title:       course.Title,
		ShortTitle:  course.ShortTitle,
		Colleges:    colleges,
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

func buildNewsResponse(news News) NewsResponse {
	return NewsResponse{
		ID:       news.ID,
		Category: news.Category,
		Title:    news.Title,
		Excerpt:  news.Excerpt,
		Content:  news.Content,
		Image:    news.Image,
		Author:   news.Author,
		Date:     news.Date,
		ReadTime: news.ReadTime,
		Source:   news.Source,
		Tags:     parseStringArrayField(news.Tags),
	}
}

func buildEventResponse(event Event) EventResponse {
	return EventResponse{
		ID:              event.ID,
		Title:           event.Title,
		Excerpt:         event.Excerpt,
		Description:     event.Description,
		Category:        event.Category,
		Organizer:       event.Organizer,
		Location:        event.Location,
		Date:            event.Date,
		Time:            event.Time,
		RegistrationFee: event.RegistrationFee,
		Image:           event.Image,
		Interested:      event.Interested,
		Trending:        event.Trending,
		Featured:        event.Featured,
	}
}

func buildBlogResponse(blog Blog) BlogResponse {
	return BlogResponse{
		ID:        blog.ID,
		Title:     blog.Title,
		Slug:      blog.Slug,
		Excerpt:   blog.Excerpt,
		Content:   blog.Content,
		Image:     blog.Image,
		Author:    blog.Author,
		Category:  blog.Category,
		Tags:      parseStringArrayField(blog.Tags),
		ReadTime:  blog.ReadTime,
		Featured:  blog.Featured,
		Published: blog.Published,
		Views:     blog.Views,
		CreatedAt: blog.CreatedAt.String(),
	}
}

func (s *Service) GetEducationRankings() ([]College, error) {
	return s.repo.FindTopRatedColleges(10)
}

func (s *Service) GetEducationExams() ([]ExamResponse, error) {
	exams, err := s.repo.FindExams()
	if err != nil {
		return nil, err
	}

	responses := make([]ExamResponse, 0, len(exams))
	for _, exam := range exams {
		responses = append(responses, buildExamResponse(exam))
	}
	return responses, nil
}

func (s *Service) GetEducationExamByID(id string) (*ExamResponse, error) {
	exam, err := s.repo.FindExamByID(id)
	if err != nil {
		return nil, err
	}
	resp := buildExamResponse(*exam)
	return &resp, nil
}

func (s *Service) GetEducationCourses() ([]CourseResponse, error) {
	courses, err := s.repo.FindCourses()
	if err != nil {
		return nil, err
	}

	responses := make([]CourseResponse, 0, len(courses))
	for _, course := range courses {
		colleges := course.CollegesCount
		count, err := s.repo.CountCourseOfferingColleges(course.ID)
		if err == nil && count > 0 {
			colleges = int(count)
		}
		responses = append(responses, buildCourseResponse(course, colleges))
	}

	instPrograms, _ := s.repo.FindPublishedInstitutionPrograms("", "")
	for _, p := range instPrograms {
		responses = append(responses, CourseResponse{
			ID:              fmt.Sprintf("inst-%d", p.ID),
			Title:           p.ProgramName,
			Affiliation:     p.InstitutionName,
			Duration:        p.Duration,
			EstFee:          p.Fee,
			Description:     p.Description,
			Location:        p.InstitutionLocation,
			Source:          "institution",
			InstitutionName: p.InstitutionName,
			Image:           p.BannerURL,
		})
	}

	return responses, nil
}

func (s *Service) GetEducationCoursesPaginated(page, limit int, search, level, field, affiliation string) ([]CourseResponse, PaginationMeta, error) {
	allCourses, _, err := s.repo.FindCoursesFiltered(1, 9999, search, level, field, affiliation)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	allResponses := make([]CourseResponse, 0, len(allCourses))
	for _, course := range allCourses {
		colleges := course.CollegesCount
		count, err := s.repo.CountCourseOfferingColleges(course.ID)
		if err == nil && count > 0 {
			colleges = int(count)
		}
		allResponses = append(allResponses, buildCourseResponse(course, colleges))
	}

	instPrograms, _ := s.repo.FindPublishedInstitutionPrograms(search, level)
	for _, p := range instPrograms {
		allResponses = append(allResponses, CourseResponse{
			ID:              fmt.Sprintf("inst-%d", p.ID),
			Title:           p.ProgramName,
			Affiliation:     p.InstitutionName,
			Duration:        p.Duration,
			EstFee:          p.Fee,
			Description:     p.Description,
			Location:        p.InstitutionLocation,
			Source:          "institution",
			InstitutionName: p.InstitutionName,
			Image:           p.BannerURL,
		})
	}

	total := int64(len(allResponses))
	pages := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		pages = 0
	}

	start := (page - 1) * limit
	if start > int(total) {
		return []CourseResponse{}, PaginationMeta{Total: total, Page: page, Limit: limit, Pages: pages}, nil
	}
	end := start + limit
	if end > int(total) {
		end = int(total)
	}

	meta := PaginationMeta{
		Total: total,
		Page:  page,
		Limit: limit,
		Pages: pages,
	}

	return allResponses[start:end], meta, nil
}

func (s *Service) GetCourseFilterCounts() (*CourseFilterCounts, error) {
	return s.repo.GetCourseFilterCounts()
}

func (s *Service) GetEducationCourseByID(id string) (*CourseResponse, error) {
	if program, err := s.repo.FindPublishedInstitutionProgramByID(id); err == nil && program != nil {
		return &CourseResponse{
			ID:              fmt.Sprintf("inst-%d", program.ID),
			Title:           program.ProgramName,
			Affiliation:     program.InstitutionName,
			Duration:        program.Duration,
			EstFee:          program.Fee,
			Description:     program.Description,
			Location:        program.InstitutionLocation,
			Source:          "institution",
			InstitutionName: program.InstitutionName,
			Image:           program.BannerURL,
		}, nil
	}

	course, err := s.repo.FindCourseByID(id)
	if err != nil {
		return nil, err
	}

	colleges := course.CollegesCount
	count, err := s.repo.CountCourseOfferingColleges(course.ID)
	if err == nil && count > 0 {
		colleges = int(count)
	}

	resp := buildCourseResponse(*course, colleges)
	return &resp, nil
}

func (s *Service) GetEducationCourseDetailsByID(id string) (*CourseDetailsResponse, error) {
	if program, err := s.repo.FindPublishedInstitutionProgramByID(id); err == nil && program != nil {
		return &CourseDetailsResponse{
			Course: CourseResponse{
				ID:              fmt.Sprintf("inst-%d", program.ID),
				Title:           program.ProgramName,
				Affiliation:     program.InstitutionName,
				Duration:        program.Duration,
				EstFee:          program.Fee,
				Description:     program.Description,
				Location:        program.InstitutionLocation,
				Source:          "institution",
				InstitutionName: program.InstitutionName,
				Image:           program.BannerURL,
			},
			About:                 []string{program.Description},
			Mode:                  "On-Campus",
			DegreeLabel:           "Program",
			AdmissionRequirements: []string{"As per institution criteria"},
			Universities:          []string{program.InstitutionName},
		}, nil
	}

	course, err := s.repo.FindCourseByID(id)
	if err != nil {
		return nil, err
	}

	colleges := course.CollegesCount
	count, err := s.repo.CountCourseOfferingColleges(course.ID)
	if err == nil && count > 0 {
		colleges = int(count)
	}

	baseCourse := buildCourseResponse(*course, colleges)

	relatedCourses, err := s.repo.FindRelatedCourses(course.ID, course.Field, course.Level, 3)
	if err != nil {
		relatedCourses = []Course{}
	}

	if len(relatedCourses) < 3 {
		fallbackCourses, err := s.repo.FindFallbackCourses(course.ID, 3-len(relatedCourses))
		if err == nil {
			relatedCourses = append(relatedCourses, fallbackCourses...)
		}
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

	mappings, err := s.repo.FindCourseMappings(course.ID)
	if err != nil {
		mappings = []CollegeUniversityCourse{}
	}

	collegeIDSet := map[uint]bool{}
	collegeIDs := make([]uint, 0)
	for _, mapping := range mappings {
		if !collegeIDSet[mapping.CollegeID] {
			collegeIDSet[mapping.CollegeID] = true
			collegeIDs = append(collegeIDs, mapping.CollegeID)
		}
	}

	collegesList, err := s.repo.FindCollegesByIDs(collegeIDs)
	if err != nil {
		collegesList = []College{}
	}

	universitiesMap := map[string]bool{}
	universities := make([]string, 0, 4)
	for _, mapping := range mappings {
		university, err := s.repo.FindUniversityByID(mapping.UniversityID)
		if err != nil {
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
	if len(collegesList) > 0 {
		contact.Email = collegesList[0].Email
		contact.Phone = collegesList[0].Phone
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

	mode := "On-Campus"
	if course.Mode != "" {
		mode = course.Mode
	}

	degreeLabel := "Bachelor's Degree"
	if course.DegreeLabel != "" {
		degreeLabel = course.DegreeLabel
	}

	response := &CourseDetailsResponse{
		Course:                baseCourse,
		About:                 about,
		Mode:                  mode,
		DegreeLabel:           degreeLabel,
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

	return response, nil
}

func (s *Service) GetEducationNews() ([]NewsResponse, error) {
	news, err := s.repo.FindNews(10)
	if err != nil {
		return nil, err
	}

	responses := make([]NewsResponse, 0, len(news))
	for _, n := range news {
		responses = append(responses, buildNewsResponse(n))
	}
	return responses, nil
}

func (s *Service) GetEducationNewsByID(id string) (*NewsResponse, error) {
	news, err := s.repo.FindNewsByID(id)
	if err != nil {
		return nil, err
	}
	resp := buildNewsResponse(*news)
	return &resp, nil
}

func (s *Service) GetEducationNewsFiltered(page, limit int, category, search, sort string) ([]NewsResponse, PaginationMeta, error) {
	news, total, err := s.repo.FindNewsFiltered(page, limit, category, search, sort)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	pages := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		pages = 0
	}

	meta := PaginationMeta{
		Total: total,
		Page:  page,
		Limit: limit,
		Pages: pages,
	}

	responses := make([]NewsResponse, 0, len(news))
	for _, n := range news {
		responses = append(responses, buildNewsResponse(n))
	}
	return responses, meta, nil
}

func (s *Service) GetNewsFilterCounts() (*NewsFilterCounts, error) {
	return s.repo.GetNewsFilterCounts()
}

func (s *Service) GetEducationEvents() ([]EventResponse, error) {
	events, err := s.repo.FindEvents()
	if err != nil {
		return nil, err
	}

	responses := make([]EventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, buildEventResponse(event))
	}
	return responses, nil
}

func (s *Service) GetEducationEventByID(id string) (*EventResponse, error) {
	event, err := s.repo.FindEventByID(id)
	if err != nil {
		return nil, err
	}
	resp := buildEventResponse(*event)
	return &resp, nil
}

func (s *Service) GetEducationEventsFiltered(page, limit int, category, search, sort, featuredStr string) ([]EventResponse, PaginationMeta, error) {
	events, total, err := s.repo.FindEventsFiltered(page, limit, category, search, sort, featuredStr)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	pages := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		pages = 0
	}

	meta := PaginationMeta{
		Total: total,
		Page:  page,
		Limit: limit,
		Pages: pages,
	}

	responses := make([]EventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, buildEventResponse(event))
	}
	return responses, meta, nil
}

func (s *Service) GetAllEventsAdmin(page, limit int) ([]EventResponse, PaginationMeta, error) {
	events, total, err := s.repo.FindAllEvents(page, limit)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	pages := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		pages = 0
	}

	meta := PaginationMeta{
		Total: total,
		Page:  page,
		Limit: limit,
		Pages: pages,
	}

	responses := make([]EventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, buildEventResponse(event))
	}
	return responses, meta, nil
}

func (s *Service) CreateEvent(req EventRequest) (*EventResponse, error) {
	event := &Event{
		Title:           req.Title,
		Excerpt:         req.Excerpt,
		Description:     req.Description,
		Category:        req.Category,
		Organizer:       req.Organizer,
		Location:        req.Location,
		Date:            req.Date,
		Time:            req.Time,
		RegistrationFee: req.RegistrationFee,
		Image:           req.Image,
	}

	if req.Featured != nil {
		event.Featured = *req.Featured
	}

	if err := s.repo.CreateEvent(event); err != nil {
		return nil, err
	}

	resp := buildEventResponse(*event)
	return &resp, nil
}

func (s *Service) UpdateEvent(id string, req UpdateEventRequest) (*EventResponse, error) {
	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Excerpt != "" {
		updates["excerpt"] = req.Excerpt
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Organizer != "" {
		updates["organizer"] = req.Organizer
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.Date != "" {
		updates["date"] = req.Date
	}
	if req.Time != "" {
		updates["time"] = req.Time
	}
	if req.RegistrationFee != "" {
		updates["registration_fee"] = req.RegistrationFee
	}
	if req.Image != "" {
		updates["image"] = req.Image
	}
	if req.Featured != nil {
		updates["featured"] = *req.Featured
	}

	event, err := s.repo.UpdateEvent(id, updates)
	if err != nil {
		return nil, err
	}

	resp := buildEventResponse(*event)
	return &resp, nil
}

func (s *Service) DeleteEvent(id string) error {
	return s.repo.DeleteEvent(id)
}

func (s *Service) ToggleEventFeatured(id string) (*EventResponse, error) {
	event, err := s.repo.ToggleEventFeatured(id)
	if err != nil {
		return nil, err
	}

	resp := buildEventResponse(*event)
	return &resp, nil
}

func (s *Service) GetEventFilterCounts() (*EventFilterCounts, error) {
	return s.repo.GetEventFilterCounts()
}

func (s *Service) GetEducationBlogs(page, limit int, category, search, sort, tags string) ([]BlogResponse, PaginationMeta, error) {
	blogs, total, err := s.repo.FindBlogs(page, limit, category, search, sort, tags)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	pages := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		pages = 0
	}

	meta := PaginationMeta{
		Total: total,
		Page:  page,
		Limit: limit,
		Pages: pages,
	}

	responses := make([]BlogResponse, 0, len(blogs))
	for _, blog := range blogs {
		responses = append(responses, buildBlogResponse(blog))
	}
	return responses, meta, nil
}

func (s *Service) GetEducationBlogByID(id string) (*BlogWithRelatedResponse, error) {
	blog, err := s.repo.FindBlogByID(id)
	if err != nil {
		return nil, err
	}

	_ = s.repo.IncrementBlogViews(blog)

	relatedBlogs, err := s.repo.FindRelatedBlogs(blog.ID, blog.Category, 3)
	if err != nil {
		relatedBlogs = []Blog{}
	}

	relatedResponses := make([]BlogResponse, 0, len(relatedBlogs))
	for _, rb := range relatedBlogs {
		relatedResponses = append(relatedResponses, buildBlogResponse(rb))
	}

	return &BlogWithRelatedResponse{
		Blog:    buildBlogResponse(*blog),
		Related: relatedResponses,
	}, nil
}

func (s *Service) GetBlogFilterCounts() (*BlogFilterCounts, error) {
	return s.repo.GetBlogFilterCounts()
}

func (s *Service) IncrementBlogView(id string) error {
	blog, err := s.repo.FindBlogByID(id)
	if err != nil {
		return err
	}
	return s.repo.IncrementBlogViews(blog)
}

// ─── Admin CRUD ──────────────────────────────────────────────────────────────

func (s *Service) GetAllBlogsAdmin(page, limit int, category, search, sort string) ([]BlogResponse, PaginationMeta, error) {
	blogs, total, err := s.repo.FindAllBlogsAdmin(page, limit, category, search, sort)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	pages := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		pages = 0
	}

	meta := PaginationMeta{
		Total: total,
		Page:  page,
		Limit: limit,
		Pages: pages,
	}

	responses := make([]BlogResponse, 0, len(blogs))
	for _, blog := range blogs {
		responses = append(responses, buildBlogResponse(blog))
	}
	return responses, meta, nil
}

func generateSlug(title string) string {
	slug := ""
	for _, c := range title {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			slug += string(c)
		} else if c >= 'A' && c <= 'Z' {
			slug += string(c + 32)
		} else if c == ' ' || c == '-' {
			if len(slug) > 0 && slug[len(slug)-1] != '-' {
				slug += "-"
			}
		}
	}
	if len(slug) > 80 {
		slug = slug[:80]
	}
	return slug
}

func (s *Service) CreateBlog(req CreateBlogRequest) (*BlogResponse, error) {
	tagsJSON, _ := json.Marshal(req.Tags)

	blog := Blog{
		Title:     req.Title,
		Slug:      generateSlug(req.Title),
		Excerpt:   req.Excerpt,
		Content:   req.Content,
		Image:     req.Image,
		Author:    req.Author,
		Category:  req.Category,
		Tags:      tagsJSON,
		Featured:  req.Featured,
		Published: req.Published,
	}

	if err := s.repo.CreateBlog(&blog); err != nil {
		return nil, err
	}

	resp := buildBlogResponse(blog)
	return &resp, nil
}

func (s *Service) UpdateBlog(id string, req UpdateBlogRequest) (*BlogResponse, error) {
	blog, err := s.repo.FindBlogByIDAdmin(id)
	if err != nil {
		return nil, err
	}

	if req.Title != "" {
		blog.Title = req.Title
		blog.Slug = generateSlug(req.Title)
	}
	if req.Excerpt != "" {
		blog.Excerpt = req.Excerpt
	}
	if req.Content != "" {
		blog.Content = req.Content
	}
	if req.Image != "" {
		blog.Image = req.Image
	}
	if req.Author != "" {
		blog.Author = req.Author
	}
	if req.Category != "" {
		blog.Category = req.Category
	}
	if req.Tags != nil {
		tagsJSON, _ := json.Marshal(req.Tags)
		blog.Tags = tagsJSON
	}
	if req.Featured != nil {
		blog.Featured = *req.Featured
	}
	if req.Published != nil {
		blog.Published = *req.Published
	}

	if err := s.repo.UpdateBlog(blog); err != nil {
		return nil, err
	}

	resp := buildBlogResponse(*blog)
	return &resp, nil
}

func (s *Service) DeleteBlog(id string) error {
	blog, err := s.repo.FindBlogByIDAdmin(id)
	if err != nil {
		return err
	}
	return s.repo.DeleteBlog(blog.ID)
}

type BlogCommentInput struct {
	BlogID  uint   `json:"blog_id"`
	Author  string `json:"author" binding:"required"`
	Avatar  string `json:"avatar"`
	Message string `json:"message" binding:"required"`
}

type BlogCommentResponse struct {
	ID      uint   `json:"id"`
	BlogID  uint   `json:"blog_id"`
	Author  string `json:"author"`
	Avatar  string `json:"avatar"`
	Message string `json:"message"`
	Likes   int    `json:"likes"`
	Time    string `json:"time"`
}

func (s *Service) CreateBlogComment(input BlogCommentInput) (*BlogCommentResponse, error) {
	fmt.Printf("[DEBUG] CreateBlogComment input: %+v\n", input)
	blog, err := s.repo.FindBlogByID(strconv.FormatUint(uint64(input.BlogID), 10))
	if err != nil {
		fmt.Printf("[ERROR] FindBlogByID failed: %v\n", err)
		return nil, err
	}

	comment := &BlogComment{
		BlogID:  blog.ID,
		Author:  input.Author,
		Avatar:  input.Avatar,
		Message: input.Message,
	}

	if err := s.repo.CreateBlogComment(comment); err != nil {
		fmt.Printf("[ERROR] CreateBlogComment repo call failed: %v\n", err)
		return nil, err
	}

	return &BlogCommentResponse{
		ID:      comment.ID,
		BlogID:  comment.BlogID,
		Author:  comment.Author,
		Avatar:  comment.Avatar,
		Message: comment.Message,
		Likes:   comment.Likes,
		Time:    formatTimeAgo(comment.CreatedAt),
	}, nil
}

func (s *Service) GetBlogComments(blogID uint) ([]BlogCommentResponse, error) {
	comments, err := s.repo.GetBlogComments(blogID)
	if err != nil {
		return nil, err
	}

	responses := make([]BlogCommentResponse, 0, len(comments))
	for _, c := range comments {
		responses = append(responses, BlogCommentResponse{
			ID:      c.ID,
			BlogID:  c.BlogID,
			Author:  c.Author,
			Avatar:  c.Avatar,
			Message: c.Message,
			Likes:   c.Likes,
			Time:    formatTimeAgo(c.CreatedAt),
		})
	}
	return responses, nil
}

func (s *Service) LikeBlogComment(commentID uint) error {
	return s.repo.IncrementCommentLikes(commentID)
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		minutes := int(d.Minutes())
		if minutes <= 1 {
			return "Just now"
		}
		return fmt.Sprintf("%d min ago", minutes)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func (s *Service) FindBlogByID(id uint) (*Blog, error) {
	return s.repo.FindBlogByID(strconv.FormatUint(uint64(id), 10))
}

func (s *Service) UploadBlogImage(file *multipart.FileHeader) ([]string, error) {
	ct := file.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("only image files are allowed")
	}

	url, err := utils.SaveUploadedImage(file, "blogs")
	if err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	return []string{url}, nil
}

func buildAdminNewsResponse(news News) AdminNewsResponse {
	return AdminNewsResponse{
		ID:        news.ID,
		Category:  news.Category,
		Title:     news.Title,
		Excerpt:   news.Excerpt,
		Content:   news.Content,
		Image:     news.Image,
		Author:    news.Author,
		Date:      news.Date,
		ReadTime:  news.ReadTime,
		Source:    news.Source,
		Tags:      parseStringArrayField(news.Tags),
		CreatedAt: news.CreatedAt.String(),
		UpdatedAt: news.UpdatedAt.String(),
	}
}

func (s *Service) GetAllNewsAdmin(page, limit int, category, search string) ([]AdminNewsResponse, PaginationMeta, error) {
	news, total, err := s.repo.FindAllNewsAdmin(page, limit, category, search)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	pages := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		pages = 0
	}

	meta := PaginationMeta{
		Total: total,
		Page:  page,
		Limit: limit,
		Pages: pages,
	}

	responses := make([]AdminNewsResponse, 0, len(news))
	for _, n := range news {
		responses = append(responses, buildAdminNewsResponse(n))
	}
	return responses, meta, nil
}

func (s *Service) CreateNewsAdmin(req CreateNewsRequest) (*AdminNewsResponse, error) {
	tagsJSON, _ := json.Marshal(req.Tags)

	news := News{
		Category: req.Category,
		Title:    req.Title,
		Excerpt:  req.Excerpt,
		Content:  req.Content,
		Image:    req.Image,
		Author:   req.Author,
		Date:     req.Date,
		ReadTime: req.ReadTime,
		Source:   req.Source,
		Tags:     tagsJSON,
	}

	if err := s.repo.CreateNews(&news); err != nil {
		return nil, err
	}

	resp := buildAdminNewsResponse(news)
	return &resp, nil
}

func (s *Service) GetNewsByIDAdmin(id string) (*AdminNewsResponse, error) {
	news, err := s.repo.FindNewsByIDAdmin(id)
	if err != nil {
		return nil, err
	}
	resp := buildAdminNewsResponse(*news)
	return &resp, nil
}

func (s *Service) UpdateNewsAdmin(id string, req UpdateNewsRequest) (*AdminNewsResponse, error) {
	news, err := s.repo.FindNewsByIDAdmin(id)
	if err != nil {
		return nil, err
	}

	if req.Title != "" {
		news.Title = req.Title
	}
	if req.Category != "" {
		news.Category = req.Category
	}
	if req.Excerpt != "" {
		news.Excerpt = req.Excerpt
	}
	if req.Content != "" {
		news.Content = req.Content
	}
	if req.Image != "" {
		news.Image = req.Image
	}
	if req.Author != "" {
		news.Author = req.Author
	}
	if req.Date != "" {
		news.Date = req.Date
	}
	if req.ReadTime != "" {
		news.ReadTime = req.ReadTime
	}
	if req.Source != "" {
		news.Source = req.Source
	}
	if req.Tags != nil {
		tagsJSON, _ := json.Marshal(req.Tags)
		news.Tags = tagsJSON
	}

	if err := s.repo.UpdateNews(news); err != nil {
		return nil, err
	}

	resp := buildAdminNewsResponse(*news)
	return &resp, nil
}

func (s *Service) DeleteNewsAdmin(id string) error {
	return s.repo.DeleteNews(id)
}

func (s *Service) UploadNewsImage(file *multipart.FileHeader) (string, error) {
	ct := file.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return "", fmt.Errorf("only image files are allowed")
	}

	url, err := utils.SaveUploadedImage(file, "news")
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	return url, nil
}

func buildPublicEntranceResponse(exam Exam) PublicEntranceResponse {
	return PublicEntranceResponse{
		ID:           exam.ID,
		Slug:         exam.Slug,
		Title:        exam.Title,
		Board:        exam.Board,
		Badges:       parseStringArrayField(exam.Badges),
		Level:        exam.Level,
		Type:         exam.Type,
		ExamDate:     exam.ExamDate,
		FormDeadline: exam.FormDeadline,
		Fee:          exam.Fee,
		Description:  exam.Description,
		Status:       exam.Status,
		ImageUrl:     exam.ImageUrl,
		University:   exam.University,
		Faculty:      exam.Faculty,
		NepaliDate:   exam.NepaliDate,
		Overview:     exam.Overview,
	}
}

func (s *Service) GetPublicEntrances(page, limit int, search, level, stream, status string) ([]PublicEntranceResponse, int64, error) {
	exams, err := s.repo.GetAllExamEntries(search, level, stream, status)
	if err != nil {
		return nil, 0, err
	}

	allResponses := make([]PublicEntranceResponse, 0, len(exams))
	for _, exam := range exams {
		allResponses = append(allResponses, buildPublicEntranceResponse(exam))
	}

	instEntrances, _ := s.repo.GetPublishedInstitutionEntrances(search)
	for _, ie := range instEntrances {
		loc := ie.InstitutionLocation
		if ie.InstitutionProvince != "" {
			if loc != "" {
				loc = ie.InstitutionProvince + ", " + loc
			} else {
				loc = ie.InstitutionProvince
			}
		}
		var overviewDetails []interface{}
		if len(ie.OverviewDetails) > 0 {
			json.Unmarshal(ie.OverviewDetails, &overviewDetails)
		}
		deadline := ""
		if len(ie.ExamDateSchedules) > 0 {
			var schedules []struct {
				EndDate string `json:"endDate"`
			}
			if err := json.Unmarshal(ie.ExamDateSchedules, &schedules); err == nil {
				for _, s := range schedules {
					if s.EndDate != "" {
						deadline = s.EndDate
						break
					}
				}
			}
		}
		allResponses = append(allResponses, PublicEntranceResponse{
			FormDeadline: deadline,
			ID:               ie.ID,
			Title:            ie.Title,
			Description:      ie.Description,
			ExamDate:         ie.Date,
			ImageUrl:         ie.HeroBanner,
			Status:           ie.Status,
			Fee:              ie.Fee,
			University:       ie.InstitutionName,
			Board:            ie.InstitutionName,
			Phone:            ie.InstitutionPhone,
			Email:            ie.InstitutionEmail,
			Website:          ie.InstitutionWebsite,
			Location:         loc,
			InstitutionLogo:  ie.InstitutionLogo,
			OverviewDetails:  overviewDetails,
			ApplicationLink:  ie.ApplicationLink,
			NoticeFile:       ie.NoticeFile,
		})
	}

	total := int64(len(allResponses))

	start := (page - 1) * limit
	if start > int(total) {
		return []PublicEntranceResponse{}, total, nil
	}
	end := start + limit
	if end > int(total) {
		end = int(total)
	}

	return allResponses[start:end], total, nil
}

func (s *Service) GetEntranceFilterCounts() (FilterCounts, error) {
	return s.repo.GetEntranceFilterCounts()
}

func (s *Service) GetPublicEntranceByID(id string) (*PublicEntranceResponse, error) {
	exam, err := s.repo.GetPublicEntranceByID(id)
	if err != nil {
		return nil, err
	}
	if exam != nil {
		resp := buildPublicEntranceResponse(*exam)
		return &resp, nil
	}

	instEntrance, err := s.repo.GetInstitutionEntranceByID(id)
	if err != nil {
		return nil, err
	}
	if instEntrance != nil {
		loc := instEntrance.InstitutionLocation
		if instEntrance.InstitutionProvince != "" {
			if loc != "" {
				loc = instEntrance.InstitutionProvince + ", " + loc
			} else {
				loc = instEntrance.InstitutionProvince
			}
		}
		var overviewDetails []interface{}
		if len(instEntrance.OverviewDetails) > 0 {
			json.Unmarshal(instEntrance.OverviewDetails, &overviewDetails)
		}
		return &PublicEntranceResponse{
			ID:               instEntrance.ID,
			Title:            instEntrance.Title,
			Description:      instEntrance.Description,
			ExamDate:         instEntrance.Date,
			ImageUrl:         instEntrance.HeroBanner,
			Status:           instEntrance.Status,
			Fee:              instEntrance.Fee,
			University:       instEntrance.InstitutionName,
			Board:            instEntrance.InstitutionName,
			Phone:            instEntrance.InstitutionPhone,
			Email:            instEntrance.InstitutionEmail,
			Website:          instEntrance.InstitutionWebsite,
			Location:         loc,
			InstitutionLogo:  instEntrance.InstitutionLogo,
			OverviewDetails:  overviewDetails,
			ApplicationLink:  instEntrance.ApplicationLink,
			NoticeFile:       instEntrance.NoticeFile,
		}, nil
	}

	return nil, gorm.ErrRecordNotFound
}

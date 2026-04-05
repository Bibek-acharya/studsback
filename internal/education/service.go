package education

import (
	"encoding/json"
	"strconv"
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
		ID:         event.ID,
		Title:      event.Title,
		Date:       event.Date,
		Location:   event.Location,
		Image:      event.Image,
		Interested: event.Interested,
		Trending:   event.Trending,
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
	return responses, nil
}

func (s *Service) GetEducationCourseByID(id string) (*CourseResponse, error) {
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

func (s *Service) GetEducationBlogs(page, limit int, category, search string) ([]BlogResponse, PaginationMeta, error) {
	blogs, total, err := s.repo.FindBlogs(page, limit, category, search)
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

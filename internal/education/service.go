package education

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"studsphere/backend/internal/shared/slug"
	"studsphere/backend/internal/shared/utils"
	"studsphere/backend/internal/system"

	"gorm.io/gorm"
)

// ResolvedProgram holds the fields education needs from an institution program.
// Avoids importing institution package (cycle).
type ResolvedProgram struct {
	InstitutionID   uint
	Fee             string
	Eligibility     string
	Capacity        int
	Status          string
	WhoShouldChoose []byte
	Features        []byte
	FullTimeCourses []byte
	FeeItems        []byte
	Overrides       []byte
	NullifiedFields []string
}

// InstitutionProgramRepo abstracts the institution repository to avoid import cycle.
type InstitutionProgramRepo interface {
	FindProgramByGlobalCourse(institutionID, globalCourseID uint) (*ResolvedProgram, error)
}

type Service struct {
	repo      *Repository
	instRepo  InstitutionProgramRepo
	systemSvc *system.Service
}

func NewService(repo *Repository, instRepo InstitutionProgramRepo, systemSvc *system.Service) *Service {
	return &Service{repo: repo, instRepo: instRepo, systemSvc: systemSvc}
}

func (s *Service) resolveAffiliationName(course *Course) string {
	if course.AffiliationID != nil {
		affiliation, err := s.repo.FindAffiliationByID(*course.AffiliationID)
		if err == nil && affiliation != nil && affiliation.Name != "" {
			return affiliation.Name
		}
		// Fallback: try universities table
		uni, err := s.repo.FindUniversityByID(*course.AffiliationID)
		if err == nil && uni != nil && uni.Name != "" {
			return uni.Name
		}
	}
	if course.NonUniversityAffiliation != "" {
		return course.NonUniversityAffiliation
	}
	return ""
}

func (s *Service) resolveAffiliationNames(courses []Course) map[uint]string {
	affiliationIDs := make([]uint, 0)
	for _, c := range courses {
		if c.AffiliationID != nil {
			affiliationIDs = append(affiliationIDs, *c.AffiliationID)
		}
	}

	result := make(map[uint]string)
	if len(affiliationIDs) > 0 {
		// Try affiliations table first
		affiliations, err := s.repo.FindAffiliationsByIDs(affiliationIDs)
		if err == nil {
			for id, aff := range affiliations {
				result[id] = aff.Name
			}
		}
		// For any IDs not found, try universities table
		var missing []uint
		for _, id := range affiliationIDs {
			if result[id] == "" {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			universities, err := s.repo.FindUniversitiesByIDs(missing)
			if err == nil {
				for id, uni := range universities {
					result[id] = uni.Name
				}
			}
		}
	}
	return result
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

func buildCourseResponse(course Course, colleges int, affiliationName string) CourseResponse {
	return CourseResponse{
		ID:              strconv.FormatUint(uint64(course.ID), 10),
		Title:           course.Title,
		ShortTitle:      course.ShortTitle,
		Colleges:        colleges,
		Affiliation:     affiliationName,
		AffiliationName: affiliationName,
		NonUniversityAffiliation: course.NonUniversityAffiliation,
		Badges:          parseStringArrayField(course.Badges),
		Level:           course.Level,
		Field:           course.Field,
		FieldOfStudy:    course.FieldOfStudy,
		Duration:        course.Duration,
		EstFee:          course.EstFee,
		GovtFee:         course.GovtFee,
		PrivateFee:      course.PrivateFee,
		Highlights:      parseStringArrayField(course.Highlights),
		CareerPath:      course.CareerPath,
		Description:     course.Description,
		Location:        course.Location,
		Mode:            course.Mode,
		DegreeLabel:     course.DegreeLabel,
		FeeStructure:    course.FeeStructure,
		EligibilityText: course.EligibilityText,
		BannerURL:       course.BannerURL,

		WhoShouldChoose:  parseJSONB[PersonaItem](course.WhoShouldChoose),
		Features:         parseJSONB[FeatureItem](course.Features),
		EligibilityRows:  parseJSONB[EligibilityRow](course.EligibilityRows),
		AdmissionSteps:   parseJSONB[AdmissionStep](course.AdmissionSteps),
		SubjectGroups:    parseJSONB[SubjectGroup](course.SubjectGroups),
		FeeItems:         parseJSONB[FeeItem](course.FeeItems),
		ScholarshipDesc:  course.ScholarshipDesc,
		ScholarshipNotes: course.ScholarshipNotes,
		Scholarships:     parseJSONB[ScholarshipItem](course.Scholarships),
		FullTimeCourses:  parseJSONB[FullTimeCourse](course.FullTimeCourses),
		FAQs:             parseJSONB[FaqItem](course.FAQs),
	}
}


func buildNewsResponse(news News) NewsResponse {
	date := news.Date
	if date == "" {
		date = news.CreatedAt.Format("2006-01-02")
	}
	return NewsResponse{
		ID:       news.ID,
		Slug:     news.Slug,
		Category: news.Category,
		Title:    news.Title,
		Excerpt:  news.Excerpt,
		Content:  news.Content,
		Image:    news.Image,
		Author:   news.Author,
		Date:     date,
		ReadTime: news.ReadTime,
		Source:   news.Source,
		Tags:     parseStringArrayField(news.Tags),
	}
}

func buildEventResponse(event Event) EventResponse {
	date := event.Date
	if date == "" {
		date = event.CreatedAt.Format("2006-01-02")
	}
	return EventResponse{
		ID:              event.ID,
		UniversityID:    event.UniversityID,
		Slug:            event.Slug,
		Title:           event.Title,
		Excerpt:         event.Excerpt,
		Description:     event.Description,
		Category:        event.Category,
		Organizer:       event.Organizer,
		Location:        event.Location,
		Date:            date,
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

	affiliationNames := s.resolveAffiliationNames(courses)

	responses := make([]CourseResponse, 0, len(courses))
	for _, course := range courses {
		colleges := course.CollegesCount
		count, err := s.repo.CountCourseOfferingColleges(course.ID)
		if err == nil && count > 0 {
			colleges = int(count)
		}
		// Also count institutions offering this global course
		instCount, err := s.repo.CountInstitutionsOfferingCourse(course.ID)
		if err == nil && instCount > 0 {
			colleges = int(instCount)
		}
		affName := ""
		if course.AffiliationID != nil {
			affName = affiliationNames[*course.AffiliationID]
		}
		if affName == "" {
			affName = course.NonUniversityAffiliation
		}
		resp := buildCourseResponse(course, colleges, affName)
		resp.IsGlobal = true
		resp.Status = "published"
		responses = append(responses, resp)
	}

	return responses, nil
}

func (s *Service) GetEducationCoursesPaginated(page, limit int, search, level, field, affiliation string) ([]CourseResponse, PaginationMeta, error) {
	allCourses, total, err := s.repo.FindCoursesFiltered(page, limit, search, level, field, affiliation)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	affiliationNames := s.resolveAffiliationNames(allCourses)

	allResponses := make([]CourseResponse, 0, len(allCourses))
	for _, course := range allCourses {
		colleges := course.CollegesCount
		count, err := s.repo.CountCourseOfferingColleges(course.ID)
		if err == nil && count > 0 {
			colleges = int(count)
		}
		instCount, err := s.repo.CountInstitutionsOfferingCourse(course.ID)
		if err == nil && instCount > 0 {
			colleges = int(instCount)
		}
		affName := ""
		if course.AffiliationID != nil {
			affName = affiliationNames[*course.AffiliationID]
		}
		if affName == "" {
			affName = course.NonUniversityAffiliation
		}
		resp := buildCourseResponse(course, colleges, affName)
		resp.IsGlobal = true
		resp.Status = "published"
		allResponses = append(allResponses, resp)
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

	return allResponses, meta, nil
}

func buildAdminCourseResponse(course Course) AdminCourseResponse {
	return AdminCourseResponse{
		ID:                       course.ID,
		Title:                    course.Title,
		ShortTitle:               course.ShortTitle,
		AffiliationID:            course.AffiliationID,
		NonUniversityAffiliation: course.NonUniversityAffiliation,
		Badges:                   parseStringArrayField(course.Badges),
		Level:                    course.Level,
		Field:                    course.Field,
		FieldOfStudy:             course.FieldOfStudy,
		Duration:                 course.Duration,
		EstFee:                   course.EstFee,
		Highlights:               parseStringArrayField(course.Highlights),
		CareerPath:               course.CareerPath,
		Description:              course.Description,
		Location:                 course.Location,
		GovtFee:                  course.GovtFee,
		PrivateFee:               course.PrivateFee,
		FeeStructure:             course.FeeStructure,
		EligibilityText:          course.EligibilityText,
		Mode:                     course.Mode,
		DegreeLabel:              course.DegreeLabel,
		About:                    parseStringArrayField(course.About),
		Curriculum:               parseJSONField(course.Curriculum),
		Admissions:               parseStringArrayField(course.Admissions),
		Careers:                  parseJSONB[CareerItem](course.Careers),
		BannerURL:                course.BannerURL,
		WhoShouldChoose:          parseJSONB[PersonaItem](course.WhoShouldChoose),
		Features:                 parseJSONB[FeatureItem](course.Features),
		EligibilityRows:          parseJSONB[EligibilityRow](course.EligibilityRows),
		AdmissionSteps:           parseJSONB[AdmissionStep](course.AdmissionSteps),
		SubjectGroups:            parseJSONB[SubjectGroup](course.SubjectGroups),
		FeeItems:                 parseJSONB[FeeItem](course.FeeItems),
		ScholarshipDesc:          course.ScholarshipDesc,
		ScholarshipNotes:         course.ScholarshipNotes,
		Scholarships:             parseJSONB[ScholarshipItem](course.Scholarships),
		FullTimeCourses:          parseJSONB[FullTimeCourse](course.FullTimeCourses),
		FAQs:                     parseJSONB[FaqItem](course.FAQs),
		IsGlobal:                 course.IsGlobal,
		Status:                   course.Status,
		CreatedBy:                course.CreatedBy,
		SourceProgramID:          course.SourceProgramID,
		CreatedAt:                course.CreatedAt.String(),
		UpdatedAt:                course.UpdatedAt.String(),
	}
}

func parseJSONField(data []byte) interface{} {
	if len(data) == 0 {
		return nil
	}
	var result interface{}
	if err := json.Unmarshal(data, &result); err == nil {
		return result
	}
	return nil
}

func parseJSONB[T any](data []byte) []T {
	if len(data) == 0 {
		return nil
	}
	var result []T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func marshalJSON(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func (s *Service) CreateCourse(req CreateCourseRequest) (*AdminCourseResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Normalize()

	badgesJSON, err := marshalJSON(req.Badges)
	if err != nil {
		return nil, fmt.Errorf("marshal badges: %w", err)
	}
	highlightsJSON, err := marshalJSON(req.Highlights)
	if err != nil {
		return nil, fmt.Errorf("marshal highlights: %w", err)
	}
	aboutJSON, err := marshalJSON(req.About)
	if err != nil {
		return nil, fmt.Errorf("marshal about: %w", err)
	}
	admissionsJSON, err := marshalJSON(req.Admissions)
	if err != nil {
		return nil, fmt.Errorf("marshal admissions: %w", err)
	}
	curriculumJSON, err := marshalJSON(req.Curriculum)
	if err != nil {
		return nil, fmt.Errorf("marshal curriculum: %w", err)
	}
	careersJSON, err := marshalJSON(req.Careers)
	if err != nil {
		return nil, fmt.Errorf("marshal careers: %w", err)
	}
	whoShouldChooseJSON, err := marshalJSON(req.WhoShouldChoose)
	if err != nil {
		return nil, fmt.Errorf("marshal whoShouldChoose: %w", err)
	}
	featuresJSON, err := marshalJSON(req.Features)
	if err != nil {
		return nil, fmt.Errorf("marshal features: %w", err)
	}
	eligibilityRowsJSON, err := marshalJSON(req.EligibilityRows)
	if err != nil {
		return nil, fmt.Errorf("marshal eligibilityRows: %w", err)
	}
	admissionStepsJSON, err := marshalJSON(req.AdmissionSteps)
	if err != nil {
		return nil, fmt.Errorf("marshal admissionSteps: %w", err)
	}
	subjectGroupsJSON, err := marshalJSON(req.SubjectGroups)
	if err != nil {
		return nil, fmt.Errorf("marshal subjectGroups: %w", err)
	}
	feeItemsJSON, err := marshalJSON(req.FeeItems)
	if err != nil {
		return nil, fmt.Errorf("marshal feeItems: %w", err)
	}
	scholarshipsJSON, err := marshalJSON(req.Scholarships)
	if err != nil {
		return nil, fmt.Errorf("marshal scholarships: %w", err)
	}
	fullTimeCoursesJSON, err := marshalJSON(req.FullTimeCourses)
	if err != nil {
		return nil, fmt.Errorf("marshal fullTimeCourses: %w", err)
	}
	faqsJSON, err := marshalJSON(req.FAQs)
	if err != nil {
		return nil, fmt.Errorf("marshal faqs: %w", err)
	}

	course := &Course{
		Title:                    req.Title,
		ShortTitle:               req.ShortTitle,
		AffiliationID:            req.AffiliationID,
		NonUniversityAffiliation: req.NonUniversityAffiliation,
		Badges:                   badgesJSON,
		Level:                    req.Level,
		Field:                    req.Field,
		FieldOfStudy:             req.FieldOfStudy,
		Duration:                 req.Duration,
		EstFee:                   req.EstFee,
		Highlights:               highlightsJSON,
		CareerPath:               req.CareerPath,
		Description:              req.Description,
		Location:                 req.Location,
		GovtFee:                  req.GovtFee,
		PrivateFee:               req.PrivateFee,
		FeeStructure:             req.FeeStructure,
		EligibilityText:          req.EligibilityText,
		Mode:                     req.Mode,
		DegreeLabel:              req.DegreeLabel,
		About:                    aboutJSON,
		Curriculum:               curriculumJSON,
		Admissions:               admissionsJSON,
		Careers:                  careersJSON,
		BannerURL:                req.BannerURL,
		WhoShouldChoose:          whoShouldChooseJSON,
		Features:                 featuresJSON,
		EligibilityRows:          eligibilityRowsJSON,
		AdmissionSteps:           admissionStepsJSON,
		SubjectGroups:            subjectGroupsJSON,
		FeeItems:                 feeItemsJSON,
		ScholarshipDesc:          req.ScholarshipDesc,
		ScholarshipNotes:         req.ScholarshipNotes,
		Scholarships:             scholarshipsJSON,
		FullTimeCourses:          fullTimeCoursesJSON,
		FAQs:                     faqsJSON,
		IsGlobal:                 true,
		Status:                   "published",
	}

	if err := s.repo.CreateCourse(course); err != nil {
		return nil, err
	}

	resp := buildAdminCourseResponse(*course)
	return &resp, nil
}

func (s *Service) GetAllCoursesAdmin(page, limit int) ([]AdminCourseResponse, PaginationMeta, error) {
	courses, total, err := s.repo.FindAllCoursesAdmin(page, limit)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	pages := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		pages = 0
	}

	responses := make([]AdminCourseResponse, len(courses))
	affiliationNames := s.resolveAffiliationNames(courses)
	for i, course := range courses {
		responses[i] = buildAdminCourseResponse(course)
		affName := ""
		if course.AffiliationID != nil {
			affName = affiliationNames[*course.AffiliationID]
		}
		if affName == "" {
			affName = course.NonUniversityAffiliation
		}
		responses[i].AffiliationName = affName
	}

	meta := PaginationMeta{
		Total: total,
		Page:  page,
		Limit: limit,
		Pages: pages,
	}

	return responses, meta, nil
}

func (s *Service) GetPendingCourses(page, limit int) ([]AdminCourseResponse, PaginationMeta, error) {
	courses, total, err := s.repo.FindPendingCourses(page, limit)
	if err != nil {
		return nil, PaginationMeta{}, err
	}

	pages := (total + int64(limit) - 1) / int64(limit)
	if total == 0 {
		pages = 0
	}

	responses := make([]AdminCourseResponse, len(courses))
	affiliationNames := s.resolveAffiliationNames(courses)
	for i, course := range courses {
		responses[i] = buildAdminCourseResponse(course)
		affName := ""
		if course.AffiliationID != nil {
			affName = affiliationNames[*course.AffiliationID]
		}
		if affName == "" {
			affName = course.NonUniversityAffiliation
		}
		responses[i].AffiliationName = affName
	}

	meta := PaginationMeta{
		Total: total,
		Page:  page,
		Limit: limit,
		Pages: pages,
	}

	return responses, meta, nil
}

func (s *Service) GetCourseByIDAdmin(id string) (*AdminCourseResponse, error) {
	course, err := s.repo.FindCourseByID(id)
	if err != nil {
		return nil, err
	}
	resp := buildAdminCourseResponse(*course)
	if course.AffiliationID != nil {
		resp.AffiliationName = s.resolveAffiliationName(course)
	}
	return &resp, nil
}

func (s *Service) UpdateCourse(id string, req UpdateCourseRequest) (*AdminCourseResponse, error) {
	course, err := s.repo.FindCourseByID(id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		course.Title = *req.Title
	}
	if req.ShortTitle != nil {
		course.ShortTitle = *req.ShortTitle
	}
	if req.AffiliationID != nil {
		course.AffiliationID = req.AffiliationID
	}
	if req.NonUniversityAffiliation != nil {
		course.NonUniversityAffiliation = *req.NonUniversityAffiliation
	}
	if req.Badges != nil {
		data, err := marshalJSON(req.Badges)
		if err != nil {
			return nil, fmt.Errorf("marshal badges: %w", err)
		}
		course.Badges = data
	}
	if req.Level != nil {
		course.Level = *req.Level
	}
	if req.Field != nil {
		course.Field = *req.Field
	}
	if req.FieldOfStudy != nil {
		course.FieldOfStudy = *req.FieldOfStudy
	}
	if req.Duration != nil {
		course.Duration = *req.Duration
	}
	if req.EstFee != nil {
		course.EstFee = *req.EstFee
	}
	if req.Highlights != nil {
		data, err := marshalJSON(req.Highlights)
		if err != nil {
			return nil, fmt.Errorf("marshal highlights: %w", err)
		}
		course.Highlights = data
	}
	if req.CareerPath != nil {
		course.CareerPath = *req.CareerPath
	}
	if req.Description != nil {
		course.Description = *req.Description
	}
	if req.Location != nil {
		course.Location = *req.Location
	}
	if req.GovtFee != nil {
		course.GovtFee = *req.GovtFee
	}
	if req.PrivateFee != nil {
		course.PrivateFee = *req.PrivateFee
	}
	if req.FeeStructure != nil {
		course.FeeStructure = *req.FeeStructure
	}
	if req.EligibilityText != nil {
		course.EligibilityText = *req.EligibilityText
	}
	if req.Mode != nil {
		course.Mode = *req.Mode
	}
	if req.DegreeLabel != nil {
		course.DegreeLabel = *req.DegreeLabel
	}
	if req.About != nil {
		data, err := marshalJSON(req.About)
		if err != nil {
			return nil, fmt.Errorf("marshal about: %w", err)
		}
		course.About = data
	}
	if req.Curriculum != nil {
		data, err := marshalJSON(req.Curriculum)
		if err != nil {
			return nil, fmt.Errorf("marshal curriculum: %w", err)
		}
		course.Curriculum = data
	}
	if req.Admissions != nil {
		data, err := marshalJSON(req.Admissions)
		if err != nil {
			return nil, fmt.Errorf("marshal admissions: %w", err)
		}
		course.Admissions = data
	}
	if req.Careers != nil {
		data, err := marshalJSON(req.Careers)
		if err != nil {
			return nil, fmt.Errorf("marshal careers: %w", err)
		}
		course.Careers = data
	}
	if req.BannerURL != nil {
		course.BannerURL = *req.BannerURL
	}
	if req.WhoShouldChoose != nil {
		data, err := marshalJSON(req.WhoShouldChoose)
		if err != nil {
			return nil, fmt.Errorf("marshal whoShouldChoose: %w", err)
		}
		course.WhoShouldChoose = data
	}
	if req.Features != nil {
		data, err := marshalJSON(req.Features)
		if err != nil {
			return nil, fmt.Errorf("marshal features: %w", err)
		}
		course.Features = data
	}
	if req.EligibilityRows != nil {
		data, err := marshalJSON(req.EligibilityRows)
		if err != nil {
			return nil, fmt.Errorf("marshal eligibilityRows: %w", err)
		}
		course.EligibilityRows = data
	}
	if req.AdmissionSteps != nil {
		data, err := marshalJSON(req.AdmissionSteps)
		if err != nil {
			return nil, fmt.Errorf("marshal admissionSteps: %w", err)
		}
		course.AdmissionSteps = data
	}
	if req.SubjectGroups != nil {
		data, err := marshalJSON(req.SubjectGroups)
		if err != nil {
			return nil, fmt.Errorf("marshal subjectGroups: %w", err)
		}
		course.SubjectGroups = data
	}
	if req.FeeItems != nil {
		data, err := marshalJSON(req.FeeItems)
		if err != nil {
			return nil, fmt.Errorf("marshal feeItems: %w", err)
		}
		course.FeeItems = data
	}
	if req.ScholarshipDesc != nil {
		course.ScholarshipDesc = *req.ScholarshipDesc
	}
	if req.ScholarshipNotes != nil {
		course.ScholarshipNotes = *req.ScholarshipNotes
	}
	if req.Scholarships != nil {
		data, err := marshalJSON(req.Scholarships)
		if err != nil {
			return nil, fmt.Errorf("marshal scholarships: %w", err)
		}
		course.Scholarships = data
	}
	if req.FullTimeCourses != nil {
		data, err := marshalJSON(req.FullTimeCourses)
		if err != nil {
			return nil, fmt.Errorf("marshal fullTimeCourses: %w", err)
		}
		course.FullTimeCourses = data
	}
	if req.FAQs != nil {
		data, err := marshalJSON(req.FAQs)
		if err != nil {
			return nil, fmt.Errorf("marshal faqs: %w", err)
		}
		course.FAQs = data
	}

	if err := s.repo.UpdateCourse(course); err != nil {
		return nil, err
	}

	resp := buildAdminCourseResponse(*course)
	return &resp, nil
}

func (s *Service) DeleteCourse(id string) error {
	course, err := s.repo.FindCourseByID(id)
	if err != nil {
		return err
	}
	return s.repo.DeleteCourse(course.ID)
}

func (s *Service) PublishCourse(id string) (*AdminCourseResponse, error) {
	course, err := s.repo.FindCourseByID(id)
	if err != nil {
		return nil, err
	}

	if err := s.repo.SetCoursePublished(course.ID); err != nil {
		return nil, err
	}

	course.IsGlobal = true
	course.Status = "published"

	resp := buildAdminCourseResponse(*course)
	return &resp, nil
}

func (s *Service) SearchGlobalCourses(query string) ([]CourseResponse, error) {
	courses, err := s.repo.FindPublishedGlobalCourses(query)
	if err != nil {
		return nil, err
	}

	affiliationNames := s.resolveAffiliationNames(courses)

	responses := make([]CourseResponse, len(courses))
	for i, course := range courses {
		affName := ""
		if course.AffiliationID != nil {
			affName = affiliationNames[*course.AffiliationID]
		}
		if affName == "" {
			affName = course.NonUniversityAffiliation
		}
		responses[i] = buildCourseResponse(course, 0, affName)
		responses[i].IsGlobal = true
		responses[i].Status = "published"
	}
	return responses, nil
}

func (s *Service) GetInstitutionCourses(instID uint) ([]CourseResponse, error) {
	entries, err := s.repo.FindInstitutionProgramsByInstitutionID(instID)
	if err != nil {
		return nil, err
	}

	responses := make([]CourseResponse, 0, len(entries))
	for _, entry := range entries {
		level := extractLevelFromProgramData(entry.Data)
		resp := CourseResponse{
			ID:              fmt.Sprintf("inst-%d", entry.ID),
			Title:           entry.ProgramName,
			Level:           level,
			Affiliation:     entry.InstitutionName,
			Duration:        entry.Duration,
			EstFee:          entry.Fee,
			Description:     entry.Description,
			Location:        entry.InstitutionLocation,
			Source:          "institution",
			InstitutionName: entry.InstitutionName,
			Image:           entry.BannerURL,
			IsGlobal:        false,
			Status:          "active",
		}

		// If linked to a global course, merge data
		if entry.GlobalCourseID != nil && *entry.GlobalCourseID > 0 {
			globalCourse, err := s.repo.FindCourseByIDOnly(*entry.GlobalCourseID)
			if err == nil && globalCourse != nil {
				if resp.Title == "" {
					resp.Title = globalCourse.Title
				}
				if resp.Duration == "" {
					resp.Duration = globalCourse.Duration
				}
				if resp.Description == "" {
					resp.Description = globalCourse.Description
				}
				if globalCourse.AffiliationID != nil {
					aff, err := s.repo.FindAffiliationByID(*globalCourse.AffiliationID)
					if err == nil && aff != nil {
						resp.Affiliation = aff.Name
					}
				} else if globalCourse.NonUniversityAffiliation != "" {
					resp.Affiliation = globalCourse.NonUniversityAffiliation
				}
				if resp.Level == "" {
					resp.Level = globalCourse.Level
				}
			}
		}

		responses = append(responses, resp)
	}

	return responses, nil
}

func (s *Service) GetCourseFilterCounts() (*CourseFilterCounts, error) {
	return s.repo.GetCourseFilterCounts()
}

func extractLevelFromProgramData(data *string) string {
	if data == nil || *data == "" {
		return ""
	}
	var parsed struct {
		Level string `json:"level"`
	}
	if err := json.Unmarshal([]byte(*data), &parsed); err != nil {
		return ""
	}
	return parsed.Level
}

func programIDFromParam(id string) (uint, bool) {
	if strings.HasPrefix(id, "inst-") {
		n, err := strconv.ParseUint(id[5:], 10, 64)
		if err == nil {
			return uint(n), true
		}
	}
	return 0, false
}

func (s *Service) GetEducationCourseByID(id string) (*CourseResponse, error) {
	if pid, ok := programIDFromParam(id); ok {
		if program, err := s.repo.FindPublishedInstitutionProgramByID(pid); err == nil && program != nil {
			respLevel := extractLevelFromProgramData(program.Data)
			resp := &CourseResponse{
				ID:              fmt.Sprintf("inst-%d", program.ID),
				Title:           program.ProgramName,
				Affiliation:     program.InstitutionName,
				Duration:        program.Duration,
				EstFee:          program.Fee,
				Description:     program.Description,
				Location:        program.InstitutionLocation,
				Level:           respLevel,
				Source:          "institution",
				InstitutionName: program.InstitutionName,
				Image:           program.BannerURL,
				IsGlobal:        false,
				Status:          "active",
			}

			// Merge with global course data if linked
			if program.GlobalCourseID != nil && *program.GlobalCourseID > 0 {
				globalCourse, err := s.repo.FindCourseByIDOnly(*program.GlobalCourseID)
				if err == nil && globalCourse != nil {
					if resp.Title == "" {
						resp.Title = globalCourse.Title
					}
					if resp.Duration == "" {
						resp.Duration = globalCourse.Duration
					}
					if resp.Description == "" {
						resp.Description = globalCourse.Description
					}
					if resp.Level == "" {
						resp.Level = globalCourse.Level
					}
					if globalCourse.AffiliationID != nil {
						aff, err := s.repo.FindAffiliationByID(*globalCourse.AffiliationID)
						if err == nil && aff != nil {
							resp.Affiliation = aff.Name
						}
					} else if globalCourse.NonUniversityAffiliation != "" {
						resp.Affiliation = globalCourse.NonUniversityAffiliation
					}
				}
			}

			return resp, nil
		}
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

	affName := s.resolveAffiliationName(course)
	resp := buildCourseResponse(*course, colleges, affName)
	resp.IsGlobal = true
	resp.Status = "published"
	return &resp, nil
}

func (s *Service) GetEducationCourseDetailsByID(id string) (*CourseDetailsResponse, error) {
	if pid, ok := programIDFromParam(id); ok {
		if program, err := s.repo.FindPublishedInstitutionProgramByID(pid); err == nil && program != nil {
			var programData interface{}
			if program.Data != nil {
				json.Unmarshal([]byte(*program.Data), &programData)
			}

			detailsLevel := extractLevelFromProgramData(program.Data)
			courseResp := CourseResponse{
				ID:              fmt.Sprintf("inst-%d", program.ID),
				Title:           program.ProgramName,
				Affiliation:     program.InstitutionName,
				Duration:        program.Duration,
				EstFee:          program.Fee,
				Description:     program.Description,
				Location:        program.InstitutionLocation,
				Level:           detailsLevel,
				Source:          "institution",
				InstitutionName: program.InstitutionName,
				Image:           program.BannerURL,
				IsGlobal:        false,
				Status:          "active",
			}

			// Merge with global course data if linked
			if program.GlobalCourseID != nil && *program.GlobalCourseID > 0 {
				globalCourse, err := s.repo.FindCourseByIDOnly(*program.GlobalCourseID)
				if err == nil && globalCourse != nil {
					if courseResp.Title == "" {
						courseResp.Title = globalCourse.Title
					}
					if courseResp.Duration == "" {
						courseResp.Duration = globalCourse.Duration
					}
					if courseResp.Description == "" {
						courseResp.Description = globalCourse.Description
					}
					if courseResp.Level == "" {
						courseResp.Level = globalCourse.Level
					}
					if globalCourse.AffiliationID != nil {
						aff, err := s.repo.FindAffiliationByID(*globalCourse.AffiliationID)
						if err == nil && aff != nil {
							courseResp.Affiliation = aff.Name
						}
					} else if globalCourse.NonUniversityAffiliation != "" {
						courseResp.Affiliation = globalCourse.NonUniversityAffiliation
					}
				}
			}

			return &CourseDetailsResponse{
				Course:                courseResp,
				About:                 []string{program.Description},
				Mode:                  "On-Campus",
				DegreeLabel:           "Program",
				AdmissionRequirements: []string{"As per institution criteria"},
				Universities:          []string{program.InstitutionName},
				Data:                  programData,
			}, nil
		}
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

	affName := s.resolveAffiliationName(course)
	baseCourse := buildCourseResponse(*course, colleges, affName)

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

	admissionRequirements := parseStringArrayField(course.Admissions)

	careers := make([]CourseCareerOpportunity, 0)
	if len(course.Careers) > 0 {
		_ = json.Unmarshal(course.Careers, &careers)
	}

	about := parseStringArrayField(course.About)

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
		HighlightsUniversity:  affName,
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

func (s *Service) GetEducationNewsBySlug(slug string) (*NewsResponse, error) {
	news, err := s.repo.FindNewsBySlug(slug)
	if err != nil {
		return nil, err
	}
	resp := buildNewsResponse(*news)
	return &resp, nil
}

func (s *Service) GetEducationNewsFiltered(page, limit int, category, search, sort string, universityID *uint) ([]NewsResponse, PaginationMeta, error) {
	news, total, err := s.repo.FindNewsFiltered(page, limit, category, search, sort, universityID)
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

func (s *Service) GetEducationEventBySlug(slug string) (*EventResponse, error) {
	event, err := s.repo.FindEventBySlug(slug)
	if err != nil {
		return nil, err
	}
	resp := buildEventResponse(*event)
	return &resp, nil
}

func (s *Service) GetEducationEventsFiltered(page, limit int, category, search, sort, featuredStr string, universityID *uint) ([]EventResponse, PaginationMeta, error) {
	events, total, err := s.repo.FindEventsFiltered(page, limit, category, search, sort, featuredStr, universityID)
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

func (s *Service) GetAllEventsAdmin(page, limit int, universityID *uint, hasUniversity bool) ([]EventResponse, PaginationMeta, error) {
	events, total, err := s.repo.FindAllEvents(page, limit, universityID, hasUniversity)
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
		UniversityID:    req.UniversityID,
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

	s.systemSvc.CreatePublicNotification(
		"New Event: "+event.Title,
		event.Excerpt,
		"event",
		fmt.Sprintf("/events/%d", event.ID),
		"fa-calendar",
		"text-purple-600",
		"bg-purple-100",
	)

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
	if req.UniversityID != 0 {
		updates["university_id"] = req.UniversityID
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

func (s *Service) GetBlogByIDAdmin(id string) (*BlogResponse, error) {
	blog, err := s.repo.FindBlogByIDAdmin(id)
	if err != nil {
		return nil, err
	}
	resp := buildBlogResponse(*blog)
	return &resp, nil
}

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

	published := true
	if req.Published != nil {
		published = *req.Published
	}
	featured := false
	if req.Featured != nil {
		featured = *req.Featured
	}

	blog := Blog{
		Title:     req.Title,
		Slug:      generateSlug(req.Title),
		Excerpt:   req.Excerpt,
		Content:   req.Content,
		Image:     req.Image,
		Author:    req.Author,
		Category:  req.Category,
		Tags:      tagsJSON,
		Featured:  featured,
		Published: published,
	}

	if err := s.repo.CreateBlog(&blog); err != nil {
		return nil, err
	}

	s.systemSvc.CreatePublicNotification(
		"New Blog: "+blog.Title,
		blog.Excerpt,
		"blog",
		fmt.Sprintf("/blog/%d", blog.ID),
		"fa-blog",
		"text-green-600",
		"bg-green-100",
	)

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
		ID:           news.ID,
		UniversityID: news.UniversityID,
		Slug:         news.Slug,
		Category:     news.Category,
		Title:        news.Title,
		Excerpt:      news.Excerpt,
		Content:      news.Content,
		Image:        news.Image,
		Author:       news.Author,
		Date:         news.Date,
		ReadTime:     news.ReadTime,
		Source:       news.Source,
		Tags:         parseStringArrayField(news.Tags),
		CreatedAt:    news.CreatedAt.String(),
		UpdatedAt:    news.UpdatedAt.String(),
	}
}

func (s *Service) GetAllNewsAdmin(page, limit int, category, search string, universityID *uint, hasUniversity bool) ([]AdminNewsResponse, PaginationMeta, error) {
	news, total, err := s.repo.FindAllNewsAdmin(page, limit, category, search, universityID, hasUniversity)
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

	slugStr := slug.GenerateUnique("edu-"+req.Title, func(slugStr string) bool {
		var count int64
		s.repo.db.Model(&News{}).Where("slug = ?", slugStr).Count(&count)
		return count > 0
	})

	news := News{
		Slug:         slugStr,
		UniversityID: req.UniversityID,
		Category:     req.Category,
		Title:        req.Title,
		Excerpt:      req.Excerpt,
		Content:      req.Content,
		Image:        req.Image,
		Author:       req.Author,
		Date:         req.Date,
		ReadTime:     req.ReadTime,
		Source:       req.Source,
		Tags:         tagsJSON,
	}

	if err := s.repo.CreateNews(&news); err != nil {
		return nil, err
	}

	s.systemSvc.CreatePublicNotification(
		"New News: "+news.Title,
		news.Excerpt,
		"news",
		fmt.Sprintf("/news/%d", news.ID),
		"fa-newspaper",
		"text-blue-600",
		"bg-blue-100",
	)

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
	if req.UniversityID != 0 {
		news.UniversityID = req.UniversityID
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

func socialLinksList(data []byte) []interface{} {
	if len(data) == 0 {
		return nil
	}
	var links []interface{}
	json.Unmarshal(data, &links)
	return links
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
		var eligibilityList []interface{}
		if len(ie.EligibilityList) > 0 {
			json.Unmarshal(ie.EligibilityList, &eligibilityList)
		}
		var applicationSteps []interface{}
		if len(ie.ApplicationSteps) > 0 {
			json.Unmarshal(ie.ApplicationSteps, &applicationSteps)
		}
		var examPattern []interface{}
		if len(ie.ExamPattern) > 0 {
			json.Unmarshal(ie.ExamPattern, &examPattern)
		}
		var subjectMarks []interface{}
		if len(ie.SubjectMarks) > 0 {
			json.Unmarshal(ie.SubjectMarks, &subjectMarks)
		}
		var modelSets []interface{}
		if len(ie.ModelSets) > 0 {
			json.Unmarshal(ie.ModelSets, &modelSets)
		}
		var upcomingDates []interface{}
		if len(ie.UpcomingDates) > 0 {
			json.Unmarshal(ie.UpcomingDates, &upcomingDates)
		}
		var contactPersons []interface{}
		if len(ie.ContactPersons) > 0 {
			json.Unmarshal(ie.ContactPersons, &contactPersons)
		}
		var faqs []interface{}
		if len(ie.Faqs) > 0 {
			json.Unmarshal(ie.Faqs, &faqs)
		}
		var examDateSchedules []interface{}
		if len(ie.ExamDateSchedules) > 0 {
			json.Unmarshal(ie.ExamDateSchedules, &examDateSchedules)
		}

		instName := ie.InstitutionName
		instLocation := loc
		instPhone := ie.InstitutionPhone
		instEmail := ie.InstitutionEmail
		instWebsite := ie.InstitutionWebsite
		instLogo := ie.InstitutionLogo

		if instName == "" {
			for _, item := range overviewDetails {
				if m, ok := item.(map[string]interface{}); ok {
					if t, _ := m["type"].(string); t == "institution_meta" {
						if n, _ := m["name"].(string); n != "" {
							instName = n
						}
						if l, _ := m["location"].(string); l != "" {
							instLocation = l
						}
						if p, _ := m["phone"].(string); p != "" {
							instPhone = p
						}
						if e, _ := m["email"].(string); e != "" {
							instEmail = e
						}
						if w, _ := m["link"].(string); w != "" {
							instWebsite = w
						}
						break
					}
				}
			}
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
			FormDeadline:      deadline,
			ID:                ie.ID,
			Title:             ie.Title,
			Description:       ie.Description,
			ExamDate:          ie.Date,
			ImageUrl:          ie.HeroBanner,
			Status:            ie.Status,
			Fee:               ie.Fee,
			University:        instName,
			Board:             instName,
			Phone:             instPhone,
			Email:             instEmail,
			Website:           instWebsite,
			Location:          instLocation,
			ContactNumber:     ie.ContactNumber,
			SocialLinks:       socialLinksList(ie.SocialLinks),
			InstitutionLogo:   instLogo,
			OverviewDetails:   overviewDetails,
			ApplicationLink:   ie.ApplicationLink,
			NoticeFile:        ie.NoticeFile,
			EligibilityList:   eligibilityList,
			ApplicationSteps:  applicationSteps,
			ExamPattern:       examPattern,
			SubjectMarks:      subjectMarks,
			ModelSets:         modelSets,
			UpcomingDates:     upcomingDates,
			ContactPersons:    contactPersons,
			Faqs:              faqs,
			ExamDateSchedules: examDateSchedules,
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
		var socialLinks []interface{}
		if len(instEntrance.SocialLinks) > 0 {
			json.Unmarshal(instEntrance.SocialLinks, &socialLinks)
		}
		var eligibilityList []interface{}
		if len(instEntrance.EligibilityList) > 0 {
			json.Unmarshal(instEntrance.EligibilityList, &eligibilityList)
		}
		var applicationSteps []interface{}
		if len(instEntrance.ApplicationSteps) > 0 {
			json.Unmarshal(instEntrance.ApplicationSteps, &applicationSteps)
		}
		var examPattern []interface{}
		if len(instEntrance.ExamPattern) > 0 {
			json.Unmarshal(instEntrance.ExamPattern, &examPattern)
		}
		var subjectMarks []interface{}
		if len(instEntrance.SubjectMarks) > 0 {
			json.Unmarshal(instEntrance.SubjectMarks, &subjectMarks)
		}
		var modelSets []interface{}
		if len(instEntrance.ModelSets) > 0 {
			json.Unmarshal(instEntrance.ModelSets, &modelSets)
		}
		var upcomingDates []interface{}
		if len(instEntrance.UpcomingDates) > 0 {
			json.Unmarshal(instEntrance.UpcomingDates, &upcomingDates)
		}
		var contactPersons []interface{}
		if len(instEntrance.ContactPersons) > 0 {
			json.Unmarshal(instEntrance.ContactPersons, &contactPersons)
		}
		var faqs []interface{}
		if len(instEntrance.Faqs) > 0 {
			json.Unmarshal(instEntrance.Faqs, &faqs)
		}
		var examDateSchedules []interface{}
		if len(instEntrance.ExamDateSchedules) > 0 {
			json.Unmarshal(instEntrance.ExamDateSchedules, &examDateSchedules)
		}
		var examinationSchedule []interface{}
		if len(instEntrance.ExaminationSchedule) > 0 {
			json.Unmarshal(instEntrance.ExaminationSchedule, &examinationSchedule)
		}
		var programsOffered []interface{}
		if len(instEntrance.ProgramsOffered) > 0 {
			json.Unmarshal(instEntrance.ProgramsOffered, &programsOffered)
		}
		email := instEntrance.Email
		if email == "" {
			email = instEntrance.InstitutionEmail
		}
		contactNumber := instEntrance.ContactNumber
		if contactNumber == "" {
			contactNumber = instEntrance.InstitutionPhone
		}
		return &PublicEntranceResponse{
			ID:                  instEntrance.ID,
			Title:               instEntrance.Title,
			Description:         instEntrance.Description,
			ExamDate:            instEntrance.Date,
			ImageUrl:            instEntrance.HeroBanner,
			Status:              instEntrance.Status,
			Fee:                 instEntrance.Fee,
			University:          instEntrance.InstitutionName,
			Board:               instEntrance.InstitutionName,
			Phone:               contactNumber,
			Email:               email,
			Website:             instEntrance.InstitutionWebsite,
			Location:            loc,
			ContactNumber:       contactNumber,
			SocialLinks:         socialLinks,
			InstitutionLogo:     instEntrance.InstitutionLogo,
			OverviewDetails:     overviewDetails,
			ApplicationLink:     instEntrance.ApplicationLink,
			NoticeFile:          instEntrance.NoticeFile,
			EligibilityList:     eligibilityList,
			ApplicationSteps:    applicationSteps,
			ExamPattern:         examPattern,
			SubjectMarks:        subjectMarks,
			ModelSets:           modelSets,
			UpcomingDates:       upcomingDates,
			ContactPersons:      contactPersons,
			Faqs:                faqs,
			ExamDateSchedules:   examDateSchedules,
			ExaminationSchedule: examinationSchedule,
			ProgramsOffered:     programsOffered,
		}, nil
	}

	return nil, gorm.ErrRecordNotFound
}

func (s *Service) ResolveCourse(globalCourseID uint, institutionID uint) (*ResolvedCourse, error) {
	course, affiliation, err := s.repo.FindCourseByIDWithAffiliation(globalCourseID)
	if err != nil {
		return nil, err
	}

	program, err := s.instRepo.FindProgramByGlobalCourse(institutionID, globalCourseID)
	if err != nil {
		return nil, err
	}

	var whoShouldChoose []PersonaItem
	json.Unmarshal(course.WhoShouldChoose, &whoShouldChoose)

	var features []FeatureItem
	json.Unmarshal(course.Features, &features)

	var eligibilityRows []EligibilityRow
	json.Unmarshal(course.EligibilityRows, &eligibilityRows)

	var admissionSteps []AdmissionStep
	json.Unmarshal(course.AdmissionSteps, &admissionSteps)

	var subjectGroups []SubjectGroup
	json.Unmarshal(course.SubjectGroups, &subjectGroups)

	var feeItems []FeeItem
	json.Unmarshal(course.FeeItems, &feeItems)

	var scholarships []ScholarshipItem
	json.Unmarshal(course.Scholarships, &scholarships)

	var fullTimeCourses []FullTimeCourse
	json.Unmarshal(course.FullTimeCourses, &fullTimeCourses)

	var careers []CareerItem
	json.Unmarshal(course.Careers, &careers)

	var faqs []FaqItem
	json.Unmarshal(course.FAQs, &faqs)

	var overrides CourseOverrides
	json.Unmarshal(program.Overrides, &overrides)

	nullifiedFields := program.NullifiedFields

	description := course.Description
	if contains(nullifiedFields, "description") {
		description = ""
	} else if overrides.Description != nil {
		description = *overrides.Description
	}

	bannerURL := course.BannerURL
	if contains(nullifiedFields, "banner_url") {
		bannerURL = ""
	} else if overrides.BannerURL != nil {
		bannerURL = *overrides.BannerURL
	}

	resolvedCareers := careers
	if contains(nullifiedFields, "careers") {
		resolvedCareers = []CareerItem{}
	} else if overrides.Careers != nil {
		resolvedCareers = overrides.Careers
	}

	resolvedFAQs := faqs
	if contains(nullifiedFields, "faqs") {
		resolvedFAQs = []FaqItem{}
	} else if overrides.FAQs != nil {
		resolvedFAQs = overrides.FAQs
	}

	var instWhoShouldChoose []PersonaItem
	json.Unmarshal(program.WhoShouldChoose, &instWhoShouldChoose)

	var instFeatures []FeatureItem
	json.Unmarshal(program.Features, &instFeatures)

	var instFullTimeCourses []FullTimeCourse
	json.Unmarshal(program.FullTimeCourses, &instFullTimeCourses)

	var instFeeItems []FeeItem
	json.Unmarshal(program.FeeItems, &instFeeItems)

	resolved := &ResolvedCourse{
		ID:              course.ID,
		Title:           course.Title,
		Duration:        course.Duration,
		Level:           course.Level,
		AffiliationID:   course.AffiliationID,
		AffiliationName: "",
		Description:     description,
		BannerURL:       bannerURL,
		Careers:         resolvedCareers,
		FAQs:            resolvedFAQs,
		EligibilityRows: eligibilityRows,
		AdmissionSteps:  admissionSteps,
		SubjectGroups:   subjectGroups,
		ScholarshipDesc: course.ScholarshipDesc,
		ScholarshipNotes: course.ScholarshipNotes,
		Scholarships:    scholarships,
		InstitutionID:   program.InstitutionID,
		Fee:             program.Fee,
		Eligibility:     program.Eligibility,
		Capacity:        program.Capacity,
		WhoShouldChoose: instWhoShouldChoose,
		Features:        instFeatures,
		FullTimeCourses: instFullTimeCourses,
		FeeItems:        instFeeItems,
		Status:          program.Status,
	}

	if affiliation != nil {
		resolved.AffiliationName = affiliation.Name
	}

	return resolved, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (s *Service) GetCoursesByLevel(level string, page, limit int) ([]Course, int64, error) {
	return s.repo.FindCoursesByLevel(level, page, limit)
}

func (s *Service) GetCoursesByAffiliation(affiliationID uint, page, limit int) ([]Course, int64, error) {
	return s.repo.FindCoursesByAffiliation(affiliationID, page, limit)
}

func (s *Service) GetSecondaryCourses(page, limit int) ([]Course, int64, error) {
	return s.repo.FindSecondaryCourses(page, limit)
}

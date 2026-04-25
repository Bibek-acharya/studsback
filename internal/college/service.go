package college

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetColleges(filters CollegeFilters) (*CollegeListResponse, error) {
	colleges, total, err := s.repo.FindAll(filters)
	if err != nil {
		return nil, errors.New("failed to fetch colleges")
	}

	totalPages := int64(math.Ceil(float64(total) / float64(filters.PageSize)))
	if totalPages == 0 {
		totalPages = 0
	}

	responses := make([]CollegeResponse, 0, len(colleges))
	for _, college := range colleges {
		responses = append(responses, buildCollegeResponse(college))
	}

	return &CollegeListResponse{
		Colleges: responses,
		Pagination: PaginationInfo{
			Page:       filters.Page,
			PageSize:   filters.PageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *Service) GetCollegeByID(id uint) (*CollegeResponse, error) {
	college, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("college not found")
	}
	resp := buildCollegeResponse(*college)
	return &resp, nil
}

func (s *Service) CreateCollege(req CreateCollegeRequest) (*CollegeResponse, error) {
	if !s.repo.UniversityExists(req.UniversityID) {
		return nil, errors.New("invalid university_id. College must be affiliated to an existing university")
	}

	university, err := s.repo.FindUniversityByID(req.UniversityID)
	if err != nil {
		return nil, errors.New("invalid university_id. College must be affiliated to an existing university")
	}

	var featuredPrograms, amenities, profileTags []byte

	if len(req.FeaturedPrograms) > 0 {
		featuredPrograms, err = json.Marshal(req.FeaturedPrograms)
		if err != nil {
			return nil, errors.New("invalid featured programs format")
		}
	}

	if len(req.Amenities) > 0 {
		amenities, err = json.Marshal(req.Amenities)
		if err != nil {
			return nil, errors.New("invalid amenities format")
		}
	}

	if len(req.ProfileTags) > 0 {
		profileTags, err = json.Marshal(req.ProfileTags)
		if err != nil {
			return nil, errors.New("invalid profile tags format")
		}
	}

	college := College{
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
		AcademicFitScore: req.AcademicFitScore,
		CampusLifeScore:  req.CampusLifeScore,
		CareerFitScore:   req.CareerFitScore,
		BalancedFitScore: req.BalancedFitScore,
		ProfileTags:      profileTags,
	}

	if err := s.repo.Create(&college); err != nil {
		return nil, errors.New("failed to create college")
	}

	created, err := s.repo.FindByID(college.ID)
	if err != nil {
		return nil, errors.New("failed to fetch created college")
	}

	resp := buildCollegeResponse(*created)
	return &resp, nil
}

func (s *Service) UploadCollegeImage(file *multipart.FileHeader) ([]string, error) {
	uploadDir := filepath.Join("uploads", "colleges")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory")
	}

	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file")
	}
	defer src.Close()

	ct := file.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("only image files are allowed")
	}

	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".jpg"
	}

	filename := fmt.Sprintf("%d_%d%s", time.Now().UnixNano(), rand.Intn(999999), ext)
	savePath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file")
	}
	defer dst.Close()

	if _, err := dst.ReadFrom(src); err != nil {
		return nil, fmt.Errorf("failed to save file")
	}

	return []string{"/uploads/colleges/" + filename}, nil
}

func (s *Service) UpdateCollege(id uint, req UpdateCollegeRequest) (*CollegeResponse, error) {
	college, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("college not found")
	}

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
		university, err := s.repo.FindUniversityByName(req.Affiliation)
		if err != nil {
			return nil, errors.New("invalid affiliation. College must be affiliated to an existing university")
		}
		college.UniversityID = university.ID
		college.Affiliation = university.Name
	}
	if req.UniversityID != nil {
		university, err := s.repo.FindUniversityByID(*req.UniversityID)
		if err != nil {
			return nil, errors.New("invalid university_id. College must be affiliated to an existing university")
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

	if req.AcademicFitScore != nil {
		college.AcademicFitScore = *req.AcademicFitScore
	}
	if req.CampusLifeScore != nil {
		college.CampusLifeScore = *req.CampusLifeScore
	}
	if req.CareerFitScore != nil {
		college.CareerFitScore = *req.CareerFitScore
	}
	if req.BalancedFitScore != nil {
		college.BalancedFitScore = *req.BalancedFitScore
	}
	if len(req.ProfileTags) > 0 {
		if data, err := json.Marshal(req.ProfileTags); err == nil {
			college.ProfileTags = data
		}
	}

	if err := s.repo.Update(college); err != nil {
		return nil, errors.New("failed to update college")
	}

	updated, err := s.repo.FindByID(college.ID)
	if err != nil {
		return nil, errors.New("failed to fetch updated college")
	}

	resp := buildCollegeResponse(*updated)
	return &resp, nil
}

func (s *Service) DeleteCollege(id uint) error {
	_, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("college not found")
	}

	if err := s.repo.Delete(id); err != nil {
		return errors.New("failed to delete college")
	}

	return nil
}

func (s *Service) ApproveCollege(id uint) (*CollegeResponse, error) {
	college, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("college not found")
	}

	if college.Verified {
		return nil, errors.New("college is already approved")
	}

	if err := s.repo.Approve(id); err != nil {
		return nil, errors.New("failed to approve college")
	}

	updated, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("failed to fetch approved college")
	}

	resp := buildCollegeResponse(*updated)
	return &resp, nil
}

func (s *Service) ToggleCollegeFeatured(id uint) (*CollegeResponse, error) {
	_, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("college not found")
	}

	if err := s.repo.ToggleFeatured(id); err != nil {
		return nil, errors.New("failed to update college featured status")
	}

	updated, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("failed to fetch updated college")
	}

	resp := buildCollegeResponse(*updated)
	return &resp, nil
}

func (s *Service) GetFeaturedColleges(limit int) (*FeaturedCollegesResponse, error) {
	colleges, err := s.repo.FindFeatured(limit)
	if err != nil {
		return nil, errors.New("failed to fetch featured colleges")
	}

	responses := make([]CollegeResponse, 0, len(colleges))
	for _, college := range colleges {
		responses = append(responses, buildCollegeResponse(college))
	}

	return &FeaturedCollegesResponse{Colleges: responses}, nil
}

func (s *Service) GetCollegeFilterCounts() (*CollegeFilterCountsResponse, error) {
	counts, err := s.repo.GetFilterCounts()
	if err != nil {
		return nil, errors.New("failed to fetch college filter counts")
	}

	return counts, nil
}

func buildCollegeResponse(college College) CollegeResponse {
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
		Featured:         college.Featured,
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
		AcademicFitScore: college.AcademicFitScore,
		CampusLifeScore:  college.CampusLifeScore,
		CareerFitScore:   college.CareerFitScore,
		BalancedFitScore: college.BalancedFitScore,
		ProfileTags:      parseJSONField(college.ProfileTags, []interface{}{}),
	}
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

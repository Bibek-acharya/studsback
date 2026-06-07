package university

import (
	"encoding/json"
	"errors"
	"strings"
)

var ErrNameRequired = errors.New("name is required")

func toUniversityResponse(uni University, colleges []College) UniversityResponse {
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
		IsNepali:        uni.IsNepali,
		Rank:            uni.Rank,
		Verified:        uni.Verified,
		IsPopular:       uni.Popular,
		Status:          uni.Status,
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

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetUniversities(search, uniType, status string, popular bool, isNepali string) ([]UniversityResponse, error) {
	universities, err := s.repo.FindAll(search, uniType, status, popular, isNepali)
	if err != nil {
		return nil, err
	}

	responses := make([]UniversityResponse, 0, len(universities))
	for _, uni := range universities {
		colleges, err := s.repo.FindCollegesByUniversityID(uni.ID)
		if err != nil {
			return nil, err
		}
		responses = append(responses, toUniversityResponse(uni, colleges))
	}

	return responses, nil
}

func (s *Service) GetUniversityByID(id uint) (*UniversityResponse, []UniversityCollegeResponse, error) {
	uni, err := s.repo.FindByID(id)
	if err != nil {
		return nil, nil, err
	}

	colleges, err := s.repo.FindCollegesByUniversityID(uni.ID)
	if err != nil {
		return nil, nil, err
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

	response := toUniversityResponse(*uni, colleges)
	return &response, collegeResponses, nil
}

func (s *Service) AdminGetUniversityByID(id uint) (*UniversityResponse, []UniversityCollegeResponse, error) {
	uni, err := s.repo.FindByIDFull(id)
	if err != nil {
		return nil, nil, err
	}

	colleges, err := s.repo.FindCollegesByUniversityID(uni.ID)
	if err != nil {
		return nil, nil, err
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

	response := toUniversityResponse(*uni, colleges)
	return &response, collegeResponses, nil
}

func (s *Service) GetUniversityFilterCounts(isNepali string) (*UniversityFilterCountsResponse, error) {
	return s.repo.GetFilterCounts(isNepali)
}

func (s *Service) GetUniversityTab(id uint, tab string) ([]byte, error) {
	return s.repo.GetTabData(id, tab)
}

func (s *Service) CreateUniversity(req CreateUniversityRequest) (*University, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, ErrNameRequired
	}

	uni := &University{
		Name:           req.Name,
		Logo:           strings.TrimSpace(req.Logo),
		Location:       strings.TrimSpace(req.Location),
		Type:           strings.TrimSpace(req.Type),
		IsNepali:       req.IsNepali,
		Rank:           req.Rank,
		Rating:         req.Rating,
		ReviewCount:    req.ReviewCount,
		Verified:       req.Verified,
		Popular:        req.Popular,
		Status:         req.Status,
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

	err := s.repo.Create(uni)
	if err != nil {
		return nil, err
	}

	return uni, nil
}

func (s *Service) UpdateUniversity(id uint, req UpdateUniversityRequest) (*University, error) {
	uni, err := s.repo.FindByIDFull(id)
	if err != nil {
		return nil, err
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
	if req.IsNepali != nil {
		uni.IsNepali = *req.IsNepali
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
	if req.Status != nil {
		uni.Status = *req.Status
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
		return nil, ErrNameRequired
	}

	err = s.repo.Update(uni)
	if err != nil {
		return nil, err
	}

	return uni, nil
}

func (s *Service) DeleteUniversity(id uint) error {
	_, err := s.repo.FindByIDFull(id)
	if err != nil {
		return err
	}
	return s.repo.Delete(id)
}

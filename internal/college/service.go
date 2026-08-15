package college

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"mime/multipart"
	"strings"

	"studsphere/backend/internal/shared/utils"
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
	var featuredPrograms, amenities, profileTags []byte
	var err error

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
		Name:             req.Name,
		FullName:         req.FullName,
		Location:         req.Location,
		Affiliation:      req.Affiliation,
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
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
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
	ct := file.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("only image files are allowed")
	}

	url, err := utils.SaveUploadedImage(file, "colleges")
	if err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	return []string{url}, nil
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
		college.Affiliation = req.Affiliation
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
	if req.Latitude != nil {
		college.Latitude = req.Latitude
	}
	if req.Longitude != nil {
		college.Longitude = req.Longitude
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

func (s *Service) GetCollegeFilterCounts(level string) (*CollegeFilterCountsResponse, error) {
	counts, err := s.repo.GetFilterCounts(level)
	if err != nil {
		log.Printf("GetCollegeFilterCounts error (level=%q): %v", level, err)
		return nil, errors.New("failed to fetch college filter counts")
	}

	return counts, nil
}

func (s *Service) GetMapColleges(north, south, east, west float64) ([]CollegeMapDTO, error) {
	var colleges []College
	var err error

	if north == 0 && south == 0 && east == 0 && west == 0 {
		colleges, err = s.repo.FindAllWithCoords()
	} else {
		colleges, err = s.repo.FindWithinBounds(north, south, east, west)
	}
	if err != nil {
		return nil, err
	}

	dtos := buildCollegeMapDTOs(colleges)

	// Enrich DTOs with institution data (gallery, logo, etc.)
	if len(colleges) > 0 {
		collegeIDs := make([]uint, len(colleges))
		for i, c := range colleges {
			collegeIDs[i] = c.ID
		}
		institutionMap, err := s.repo.FindInstitutionsByCollegeIDs(collegeIDs)
		if err == nil {
			for i, dto := range dtos {
				if inst, ok := institutionMap[dto.ID]; ok {
					// Prefer institution logo over college image
					if inst.LogoURL != "" {
						dtos[i].Logo = inst.LogoURL
					}
					// Get gallery from institution profile_data
					if inst.ProfileData != nil {
						var profileData map[string]interface{}
						if err := json.Unmarshal([]byte(*inst.ProfileData), &profileData); err == nil {
							if gallery, ok := profileData["gallery_data"]; ok {
								dtos[i].Gallery = gallery
							}
						}
					}
				}
			}
		}

		// Enrich with review aggregations from reviews table
		reviewMap, err := s.repo.FindReviewAggregations(collegeIDs)
		if err == nil {
			for i, dto := range dtos {
				if agg, ok := reviewMap[dto.ID]; ok {
					dtos[i].Rating = agg.Rating
					dtos[i].Reviews = agg.Reviews
				}
			}
		}
	}

	institutions, err := s.repo.FindInstitutionsWithCoords()
	if err == nil {
		for _, inst := range institutions {
			dtos = append(dtos, CollegeMapDTO{
				ID:        inst.ID,
				Name:      inst.Name,
				Latitude:  inst.Latitude,
				Longitude: inst.Longitude,
				District:  inst.District,
				Province:  inst.Province,
				Type:      inst.Type,
			})
		}
	}

	return dtos, nil
}

func (s *Service) UpdateCollegeLocation(id uint, lat, lng float64) error {
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return errors.New("invalid coordinates: lat -90..90, lng -180..180")
	}
	return s.repo.UpdateLocation(id, lat, lng)
}

func buildCollegeMapDTOs(colleges []College) []CollegeMapDTO {
	dtos := make([]CollegeMapDTO, len(colleges))
	for i, c := range colleges {
		dtos[i] = CollegeMapDTO{
			ID:        c.ID,
			Name:      c.Name,
			Latitude:  *c.Latitude,
			Longitude: *c.Longitude,
			Logo:      c.ImageURL,
			District:  c.Location,
			Type:      c.CollegeType,
			Rating:    c.Rating,
			Reviews:   c.Reviews,
			Gallery:   parseJSONField(c.Gallery, []interface{}{}),
		}
	}
	return dtos
}

func buildCollegeResponse(college College) CollegeResponse {
	affiliation := college.Affiliation
	if len(college.UniversityAffiliations) > 0 {
		var uniIDs []uint
		if err := json.Unmarshal(college.UniversityAffiliations, &uniIDs); err == nil && len(uniIDs) > 0 {
			affiliation = fmt.Sprintf("University IDs: %v", uniIDs)
		}
	}

	return CollegeResponse{
		ID:                       college.ID,
		CreatedAt:                college.CreatedAt,
		UpdatedAt:                college.UpdatedAt,
		Name:                     college.Name,
		FullName:                 college.FullName,
		Location:                 college.Location,
		Affiliation:              affiliation,
		CollegeType:              college.CollegeType,
		Verified:                 college.Verified,
		Claimed:                  college.Claimed,
		Popular:                  college.Popular,
		Featured:                 college.Featured,
		Rating:                   college.Rating,
		Reviews:                  college.Reviews,
		Programs:                 college.Programs,
		Established:              college.Established,
		Students:                 college.Students,
		Description:              college.Description,
		Website:                  college.Website,
		Email:                    college.Email,
		Phone:                    college.Phone,
		ImageURL:                 college.ImageURL,
		FeaturedPrograms:         parseJSONField(college.FeaturedPrograms, []interface{}{}),
		Amenities:                parseJSONField(college.Amenities, []interface{}{}),
		Courses:                  parseJSONField(college.Courses, []interface{}{}),
		Scholarships:             parseJSONField(college.Scholarships, []interface{}{}),
		Gallery:                  parseJSONField(college.Gallery, []interface{}{}),
		ProgramsList:             parseJSONField(college.ProgramsList, []interface{}{}),
		About:                    parseJSONField(college.About, map[string]interface{}{}),
		Admissions:               parseJSONField(college.Admissions, map[string]interface{}{}),
		AdmissionCards:           parseJSONField(college.AdmissionCards, []interface{}{}),
		OfferedPrograms:          parseJSONField(college.OfferedPrograms, []interface{}{}),
		Alumni:                   parseJSONField(college.Alumni, []interface{}{}),
		Departments:              parseJSONField(college.Departments, []interface{}{}),
		CollegeReviews:           parseJSONField(college.CollegeReviews, []interface{}{}),
		AcademicFitScore:         college.AcademicFitScore,
		CampusLifeScore:          college.CampusLifeScore,
		CareerFitScore:           college.CareerFitScore,
		BalancedFitScore:         college.BalancedFitScore,
		ProfileTags:              parseJSONField(college.ProfileTags, []interface{}{}),
		Latitude:                 college.Latitude,
		Longitude:                college.Longitude,
		UniversityAffiliations:   parseJSONField(college.UniversityAffiliations, []uint{}),
		NonUniversityAffiliation: college.NonUniversityAffiliation,
		OffersCourse:             college.OffersCourse,
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

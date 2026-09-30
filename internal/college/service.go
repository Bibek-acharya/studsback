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
	// tenants answers "which college does this caller administer?" from
	// institution_users.college_id. It is the field every non-admin rule in
	// access.go turns on, and it is an interface so that rule can be tested
	// against a stub. Nil in a service built without it, in which case
	// TenantCollegeID returns 0 and every non-admin caller is denied — access.go
	// explains why that is the safe direction.
	tenants TenantCollegeLookup
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// WithTenantLookup supplies the institution_users lookup that access.go needs.
// Called by cmd/server/main.go, which already builds an institution.Repository
// and passes it to college.NewHandler.
func (s *Service) WithTenantLookup(tenants TenantCollegeLookup) *Service {
	s.tenants = tenants
	return s
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

// CreateCollege adds a row to the platform catalogue. Platform-admin only; see
// access.go for why a college is a first-party catalogue entry with no tenant
// column of its own.
func (s *Service) CreateCollege(v Viewer, req CreateCollegeRequest) (*CollegeResponse, error) {
	if err := s.authorizeCatalogue(v, "college create"); err != nil {
		return nil, err
	}

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

// UploadCollegeImage stores an image for the college editor. Platform-admin
// only, and the check is first so a refused caller does not get to push bytes
// into the object store.
func (s *Service) UploadCollegeImage(v Viewer, file *multipart.FileHeader) ([]string, error) {
	if err := s.authorizeCatalogue(v, "college image upload"); err != nil {
		return nil, err
	}
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

// UpdateCollege rewrites a college profile. Platform-admin only, and not merely
// because the route is called /admin: the fields it can set include Verified,
// Claimed, Featured, Popular, Rating and Reviews, which are the platform's
// curation of the catalogue rather than anything an institution owns. access.go.
func (s *Service) UpdateCollege(v Viewer, id uint, req UpdateCollegeRequest) (*CollegeResponse, error) {
	if err := s.authorizeCatalogue(v, "college update"); err != nil {
		return nil, err
	}
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

// DeleteCollege removes a college from the catalogue. Platform-admin only. It
// is a soft delete (College carries DeletedAt), so a mistake here is recoverable
// in the database but not from the API — there is no restore route.
func (s *Service) DeleteCollege(v Viewer, id uint) error {
	if err := s.authorizeCatalogue(v, "college delete"); err != nil {
		return err
	}
	_, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("college not found")
	}

	if err := s.repo.Delete(id); err != nil {
		return errors.New("failed to delete college")
	}

	return nil
}

// ApproveCollege is moderation. Platform-admin only.
func (s *Service) ApproveCollege(v Viewer, id uint) (*CollegeResponse, error) {
	if err := s.authorizeCatalogue(v, "college approve"); err != nil {
		return nil, err
	}
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

// ToggleCollegeFeatured is curation. Platform-admin only.
func (s *Service) ToggleCollegeFeatured(v Viewer, id uint) (*CollegeResponse, error) {
	if err := s.authorizeCatalogue(v, "college featured toggle"); err != nil {
		return nil, err
	}
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
					if inst.BannerURL != "" {
						dtos[i].Banner = inst.BannerURL
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
				Banner:    inst.BannerURL,
				Phone:     inst.Phone,
			})
		}
	}

	return dtos, nil
}

// UpdateCollegeLocation moves a college's map pin.
//
// The ONE tenant-scoped rule in this module, and the reason the module is not
// simply admin-only: a platform operator may move any college's pin, and an
// institution account may move its own, where "its own" is decided by
// institution_users.college_id. Before this took a Viewer, the route took an
// arbitrary :id and sat behind the shared roleMW, so any institution account
// could move any college's pin and corrupt a competitor's position in the
// find-college map.
//
// The institution's dedicated /institution/college/location route is untouched
// and still resolves the college from its own account, so nothing an institution
// legitimately did here stops working.
func (s *Service) UpdateCollegeLocation(v Viewer, id uint, lat, lng float64) error {
	if err := s.authorizeLocation(v, id); err != nil {
		return err
	}
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
			Phone:     c.Phone,
		}
	}
	return dtos
}

// calculateFitScores computes fit scores based on college data
func calculateFitScores(college College) (academic, campus, career, balanced int) {
	// Parse JSON fields to get counts
	featuredPrograms := parseJSONField(college.FeaturedPrograms, []interface{}{})
	amenities := parseJSONField(college.Amenities, []interface{}{})
	gallery := parseJSONField(college.Gallery, []interface{}{})
	alumni := parseJSONField(college.Alumni, []interface{}{})
	offeredPrograms := parseJSONField(college.OfferedPrograms, []interface{}{})
	courses := parseJSONField(college.Courses, []interface{}{})

	// Programs count: prefer actual count from offered_programs or courses
	programCount := len(offeredPrograms.([]interface{}))
	if programCount == 0 {
		programCount = len(courses.([]interface{}))
	}
	if programCount == 0 {
		programCount = college.Programs // fallback to static field
	}

	amenityCount := len(amenities.([]interface{}))
	galleryCount := len(gallery.([]interface{}))
	alumniCount := len(alumni.([]interface{}))
	featuredCount := len(featuredPrograms.([]interface{}))
	reviewCount := college.Reviews

	// Academic Fit: rating (40%) + programs (30%) + reviews (30%)
	academic = int(college.Rating*4 + float64(min(programCount, 10))*3 + float64(min(reviewCount, 50))*3/10)
	if academic > 10 {
		academic = 10
	}
	if academic < 1 {
		academic = 1
	}

	// Campus Life: amenities (40%) + gallery (30%) + rating (30%)
	campus = int(float64(min(amenityCount, 10))*4 + float64(min(galleryCount, 10))*3 + college.Rating*3)
	if campus > 10 {
		campus = 10
	}
	if campus < 1 {
		campus = 1
	}

	// Career Fit: programs (40%) + alumni (30%) + featured programs (30%)
	career = int(float64(min(programCount, 10))*4 + float64(min(alumniCount, 10))*3 + float64(min(featuredCount, 10))*3)
	if career > 10 {
		career = 10
	}
	if career < 1 {
		career = 1
	}

	// Balanced: average of three
	balanced = (academic + campus + career) / 3
	if balanced < 1 {
		balanced = 1
	}

	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func buildCollegeResponse(college College) CollegeResponse {
	affiliation := college.Affiliation
	if len(college.UniversityAffiliations) > 0 {
		var uniIDs []uint
		if err := json.Unmarshal(college.UniversityAffiliations, &uniIDs); err == nil && len(uniIDs) > 0 {
			affiliation = fmt.Sprintf("University IDs: %v", uniIDs)
		}
	}

	// Calculate fit scores dynamically
	academicFit, campusLifeFit, careerFit, balancedFit := calculateFitScores(college)

	// Calculate program count from actual data
	offeredPrograms := parseJSONField(college.OfferedPrograms, []interface{}{})
	courses := parseJSONField(college.Courses, []interface{}{})
	programCount := len(offeredPrograms.([]interface{}))
	if programCount == 0 {
		programCount = len(courses.([]interface{}))
	}
	if programCount == 0 {
		programCount = college.Programs // fallback to static field
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
		Programs:                 programCount,
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
		AcademicFitScore:         academicFit,
		CampusLifeScore:          campusLifeFit,
		CareerFitScore:           careerFit,
		BalancedFitScore:         balancedFit,
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

// Comparison History methods

func (s *Service) ValidateCollegeExists(collegeID uint) error {
	return s.repo.ValidateCollegeExists(collegeID)
}

func (s *Service) LogComparison(college1ID, college2ID uint, college1Name, college2Name string) error {
	return s.repo.LogComparison(college1ID, college2ID, college1Name, college2Name)
}

func (s *Service) GetPopularComparisons(limit int) ([]PopularComparison, error) {
	if limit <= 0 || limit > 20 {
		limit = 6
	}
	return s.repo.GetPopularComparisons(limit)
}

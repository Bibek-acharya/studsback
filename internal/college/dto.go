package college

type CreateCollegeRequest struct {
	UniversityID     uint     `json:"university_id" binding:"required"`
	Name             string   `json:"name" binding:"required"`
	FullName         string   `json:"full_name"`
	Location         string   `json:"location" binding:"required"`
	Affiliation      string   `json:"affiliation"`
	CollegeType      string   `json:"type"`
	Verified         bool     `json:"verified"`
	Popular          bool     `json:"popular"`
	Rating           float64  `json:"rating"`
	Reviews          int      `json:"reviews"`
	Programs         int      `json:"programs"`
	Established      string   `json:"established"`
	Students         string   `json:"students"`
	Description      string   `json:"description"`
	Website          string   `json:"website"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone"`
	ImageURL         string   `json:"image_url"`
	FeaturedPrograms []string `json:"featured_programs"`
	Amenities        []string `json:"amenities"`
	AcademicFitScore int      `json:"academic_fit_score"`
	CampusLifeScore  int      `json:"campus_life_score"`
	CareerFitScore   int      `json:"career_fit_score"`
	BalancedFitScore int      `json:"balanced_fit_score"`
	ProfileTags      []string `json:"profile_tags"`
}

type UpdateCollegeRequest struct {
	UniversityID     *uint    `json:"university_id"`
	Name             string   `json:"name"`
	FullName         string   `json:"full_name"`
	Location         string   `json:"location"`
	Affiliation      string   `json:"affiliation"`
	CollegeType      string   `json:"type"`
	Verified         *bool    `json:"verified"`
	Popular          *bool    `json:"popular"`
	Rating           *float64 `json:"rating"`
	Reviews          *int     `json:"reviews"`
	Programs         *int     `json:"programs"`
	Established      string   `json:"established"`
	Students         string   `json:"students"`
	Description      string   `json:"description"`
	Website          string   `json:"website"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone"`
	ImageURL         string   `json:"image_url"`
	FeaturedPrograms []string `json:"featured_programs"`
	Amenities        []string `json:"amenities"`
	AcademicFitScore *int     `json:"academic_fit_score"`
	CampusLifeScore  *int     `json:"campus_life_score"`
	CareerFitScore   *int     `json:"career_fit_score"`
	BalancedFitScore *int     `json:"balanced_fit_score"`
	ProfileTags      []string `json:"profile_tags"`
}

type CollegeResponse struct {
	ID               uint        `json:"id"`
	UniversityID     uint        `json:"university_id"`
	CreatedAt        interface{} `json:"created_at"`
	UpdatedAt        interface{} `json:"updated_at"`
	Name             string      `json:"name"`
	FullName         string      `json:"full_name,omitempty"`
	Location         string      `json:"location"`
	Affiliation      string      `json:"affiliation"`
	CollegeType      string      `json:"type"`
	Verified         bool        `json:"verified"`
	Claimed          bool        `json:"claimed"`
	Popular          bool        `json:"popular"`
	Featured         bool        `json:"featured"`
	Rating           float64     `json:"rating"`
	Reviews          int         `json:"reviews"`
	Programs         int         `json:"programs"`
	Established      string      `json:"established,omitempty"`
	Students         string      `json:"students,omitempty"`
	Description      string      `json:"description,omitempty"`
	Website          string      `json:"website,omitempty"`
	Email            string      `json:"email,omitempty"`
	Phone            string      `json:"phone,omitempty"`
	ImageURL         string      `json:"image_url,omitempty"`
	FeaturedPrograms interface{} `json:"featured_programs,omitempty"`
	Amenities        interface{} `json:"amenities,omitempty"`
	Courses          interface{} `json:"courses,omitempty"`
	Scholarships     interface{} `json:"scholarships,omitempty"`
	Gallery          interface{} `json:"gallery,omitempty"`
	ProgramsList     interface{} `json:"programs_list,omitempty"`
	About            interface{} `json:"about,omitempty"`
	Admissions       interface{} `json:"admissions,omitempty"`
	AdmissionCards   interface{} `json:"admission_cards,omitempty"`
	OfferedPrograms  interface{} `json:"offered_programs,omitempty"`
	Alumni           interface{} `json:"alumni,omitempty"`
	Departments      interface{} `json:"departments,omitempty"`
	CollegeReviews   interface{} `json:"college_reviews,omitempty"`
	AcademicFitScore int         `json:"academic_fit_score"`
	CampusLifeScore  int         `json:"campus_life_score"`
	CareerFitScore   int         `json:"career_fit_score"`
	BalancedFitScore int         `json:"balanced_fit_score"`
	ProfileTags      interface{} `json:"profile_tags,omitempty"`
}

type PaginationInfo struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

type CollegeListResponse struct {
	Colleges   []CollegeResponse `json:"colleges"`
	Pagination PaginationInfo    `json:"pagination"`
}

type CollegeFilters struct {
	UniversityID    string
	Location        string
	Affiliation     string
	Type            string
	Academic        []string
	Program         []string
	Province        []string
	District        []string
	Local           []string
	Scholarship     []string
	Facilities      []string
	FeeMax          int
	Verified        string
	Popular         string
	DirectAdmission bool
	MinRating       string
	Search          string
	CourseID        string
	Sort            string
	Order           string
	Page            int
	PageSize        int
}

type FeaturedCollegesResponse struct {
	Colleges []CollegeResponse `json:"colleges"`
}

type CollegeFilterCountsResponse struct {
	Total           int64            `json:"total"`
	TypeCounts      map[string]int64 `json:"type_counts"`
	TypeCountsByID  map[string]int64 `json:"type_counts_by_id"`
	FacetCountsByID map[string]int64 `json:"facet_counts_by_id"`
	Featured        int64            `json:"featured"`
	Verified        int64            `json:"verified"`
	Popular         int64            `json:"popular"`
}

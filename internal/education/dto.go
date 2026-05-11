package education

type FilterCounts struct {
	Levels  []string `json:"levels"`
	Streams []string `json:"streams"`
	Status  []string `json:"status"`
}

type ExamResponse struct {
	ID           uint     `json:"id"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Board        string   `json:"board"`
	Badges       []string `json:"badges"`
	Level        string   `json:"level"`
	Type         string   `json:"type"`
	ExamDate     string   `json:"examDate"`
	FormDeadline string   `json:"formDeadline"`
	Fee          string   `json:"fee"`
	Highlights   []string `json:"highlights"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	ImageUrl     string   `json:"imageUrl"`
	University   string   `json:"university"`
	Faculty      string   `json:"faculty"`
	NepaliDate   string   `json:"nepaliDate"`
	Overview     string   `json:"overview"`
	Weightage    []byte   `json:"weightage"`
	Timeline     []byte   `json:"timeline"`
	Notices      []byte   `json:"notices"`
	Faqs         []byte   `json:"faqs"`
}

type CourseResponse struct {
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

type CourseDetailsResponse struct {
	Course                CourseResponse             `json:"course"`
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

type NewsResponse struct {
	ID       uint     `json:"id"`
	Category string   `json:"category"`
	Title    string   `json:"title"`
	Excerpt  string   `json:"excerpt"`
	Content  string   `json:"content"`
	Image    string   `json:"image"`
	Author   string   `json:"author"`
	Date     string   `json:"date"`
	ReadTime string   `json:"readTime"`
	Source   string   `json:"source"`
	Tags     []string `json:"tags"`
}

type EventResponse struct {
	ID              uint   `json:"id"`
	Title           string `json:"title"`
	Excerpt         string `json:"excerpt"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	Organizer       string `json:"organizer"`
	Location        string `json:"location"`
	Date            string `json:"date"`
	Time            string `json:"time"`
	RegistrationFee string `json:"registrationFee"`
	Image           string `json:"image"`
	Interested      int    `json:"interested"`
	Trending        bool   `json:"trending"`
	Featured        bool   `json:"featured"`
}

type EventRequest struct {
	Title           string `json:"title" binding:"required"`
	Excerpt         string `json:"excerpt"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	Organizer       string `json:"organizer"`
	Location        string `json:"location"`
	Date            string `json:"date"`
	Time            string `json:"time"`
	RegistrationFee string `json:"registrationFee"`
	Image           string `json:"image"`
	Featured        *bool  `json:"featured"`
}

type UpdateEventRequest struct {
	Title           string `json:"title"`
	Excerpt         string `json:"excerpt"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	Organizer       string `json:"organizer"`
	Location        string `json:"location"`
	Date            string `json:"date"`
	Time            string `json:"time"`
	RegistrationFee string `json:"registrationFee"`
	Image           string `json:"image"`
	Featured        *bool  `json:"featured"`
}

type BlogResponse struct {
	ID        uint     `json:"id"`
	Title     string   `json:"title"`
	Slug      string   `json:"slug"`
	Excerpt   string   `json:"excerpt"`
	Content   string   `json:"content"`
	Image     string   `json:"image"`
	Author    string   `json:"author"`
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	ReadTime  string   `json:"read_time"`
	Featured  bool     `json:"featured"`
	Published bool     `json:"published"`
	Views     int      `json:"views"`
	CreatedAt string   `json:"created_at"`
}

type BlogWithRelatedResponse struct {
	Blog    BlogResponse   `json:"blog"`
	Related []BlogResponse `json:"related"`
}

type PaginationMeta struct {
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Pages int64 `json:"pages"`
}

type PaginatedBlogsResponse struct {
	Blogs []BlogResponse `json:"blogs"`
	Meta  PaginationMeta `json:"meta"`
}

type PaginatedCoursesResponse struct {
	Courses []CourseResponse `json:"courses"`
	Meta    PaginationMeta   `json:"meta"`
}

type CourseFilterCountsResponse struct {
	LevelCounts       map[string]int64 `json:"level_counts"`
	FieldCounts       map[string]int64 `json:"field_counts"`
	AffiliationCounts map[string]int64 `json:"affiliation_counts"`
	Total             int64            `json:"total"`
}

type CreateBlogRequest struct {
	Title     string   `json:"title" binding:"required"`
	Excerpt   string   `json:"excerpt"`
	Content   string   `json:"content" binding:"required"`
	Image     string   `json:"image"`
	Author    string   `json:"author" binding:"required"`
	Category  string   `json:"category" binding:"required"`
	Tags      []string `json:"tags"`
	Featured  bool     `json:"featured"`
	Published bool     `json:"published"`
}

// ─── Entrance DTOs ─────────────────────────────────────────────────────────

type EntranceFilterRequest struct {
	Search        string   `json:"search"`
	AcademicLevel []string `json:"academicLevel"`
	Stream        []string `json:"stream"`
	Status        []string `json:"status"`
	SortBy        string   `json:"sortBy"`
	Page          int      `json:"page"`
	PageSize      int      `json:"pageSize"`
}

type PublicEntranceResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Institution string `json:"institution"`
	Location    string `json:"location"`
	Affiliation string `json:"affiliation"`
	ExamDate    string `json:"examDate"`
	Deadline    string `json:"deadline"`
	Status      string `json:"status"`
	Level       string `json:"level"`
	Stream      string `json:"stream"`
	Verified    bool   `json:"verified"`
	Image       string `json:"image"`
}

type EntranceFilterCountsResponse struct {
	Total               int64            `json:"total"`
	AcademicLevelCounts map[string]int64 `json:"academic_level_counts"`
	StreamCounts        map[string]int64 `json:"stream_counts"`
	StatusCounts        map[string]int64 `json:"status_counts"`
}

type UpdateBlogRequest struct {
	Title     string   `json:"title"`
	Excerpt   string   `json:"excerpt"`
	Content   string   `json:"content"`
	Image     string   `json:"image"`
	Author    string   `json:"author"`
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	Featured  *bool    `json:"featured"`
	Published *bool    `json:"published"`
}

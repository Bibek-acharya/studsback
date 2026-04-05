package scholarship

type CreateScholarshipRequest struct {
	Title           string   `json:"title" binding:"required"`
	Provider        string   `json:"provider" binding:"required"`
	Location        string   `json:"location"`
	Value           string   `json:"value"`
	Deadline        string   `json:"deadline"`
	DegreeLevel     string   `json:"degree_level"`
	FundingType     string   `json:"funding_type"`
	ScholarshipType string   `json:"scholarship_type"`
	Description     string   `json:"description"`
	ImageURL        string   `json:"image_url"`
	FieldOfStudy    []string `json:"field_of_study"`
}

type ScholarshipApplicationRequest struct {
	NationalID           string   `json:"national_id" binding:"required"`
	FirstName            string   `json:"first_name" binding:"required"`
	LastName             string   `json:"last_name" binding:"required"`
	DateOfBirth          string   `json:"date_of_birth" binding:"required"`
	Gender               string   `json:"gender" binding:"required"`
	StreetAddress        string   `json:"street_address" binding:"required"`
	City                 string   `json:"city" binding:"required"`
	PostCode             string   `json:"post_code" binding:"required"`
	Country              string   `json:"country" binding:"required"`
	PhoneCode            string   `json:"phone_code" binding:"required"`
	PhoneNumber          string   `json:"phone_number" binding:"required"`
	Email                string   `json:"email" binding:"required,email"`
	LatestInstitution    string   `json:"latest_institution" binding:"required"`
	LevelCompleted       string   `json:"level_completed" binding:"required"`
	GPAPercentage        string   `json:"gpa_percentage" binding:"required"`
	AnnualFamilyIncome   string   `json:"annual_family_income" binding:"required"`
	PrimaryIncomeSource  string   `json:"primary_income_source" binding:"required"`
	SpecialCircumstances []string `json:"special_circumstances"`
	PersonalStatement    string   `json:"personal_statement" binding:"required"`
}

type UpdateScholarshipApplicationRequest struct {
	NationalID           *string  `json:"national_id"`
	FirstName            *string  `json:"first_name"`
	LastName             *string  `json:"last_name"`
	DateOfBirth          *string  `json:"date_of_birth"`
	Gender               *string  `json:"gender"`
	StreetAddress        *string  `json:"street_address"`
	City                 *string  `json:"city"`
	PostCode             *string  `json:"post_code"`
	Country              *string  `json:"country"`
	PhoneCode            *string  `json:"phone_code"`
	PhoneNumber          *string  `json:"phone_number"`
	Email                *string  `json:"email"`
	LatestInstitution    *string  `json:"latest_institution"`
	LevelCompleted       *string  `json:"level_completed"`
	GPAPercentage        *string  `json:"gpa_percentage"`
	AnnualFamilyIncome   *string  `json:"annual_family_income"`
	PrimaryIncomeSource  *string  `json:"primary_income_source"`
	SpecialCircumstances []string `json:"special_circumstances"`
	PersonalStatement    *string  `json:"personal_statement"`
}

type UpdateScholarshipApplicationStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending under_review approved rejected shortlisted"`
	Notes  string `json:"notes"`
}

type ScholarshipResponse struct {
	ID               uint          `json:"id"`
	Title            string        `json:"title"`
	Provider         string        `json:"provider"`
	Location         string        `json:"location"`
	Type             string        `json:"type"`
	Amount           string        `json:"amount"`
	Deadline         string        `json:"deadline"`
	Status           string        `json:"status"`
	Category         string        `json:"category"`
	Description      string        `json:"description"`
	Image            string        `json:"image"`
	Eligibility      string        `json:"eligibility"`
	Tags             []string      `json:"tags"`
	ScholarshipType  string        `json:"scholarship_type"`
	FundingType      string        `json:"funding_type"`
	DegreeLevel      string        `json:"degree_level"`
	FieldOfStudy     []string      `json:"field_of_study,omitempty"`
	SelectionProcess []DetailField `json:"selection_process,omitempty"`
	EligibilityCrit  []DetailField `json:"eligibility_criteria,omitempty"`
	ExcludedRegions  []string      `json:"excluded_regions,omitempty"`
	RequiredDocs     []DetailField `json:"required_documents,omitempty"`
	Timeline         []DetailField `json:"timeline,omitempty"`
	Benefits         []DetailField `json:"benefits,omitempty"`
	FAQs             []DetailField `json:"faqs,omitempty"`
}

type ScholarshipApplicationResponse struct {
	ID                   uint                `json:"id"`
	CreatedAt            string              `json:"created_at"`
	UpdatedAt            string              `json:"updated_at"`
	ScholarshipID        uint                `json:"scholarship_id"`
	UserID               uint                `json:"user_id"`
	NationalID           string              `json:"national_id"`
	FirstName            string              `json:"first_name"`
	LastName             string              `json:"last_name"`
	DateOfBirth          string              `json:"date_of_birth"`
	Gender               string              `json:"gender"`
	StreetAddress        string              `json:"street_address"`
	City                 string              `json:"city"`
	PostCode             string              `json:"post_code"`
	Country              string              `json:"country"`
	PhoneCode            string              `json:"phone_code"`
	PhoneNumber          string              `json:"phone_number"`
	Email                string              `json:"email"`
	LatestInstitution    string              `json:"latest_institution"`
	LevelCompleted       string              `json:"level_completed"`
	GPAPercentage        string              `json:"gpa_percentage"`
	AnnualFamilyIncome   string              `json:"annual_family_income"`
	PrimaryIncomeSource  string              `json:"primary_income_source"`
	SpecialCircumstances []string            `json:"special_circumstances,omitempty"`
	PersonalStatement    string              `json:"personal_statement"`
	Status               string              `json:"status"`
	Scholarship          *ScholarshipSummary `json:"scholarship,omitempty"`
	User                 *UserSummary        `json:"user,omitempty"`
}

type ScholarshipListResponse struct {
	Scholarships []ScholarshipResponse `json:"scholarships"`
	Categories   []CategoryResponse    `json:"categories"`
}

type ScholarshipSummary struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Provider    string `json:"provider"`
	Deadline    string `json:"deadline"`
	Status      string `json:"status"`
	Location    string `json:"location"`
	FundingType string `json:"funding_type"`
	DegreeLevel string `json:"degree_level"`
	ImageURL    string `json:"image_url"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type CategoryResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Count    int    `json:"count"`
	Subtitle string `json:"subtitle"`
	Desc     string `json:"desc"`
	Icon     string `json:"icon"`
	Color    string `json:"color"`
}

type UserSummary struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type DetailField struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Stage       string `json:"stage,omitempty"`
	Criterion   string `json:"criterion,omitempty"`
	Name        string `json:"name,omitempty"`
	Date        string `json:"date,omitempty"`
	Event       string `json:"event,omitempty"`
	Question    string `json:"question,omitempty"`
	Answer      string `json:"answer,omitempty"`
}

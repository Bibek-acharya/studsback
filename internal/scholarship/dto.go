package scholarship

const (
	ApplicationStatusPending      = "pending"
	ApplicationStatusUnderReview  = "under_review"
	ApplicationStatusApproved     = "approved"
	ApplicationStatusRejected     = "rejected"
	ApplicationStatusShortlisted  = "shortlisted"
	ApplicationStatusPendingPayment = "pending_payment"
)

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
	Status          string   `json:"status"`
	FormConfig      any      `json:"form_config"`
	PaymentConfig   any      `json:"payment_config"`
}

type ScholarshipApplicationRequest struct {
	FullName        string  `json:"full_name" binding:"required"`
	Gender          string  `json:"gender" binding:"required"`
	Ethnicity       string  `json:"ethnicity"`
	EthnicityOther  string  `json:"ethnicity_other"`
	DateOfBirthBS   string  `json:"date_of_birth_bs" binding:"required"`
	DateOfBirthAD   string  `json:"date_of_birth_ad" binding:"required"`
	Age             int     `json:"age"`
	PhoneNumber     string  `json:"phone_number" binding:"required"`
	Email           string  `json:"email" binding:"required,email"`
	PhotoURL        string  `json:"photo_url"`

	SEEGPA             string `json:"see_gpa" binding:"required"`
	SchoolType         string `json:"school_type" binding:"required"`
	SchoolName         string `json:"school_name" binding:"required"`
	SchoolProvince     string `json:"school_province" binding:"required"`
	SchoolDistrict     string `json:"school_district" binding:"required"`
	SchoolMunicipality string `json:"school_municipality" binding:"required"`
	SchoolTole         string `json:"school_tole" binding:"required"`

	PermanentProvince     string `json:"permanent_province" binding:"required"`
	PermanentDistrict     string `json:"permanent_district" binding:"required"`
	PermanentMunicipality string `json:"permanent_municipality" binding:"required"`
	PermanentWard         string `json:"permanent_ward" binding:"required"`
	PermanentTole         string `json:"permanent_tole"`

	TemporaryProvince     string `json:"temporary_province" binding:"required"`
	TemporaryDistrict     string `json:"temporary_district" binding:"required"`
	TemporaryMunicipality string `json:"temporary_municipality" binding:"required"`
	TemporaryWard         string `json:"temporary_ward" binding:"required"`
	TemporaryTole         string `json:"temporary_tole"`

	GuardianName          string  `json:"guardian_name" binding:"required"`
	GuardianPhone         string  `json:"guardian_phone" binding:"required"`
	GuardianEmail         string  `json:"guardian_email"`
	FatherOccupation      string  `json:"father_occupation" binding:"required"`
	FatherOccupationOther string  `json:"father_occupation_other"`
	MotherOccupation      string  `json:"mother_occupation" binding:"required"`
	MotherOccupationOther string  `json:"mother_occupation_other"`
	FamilyMonthlyIncome   float64 `json:"family_monthly_income" binding:"required"`
	FamilyMembersCount    int     `json:"family_members_count" binding:"required"`

	Stream     string `json:"stream" binding:"required"`
	ExamCenter string `json:"exam_center" binding:"required"`

	Documents         []DetailField `json:"documents"`
	PersonalStatement string        `json:"personal_statement"`
	RequiresPayment   bool          `json:"requires_payment"`
}

type UpdateScholarshipApplicationRequest struct {
	FullName             *string   `json:"full_name"`
	Gender               *string   `json:"gender"`
	Ethnicity            *string   `json:"ethnicity"`
	EthnicityOther       *string   `json:"ethnicity_other"`
	DateOfBirthBS        *string   `json:"date_of_birth_bs"`
	DateOfBirthAD        *string   `json:"date_of_birth_ad"`
	Age                  *int      `json:"age"`
	PhoneNumber          *string   `json:"phone_number"`
	Email                *string   `json:"email"`
	PhotoURL             *string   `json:"photo_url"`
	SEEGPA               *string   `json:"see_gpa"`
	SchoolType           *string   `json:"school_type"`
	SchoolName           *string   `json:"school_name"`
	SchoolProvince       *string   `json:"school_province"`
	SchoolDistrict       *string   `json:"school_district"`
	SchoolMunicipality    *string   `json:"school_municipality"`
	SchoolTole           *string   `json:"school_tole"`
	PermanentProvince    *string   `json:"permanent_province"`
	PermanentDistrict    *string   `json:"permanent_district"`
	PermanentMunicipality *string  `json:"permanent_municipality"`
	PermanentWard        *string   `json:"permanent_ward"`
	PermanentTole        *string   `json:"permanent_tole"`
	TemporaryProvince    *string   `json:"temporary_province"`
	TemporaryDistrict    *string   `json:"temporary_district"`
	TemporaryMunicipality *string  `json:"temporary_municipality"`
	TemporaryWard        *string   `json:"temporary_ward"`
	TemporaryTole        *string   `json:"temporary_tole"`
	GuardianName         *string   `json:"guardian_name"`
	GuardianPhone        *string   `json:"guardian_phone"`
	GuardianEmail        *string   `json:"guardian_email"`
	FatherOccupation     *string   `json:"father_occupation"`
	FatherOccupationOther *string  `json:"father_occupation_other"`
	MotherOccupation     *string   `json:"mother_occupation"`
	MotherOccupationOther *string  `json:"mother_occupation_other"`
	FamilyMonthlyIncome  *float64  `json:"family_monthly_income"`
	FamilyMembersCount   *int      `json:"family_members_count"`
	Stream               *string   `json:"stream"`
	ExamCenter           *string   `json:"exam_center"`
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
	PaymentConfig    interface{}   `json:"payment_config,omitempty"`
	ExamDate         string        `json:"exam_date,omitempty"`
	ExamTime         string        `json:"exam_time,omitempty"`
}

type ScholarshipApplicationResponse struct {
	ID                   uint                `json:"id"`
	CreatedAt            string              `json:"created_at"`
	UpdatedAt            string              `json:"updated_at"`
	ScholarshipID        uint                `json:"scholarship_id"`
	UserID               uint                `json:"user_id"`
	FullName             string              `json:"full_name"`
	Gender               string              `json:"gender"`
	Ethnicity            string              `json:"ethnicity"`
	EthnicityOther       string              `json:"ethnicity_other"`
	DateOfBirthBS        string              `json:"date_of_birth_bs"`
	DateOfBirthAD        string              `json:"date_of_birth_ad"`
	Age                  int                 `json:"age"`
	PhoneNumber          string              `json:"phone_number"`
	Email                string              `json:"email"`
	PhotoURL             string              `json:"photo_url"`
	SEEGPA               string              `json:"see_gpa"`
	SchoolType           string              `json:"school_type"`
	SchoolName           string              `json:"school_name"`
	SchoolProvince       string              `json:"school_province"`
	SchoolDistrict       string              `json:"school_district"`
	SchoolMunicipality   string              `json:"school_municipality"`
	SchoolTole           string              `json:"school_tole"`
	PermanentProvince    string              `json:"permanent_province"`
	PermanentDistrict    string              `json:"permanent_district"`
	PermanentMunicipality string             `json:"permanent_municipality"`
	PermanentWard        string              `json:"permanent_ward"`
	PermanentTole        string              `json:"permanent_tole"`
	TemporaryProvince    string              `json:"temporary_province"`
	TemporaryDistrict    string              `json:"temporary_district"`
	TemporaryMunicipality string             `json:"temporary_municipality"`
	TemporaryWard        string              `json:"temporary_ward"`
	TemporaryTole        string              `json:"temporary_tole"`
	GuardianName         string              `json:"guardian_name"`
	GuardianPhone        string              `json:"guardian_phone"`
	GuardianEmail        string              `json:"guardian_email"`
	FatherOccupation     string              `json:"father_occupation"`
	FatherOccupationOther string             `json:"father_occupation_other"`
	MotherOccupation     string              `json:"mother_occupation"`
	MotherOccupationOther string             `json:"mother_occupation_other"`
	FamilyMonthlyIncome  float64             `json:"family_monthly_income"`
	FamilyMembersCount   int                 `json:"family_members_count"`
	Stream               string              `json:"stream"`
	ExamCenter           string              `json:"exam_center"`
	Status               string              `json:"status"`
	Scholarship          *ScholarshipSummary `json:"scholarship,omitempty"`
	PersonalStatement    string              `json:"personal_statement"`
	Documents            []DetailField       `json:"documents"`
	Payment              *PaymentResponse    `json:"payment,omitempty"`
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
	Year           string `json:"year,omitempty"`
	Title          string `json:"title,omitempty"`
	Description    string `json:"description,omitempty"`
	Icon           string `json:"icon,omitempty"`
	Folder         string `json:"folder,omitempty"`
	Stage          string `json:"stage,omitempty"`
	Step           int    `json:"step,omitempty"`
	Criterion      string `json:"criterion,omitempty"`
	Criteria       string `json:"criteria,omitempty"`
	Weight         string `json:"weight,omitempty"`
	Name           string `json:"name,omitempty"`
	Type           string `json:"type,omitempty"`
	Seats          string `json:"seats,omitempty"`
	Coverage       string `json:"coverage,omitempty"`
	Eligibility    string `json:"eligibility,omitempty"`
	URL            string `json:"url,omitempty"`
	CenterName     string `json:"centerName,omitempty"`
	Province       string `json:"province,omitempty"`
	Info           string `json:"info,omitempty"`
	HeaderColor    string `json:"headerColor,omitempty"`
	PhoneNumber    string `json:"phoneNumber,omitempty"`
	ContactPerson  string `json:"contactPerson,omitempty"`
	MapCoordinates string `json:"mapCoordinates,omitempty"`
	Date           string `json:"date,omitempty"`
	Event          string `json:"event,omitempty"`
	Question       string `json:"question,omitempty"`
	Answer         string `json:"answer,omitempty"`
	GroupHeading   string `json:"groupHeading,omitempty"`
	Website        string `json:"website,omitempty"`
	Logo           string `json:"logo,omitempty"`
	Label          string `json:"label,omitempty"`
	Message        string `json:"message,omitempty"`
	Tag            string `json:"tag,omitempty"`
	Badge          string `json:"badge,omitempty"`
	Tags           string `json:"tags,omitempty"`
	Link           string `json:"link,omitempty"`
}

type PartnerGroupResponse struct {
	GroupHeading string                `json:"groupHeading"`
	Partners     []PartnerEntryResponse `json:"partners"`
}

type PartnerEntryResponse struct {
	Name    string `json:"name"`
	Website string `json:"website"`
	Logo    string `json:"logo"`
}


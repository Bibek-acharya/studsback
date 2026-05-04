package scholarship

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(search, typeFilter, locationFilter, levelFilter, sortBy, order string) ([]Scholarship, error) {
	var scholarships []Scholarship
	query := r.db.Model(&Scholarship{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("title ILIKE ? OR provider ILIKE ? OR description ILIKE ?", like, like, like)
	}

	if typeFilter != "" {
		like := "%" + typeFilter + "%"
		query = query.Where("funding_type ILIKE ? OR scholarship_type ILIKE ?", like, like)
	}

	if locationFilter != "" {
		locations := splitFilterValues(locationFilter)
		for _, location := range locations {
			query = query.Where("location ILIKE ?", "%"+location+"%")
		}
	}

	if levelFilter != "" {
		levels := splitFilterValues(levelFilter)
		for _, level := range levels {
			query = query.Where("degree_level ILIKE ?", "%"+level+"%")
		}
	}

	switch sortBy {
	case "latest":
		query = query.Order("created_at " + order)
	case "title":
		query = query.Order("title " + order)
	default:
		query = query.Order("deadline " + order)
	}

	err := query.Find(&scholarships).Error
	return scholarships, err
}

type scholarshipCategoryRow struct {
	ScholarshipType string
	FundingType     string
	Deadline        time.Time
}

func (r *Repository) FindAllCategoryRows() ([]scholarshipCategoryRow, error) {
	var rows []scholarshipCategoryRow
	err := r.db.Model(&Scholarship{}).
		Select("scholarship_type", "funding_type", "deadline").
		Find(&rows).Error
	return rows, err
}

func (r *Repository) FindAllAdmin() ([]Scholarship, error) {
	var scholarships []Scholarship
	err := r.db.Order("created_at DESC").Find(&scholarships).Error
	return scholarships, err
}

type ProviderScholarship struct {
	ID                       uint   `gorm:"primaryKey"`
	ProviderID               uint   `gorm:"index"`
	Title                    string `gorm:"not null"`
	Description              string `gorm:"type:text"`
	ImageURL                 *string
	BannerBackgroundImageURL *string `gorm:"column:banner_background_image_url"`
	Location                 string
	Value                    string
	Deadline                 time.Time
	TotalSeats               int
	AmountPerStudent         float64
	ApplicationStartDate     time.Time
	ResultPublicationDate    time.Time
	DegreeLevel              string
	FundingType              string
	ScholarshipType          string
	Status                   string
	FieldOfStudy             []byte `gorm:"type:jsonb"`
	SelectionProcessSteps    []byte `gorm:"type:jsonb"`
	BasicEligibilityCriteria []byte `gorm:"type:jsonb"`
	RequiredDocuments        []byte `gorm:"type:jsonb"`
	Timeline                 []byte `gorm:"type:jsonb"`
	Benefits                 []byte `gorm:"type:jsonb"`
	FAQs                     []byte `gorm:"type:jsonb"`
	PaymentConfig            []byte `gorm:"type:jsonb"`
	ExamCenters              []byte `gorm:"type:jsonb"`
	ExamCentersNew           []byte `gorm:"type:jsonb"`
	ProviderName             string
	FundingTypeOther         string
	ScholarshipTypeOther     string
	EducationLevel           string
	EducationLevelOther      string
	ApplyLink                string
	CoverageArea             string
	ContactEmail             string
	PrimaryPhone             string
	SecondaryPhone           string
	WebsiteUrl               string
	OfficeAddress            string
	MapUrl                   string
	AboutParagraph1          string
	VideoTutorials           []byte `gorm:"type:jsonb"`
	JourneyTimeline          []byte `gorm:"type:jsonb"`
	ScholarshipSectionTitle  string
	ScholarshipSubtitle      string
	ScholarshipDescription1  string
	ScholarshipDescription2  string
	ScholarshipTypes         []byte `gorm:"type:jsonb"`
	ScholarshipTypesNew      []byte `gorm:"type:jsonb"`
	SelectionRubric          []byte `gorm:"type:jsonb"`
	SelectionRubricNew       []byte `gorm:"type:jsonb"`
	EligibilitySectionTitle  string
	EligibilitySubtitle      string
	FullyFundedCriteria      []byte `gorm:"type:jsonb"`
	PartiallyFundedCriteria  []byte `gorm:"type:jsonb"`
	FAQsNew                  []byte `gorm:"type:jsonb"`
	GalleryImages            []byte `gorm:"type:jsonb"`
	GalleryImagesNew         []byte `gorm:"type:jsonb"`
	PartnerGroups            []byte `gorm:"type:jsonb"`
	PartnerMessages          []byte `gorm:"type:jsonb"`
	Downloads                []byte `gorm:"type:jsonb"`
}

func (r *Repository) FindPublishedProviderScholarships() ([]ProviderScholarship, error) {
	var scholarships []ProviderScholarship
	err := r.db.Table("provider_scholarships").
		Where("status IN ?", []string{"published", "active"}).
		Find(&scholarships).Error
	return scholarships, err
}

func (r *Repository) FindProviderScholarshipByID(id uint) (*ProviderScholarship, error) {
	var scholarship ProviderScholarship
	err := r.db.Table("provider_scholarships").
		Where("id = ?", id).
		First(&scholarship).Error
	if err != nil {
		return nil, err
	}
	return &scholarship, nil
}

func (r *Repository) FindByProviderScholarshipID(providerScholarshipID uint) (*Scholarship, error) {
	var scholarship Scholarship
	err := r.db.Where("provider_scholarship_id = ?", providerScholarshipID).First(&scholarship).Error
	if err != nil {
		return nil, err
	}
	return &scholarship, nil
}

func (r *Repository) FindByID(id uint) (*Scholarship, error) {
	var scholarship Scholarship
	err := r.db.First(&scholarship, id).Error
	if err != nil {
		return nil, err
	}

	if scholarship.ProviderScholarshipID != nil {
		var ps ProviderScholarship
		if err := r.db.Select("provider_id").Where("id = ?", *scholarship.ProviderScholarshipID).First(&ps).Error; err == nil {
			scholarship.ProviderID = ps.ProviderID
		}
	}

	return &scholarship, nil
}

func (r *Repository) FindSimilar(scholarship *Scholarship) ([]Scholarship, error) {
	var similar []Scholarship
	err := r.db.Model(&Scholarship{}).
		Where("id <> ?", scholarship.ID).
		Where(
			"(degree_level ILIKE ? OR funding_type ILIKE ? OR scholarship_type ILIKE ? OR location ILIKE ?)",
			"%"+scholarship.DegreeLevel+"%",
			"%"+scholarship.FundingType+"%",
			"%"+scholarship.ScholarshipType+"%",
			"%"+scholarship.Location+"%",
		).
		Order("deadline ASC").
		Limit(5).
		Find(&similar).Error
	return similar, err
}

func (r *Repository) FindFallbackSimilar(scholarship *Scholarship) ([]Scholarship, error) {
	var similar []Scholarship
	err := r.db.Model(&Scholarship{}).
		Where("id <> ?", scholarship.ID).
		Order("deadline ASC").
		Limit(5).
		Find(&similar).Error
	return similar, err
}

func (r *Repository) Create(scholarship *Scholarship) error {
	return r.db.Create(scholarship).Error
}

func (r *Repository) Save(scholarship *Scholarship) error {
	return r.db.Save(scholarship).Error
}

func (r *Repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&Scholarship{}, id).Error
}

func (r *Repository) ApplicationCreate(app *ScholarshipApplication) error {
	return r.db.Create(app).Error
}

func (r *Repository) ApplicationFindByUserID(userID uint) ([]ScholarshipApplication, error) {
	var applications []ScholarshipApplication
	err := r.db.Where("user_id = ?", userID).Preload("Scholarship").Order("created_at DESC").Find(&applications).Error
	return applications, err
}

func (r *Repository) ApplicationFindByID(id uint) (*ScholarshipApplication, error) {
	var app ScholarshipApplication
	err := r.db.Preload("Scholarship").Preload("User").First(&app, id).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) ApplicationSave(app *ScholarshipApplication) error {
	return r.db.Save(app).Error
}

func (r *Repository) ApplicationDelete(id uint) error {
	return r.db.Unscoped().Delete(&ScholarshipApplication{}, id).Error
}

func (r *Repository) ApplicationFindAll(status string) ([]ScholarshipApplication, error) {
	var applications []ScholarshipApplication
	query := r.db.Preload("Scholarship").Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").Find(&applications).Error
	return applications, err
}

func (r *Repository) ApplicationFindByScholarshipID(scholarshipID string, status string) ([]ScholarshipApplication, error) {
	var apps []ScholarshipApplication
	query := r.db.Where("scholarship_id = ?", scholarshipID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Find(&apps).Error
	return apps, err
}

func (r *Repository) ApplicationFindByUserAndScholarshipID(scholarshipID uint, userID uint) (*ScholarshipApplication, error) {
	var app ScholarshipApplication
	err := r.db.Where("scholarship_id = ? AND user_id = ?", scholarshipID, userID).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) ApplicationExists(scholarshipID uint, userID uint) bool {
	var count int64
	r.db.Model(&ScholarshipApplication{}).Where("scholarship_id = ? AND user_id = ?", scholarshipID, userID).Count(&count)
	return count > 0
}

func (r *Repository) CreateProviderApplication(providerScholarshipID uint, app *ScholarshipApplication) error {
	nameParts := splitFullName(app.FullName)
	now := time.Now()
	return r.db.Table("provider_applications").Create(map[string]interface{}{
		"scholarship_id":          providerScholarshipID,
		"user_id":                 app.UserID,
		"full_name":               app.FullName,
		"first_name":              nameParts[0],
		"last_name":               nameParts[1],
		"email":                   app.Email,
		"phone_number":            app.PhoneNumber,
		"gender":                  app.Gender,
		"ethnicity":               app.Ethnicity,
		"ethnicity_other":         app.EthnicityOther,
		"date_of_birth_bs":        app.DateOfBirthBS,
		"date_of_birth_ad":        app.DateOfBirthAD,
		"age":                     app.Age,
		"photo_url":               app.PhotoURL,
		"see_gpa":                 app.SEEGPA,
		"gpa":                     parseGPA(app.SEEGPA),
		"school_type":             app.SchoolType,
		"school_name":             app.SchoolName,
		"school_province":         app.SchoolProvince,
		"school_district":         app.SchoolDistrict,
		"school_municipality":     app.SchoolMunicipality,
		"school_tole":             app.SchoolTole,
		"province":                app.PermanentProvince,
		"district":                app.PermanentDistrict,
		"permanent_province":      app.PermanentProvince,
		"permanent_district":      app.PermanentDistrict,
		"permanent_municipality":  app.PermanentMunicipality,
		"permanent_ward":          app.PermanentWard,
		"permanent_tole":          app.PermanentTole,
		"temporary_province":      app.TemporaryProvince,
		"temporary_district":      app.TemporaryDistrict,
		"temporary_municipality":  app.TemporaryMunicipality,
		"temporary_ward":          app.TemporaryWard,
		"temporary_tole":          app.TemporaryTole,
		"guardian_name":           app.GuardianName,
		"guardian_phone":          app.GuardianPhone,
		"guardian_email":          app.GuardianEmail,
		"father_occupation":       app.FatherOccupation,
		"father_occupation_other": app.FatherOccupationOther,
		"mother_occupation":       app.MotherOccupation,
		"mother_occupation_other": app.MotherOccupationOther,
		"family_monthly_income":   app.FamilyMonthlyIncome,
		"family_members_count":    app.FamilyMembersCount,
		"status":                  "pending",
		"stream":                  app.Stream,
		"exam_center":             app.ExamCenter,
		"personal_statement":       app.PersonalStatement,
		"documents":                app.Documents,
		"scholarship_application_id": app.ID,
		"created_at":               now,
		"updated_at":              now,
	}).Error
}

func splitFullName(fullName string) [2]string {
	parts := splitAndTrim(fullName)
	if len(parts) == 0 {
		return [2]string{"", ""}
	}
	if len(parts) == 1 {
		return [2]string{parts[0], parts[0]}
	}
	return [2]string{parts[0], joinRest(parts[1:])}
}

func splitAndTrim(s string) []string {
	var result []string
	current := ""
	for _, r := range s {
		if r == ' ' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func joinRest(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}

func parseGPA(gpa string) float64 {
	if gpa == "" {
		return 0
	}
	var f float64
	_, _ = fmt.Sscanf(gpa, "%f", &f)
	return f
}

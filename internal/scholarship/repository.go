package scholarship

import (
	"encoding/json"
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
	now := time.Now().Truncate(24 * time.Hour)
	query := r.db.Model(&Scholarship{}).
		Where("deadline >= ? OR deadline = ?", now, time.Time{})

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

func (r *Repository) FindAllScholarships() ([]Scholarship, error) {
	var scholarships []Scholarship
	if err := r.db.Order("title asc").Find(&scholarships).Error; err != nil {
		return nil, err
	}
	return scholarships, nil
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
	Slug                     string
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
	ExamDate                 string `gorm:"column:exam_date;default:''"`
	ExamTime                 string `gorm:"column:exam_time;default:''"`
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

func (r *Repository) FindProviderScholarshipBySlug(slug string) (*ProviderScholarship, error) {
	var scholarship ProviderScholarship
	err := r.db.Table("provider_scholarships").
		Where("slug = ?", slug).
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

func (r *Repository) FindBySlug(slug string) (*Scholarship, error) {
	var scholarship Scholarship
	err := r.db.Where("slug = ?", slug).First(&scholarship).Error
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

func (r *Repository) CascadeDelete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Exec("DELETE FROM scholarship_payments WHERE scholarship_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().
			Exec("DELETE FROM scholarship_applications WHERE scholarship_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&Scholarship{}, id).Error
	})
}

func (r *Repository) ApplicationCreate(app *ScholarshipApplication) error {
	return r.db.Create(app).Error
}

func (r *Repository) UpdateApplicationRollNumber(id uint, rollNumber string) error {
	return r.db.Model(&ScholarshipApplication{}).Where("id = ?", id).Update("roll_number", rollNumber).Error
}

func (r *Repository) GetNextRollNumber() (int, error) {
	var seq int
	err := r.db.Raw("SELECT nextval('scholarship_roll_number_seq')").Scan(&seq).Error
	return seq, err
}

func (r *Repository) UpdateProviderApplicationRollNumber(scholarshipApplicationID uint, rollNumber string) error {
	return r.db.Table("provider_applications").Where("scholarship_application_id = ?", scholarshipApplicationID).Update("roll_number", rollNumber).Error
}

func (r *Repository) ApplicationFindByUserID(userID uint) ([]ScholarshipApplication, error) {
	var applications []ScholarshipApplication
	err := r.db.Where("user_id = ? AND status != ?", userID, ApplicationStatusDraft).Preload("Scholarship").Order("created_at DESC").Find(&applications).Error
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
	} else {
		query = query.Where("status != ?", ApplicationStatusDraft)
	}

	err := query.Order("created_at DESC").Find(&applications).Error
	return applications, err
}

func (r *Repository) ApplicationFindByScholarshipID(scholarshipID string, status string) ([]ScholarshipApplication, error) {
	var apps []ScholarshipApplication
	query := r.db.Where("scholarship_id = ?", scholarshipID)
	if status != "" {
		query = query.Where("status = ?", status)
	} else {
		query = query.Where("status != ?", ApplicationStatusDraft)
	}
	err := query.Find(&apps).Error
	return apps, err
}

func (r *Repository) ApplicationDeleteOlderThan(cutoff time.Time) (int64, error) {
	result := r.db.Where("status = ? AND created_at < ?", ApplicationStatusDraft, cutoff).Delete(&ScholarshipApplication{})
	return result.RowsAffected, result.Error
}

func (r *Repository) ApplicationFindByUserAndScholarshipID(scholarshipID uint, userID uint) (*ScholarshipApplication, error) {
	var app ScholarshipApplication
	err := r.db.Where("scholarship_id = ? AND user_id = ?", scholarshipID, userID).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) UpdateProviderApplicationStatus(appID uint, status string) error {
	return r.db.Table("provider_applications").
		Where("scholarship_application_id = ?", appID).
		Update("status", status).Error
}

func (r *Repository) ApplicationExists(scholarshipID uint, userID uint) bool {
	var count int64
	r.db.Model(&ScholarshipApplication{}).Where("scholarship_id = ? AND user_id = ?", scholarshipID, userID).Count(&count)
	return count > 0
}

func (r *Repository) ApplicationExistsByEmail(scholarshipID uint, email string) bool {
	var count int64
	r.db.Model(&ScholarshipApplication{}).
		Where("scholarship_id = ? AND email = ? AND status NOT IN ('draft', 'rejected')", scholarshipID, email).
		Count(&count)
	return count > 0
}

func (r *Repository) CountApplicationsByExamCenter(scholarshipID uint, centerName string) (int64, error) {
	var count int64
	err := r.db.Model(&ScholarshipApplication{}).
		Where("scholarship_id = ? AND exam_center = ? AND status NOT IN ?", scholarshipID, centerName, []string{ApplicationStatusRejected, ApplicationStatusPendingPayment}).
		Where(`scholarship_applications.id IN (
			SELECT sp.application_id FROM scholarship_payments sp
			WHERE (sp.method = 'esewa' AND sp.status = 'completed')
			   OR (sp.method = 'bank' AND sp.status IN ('pending_approval', 'completed'))
		)`).
		Count(&count).Error
	return count, err
}

func (r *Repository) CreateProviderApplication(providerScholarshipID uint, app *ScholarshipApplication) error {
	nameParts := splitFullName(app.FullName)
	now := time.Now()
	return r.db.Table("provider_applications").Create(map[string]interface{}{
		"scholarship_id":             providerScholarshipID,
		"user_id":                    app.UserID,
		"full_name":                  app.FullName,
		"first_name":                 nameParts[0],
		"last_name":                  nameParts[1],
		"email":                      app.Email,
		"phone_number":               app.PhoneNumber,
		"gender":                     app.Gender,
		"ethnicity":                  app.Ethnicity,
		"ethnicity_other":            app.EthnicityOther,
		"date_of_birth_bs":           app.DateOfBirthBS,
		"date_of_birth_ad":           app.DateOfBirthAD,
		"age":                        app.Age,
		"photo_url":                  app.PhotoURL,
		"see_gpa":                    app.SEEGPA,
		"gpa":                        parseGPA(app.SEEGPA),
		"school_type":                app.SchoolType,
		"school_name":                app.SchoolName,
		"school_province":            app.SchoolProvince,
		"school_district":            app.SchoolDistrict,
		"school_municipality":        app.SchoolMunicipality,
		"school_tole":                app.SchoolTole,
		"province":                   app.PermanentProvince,
		"district":                   app.PermanentDistrict,
		"permanent_province":         app.PermanentProvince,
		"permanent_district":         app.PermanentDistrict,
		"permanent_municipality":     app.PermanentMunicipality,
		"permanent_ward":             app.PermanentWard,
		"permanent_tole":             app.PermanentTole,
		"temporary_province":         app.TemporaryProvince,
		"temporary_district":         app.TemporaryDistrict,
		"temporary_municipality":     app.TemporaryMunicipality,
		"temporary_ward":             app.TemporaryWard,
		"temporary_tole":             app.TemporaryTole,
		"guardian_name":              app.GuardianName,
		"guardian_phone":             app.GuardianPhone,
		"guardian_email":             app.GuardianEmail,
		"father_occupation":          app.FatherOccupation,
		"father_occupation_other":    app.FatherOccupationOther,
		"mother_occupation":          app.MotherOccupation,
		"mother_occupation_other":    app.MotherOccupationOther,
		"family_monthly_income":      app.FamilyMonthlyIncome,
		"family_members_count":       app.FamilyMembersCount,
		"status":                     statusForProviderApp(app.Status),
		"stream":                     app.Stream,
		"exam_center":                app.ExamCenter,
		"roll_number":                app.RollNumber,
		"personal_statement":         app.PersonalStatement,
		"documents":                  app.Documents,
		"scholarship_application_id": app.ID,
		"created_at":                 now,
		"updated_at":                 now,
	}).Error
}

func statusForProviderApp(appStatus string) string {
	if appStatus == ApplicationStatusDraft {
		return "pending_payment"
	}
	return appStatus
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

func (r *Repository) CreateProviderNotification(providerID uint, app *ScholarshipApplication, scholarshipTitle string) error {
	now := time.Now()
	return r.db.Table("provider_notifications").Create(map[string]interface{}{
		"provider_id": providerID,
		"title":       "New Application Received",
		"message":     fmt.Sprintf("%s submitted an application for your scholarship: %s", app.FullName, scholarshipTitle),
		"type":        "application",
		"link":        "applications",
		"read":        false,
		"created_at":  now,
		"updated_at":  now,
	}).Error
}

func parseGPA(gpa string) float64 {
	if gpa == "" {
		return 0
	}
	var f float64
	_, _ = fmt.Sscanf(gpa, "%f", &f)
	return f
}

func (r *Repository) FindAllForRecommendation() ([]Scholarship, error) {
	var scholarships []Scholarship
	err := r.db.Model(&Scholarship{}).
		Where("status NOT IN ?", []string{"draft", "closed"}).
		Where("deleted_at IS NULL").
		Order("deadline ASC").
		Find(&scholarships).Error
	return scholarships, err
}

func (r *Repository) FindProviderScholarshipsForRecommendation() ([]ProviderScholarship, error) {
	var scholarships []ProviderScholarship
	err := r.db.Table("provider_scholarships").
		Where("status IN ?", []string{"published", "active"}).
		Where("deleted_at IS NULL").
		Find(&scholarships).Error
	return scholarships, err
}

func (r *Repository) GetUserProfileForRecommendation(userID uint) (*ProfileData, error) {
	type eduRow struct {
		Level           string
		Stream          string
		Grade           string
		GradingSystem   string `gorm:"column:grading_system"`
		InstitutionName string `gorm:"column:institution_name"`
	}
	var entries []eduRow
	if err := r.db.Table("education_entries").
		Select("level, stream, grade, grading_system, institution_name").
		Where("user_id = ?", userID).
		Find(&entries).Error; err != nil {
		return nil, err
	}

	eduData := make([]EducationEntryData, len(entries))
	for i, e := range entries {
		eduData[i] = EducationEntryData{
			Level:           e.Level,
			Stream:          e.Stream,
			Grade:           e.Grade,
			GradingSystem:   e.GradingSystem,
			InstitutionName: e.InstitutionName,
		}
	}

	type prefsRow struct {
		Preferences []byte `gorm:"column:preferences"`
	}
	var pRow prefsRow
	if err := r.db.Table("users").
		Select("preferences").
		Where("id = ?", userID).
		First(&pRow).Error; err != nil {
		return nil, err
	}

	var prefs *PreferencesData
	if len(pRow.Preferences) > 0 {
		var p PreferencesData
		if err := json.Unmarshal(pRow.Preferences, &p); err == nil {
			prefs = &p
		}
	}

	type bookmarkRow struct {
		Field string
	}
	var bookmarks []bookmarkRow
	r.db.Table("bookmarks").
		Select("DISTINCT field").
		Where("user_id = ? AND entity_type = 'scholarship'", userID).
		Scan(&bookmarks)

	fields := make([]string, 0, len(bookmarks))
	for _, b := range bookmarks {
		if b.Field != "" {
			fields = append(fields, b.Field)
		}
	}

	return &ProfileData{
		EducationEntries: eduData,
		Preferences:      prefs,
		BookmarkedFields: fields,
	}, nil
}

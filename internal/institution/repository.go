package institution

import (
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CountProgramsByInstitution(instID uint) (int64, error) {
	var count int64
	err := r.db.Model(&InstitutionProgram{}).Where("institution_id = ?", instID).Count(&count).Error
	return count, err
}

func (r *Repository) CountDistinctStudentsByInstitution(instID uint) (int64, error) {
	var count int64
	err := r.db.Model(&InstitutionEntranceApplicant{}).
		Joins("JOIN institution_entrances ON institution_entrances.id = institution_entrance_applicants.entrance_id").
		Where("institution_entrances.institution_id = ?", instID).
		Distinct("institution_entrance_applicants.user_id").Count(&count).Error
	return count, err
}

func (r *Repository) CountActiveEntrances(instID uint) (int64, error) {
	var count int64
	err := r.db.Model(&InstitutionEntrance{}).
		Where("institution_id = ? AND status = ?", instID, "upcoming").Count(&count).Error
	return count, err
}

func (r *Repository) CountPendingBookings(instID uint) (int64, error) {
	var count int64
	err := r.db.Model(&InstitutionCounsellingBooking{}).
		Joins("JOIN institution_counselling_sessions ON institution_counselling_sessions.id = institution_counselling_bookings.session_id").
		Where("institution_counselling_sessions.institution_id = ? AND institution_counselling_bookings.status = ?", instID, "pending").
		Count(&count).Error
	return count, err
}

func (r *Repository) CountUnreadMessages(instID uint) (int64, error) {
	var count int64
	err := r.db.Model(&InstitutionMessage{}).
		Where("institution_id = ? AND read = ?", instID, false).Count(&count).Error
	return count, err
}

func (r *Repository) FindProgramsByInstitution(instID uint, page, limit int) ([]InstitutionProgram, int64, error) {
	var programs []InstitutionProgram
	var total int64

	if err := r.db.Model(&InstitutionProgram{}).Where("institution_id = ?", instID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&programs).Error
	return programs, total, err
}

func (r *Repository) FindProgramByIDAndInstitution(id uint, instID uint) (*InstitutionProgram, error) {
	var program InstitutionProgram
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&program).Error
	if err != nil {
		return nil, err
	}
	return &program, nil
}

func (r *Repository) CreateProgram(program *InstitutionProgram) error {
	return r.db.Create(program).Error
}

func (r *Repository) SaveProgram(program *InstitutionProgram) error {
	return r.db.Save(program).Error
}

func (r *Repository) DeleteProgram(id uint, instID uint) error {
	result := r.db.Where("id = ? AND institution_id = ?", id, instID).Delete(&InstitutionProgram{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindAllProgramsByInstitution(instID uint) ([]InstitutionProgram, error) {
	var programs []InstitutionProgram
	err := r.db.Where("institution_id = ?", instID).Find(&programs).Error
	return programs, err
}

func (r *Repository) CountEntrancesByInstitution(instID uint) (int64, error) {
	var count int64
	err := r.db.Model(&InstitutionEntrance{}).Where("institution_id = ?", instID).Count(&count).Error
	return count, err
}

func (r *Repository) CountTotalApplicants(instID uint) (int64, error) {
	var count int64
	err := r.db.Model(&InstitutionEntranceApplicant{}).
		Joins("JOIN institution_entrances ON institution_entrances.id = institution_entrance_applicants.entrance_id").
		Where("institution_entrances.institution_id = ?", instID).Count(&count).Error
	return count, err
}

func (r *Repository) FindInstitutionUserByID(id uint) (*InstitutionUser, error) {
	var user InstitutionUser
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) SaveInstitutionUser(user *InstitutionUser) error {
	return r.db.Save(user).Error
}

func (r *Repository) FindPublicInstitutions(page, pageSize int, search, location string) ([]InstitutionUser, int64, error) {
	var users []InstitutionUser
	var total int64

	query := r.db.Model(&InstitutionUser{}).
		Joins("LEFT JOIN institution_settings ON institution_settings.institution_id = institution_users.id").
		Where("(institution_settings.public_profile = ? OR institution_settings.id IS NULL)", true).
		Where("institution_users.deleted_at IS NULL")

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(institution_users.institution_name ILIKE ? OR institution_users.district ILIKE ?)", like, like)
	}
	if location != "" {
		query = query.Where("institution_users.district ILIKE ?", "%"+location+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Select("institution_users.*").
		Order("institution_users.created_at desc").
		Offset(offset).Limit(pageSize).
		Find(&users).Error

	return users, total, err
}

func (r *Repository) FindPublicInstitutionByID(id uint) (*InstitutionUser, error) {
	var user InstitutionUser
	err := r.db.Joins("LEFT JOIN institution_settings ON institution_settings.institution_id = institution_users.id").
		Where("institution_users.id = ?", id).
		Where("(institution_settings.public_profile = ? OR institution_settings.id IS NULL)", true).
		Where("institution_users.deleted_at IS NULL").
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindMediaByInstitution(instID uint) ([]InstitutionMedia, error) {
	var media []InstitutionMedia
	err := r.db.Where("institution_id = ?", instID).Order("created_at desc").Find(&media).Error
	return media, err
}

func (r *Repository) CreateMedia(media *InstitutionMedia) error {
	return r.db.Create(media).Error
}

func (r *Repository) DeleteMedia(id uint, instID uint) error {
	result := r.db.Where("id = ? AND institution_id = ?", id, instID).Delete(&InstitutionMedia{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindCounsellingSessionsByInstitution(instID uint) ([]InstitutionCounsellingSession, error) {
	var sessions []InstitutionCounsellingSession
	err := r.db.Where("institution_id = ?", instID).Order("scheduled_at asc").Find(&sessions).Error
	return sessions, err
}

func (r *Repository) FindUpcomingSessionsByInstitution(instID uint) ([]InstitutionCounsellingSession, error) {
	var sessions []InstitutionCounsellingSession
	err := r.db.Where("institution_id = ? AND status = ? AND scheduled_at >= ?", instID, "scheduled", time.Now()).
		Order("scheduled_at asc").Find(&sessions).Error
	return sessions, err
}

func (r *Repository) FindCounsellingSessionByID(id uint, instID uint) (*InstitutionCounsellingSession, error) {
	var session InstitutionCounsellingSession
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *Repository) FindCounsellingSessionByIDOnly(id uint) (*InstitutionCounsellingSession, error) {
	var session InstitutionCounsellingSession
	err := r.db.First(&session, id).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *Repository) CreateCounsellingSession(session *InstitutionCounsellingSession) error {
	return r.db.Create(session).Error
}

func (r *Repository) DeleteCounsellingSession(session *InstitutionCounsellingSession) error {
	return r.db.Delete(session).Error
}

func (r *Repository) FindCounsellingBookingsByInstitution(instID uint) ([]InstitutionCounsellingBooking, error) {
	var bookings []InstitutionCounsellingBooking
	err := r.db.
		Joins("JOIN institution_counselling_sessions ON institution_counselling_sessions.id = institution_counselling_bookings.session_id").
		Where("institution_counselling_sessions.institution_id = ?", instID).
		Preload("Session").
		Order("institution_counselling_bookings.created_at desc").
		Find(&bookings).Error
	return bookings, err
}

func (r *Repository) FindBookingByIDWithSession(id uint, instID uint) (*InstitutionCounsellingBooking, error) {
	var booking InstitutionCounsellingBooking
	err := r.db.
		Joins("JOIN institution_counselling_sessions ON institution_counselling_sessions.id = institution_counselling_bookings.session_id").
		Where("institution_counselling_bookings.id = ? AND institution_counselling_sessions.institution_id = ?", id, instID).
		First(&booking).Error
	if err != nil {
		return nil, err
	}
	return &booking, nil
}

func (r *Repository) SaveBooking(booking *InstitutionCounsellingBooking) error {
	return r.db.Save(booking).Error
}

func (r *Repository) CheckUserSessionBooking(userID uint, sessionID uint) bool {
	var count int64
	r.db.Model(&InstitutionCounsellingBooking{}).
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Count(&count)
	return count > 0
}

func (r *Repository) CreateBooking(booking *InstitutionCounsellingBooking) error {
	return r.db.Create(booking).Error
}

func (r *Repository) IncrementBookedSeats(sessionID uint) error {
	return r.db.Model(&InstitutionCounsellingSession{}).
		Where("id = ?", sessionID).
		UpdateColumn("booked_seats", gorm.Expr("booked_seats + 1")).Error
}

func (r *Repository) FindEntrancesByInstitution(instID uint, status string, page, limit int) ([]InstitutionEntrance, int64, error) {
	var entrances []InstitutionEntrance
	var total int64

	query := r.db.Model(&InstitutionEntrance{}).Where("institution_id = ?", instID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("date desc").Offset(offset).Limit(limit).Find(&entrances).Error
	return entrances, total, err
}

func (r *Repository) FindEntranceByIDAndInstitution(id uint, instID uint) (*InstitutionEntrance, error) {
	var entrance InstitutionEntrance
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&entrance).Error
	if err != nil {
		return nil, err
	}
	return &entrance, nil
}

func (r *Repository) CreateEntrance(entrance *InstitutionEntrance) error {
	return r.db.Create(entrance).Error
}

func (r *Repository) SaveEntrance(entrance *InstitutionEntrance) error {
	return r.db.Save(entrance).Error
}

func (r *Repository) DeleteEntrance(id uint, instID uint) error {
	result := r.db.Where("id = ? AND institution_id = ?", id, instID).Delete(&InstitutionEntrance{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindEntranceApplicants(entranceID uint) ([]InstitutionEntranceApplicant, error) {
	var applicants []InstitutionEntranceApplicant
	err := r.db.Where("entrance_id = ?", entranceID).Order("rank asc").Find(&applicants).Error
	return applicants, err
}

func (r *Repository) FindEventsByInstitution(instID uint, page, limit int) ([]InstitutionEvent, int64, error) {
	var events []InstitutionEvent
	var total int64

	if err := r.db.Model(&InstitutionEvent{}).Where("institution_id = ?", instID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Where("institution_id = ?", instID).
		Order("date desc").Offset(offset).Limit(limit).Find(&events).Error
	return events, total, err
}

func (r *Repository) FindEventByIDAndInstitution(id uint, instID uint) (*InstitutionEvent, error) {
	var event InstitutionEvent
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) CreateEvent(event *InstitutionEvent) error {
	return r.db.Create(event).Error
}

func (r *Repository) SaveEvent(event *InstitutionEvent) error {
	return r.db.Save(event).Error
}

func (r *Repository) DeleteEvent(id uint, instID uint) error {
	result := r.db.Where("id = ? AND institution_id = ?", id, instID).Delete(&InstitutionEvent{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindNewsByInstitution(instID uint, page, limit int) ([]InstitutionNews, int64, error) {
	var news []InstitutionNews
	var total int64

	if err := r.db.Model(&InstitutionNews{}).Where("institution_id = ?", instID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&news).Error
	return news, total, err
}

func (r *Repository) FindNewsByIDAndInstitution(id uint, instID uint) (*InstitutionNews, error) {
	var news InstitutionNews
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&news).Error
	if err != nil {
		return nil, err
	}
	return &news, nil
}

func (r *Repository) CreateNews(news *InstitutionNews) error {
	return r.db.Create(news).Error
}

func (r *Repository) SaveNews(news *InstitutionNews) error {
	return r.db.Save(news).Error
}

func (r *Repository) DeleteNews(id uint, instID uint) error {
	result := r.db.Where("id = ? AND institution_id = ?", id, instID).Delete(&InstitutionNews{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindBlogsByInstitution(instID uint, page, limit int) ([]InstitutionBlog, int64, error) {
	var blogs []InstitutionBlog
	var total int64

	if err := r.db.Model(&InstitutionBlog{}).Where("institution_id = ?", instID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&blogs).Error
	return blogs, total, err
}

func (r *Repository) FindBlogByIDAndInstitution(id uint, instID uint) (*InstitutionBlog, error) {
	var blog InstitutionBlog
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&blog).Error
	if err != nil {
		return nil, err
	}
	return &blog, nil
}

func (r *Repository) CreateBlog(blog *InstitutionBlog) error {
	return r.db.Create(blog).Error
}

func (r *Repository) SaveBlog(blog *InstitutionBlog) error {
	return r.db.Save(blog).Error
}

func (r *Repository) DeleteBlog(id uint, instID uint) error {
	result := r.db.Where("id = ? AND institution_id = ?", id, instID).Delete(&InstitutionBlog{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindQMSByInstitution(instID uint, page, limit int) ([]InstitutionQMS, int64, error) {
	var qms []InstitutionQMS
	var total int64

	if err := r.db.Model(&InstitutionQMS{}).Where("institution_id = ?", instID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&qms).Error
	return qms, total, err
}

func (r *Repository) FindQMSByIDAndInstitution(id uint, instID uint) (*InstitutionQMS, error) {
	var qms InstitutionQMS
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&qms).Error
	if err != nil {
		return nil, err
	}
	return &qms, nil
}

func (r *Repository) CreateQMS(qms *InstitutionQMS) error {
	return r.db.Create(qms).Error
}

func (r *Repository) SaveQMS(qms *InstitutionQMS) error {
	return r.db.Save(qms).Error
}

func (r *Repository) DeleteQMS(id uint, instID uint) error {
	result := r.db.Where("id = ? AND institution_id = ?", id, instID).Delete(&InstitutionQMS{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindMessagesByInstitution(instID uint, page, limit int) ([]InstitutionMessage, int64, error) {
	var messages []InstitutionMessage
	var total int64

	if err := r.db.Model(&InstitutionMessage{}).Where("institution_id = ?", instID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&messages).Error
	return messages, total, err
}

func (r *Repository) FindMessageByIDAndInstitution(id uint, instID uint) (*InstitutionMessage, error) {
	var message InstitutionMessage
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&message).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *Repository) CreateMessage(message *InstitutionMessage) error {
	return r.db.Create(message).Error
}

func (r *Repository) FindAllMessagesByInstitution(instID uint) ([]InstitutionMessage, error) {
	var messages []InstitutionMessage
	err := r.db.Where("institution_id = ?", instID).Order("created_at desc").Find(&messages).Error
	return messages, err
}

func (r *Repository) FindUserByID(id uint) (*User, error) {
	var user User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) SaveMessage(message *InstitutionMessage) error {
	return r.db.Save(message).Error
}

func (r *Repository) FindOrCreateSettings(instID uint) (*InstitutionSettings, error) {
	var settings InstitutionSettings
	err := r.db.Where("institution_id = ?", instID).FirstOrCreate(&settings, InstitutionSettings{
		InstitutionID: instID,
	}).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *Repository) SaveSettings(settings *InstitutionSettings) error {
	return r.db.Save(settings).Error
}

func (r *Repository) FindCollegeByUniversityID(universityID uint) (*College, error) {
	var college College
	err := r.db.Where("university_id = ?", universityID).First(&college).Error
	if err != nil {
		return nil, err
	}
	return &college, nil
}

func (r *Repository) FindScholarshipsByLocation(like string) ([]Scholarship, error) {
	var scholarships []Scholarship
	err := r.db.Where("location ILIKE ?", like).Find(&scholarships).Error
	return scholarships, err
}

func (r *Repository) CreateScholarship(scholarship *Scholarship) error {
	return r.db.Create(scholarship).Error
}

func (r *Repository) FindScholarshipByID(id uint) (*Scholarship, error) {
	var scholarship Scholarship
	err := r.db.First(&scholarship, id).Error
	if err != nil {
		return nil, err
	}
	return &scholarship, nil
}

func (r *Repository) SaveScholarship(scholarship *Scholarship) error {
	return r.db.Save(scholarship).Error
}

func (r *Repository) DeleteScholarship(id uint) error {
	return r.db.Unscoped().Delete(&Scholarship{}, id).Error
}

func (r *Repository) FindAdmissionsByCollegeID(collegeID uint, status string) ([]Admission, error) {
	var admissions []Admission
	query := r.db.Where("college_id = ?", collegeID).Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").Find(&admissions).Error
	return admissions, err
}

func (r *Repository) FindAdmissionByID(id uint) (*Admission, error) {
	var admission Admission
	err := r.db.First(&admission, id).Error
	if err != nil {
		return nil, err
	}
	return &admission, nil
}

func (r *Repository) SaveAdmission(admission *Admission) error {
	return r.db.Save(admission).Error
}

func (r *Repository) FindScholarshipsByProvider(provider string) ([]Scholarship, error) {
	var scholarships []Scholarship
	err := r.db.Where("provider = ?", provider).Find(&scholarships).Error
	return scholarships, err
}

func (r *Repository) FindScholarshipApplicationsByIDs(scholarshipIDs []uint, status string) ([]ScholarshipApplication, error) {
	var applications []ScholarshipApplication
	query := r.db.Where("scholarship_id IN ?", scholarshipIDs).Preload("Scholarship").Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("created_at DESC").Find(&applications).Error
	return applications, err
}

func (r *Repository) FindScholarshipApplicationByID(id uint) (*ScholarshipApplication, error) {
	var application ScholarshipApplication
	err := r.db.Preload("Scholarship").First(&application, id).Error
	if err != nil {
		return nil, err
	}
	return &application, nil
}

func (r *Repository) SaveScholarshipApplication(application *ScholarshipApplication) error {
	return r.db.Save(application).Error
}

// --- Admission Page Repository ---

func (r *Repository) FindAdmissionPagesByInstitution(instID uint, status string, page, limit int) ([]AdmissionPage, int64, error) {
	var pages []AdmissionPage
	var total int64

	query := r.db.Model(&AdmissionPage{}).Where("institution_id = ?", instID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Where("institution_id = ?", instID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&pages).Error
	return pages, total, err
}

func (r *Repository) FindAdmissionPageByID(id uint, instID uint) (*AdmissionPage, error) {
	var page AdmissionPage
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&page).Error
	if err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *Repository) CreateAdmissionPage(page *AdmissionPage) error {
	return r.db.Create(page).Error
}

func (r *Repository) SaveAdmissionPage(page *AdmissionPage) error {
	return r.db.Save(page).Error
}

func (r *Repository) DeleteAdmissionPage(id uint, instID uint) error {
	result := r.db.Where("id = ? AND institution_id = ?", id, instID).Delete(&AdmissionPage{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindPublishedAdmissionPages(page, limit int) ([]AdmissionPage, int64, error) {
	var pages []AdmissionPage
	var total int64

	query := r.db.Model(&AdmissionPage{}).Where("status = ?", "published")
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("published_at desc").Offset(offset).Limit(limit).Find(&pages).Error
	return pages, total, err
}

func (r *Repository) FindPublishedAdmissionByInstitutionID(instID uint) (*AdmissionPage, error) {
	var page AdmissionPage
	err := r.db.Where("institution_id = ? AND status = ? AND deleted_at IS NULL", instID, "published").
		Order("published_at DESC").
		First(&page).Error
	if err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *Repository) FindPublishedAdmissionInstitutionByID(id uint) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	query := `SELECT 
		iu.id,
		iu.institution_name AS name,
		COALESCE(iu.logo_url, '') AS image_url,
		COALESCE(iu.province, '') || CASE WHEN iu.district IS NOT NULL AND iu.district != '' THEN ', ' || iu.district ELSE '' END AS location,
		COALESCE(iu.organization_type, 'College') AS type,
		0.0 AS rating,
		COALESCE(iu.website_url, '') AS website,
		COALESCE(iu.affiliation, '') AS affiliation,
		COALESCE(iu.verified, false) AS verified,
		COALESCE(iu.contact_email, '') AS contact_email,
		COALESCE(iu.contact_phone, '') AS contact_phone
		FROM institution_users iu
		WHERE iu.id = ? AND iu.deleted_at IS NULL`
	if err := r.db.Raw(query, id).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *Repository) FindPublishedAdmissionInstitutions(page, limit int, level string) ([]map[string]interface{}, int64, error) {
	var total int64
	var results []map[string]interface{}

	subQuery := `SELECT DISTINCT ap.institution_id FROM admission_pages ap WHERE ap.status = 'published' AND ap.deleted_at IS NULL`
	args := []interface{}{}
	if level != "" {
		subQuery += ` AND ap.data->'overview_data'->>'level' = ?`
		args = append(args, level)
	}

	countQuery := `SELECT COUNT(*) FROM institution_users iu 
		WHERE iu.id IN (` + subQuery + `)
		AND iu.deleted_at IS NULL`

	if err := r.db.Raw(countQuery, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	dataArgs := append(args, offset, limit)

	dataQuery := `SELECT 
		iu.id,
		iu.institution_name AS name,
		COALESCE(iu.logo_url, '') AS image_url,
		COALESCE(iu.province, '') || CASE WHEN iu.district IS NOT NULL AND iu.district != '' THEN ', ' || iu.district ELSE '' END AS location,
		COALESCE(iu.organization_type, 'College') AS type,
		0.0 AS rating,
		COALESCE(iu.website_url, '') AS website,
		COALESCE(iu.affiliation, '') AS affiliation,
		COALESCE(iu.verified, false) AS verified,
		COALESCE(
			(SELECT ap.data->'overview_data'->>'heroBanner' FROM admission_pages ap
				WHERE ap.institution_id = iu.id AND ap.status = 'published' AND ap.deleted_at IS NULL
				ORDER BY ap.published_at DESC LIMIT 1
			),
			''
		) AS hero_banner,
		COALESCE(
			(SELECT json_agg(json_build_object('title', pg->>'title', 'admissionStatus', pg->>'admissionStatus'))::jsonb FROM (
				SELECT jsonb_array_elements(sub.pd) AS pg
				FROM (SELECT ap.data->'programs_data' AS pd FROM admission_pages ap
					WHERE ap.institution_id = iu.id AND ap.status = 'published' AND ap.deleted_at IS NULL
					ORDER BY ap.published_at DESC LIMIT 1
				) sub
			) sub2),
			'[]'::jsonb
		) AS featured_programs
		FROM institution_users iu
		WHERE iu.id IN (` + subQuery + `)
		AND iu.deleted_at IS NULL
		ORDER BY (SELECT MAX(ap2.published_at) FROM admission_pages ap2 WHERE ap2.institution_id = iu.id AND ap2.status = 'published') DESC
		OFFSET ? LIMIT ?`

	if err := r.db.Raw(dataQuery, dataArgs...).Scan(&results).Error; err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

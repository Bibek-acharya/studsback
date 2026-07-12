package institution

import (
	"encoding/json"
	"strings"
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

func (r *Repository) FindProgramByID(id uint) (*InstitutionProgram, error) {
	var program InstitutionProgram
	err := r.db.First(&program, id).Error
	if err != nil {
		return nil, err
	}
	return &program, nil
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

func (r *Repository) DeleteProgramByID(id uint) error {
	result := r.db.Delete(&InstitutionProgram{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindGlobalCourseByID(id uint) (map[string]interface{}, error) {
	var course struct {
		ID          uint   `gorm:"column:id"`
		Title       string `gorm:"column:title"`
		Description string `gorm:"column:description"`
		Duration    string `gorm:"column:duration"`
		Level       string `gorm:"column:level"`
		Field       string `gorm:"column:field"`
		EstFee      string `gorm:"column:est_fee"`
		Affiliation string `gorm:"column:affiliation"`
		Location    string `gorm:"column:location"`
		Mode        string `gorm:"column:mode"`
		DegreeLabel string `gorm:"column:degree_label"`
	}
	err := r.db.Table("courses").
		Where("id = ? AND is_global = ? AND status = ?", id, true, "published").
		First(&course).Error
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"id":           course.ID,
		"title":        course.Title,
		"description":  course.Description,
		"duration":     course.Duration,
		"level":        course.Level,
		"field":        course.Field,
		"est_fee":      course.EstFee,
		"affiliation":  course.Affiliation,
		"location":     course.Location,
		"mode":         course.Mode,
		"degree_label": course.DegreeLabel,
	}
	return result, nil
}

type courseDraft struct {
	ID              uint   `gorm:"column:id"`
	Title           string `gorm:"column:title"`
	Description     string `gorm:"column:description"`
	Duration        string `gorm:"column:duration"`
	Level           string `gorm:"column:level"`
	Location        string `gorm:"column:location"`
	IsGlobal        bool   `gorm:"column:is_global"`
	Status          string `gorm:"column:status"`
	CreatedBy       uint   `gorm:"column:created_by"`
	SourceProgramID *uint  `gorm:"column:source_program_id"`
}

func (courseDraft) TableName() string {
	return "courses"
}

func (r *Repository) CreateCourseFromProgram(program *InstitutionProgram) (uint, error) {
	course := &courseDraft{
		Title:           program.Name,
		Description:     program.Description,
		Duration:        program.Duration,
		Level:           extractLevelFromData(program.Data),
		Location:        program.InstitutionLocation,
		IsGlobal:        false,
		Status:          "draft",
		CreatedBy:       program.InstitutionID,
		SourceProgramID: &program.ID,
	}
	result := r.db.Table("courses").Create(course)
	if result.Error != nil {
		return 0, result.Error
	}
	return course.ID, nil
}

func extractLevelFromData(data *string) string {
	if data == nil || *data == "" {
		return ""
	}
	var parsed struct {
		Level string `json:"level"`
	}
	if err := json.Unmarshal([]byte(*data), &parsed); err != nil {
		return ""
	}
	return parsed.Level
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

func (r *Repository) FindPublicInstitutions(page, pageSize int, search, location, instType string) ([]InstitutionUser, int64, error) {
	var users []InstitutionUser
	var total int64

	query := r.db.Model(&InstitutionUser{}).
		Joins("LEFT JOIN institution_settings ON institution_settings.institution_id = institution_users.id").
		Where("(institution_settings.public_profile = ? OR institution_settings.id IS NULL)", true).
		Where("institution_users.profile_status = ?", "published").
		Where("institution_users.deleted_at IS NULL").
		Where("institution_users.status = ?", "approved")

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(institution_users.institution_name ILIKE ? OR institution_users.district ILIKE ?)", like, like)
	}
	if location != "" {
		query = query.Where("institution_users.district ILIKE ?", "%"+location+"%")
	}
	if instType != "" {
		query = query.Where("institution_users.organization_type = ?", instType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.
		Select("institution_users.*").
		Order("institution_users.verified DESC, institution_users.created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&users).Error

	return users, total, err
}

func (r *Repository) FindPublicInstitutionByID(id uint) (*InstitutionUser, error) {
	var user InstitutionUser
	err := r.db.Joins("LEFT JOIN institution_settings ON institution_settings.institution_id = institution_users.id").
		Where("institution_users.id = ?", id).
		Where("institution_users.profile_status = ?", "published").
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

func (r *Repository) DeleteEntranceByID(id uint) error {
	result := r.db.Delete(&InstitutionEntrance{}, id)
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

func (r *Repository) FindAllPublishedEvents(page, limit int) ([]InstitutionEvent, int64, error) {
	var events []InstitutionEvent
	var total int64
	offset := (page - 1) * limit
	if err := r.db.Model(&InstitutionEvent{}).Where("status IN ?", []string{"upcoming", "published"}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Where("status IN ?", []string{"upcoming", "published"}).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&events).Error
	return events, total, err
}

func (r *Repository) FindPublishedEventByID(id uint) (*InstitutionEvent, error) {
	var event InstitutionEvent
	err := r.db.Where("id = ? AND status IN ?", id, []string{"upcoming", "published"}).First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) FindEventBySlug(slug string) (*InstitutionEvent, error) {
	var event InstitutionEvent
	err := r.db.Where("slug = ?", slug).First(&event).Error
	return &event, err
}

func (r *Repository) FindEventsByInstitution(instID uint, page, limit int) ([]InstitutionEvent, int64, error) {
	var events []InstitutionEvent
	var total int64

	if err := r.db.Model(&InstitutionEvent{}).Where("institution_id = ?", instID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.Where("institution_id = ?", instID).
		Order("start_date desc").Offset(offset).Limit(limit).Find(&events).Error
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

func (r *Repository) FindAllPublishedNews(page, limit int) ([]InstitutionNews, int64, error) {
	var news []InstitutionNews
	var total int64
	offset := (page - 1) * limit
	if err := r.db.Model(&InstitutionNews{}).Where("status = ?", "published").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Where("status = ?", "published").
		Order("created_at desc").Offset(offset).Limit(limit).Find(&news).Error
	return news, total, err
}

func (r *Repository) FindPublishedNewsByID(id uint) (*InstitutionNews, error) {
	var news InstitutionNews
	err := r.db.Where("id = ? AND status = ?", id, "published").First(&news).Error
	if err != nil {
		return nil, err
	}
	return &news, nil
}

func (r *Repository) FindNewsBySlug(slug string) (*InstitutionNews, error) {
	var news InstitutionNews
	err := r.db.Where("slug = ?", slug).First(&news).Error
	return &news, err
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

func (r *Repository) FindPublishedBlogs(page, limit int) ([]InstitutionBlog, int64, error) {
	var blogs []InstitutionBlog
	var total int64
	offset := (page - 1) * limit

	query := r.db.Model(&InstitutionBlog{}).Where("status = ?", "published")
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&blogs).Error
	return blogs, total, err
}

func (r *Repository) FindBlogBySlug(slug string) (*InstitutionBlog, error) {
	var blog InstitutionBlog
	err := r.db.Where("slug = ?", slug).First(&blog).Error
	return &blog, err
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

func (r *Repository) CreateUserMessage(senderID, receiverID uint, subject, content, direction string) error {
	return r.db.Table("messages").Create(map[string]interface{}{
		"sender_id":   senderID,
		"receiver_id": receiverID,
		"subject":     subject,
		"content":     content,
		"direction":   direction,
		"read":        false,
	}).Error
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

func (r *Repository) FindScholarshipsByInstitution(instID uint) ([]Scholarship, error) {
	var scholarships []Scholarship
	err := r.db.Where("institution_id = ?", instID).Order("created_at desc").Find(&scholarships).Error
	return scholarships, err
}

func (r *Repository) FindScholarshipByIDAndInstitution(id, instID uint) (*Scholarship, error) {
	var scholarship Scholarship
	err := r.db.Where("id = ? AND institution_id = ?", id, instID).First(&scholarship).Error
	if err != nil {
		return nil, err
	}
	return &scholarship, nil
}

func (r *Repository) FindAllPublishedScholarships(page, limit int) ([]Scholarship, int64, error) {
	var scholarships []Scholarship
	var total int64
	offset := (page - 1) * limit
	if err := r.db.Model(&Scholarship{}).Where("status = ?", "published").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Where("status = ?", "published").
		Order("created_at desc").Offset(offset).Limit(limit).Find(&scholarships).Error
	return scholarships, total, err
}

func (r *Repository) FindPublishedScholarshipByID(id uint) (*Scholarship, error) {
	var scholarship Scholarship
	err := r.db.Where("id = ? AND status = ?", id, "published").First(&scholarship).Error
	if err != nil {
		return nil, err
	}
	return &scholarship, nil
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

func (r *Repository) DeleteAdmissionPageByID(id uint) error {
	result := r.db.Delete(&AdmissionPage{}, id)
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

// --- Superadmin Repository Methods ---

func (r *Repository) FindAllPrograms(page, limit int) ([]InstitutionProgram, int64, error) {
	var programs []InstitutionProgram
	var total int64
	if err := r.db.Model(&InstitutionProgram{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := r.db.Order("created_at desc").Offset(offset).Limit(limit).Find(&programs).Error
	return programs, total, err
}

func (r *Repository) FindAllEntrances(status string, page, limit int) ([]InstitutionEntrance, int64, error) {
	var entrances []InstitutionEntrance
	var total int64
	query := r.db.Model(&InstitutionEntrance{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&entrances).Error
	return entrances, total, err
}

func (r *Repository) FindAllAdmissionPages(status string, page, limit int) ([]AdmissionPage, int64, error) {
	var pages []AdmissionPage
	var total int64
	query := r.db.Model(&AdmissionPage{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&pages).Error
	return pages, total, err
}

func (r *Repository) FindEntranceByID(id uint) (*InstitutionEntrance, error) {
	var entrance InstitutionEntrance
	err := r.db.First(&entrance, id).Error
	if err != nil {
		return nil, err
	}
	return &entrance, nil
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

	// Count institutions with published admissions
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

func (r *Repository) GetPublicFilterCounts() (*PublicInstitutionFilterCountsResponse, error) {
	resp := &PublicInstitutionFilterCountsResponse{
		TypeCounts:      map[string]int64{},
		TypeCountsByID:  map[string]int64{},
		FacetCountsByID: map[string]int64{},
	}

	baseQuery := r.db.Model(&InstitutionUser{}).
		Joins("LEFT JOIN institution_settings ON institution_settings.institution_id = institution_users.id").
		Where("(institution_settings.public_profile = ? OR institution_settings.id IS NULL)", true).
		Where("institution_users.profile_status = ?", "published").
		Where("institution_users.deleted_at IS NULL").
		Where("institution_users.status = ?", "approved")

	if err := baseQuery.Count(&resp.Total).Error; err != nil {
		return nil, err
	}

	featuredQuery := r.db.Model(&InstitutionUser{}).
		Joins("LEFT JOIN institution_settings ON institution_settings.institution_id = institution_users.id").
		Where("(institution_settings.public_profile = ? OR institution_settings.id IS NULL)", true).
		Where("institution_users.profile_status = ?", "published").
		Where("institution_users.featured = ?", true).
		Where("institution_users.deleted_at IS NULL").
		Where("institution_users.status = ?", "approved")
	if err := featuredQuery.Count(&resp.Featured).Error; err != nil {
		return nil, err
	}

	verifiedQuery := r.db.Model(&InstitutionUser{}).
		Joins("LEFT JOIN institution_settings ON institution_settings.institution_id = institution_users.id").
		Where("(institution_settings.public_profile = ? OR institution_settings.id IS NULL)", true).
		Where("institution_users.profile_status = ?", "published").
		Where("institution_users.verified = ?", true).
		Where("institution_users.deleted_at IS NULL").
		Where("institution_users.status = ?", "approved")
	if err := verifiedQuery.Count(&resp.Verified).Error; err != nil {
		return nil, err
	}

	typeMapping := map[string]struct{ id, label string }{
		"Private":            {"ct_private", "Private"},
		"Public / Govt":      {"ct_public", "Public / Govt"},
		"Community":          {"ct_community", "Community"},
		"Constituent":        {"ct_constituent", "Constituent"},
		"Foreign Affiliated": {"ct_foreign", "Foreign Affiliated"},
	}
	for orgType, entry := range typeMapping {
		var count int64
		typeQuery := r.db.Model(&InstitutionUser{}).
			Joins("LEFT JOIN institution_settings ON institution_settings.institution_id = institution_users.id").
			Where("(institution_settings.public_profile = ? OR institution_settings.id IS NULL)", true).
			Where("institution_users.profile_status = ?", "published").
			Where("institution_users.deleted_at IS NULL").
			Where("institution_users.status = ?", "approved").
			Where("institution_users.organization_type = ?", orgType)
		if err := typeQuery.Count(&count).Error; err != nil {
			return nil, err
		}
		resp.TypeCounts[entry.label] = count
		resp.TypeCountsByID[entry.id] = count
	}

	facetKeywordsByID := map[string][]string{
		"plus2":          {"+2", "higher secondary", "10+2", "neb"},
		"alevel":         {"a level", "alevel"},
		"bachelor":       {"bachelor", "bsc", "be ", "bba", "bbs", "bim", "mbbs", "bds"},
		"master":         {"master", "msc", "mba", "mbs", "mca", "mit"},
		"diploma":        {"diploma", "ctevt", "pcl", "health assistant", "ha "},
		"p2_sci":         {"science"},
		"p2_mgmt":        {"management"},
		"p2_hum":         {"humanities"},
		"p2_edu":         {"education"},
		"p2_law":         {"law"},
		"al_sci":         {"a level - science", "a level science"},
		"al_nonsci":      {"a level - non-science", "a level - non-science/mgmt", "a level management"},
		"b_it":           {"information technology", "computer science", "it", "cs"},
		"b_eng":          {"engineering"},
		"b_biz":          {"business", "management"},
		"b_med":          {"medical", "healthcare", "nursing", "pharmacy"},
		"b_hum":          {"humanities", "social sciences"},
		"b_agr":          {"agriculture", "forestry"},
		"m_biz":          {"master business", "mba", "mbs"},
		"m_it":           {"mca", "mit", "msc csit", "master it"},
		"m_eng":          {"master engineering", "m.e", "meng"},
		"m_hum":          {"master humanities", "master social sciences"},
		"d_eng":          {"diploma engineering", "ctevt engineering"},
		"d_med":          {"pcl nursing", "ha", "diploma medical", "ctevt nursing"},
		"d_hm":           {"hotel management", "tourism"},
		"d_agr":          {"diploma agriculture", "diploma forestry", "ctevt agriculture"},
		"c_bsc_csit":     {"bsc csit"},
		"c_bca":          {"bca"},
		"c_bit":          {"bit"},
		"c_bim":          {"bim"},
		"c_civil":        {"civil engineering"},
		"c_comp":         {"computer engineering"},
		"c_arch":         {"architecture"},
		"c_elec":         {"electrical", "electronics"},
		"c_bba":          {"bba"},
		"c_bbs":          {"bbs"},
		"c_bbm":          {"bbm"},
		"c_bhm":          {"bhm", "hotel management"},
		"c_mbbs":         {"mbbs"},
		"c_bds":          {"bds"},
		"c_nursing":      {"bsc nursing", "nursing"},
		"c_pharma":       {"pharmacy", "pharma"},
		"c_bsc_ag":       {"bsc agriculture"},
		"c_bsc_forestry": {"bsc forestry"},
		"c_mba":          {"mba"},
		"c_mbs":          {"mbs"},
		"c_msc_csit":     {"msc csit"},
		"c_mca":          {"mca"},
		"c_mit":          {"mit"},
		"c_dip_civil":    {"diploma in civil", "diploma civil"},
		"c_dip_comp":     {"diploma in computer", "diploma computer"},
		"c_pcl_nurs":     {"pcl nursing"},
		"c_ha":           {"health assistant", "ha (general medicine)", " ha "},
		"prov_koshi":     {"koshi"},
		"prov_madhesh":   {"madhesh"},
		"prov_bagmati":   {"bagmati"},
		"prov_gandaki":   {"gandaki"},
		"prov_lumbini":   {"lumbini"},
		"prov_karnali":   {"karnali"},
		"prov_sudur":     {"sudurpashchim"},
		"d_jhapa":        {"jhapa"},
		"d_morang":       {"morang"},
		"d_sunsari":      {"sunsari"},
		"d_dhanusha":     {"dhanusha"},
		"d_parsa":        {"parsa"},
		"d_bhaktapur":    {"bhaktapur"},
		"d_chitwan":      {"chitwan"},
		"d_kathmandu":    {"kathmandu"},
		"d_lalitpur":     {"lalitpur"},
		"d_kavre":        {"kavrepalanchok", "kavre"},
		"d_kaski":        {"kaski"},
		"d_nawalpur":     {"nawalpur"},
		"d_tanahun":      {"tanahun"},
		"d_banke":        {"banke"},
		"d_rupandehi":    {"rupandehi"},
		"d_dang":         {"dang"},
		"d_surkhet":      {"surkhet"},
		"d_jumla":        {"jumla"},
		"d_kailali":      {"kailali"},
		"d_kanchanpur":   {"kanchanpur"},
		"u_tu":           {"tribhuvan university"},
		"u_ku":           {"kathmandu university"},
		"u_pu":           {"pokhara university"},
		"u_purbanchal":   {"purbanchal university"},
		"u_mwu":          {"mid-western university"},
		"u_fwu":          {"far-western university"},
		"u_afu":          {"agriculture & forestry university", "agriculture and forestry university"},
		"u_lincoln":      {"lincoln university"},
		"u_london_met":   {"london metropolitan university"},
		"u_west_england": {"university of the west of england"},
		"1 Year":         {"1 year", "one year"},
		"2 Years":        {"2 years", "two years"},
		"3 Years":        {"3 years", "three years"},
		"4 Years":        {"4 years", "four years"},
		"5+ Years":       {"5 years", "5+ years", "five years"},
	}

	var facetRows []struct {
		District    string
		Affiliation string
	}

	if err := r.db.Model(&InstitutionUser{}).
		Select("district, affiliation").
		Joins("LEFT JOIN institution_settings ON institution_settings.institution_id = institution_users.id").
		Where("(institution_settings.public_profile = ? OR institution_settings.id IS NULL)", true).
		Where("institution_users.profile_status = ?", "published").
		Where("institution_users.deleted_at IS NULL").
		Where("institution_users.status = ?", "approved").
		Scan(&facetRows).Error; err != nil {
		return nil, err
	}

	for _, row := range facetRows {
		searchText := strings.ToLower(row.District + " " + row.Affiliation)

		for facetID, keywords := range facetKeywordsByID {
			for _, keyword := range keywords {
				if keyword == "" {
					continue
				}
				if strings.Contains(searchText, strings.ToLower(keyword)) {
					resp.FacetCountsByID[facetID]++
					break
				}
			}
		}
	}

	return resp, nil
}

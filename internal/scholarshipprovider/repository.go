package scholarshipprovider

import (
	"studsphere/backend/internal/scholarship"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetNextRollNumber() (int, error) {
	var seq int
	err := r.db.Raw("SELECT nextval('scholarship_roll_number_seq')").Scan(&seq).Error
	return seq, err
}

func (r *Repository) UpdateRollNumber(id uint, rollNumber string) error {
	return r.db.Table("provider_applications").Where("id = ?", id).Update("roll_number", rollNumber).Error
}

func (r *Repository) GetDashboardCounts(providerID uint) (int64, int64, int64, int64, int64, error) {
	var totalScholarships, totalApplications, pendingApplications, totalInterviews, unreadMessages int64

	if err := r.db.Model(&ProviderScholarship{}).Where("provider_id = ?", providerID).Count(&totalScholarships).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}

	if err := r.db.Model(&ProviderApplication{}).
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_scholarships.provider_id = ?", providerID).Count(&totalApplications).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}

	if err := r.db.Model(&ProviderApplication{}).
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_scholarships.provider_id = ? AND provider_applications.status = ?", providerID, "pending").
		Count(&pendingApplications).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}

	if err := r.db.Model(&ProviderInterview{}).Where("provider_id = ?", providerID).Count(&totalInterviews).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}

	if err := r.db.Model(&ProviderMessage{}).Where("provider_id = ? AND read = ?", providerID, false).Count(&unreadMessages).Error; err != nil {
		return 0, 0, 0, 0, 0, err
	}

	return totalScholarships, totalApplications, pendingApplications, totalInterviews, unreadMessages, nil
}

func (r *Repository) GetAnalytics(providerID uint) ([]ProviderApplication, []ProviderScholarship, error) {
	var applications []ProviderApplication
	if err := r.db.
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_scholarships.provider_id = ?", providerID).
		Find(&applications).Error; err != nil {
		return nil, nil, err
	}

	var scholarships []ProviderScholarship
	if err := r.db.Where("provider_id = ?", providerID).Find(&scholarships).Error; err != nil {
		return nil, nil, err
	}

	return applications, scholarships, nil
}

func (r *Repository) GetFilteredApplications(providerID uint, filters DetailedAnalyticsFilters) ([]ProviderApplication, error) {
	var applications []ProviderApplication
	query := r.db.Model(&ProviderApplication{}).
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_scholarships.provider_id = ?", providerID)

	if filters.Province != "" {
		query = query.Where("provider_applications.province = ?", filters.Province)
	}
	if filters.District != "" {
		query = query.Where("provider_applications.district = ?", filters.District)
	}
	if filters.SchoolType != "" {
		query = query.Where("provider_applications.school_type = ?", filters.SchoolType)
	}
	switch filters.ScholarshipStatus {
	case "recipients":
		query = query.Where("provider_applications.status = ?", "approved")
	case "non-recipients":
		query = query.Where("provider_applications.status != ?", "approved")
	}
	if filters.EthnicityProvince != "" {
		query = query.Where("provider_applications.province = ?", filters.EthnicityProvince)
	}

	if err := query.Find(&applications).Error; err != nil {
		return nil, err
	}
	return applications, nil
}

func (r *Repository) GetPaymentsByApplicationIDs(applicationIDs []uint) ([]scholarship.Payment, error) {
	if len(applicationIDs) == 0 {
		return nil, nil
	}
	var payments []scholarship.Payment
	if err := r.db.Where("application_id IN ?", applicationIDs).Find(&payments).Error; err != nil {
		return nil, err
	}
	return payments, nil
}

func (r *Repository) GetApplicationCountByScholarship(scholarshipID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&ProviderApplication{}).Where("scholarship_id = ?", scholarshipID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) CreateScholarship(scholarship *ProviderScholarship) error {
	return r.db.Create(scholarship).Error
}

func (r *Repository) CreatePublicScholarship(s *scholarship.Scholarship, providerScholarshipID uint) error {
	s.ProviderScholarshipID = &providerScholarshipID
	r.db.Unscoped().Where("provider_scholarship_id = ?", providerScholarshipID).Delete(&scholarship.Scholarship{})
	return r.db.Create(s).Error
}

func (r *Repository) UpdatePublicScholarshipProviderID(publicID, providerScholarshipID uint) error {
	return r.db.Model(&scholarship.Scholarship{}).Where("id = ?", publicID).
		Update("provider_scholarship_id", providerScholarshipID).Error
}

func (r *Repository) FindPublicScholarship(title, provider string) (*scholarship.Scholarship, error) {
	var s scholarship.Scholarship
	err := r.db.Where("title = ? AND provider = ?", title, provider).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) FindPublicScholarshipByProviderScholarshipID(providerScholarshipID uint) (*scholarship.Scholarship, error) {
	var s scholarship.Scholarship
	err := r.db.Where("provider_scholarship_id = ?", providerScholarshipID).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) UpdatePublicScholarship(id uint, updates map[string]interface{}) error {
	return r.db.Model(&scholarship.Scholarship{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) DeletePublicScholarship(title, provider string) error {
	return r.db.Unscoped().Where("title = ? AND provider = ?", title, provider).Delete(&scholarship.Scholarship{}).Error
}

func (r *Repository) DeletePublicScholarshipByProviderScholarshipID(providerScholarshipID uint) error {
	return r.db.Unscoped().Where("provider_scholarship_id = ?", providerScholarshipID).Delete(&scholarship.Scholarship{}).Error
}

func (r *Repository) GetScholarshipsByProvider(providerID uint, page, limit int) ([]ProviderScholarship, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderScholarship{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var scholarships []ProviderScholarship
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&scholarships).Error; err != nil {
		return nil, 0, err
	}

	return scholarships, total, nil
}

func (r *Repository) GetPublishedScholarshipsByProvider(providerID uint, page, limit int) ([]ProviderScholarship, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderScholarship{}).Where("provider_id = ? AND status = ?", providerID, "published").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var scholarships []ProviderScholarship
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ? AND status = ?", providerID, "published").
		Order("created_at desc").Offset(offset).Limit(limit).Find(&scholarships).Error; err != nil {
		return nil, 0, err
	}

	return scholarships, total, nil
}


func (r *Repository) GetScholarshipByIDAndProvider(id uint, providerID uint) (*ProviderScholarship, error) {
	var scholarship ProviderScholarship
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&scholarship).Error; err != nil {
		return nil, err
	}
	return &scholarship, nil
}

func (r *Repository) UpdateScholarship(scholarship *ProviderScholarship, updates map[string]interface{}) error {
	return r.db.Model(scholarship).Updates(updates).Error
}

func (r *Repository) DeleteScholarship(id uint, providerID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("scholarship_id = ?", id).Delete(&ProviderApplication{}).Error; err != nil {
			return err
		}

		result := tx.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderScholarship{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *Repository) GetApplicationsByProvider(providerID uint, page, limit int, status, scholarshipID string) ([]ProviderApplication, int64, error) {
	query := r.db.Model(&ProviderApplication{}).
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_scholarships.provider_id = ?", providerID)

	if status != "" {
		query = query.Where("provider_applications.status = ?", status)
	} else {
		query = query.Where("provider_applications.status != ?", "pending_payment")
	}
	if scholarshipID != "" {
		query = query.Where("provider_applications.scholarship_id = ?", scholarshipID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var applications []ProviderApplication
	offset := (page - 1) * limit
	if err := query.Preload("Scholarship").Order("created_at desc").Offset(offset).Limit(limit).Find(&applications).Error; err != nil {
		return nil, 0, err
	}

	// Fetch payment for each application
	for i, app := range applications {
		if payment := r.findPaymentByApplication(&app); payment != nil {
			applications[i].Payment = payment
		}
	}

	return applications, total, nil
}

func (r *Repository) findPaymentByApplication(app *ProviderApplication) *ProviderPayment {
	if app.ScholarshipApplicationID == nil {
		return nil
	}
	var payment ProviderPayment
	if err := r.db.Table("scholarship_payments").
		Where("application_id = ?", *app.ScholarshipApplicationID).
		Order("created_at desc").
		First(&payment).Error; err != nil {
		return nil
	}
	return &payment
}

func (r *Repository) GetApplicationByIDAndProvider(id uint, providerID uint) (*ProviderApplication, error) {
	var application ProviderApplication
	if err := r.db.
		Joins("JOIN provider_scholarships ON provider_scholarships.id = provider_applications.scholarship_id").
		Where("provider_applications.id = ? AND provider_scholarships.provider_id = ?", id, providerID).
		Preload("Scholarship").
		First(&application).Error; err != nil {
		return nil, err
	}

	application.Payment = r.findPaymentByApplication(&application)

	return &application, nil
}

func (r *Repository) EvaluateApplication(application *ProviderApplication, score *int, passing bool, notes string) error {
	return r.db.Model(application).Updates(map[string]interface{}{
		"evaluation_score":  score,
		"evaluation_passed": passing,
		"evaluation_notes":  notes,
	}).Error
}

func (r *Repository) UpdateApplicationStatus(application *ProviderApplication, status string) error {
	updates := map[string]interface{}{"status": status}
	if status == "rejected" && application.RejectionReason != "" {
		updates["rejection_reason"] = application.RejectionReason
	}
	err := r.db.Model(application).Updates(updates).Error
	if err != nil {
		return err
	}
	if application.ScholarshipApplicationID != nil {
		r.db.Model(&scholarship.ScholarshipApplication{}).
			Where("id = ?", *application.ScholarshipApplicationID).
			Updates(updates)
	}
	return nil
}

func (r *Repository) GetInterviewsByProvider(providerID uint) ([]ProviderInterview, error) {
	var interviews []ProviderInterview
	if err := r.db.Where("provider_id = ?", providerID).
		Order("scheduled_at asc").Find(&interviews).Error; err != nil {
		return nil, err
	}
	return interviews, nil
}

func (r *Repository) CreateInterview(interview *ProviderInterview) error {
	return r.db.Create(interview).Error
}

func (r *Repository) GetInterviewByIDAndProvider(id uint, providerID uint) (*ProviderInterview, error) {
	var interview ProviderInterview
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&interview).Error; err != nil {
		return nil, err
	}
	return &interview, nil
}

func (r *Repository) UpdateInterview(interview *ProviderInterview, updates map[string]interface{}) error {
	return r.db.Model(interview).Updates(updates).Error
}

func (r *Repository) GetMessagesByProvider(providerID uint, page, limit int) ([]ProviderMessage, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderMessage{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var messages []ProviderMessage
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func (r *Repository) GetDB() *gorm.DB {
	return r.db
}

func (r *Repository) CreateMessage(message *ProviderMessage) error {
	return r.db.Create(message).Error
}

func (r *Repository) GetMessageByIDAndProvider(id uint, providerID uint) (*ProviderMessage, error) {
	var message ProviderMessage
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&message).Error; err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *Repository) MarkMessageRead(message *ProviderMessage) error {
	return r.db.Model(message).Update("read", true).Error
}

func (r *Repository) GetProviderProfile(providerID uint) (*ScholarshipProviderUser, error) {
	var provider ScholarshipProviderUser
	if err := r.db.First(&provider, providerID).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *Repository) UpdateProviderProfile(provider *ScholarshipProviderUser, updates map[string]interface{}) error {
	return r.db.Model(provider).Updates(updates).Error
}

func (r *Repository) GetProviderSettings(providerID uint) (*ProviderSettings, error) {
	var settings ProviderSettings
	if err := r.db.Where("provider_id = ?", providerID).FirstOrCreate(&settings, ProviderSettings{
		ProviderID: providerID,
	}).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *Repository) UpdateProviderSettings(settings *ProviderSettings, updates map[string]interface{}) error {
	return r.db.Model(settings).Updates(updates).Error
}

func (r *Repository) GetNotificationsByProvider(providerID uint, page, limit int) ([]ProviderNotification, int64, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderNotification{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}

	var notifications []ProviderNotification
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).
		Order("read asc, created_at desc").Offset(offset).Limit(limit).Find(&notifications).Error; err != nil {
		return nil, 0, 0, err
	}

	var unreadCount int64
	if err := r.db.Model(&ProviderNotification{}).Where("provider_id = ? AND read = ?", providerID, false).Count(&unreadCount).Error; err != nil {
		return nil, 0, 0, err
	}

	return notifications, total, unreadCount, nil
}

func (r *Repository) GetNotificationByIDAndProvider(id uint, providerID uint) (*ProviderNotification, error) {
	var notification ProviderNotification
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&notification).Error; err != nil {
		return nil, err
	}
	return &notification, nil
}

func (r *Repository) MarkNotificationRead(notification *ProviderNotification) error {
	return r.db.Model(notification).Update("read", true).Error
}

func (r *Repository) MarkAllNotificationsRead(providerID uint) error {
	return r.db.Model(&ProviderNotification{}).Where("provider_id = ? AND read = ?", providerID, false).
		Update("read", true).Error
}

func (r *Repository) CreateNotification(notification *ProviderNotification) error {
	return r.db.Create(notification).Error
}

func (r *Repository) CheckNotificationExists(providerID uint, title string) (bool, error) {
	var count int64
	err := r.db.Model(&ProviderNotification{}).
		Where("provider_id = ? AND title = ?", providerID, title).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) CreateNews(news *ProviderNews) error {
	return r.db.Create(news).Error
}

func (r *Repository) GetNewsByProvider(providerID uint, page, limit int) ([]ProviderNews, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderNews{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var news []ProviderNews
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&news).Error; err != nil {
		return nil, 0, err
	}

	return news, total, nil
}

func (r *Repository) GetPublishedNewsByProvider(providerID uint, page, limit int) ([]ProviderNews, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderNews{}).Where("provider_id = ? AND status = ?", providerID, "published").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var news []ProviderNews
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ? AND status = ?", providerID, "published").
		Order("created_at desc").Offset(offset).Limit(limit).Find(&news).Error; err != nil {
		return nil, 0, err
	}

	return news, total, nil
}


func (r *Repository) GetNewsByIDAndProvider(id uint, providerID uint) (*ProviderNews, error) {
	var news ProviderNews
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&news).Error; err != nil {
		return nil, err
	}
	return &news, nil
}

func (r *Repository) UpdateNews(news *ProviderNews, updates map[string]interface{}) error {
	return r.db.Model(news).Updates(updates).Error
}

func (r *Repository) DeleteNews(id uint, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderNews{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) CreateEvent(event *ProviderEvent) error {
	return r.db.Create(event).Error
}

func (r *Repository) GetEventsByProvider(providerID uint, page, limit int) ([]ProviderEvent, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderEvent{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var events []ProviderEvent
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).
		Order("start_date desc").Offset(offset).Limit(limit).Find(&events).Error; err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

func (r *Repository) GetEventByIDAndProvider(id uint, providerID uint) (*ProviderEvent, error) {
	var event ProviderEvent
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) UpdateEvent(event *ProviderEvent, updates map[string]interface{}) error {
	return r.db.Model(event).Updates(updates).Error
}

func (r *Repository) DeleteEvent(id uint, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderEvent{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) CreateBlog(blog *ProviderBlog) error {
	return r.db.Create(blog).Error
}

func (r *Repository) GetBlogsByProvider(providerID uint, page, limit int) ([]ProviderBlog, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderBlog{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var blogs []ProviderBlog
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&blogs).Error; err != nil {
		return nil, 0, err
	}

	return blogs, total, nil
}

func (r *Repository) GetBlogByIDAndProvider(id uint, providerID uint) (*ProviderBlog, error) {
	var blog ProviderBlog
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&blog).Error; err != nil {
		return nil, err
	}
	return &blog, nil
}

func (r *Repository) UpdateBlog(blog *ProviderBlog, updates map[string]interface{}) error {
	return r.db.Model(blog).Updates(updates).Error
}

func (r *Repository) DeleteBlog(id uint, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderBlog{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) CreateCalendarEvent(event *ProviderCalendarEvent) error {
	return r.db.Create(event).Error
}

func (r *Repository) GetCalendarEventsByProvider(providerID uint) ([]ProviderCalendarEvent, error) {
	var events []ProviderCalendarEvent
	if err := r.db.Where("provider_id = ?", providerID).
		Order("start_date asc").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (r *Repository) GetCalendarEventByIDAndProvider(id uint, providerID uint) (*ProviderCalendarEvent, error) {
	var event ProviderCalendarEvent
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) UpdateCalendarEvent(event *ProviderCalendarEvent, updates map[string]interface{}) error {
	return r.db.Model(event).Updates(updates).Error
}

func (r *Repository) DeleteCalendarEvent(id uint, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderCalendarEvent{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) CreateResult(result *ProviderResult) error {
	return r.db.Create(result).Error
}

func (r *Repository) GetResultsByProvider(providerID uint, page, limit int) ([]ProviderResult, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderResult{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var results []ProviderResult
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&results).Error; err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (r *Repository) GetResultByIDAndProvider(id uint, providerID uint) (*ProviderResult, error) {
	var result ProviderResult
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Repository) UpdateResult(result *ProviderResult, updates map[string]interface{}) error {
	return r.db.Model(result).Updates(updates).Error
}

func (r *Repository) DeleteResult(id uint, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderResult{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) CreateWrittenExam(exam *WrittenExam) error {
	return r.db.Create(exam).Error
}

func (r *Repository) GetWrittenExamsByProvider(providerID uint, page, limit int) ([]WrittenExam, int64, error) {
	var total int64
	if err := r.db.Model(&WrittenExam{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var exams []WrittenExam
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).Order("created_at desc").Offset(offset).Limit(limit).Find(&exams).Error; err != nil {
		return nil, 0, err
	}
	return exams, total, nil
}

func (r *Repository) GetWrittenExamsByScholarship(providerID, scholarshipID uint) ([]WrittenExam, error) {
	var exams []WrittenExam
	if err := r.db.Where("provider_id = ? AND scholarship_id = ?", providerID, scholarshipID).Order("created_at desc").Find(&exams).Error; err != nil {
		return nil, err
	}
	return exams, nil
}

func (r *Repository) GetWrittenExamByIDAndProvider(id, providerID uint) (*WrittenExam, error) {
	var exam WrittenExam
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&exam).Error; err != nil {
		return nil, err
	}
	return &exam, nil
}

func (r *Repository) UpdateWrittenExam(exam *WrittenExam, updates map[string]interface{}) error {
	return r.db.Model(exam).Updates(updates).Error
}

func (r *Repository) DeleteWrittenExam(id, providerID uint) error {
	r.db.Where("written_exam_id = ?", id).Delete(&WrittenExamResult{})
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&WrittenExam{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) GetWrittenExamResults(examID uint) ([]WrittenExamResult, error) {
	var results []WrittenExamResult
	if err := r.db.Where("written_exam_id = ?", examID).Order("id asc").Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *Repository) CreateWrittenExamResult(result *WrittenExamResult) error {
	return r.db.Create(result).Error
}

func (r *Repository) GetWrittenExamResultByID(id, examID uint) (*WrittenExamResult, error) {
	var result WrittenExamResult
	if err := r.db.Where("id = ? AND written_exam_id = ?", id, examID).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Repository) UpdateWrittenExamResult(result *WrittenExamResult, updates map[string]interface{}) error {
	return r.db.Model(result).Updates(updates).Error
}

func (r *Repository) DeleteWrittenExamResult(id, examID uint) error {
	result := r.db.Where("id = ? AND written_exam_id = ?", id, examID).Delete(&WrittenExamResult{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}


func (r *Repository) CreateAccess(access *ProviderAccess) error {
	return r.db.Create(access).Error
}

func (r *Repository) GetAccessByProvider(providerID uint, page, limit int) ([]ProviderAccess, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderAccess{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var access []ProviderAccess
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&access).Error; err != nil {
		return nil, 0, err
	}

	return access, total, nil
}

func (r *Repository) GetAccessByIDAndProvider(id uint, providerID uint) (*ProviderAccess, error) {
	var access ProviderAccess
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&access).Error; err != nil {
		return nil, err
	}
	return &access, nil
}

func (r *Repository) UpdateAccess(access *ProviderAccess, updates map[string]interface{}) error {
	return r.db.Model(access).Updates(updates).Error
}

func (r *Repository) DeleteAccess(id uint, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderAccess{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) GetPublishedNews(page, limit int) ([]ProviderNews, int64, error) {
	var news []ProviderNews
	var total int64

	offset := (page - 1) * limit

	if err := r.db.Model(&ProviderNews{}).Where("status = ?", "published").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.Where("status = ?", "published").
		Order("created_at desc").
		Offset(offset).Limit(limit).
		Find(&news).Error; err != nil {
		return nil, 0, err
	}

	return news, total, nil
}

func (r *Repository) GetPublishedNewsByID(id uint) (*ProviderNews, error) {
	var news ProviderNews
	if err := r.db.Where("id = ? AND status = ?", id, "published").First(&news).Error; err != nil {
		return nil, err
	}
	return &news, nil
}

func (r *Repository) GetPublishedEvents(page, limit int) ([]ProviderEvent, int64, error) {
	var events []ProviderEvent
	var total int64

	offset := (page - 1) * limit

	if err := r.db.Model(&ProviderEvent{}).Where("status = ?", "upcoming").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.Where("status = ?", "upcoming").
		Order("start_date asc").
		Offset(offset).Limit(limit).
		Find(&events).Error; err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

func (r *Repository) GetPublishedEventByID(id uint) (*ProviderEvent, error) {
	var event ProviderEvent
	if err := r.db.Where("id = ? AND status = ?", id, "upcoming").First(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) GetPublishedBlogs(page, limit int) ([]ProviderBlog, int64, error) {
	var blogs []ProviderBlog
	var total int64

	offset := (page - 1) * limit

	if err := r.db.Model(&ProviderBlog{}).Where("status = ?", "published").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.Where("status = ?", "published").
		Order("created_at desc").
		Offset(offset).Limit(limit).
		Find(&blogs).Error; err != nil {
		return nil, 0, err
	}

	return blogs, total, nil
}

func (r *Repository) GetPublishedBlogByID(id uint) (*ProviderBlog, error) {
	var blog ProviderBlog
	if err := r.db.Where("id = ? AND status = ?", id, "published").First(&blog).Error; err != nil {
		return nil, err
	}
	return &blog, nil
}

func (r *Repository) CreateAccessUser(user *ProviderAccessUser) error {
	return r.db.Create(user).Error
}

func (r *Repository) GetAccessUserByID(id uint) (*ProviderAccessUser, error) {
	var user ProviderAccessUser
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetAccessUserByEmail(email string, providerID uint) (*ProviderAccessUser, error) {
	var user ProviderAccessUser
	if err := r.db.Where("email = ? AND provider_id = ?", email, providerID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetAccessUserByEmailOnly(email string) (*ProviderAccessUser, error) {
	var user ProviderAccessUser
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetAccessUsers(providerID uint, page, limit int) ([]ProviderAccessUser, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderAccessUser{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []ProviderAccessUser
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *Repository) UpdateAccessUser(user *ProviderAccessUser) error {
	return r.db.Save(user).Error
}

func (r *Repository) DeleteAccessUser(id uint) error {
	result := r.db.Delete(&ProviderAccessUser{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) UpdateAccessUserPermissions(id uint, permissions []byte) error {
	return r.db.Model(&ProviderAccessUser{}).Where("id = ?", id).Update("permissions", permissions).Error
}

func (r *Repository) GetDashboardDetails(providerID uint) ([]ScholarshipStat, error) {
	var stats []ScholarshipStat

	err := r.db.Model(&ProviderScholarship{}).
		Select("id, title, status, (SELECT COUNT(*) FROM provider_applications WHERE provider_applications.scholarship_id = provider_scholarships.id) AS applications").
		Where("provider_id = ?", providerID).
		Scan(&stats).Error

	if err != nil {
		return nil, err
	}

	return stats, nil
}
func (r *Repository) IsEmailTaken(email string) (bool, error) {
	var count int64
	// Check in main providers
	if err := r.db.Model(&ScholarshipProviderUser{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	// Check in sub users
	if err := r.db.Model(&ProviderAccessUser{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) UpdateProviderEmail(id uint, newEmail string) error {
	return r.db.Model(&ScholarshipProviderUser{}).Where("id = ?", id).Update("email", newEmail).Error
}

func (r *Repository) UpdateAccessUserEmail(id uint, newEmail string) error {
	return r.db.Model(&ProviderAccessUser{}).Where("id = ?", id).Update("email", newEmail).Error
}

func (r *Repository) UpdateAccessUserField(id uint, field string, value interface{}) error {
	return r.db.Model(&ProviderAccessUser{}).Where("id = ?", id).Update(field, value).Error
}

func (r *Repository) UpdateApplicationStatusOnly(id uint, status string) (*ProviderApplication, error) {
	var app ProviderApplication
	if err := r.db.Model(&app).Where("id = ?", id).Update("status", status).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) FindPaymentByApplicationID(applicationID uint) (*scholarship.Payment, error) {
	var payment scholarship.Payment
	if err := r.db.Where("application_id = ?", applicationID).Order("created_at desc").First(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *Repository) UpdatePayment(payment *scholarship.Payment) error {
	return r.db.Save(payment).Error
}

func (r *Repository) FindScholarshipByID(id uint) (*scholarship.Scholarship, error) {
	var s scholarship.Scholarship
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// ─── Provider Profile (Public) ───────────────────────────────────
func (r *Repository) GetProviderByID(id uint) (*ScholarshipProviderUser, error) {
	var provider ScholarshipProviderUser
	if err := r.db.First(&provider, id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *Repository) CountProviderContent(providerID uint) (scholarships, news, events, blogs int64, err error) {
	r.db.Model(&ProviderScholarship{}).Where("provider_id = ?", providerID).Count(&scholarships)
	r.db.Model(&ProviderNews{}).Where("provider_id = ?", providerID).Count(&news)
	r.db.Model(&ProviderEvent{}).Where("provider_id = ?", providerID).Count(&events)
	r.db.Model(&ProviderBlog{}).Where("provider_id = ?", providerID).Count(&blogs)
	return
}

func (r *Repository) CountPublishedProviderContent(providerID uint) (scholarships, news, events, blogs int64, err error) {
	r.db.Model(&ProviderScholarship{}).Where("provider_id = ? AND status = ?", providerID, "published").Count(&scholarships)
	r.db.Model(&ProviderNews{}).Where("provider_id = ? AND status = ?", providerID, "published").Count(&news)
	r.db.Model(&ProviderEvent{}).Where("provider_id = ? AND status = ?", providerID, "published").Count(&events)
	r.db.Model(&ProviderBlog{}).Where("provider_id = ? AND status = ?", providerID, "published").Count(&blogs)
	return
}


// ─── Services ────────────────────────────────────────────────────
func (r *Repository) GetServicesByProvider(providerID uint) ([]ProviderService, error) {
	var items []ProviderService
	if err := r.db.Where("provider_id = ?", providerID).Order("sort_order asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetServiceByIDAndProvider(id, providerID uint) (*ProviderService, error) {
	var item ProviderService
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateService(item *ProviderService) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateService(item *ProviderService, updates map[string]interface{}) error {
	return r.db.Model(item).Updates(updates).Error
}

func (r *Repository) DeleteService(id, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderService{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ─── Sectors ─────────────────────────────────────────────────────
func (r *Repository) GetSectorsByProvider(providerID uint) ([]ProviderSector, error) {
	var items []ProviderSector
	if err := r.db.Where("provider_id = ?", providerID).Order("sort_order asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetSectorByIDAndProvider(id, providerID uint) (*ProviderSector, error) {
	var item ProviderSector
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateSector(item *ProviderSector) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateSector(item *ProviderSector, updates map[string]interface{}) error {
	return r.db.Model(item).Updates(updates).Error
}

func (r *Repository) DeleteSector(id, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderSector{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ─── Projects ────────────────────────────────────────────────────
func (r *Repository) GetProjectsByProvider(providerID uint) ([]ProviderProject, error) {
	var items []ProviderProject
	if err := r.db.Where("provider_id = ?", providerID).Order("sort_order asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetProjectByIDAndProvider(id, providerID uint) (*ProviderProject, error) {
	var item ProviderProject
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateProject(item *ProviderProject) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateProject(item *ProviderProject, updates map[string]interface{}) error {
	return r.db.Model(item).Updates(updates).Error
}

func (r *Repository) DeleteProject(id, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderProject{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ─── Gallery Images ──────────────────────────────────────────────
func (r *Repository) GetGalleryImagesByProvider(providerID uint) ([]ProviderGalleryImage, error) {
	var items []ProviderGalleryImage
	if err := r.db.Where("provider_id = ?", providerID).Order("sort_order asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetGalleryImageByIDAndProvider(id, providerID uint) (*ProviderGalleryImage, error) {
	var item ProviderGalleryImage
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateGalleryImage(item *ProviderGalleryImage) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateGalleryImage(item *ProviderGalleryImage, updates map[string]interface{}) error {
	return r.db.Model(item).Updates(updates).Error
}

func (r *Repository) DeleteGalleryImage(id, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderGalleryImage{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ─── Reviews ─────────────────────────────────────────────────────
func (r *Repository) GetReviewsByProvider(providerID uint) ([]ProviderReview, error) {
	var items []ProviderReview
	if err := r.db.Where("provider_id = ?", providerID).Order("created_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetReviewByIDAndProvider(id, providerID uint) (*ProviderReview, error) {
	var item ProviderReview
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateReview(item *ProviderReview) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateReview(item *ProviderReview, updates map[string]interface{}) error {
	return r.db.Model(item).Updates(updates).Error
}

func (r *Repository) DeleteReview(id, providerID uint) error {
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderReview{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

type userRow struct {
	ID        uint   `gorm:"column:id"`
	FirstName string `gorm:"column:first_name"`
	LastName  string `gorm:"column:last_name"`
	Email     string `gorm:"column:email"`
	Phone     string `gorm:"column:phone"`
	Gender    string `gorm:"column:gender"`
	Address   string `gorm:"column:address"`
	Bio       string `gorm:"column:bio"`
	Role      string `gorm:"column:role"`
}

func (r *Repository) GetUserByID(userID uint) (*userRow, error) {
	var user userRow
	if err := r.db.Table("users").First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetPublishedReviews(providerID uint) ([]ProviderReview, error) {
	var reviews []ProviderReview
	if err := r.db.Where("provider_id = ? AND status = ?", providerID, "published").
		Order("created_at desc").Find(&reviews).Error; err != nil {
		return nil, err
	}
	return reviews, nil
}

// ─── Volunteer CRUD ─────────────────────────────────────────────────

func (r *Repository) CreateVolunteer(v *ProviderVolunteer) error {
	return r.db.Create(v).Error
}

func (r *Repository) GetVolunteersByProvider(providerID uint, page, limit int) ([]ProviderVolunteer, int64, error) {
	var total int64
	if err := r.db.Model(&ProviderVolunteer{}).Where("provider_id = ?", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var volunteers []ProviderVolunteer
	offset := (page - 1) * limit
	if err := r.db.Where("provider_id = ?", providerID).Order("created_at desc").Offset(offset).Limit(limit).Find(&volunteers).Error; err != nil {
		return nil, 0, err
	}
	for i := range volunteers {
		var count int64
		if cnt := r.db.Model(&VolunteerApplication{}).Where("volunteer_id = ?", volunteers[i].ID).Count(&count); cnt.Error != nil {
			count = 0
		}
		volunteers[i].ApplicantCount = count
	}
	return volunteers, total, nil
}

func (r *Repository) GetVolunteerByID(id uint) (*ProviderVolunteer, error) {
	var v ProviderVolunteer
	if err := r.db.First(&v, id).Error; err != nil {
		return nil, err
	}
	var count int64
	if cnt := r.db.Model(&VolunteerApplication{}).Where("volunteer_id = ?", v.ID).Count(&count); cnt.Error != nil {
		count = 0
	}
	v.ApplicantCount = count
	return &v, nil
}

func (r *Repository) GetVolunteerBySlug(slug string) (*ProviderVolunteer, error) {
	var v ProviderVolunteer
	if err := r.db.Where("slug = ?", slug).First(&v).Error; err != nil {
		return nil, err
	}
	var count int64
	if cnt := r.db.Model(&VolunteerApplication{}).Where("volunteer_id = ?", v.ID).Count(&count); cnt.Error != nil {
		count = 0
	}
	v.ApplicantCount = count
	return &v, nil
}

func (r *Repository) GetVolunteerByIDAndProvider(id, providerID uint) (*ProviderVolunteer, error) {
	var v ProviderVolunteer
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *Repository) UpdateVolunteer(v *ProviderVolunteer, updates map[string]interface{}) error {
	return r.db.Model(v).Updates(updates).Error
}

func (r *Repository) DeleteVolunteer(id, providerID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("volunteer_id = ?", id).Delete(&VolunteerApplication{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderVolunteer{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *Repository) ToggleVolunteerActive(id, providerID uint) (*ProviderVolunteer, error) {
	var v ProviderVolunteer
	if err := r.db.Where("id = ? AND provider_id = ?", id, providerID).First(&v).Error; err != nil {
		return nil, err
	}
	v.Active = !v.Active
	if err := r.db.Model(&v).Update("active", v.Active).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *Repository) GetPublicVolunteers(page, limit int, search, volunteerType string) ([]ProviderVolunteer, int64, error) {
	var total int64
	today := time.Now().Format("2006-01-02")
	query := r.db.Model(&ProviderVolunteer{}).Where("active = ?", true).
		Where("application_deadline >= ? OR application_deadline = '' OR application_deadline IS NULL", today)
	if search != "" {
		q := "%" + search + "%"
		query = query.Where("title ILIKE ? OR description ILIKE ?", q, q)
	}
	if volunteerType != "" {
		query = query.Where("volunteer_type = ?", volunteerType)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var volunteers []ProviderVolunteer
	offset := (page - 1) * limit
	dataQuery := r.db.Where("active = ?", true).
		Where("application_deadline >= ? OR application_deadline = '' OR application_deadline IS NULL", today)
	if search != "" {
		q := "%" + search + "%"
		dataQuery = dataQuery.Where("title ILIKE ? OR description ILIKE ?", q, q)
	}
	if volunteerType != "" {
		dataQuery = dataQuery.Where("volunteer_type = ?", volunteerType)
	}
	if err := dataQuery.Order("created_at desc").Offset(offset).Limit(limit).Find(&volunteers).Error; err != nil {
		return nil, 0, err
	}
	for i := range volunteers {
		var count int64
		if cnt := r.db.Model(&VolunteerApplication{}).Where("volunteer_id = ?", volunteers[i].ID).Count(&count); cnt.Error != nil {
			count = 0
		}
		volunteers[i].ApplicantCount = count
	}
	return volunteers, total, nil
}

// ─── Volunteer Application CRUD ─────────────────────────────────────

func (r *Repository) CreateVolunteerApplication(a *VolunteerApplication) error {
	return r.db.Create(a).Error
}

func (r *Repository) GetVolunteerApplicationsByProvider(providerID uint, volunteerID *uint, page, limit int, status *string) ([]VolunteerApplication, int64, error) {
	query := r.db.Model(&VolunteerApplication{}).
		Joins("JOIN provider_volunteers ON provider_volunteers.id = volunteer_applications.volunteer_id").
		Where("provider_volunteers.provider_id = ?", providerID)

	if volunteerID != nil && *volunteerID > 0 {
		query = query.Where("volunteer_applications.volunteer_id = ?", *volunteerID)
	}

	if status != nil && *status != "" {
		query = query.Where("volunteer_applications.status = ?", *status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var apps []VolunteerApplication
	offset := (page - 1) * limit
	if err := query.Order("volunteer_applications.created_at desc").Offset(offset).Limit(limit).Find(&apps).Error; err != nil {
		return nil, 0, err
	}
	return apps, total, nil
}

func (r *Repository) GetVolunteerApplicationByID(id uint) (*VolunteerApplication, error) {
	var a VolunteerApplication
	if err := r.db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) UpdateVolunteerApplicationStatus(id uint, status string) error {
	return r.db.Model(&VolunteerApplication{}).Where("id = ?", id).Update("status", status).Error
}

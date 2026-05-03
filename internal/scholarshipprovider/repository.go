package scholarshipprovider

import (
	"studsphere/backend/internal/scholarship"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
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
	return r.db.Where("title = ? AND provider = ?", title, provider).Delete(&scholarship.Scholarship{}).Error
}

func (r *Repository) DeletePublicScholarshipByProviderScholarshipID(providerScholarshipID uint) error {
	return r.db.Where("provider_scholarship_id = ?", providerScholarshipID).Delete(&scholarship.Scholarship{}).Error
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

	return applications, total, nil
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

	var payment ProviderPayment
	err := r.db.Table("scholarship_payments").
		Select("scholarship_payments.*").
		Joins("JOIN scholarships ON scholarships.id = scholarship_payments.scholarship_id").
		Where("scholarship_payments.user_id = ? AND scholarships.provider_scholarship_id = ?", application.UserID, application.ScholarshipID).
		Order("scholarship_payments.created_at desc").
		First(&payment).Error
	
	if err == nil {
		application.Payment = &payment
	}

	return &application, nil
}

func (r *Repository) EvaluateApplication(application *ProviderApplication, score int, passing bool, notes string) error {
	return r.db.Model(application).Updates(map[string]interface{}{
		"evaluation_score":  score,
		"evaluation_passed": passing,
		"evaluation_notes":  notes,
	}).Error
}

func (r *Repository) UpdateApplicationStatus(application *ProviderApplication, status string) error {
	return r.db.Model(application).Update("status", status).Error
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

func (r *Repository) CreateAccessUser(user *ProviderAccessUser) error {
	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}
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
	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}
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

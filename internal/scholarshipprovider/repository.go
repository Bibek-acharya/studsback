package scholarshipprovider

import "gorm.io/gorm"

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
	result := r.db.Where("id = ? AND provider_id = ?", id, providerID).Delete(&ProviderScholarship{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
	return &application, nil
}

func (r *Repository) EvaluateApplication(application *ProviderApplication, notes string) error {
	return r.db.Model(application).Updates(map[string]interface{}{
		"evaluation_notes": notes,
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

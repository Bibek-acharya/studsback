package system

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

func (r *Repository) CreateContactInquiry(inquiry *ContactInquiry) error {
	return r.db.Create(inquiry).Error
}

func (r *Repository) FindContactInquiries(page, limit int, status, inquiryType string) ([]ContactInquiry, int64, error) {
	var inquiries []ContactInquiry
	var total int64

	query := r.db.Model(&ContactInquiry{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if inquiryType != "" {
		query = query.Where("type = ?", inquiryType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&inquiries).Error; err != nil {
		return nil, 0, err
	}

	return inquiries, total, nil
}

func (r *Repository) FindContactInquiryByID(id uint) (*ContactInquiry, error) {
	var inquiry ContactInquiry
	err := r.db.First(&inquiry, id).Error
	if err != nil {
		return nil, err
	}
	return &inquiry, nil
}

func (r *Repository) UpdateContactInquiryStatus(id uint, status string) (*ContactInquiry, error) {
	inquiry, err := r.FindContactInquiryByID(id)
	if err != nil {
		return nil, err
	}
	if err := r.db.Model(inquiry).Update("status", status).Error; err != nil {
		return nil, err
	}
	return r.FindContactInquiryByID(id)
}

func (r *Repository) DeleteContactInquiry(id uint) error {
	result := r.db.Delete(&ContactInquiry{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindAds(page, limit int, pageFilter string, active *bool) ([]Ad, int64, error) {
	var ads []Ad
	var total int64

	query := r.db.Model(&Ad{})
	if pageFilter != "" {
		query = query.Where("page = ?", pageFilter)
	}
	if active != nil {
		query = query.Where("active = ?", *active)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("priority desc, created_at desc").Offset(offset).Limit(limit).Find(&ads).Error; err != nil {
		return nil, 0, err
	}

	return ads, total, nil
}

func (r *Repository) FindActiveAds(page string) ([]Ad, error) {
	var ads []Ad
	now := time.Now()

	query := r.db.Model(&Ad{}).
		Where("active = ? AND start_date <= ? AND (end_date IS NULL OR end_date >= ?)", true, now, now)

	if page != "" {
		query = query.Where("page = ?", page)
	}

	if err := query.Order("priority desc, created_at desc").Find(&ads).Error; err != nil {
		return nil, err
	}

	if len(ads) > 0 {
		ids := make([]uint, len(ads))
		for i, ad := range ads {
			ids[i] = ad.ID
		}
		r.db.Model(&Ad{}).Where("id IN ?", ids).Update("impressions", gorm.Expr("impressions + 1"))
	}

	return ads, nil
}

func (r *Repository) FindAdByID(id uint) (*Ad, error) {
	var ad Ad
	err := r.db.First(&ad, id).Error
	if err != nil {
		return nil, err
	}
	return &ad, nil
}

func (r *Repository) CreateAd(ad *Ad) error {
	return r.db.Create(ad).Error
}

func (r *Repository) UpdateAd(id uint, updates map[string]interface{}) (*Ad, error) {
	ad, err := r.FindAdByID(id)
	if err != nil {
		return nil, err
	}
	if err := r.db.Model(ad).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.FindAdByID(id)
}

func (r *Repository) DeleteAd(id uint) error {
	result := r.db.Delete(&Ad{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) TrackAdClick(id uint) (*Ad, error) {
	ad, err := r.FindAdByID(id)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"clicks":      ad.Clicks + 1,
		"impressions": ad.Impressions + 1,
	}
	if err := r.db.Model(ad).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.FindAdByID(id)
}

func (r *Repository) FindCarouselSlides(page string, active *bool) ([]CarouselSlide, error) {
	var slides []CarouselSlide

	query := r.db.Model(&CarouselSlide{}).Where("page = ?", page)
	if active != nil {
		query = query.Where("active = ?", *active)
	}

	if err := query.Order("order asc, created_at desc").Find(&slides).Error; err != nil {
		return nil, err
	}

	return slides, nil
}

func (r *Repository) FindCarouselSlideByID(id uint) (*CarouselSlide, error) {
	var slide CarouselSlide
	err := r.db.First(&slide, id).Error
	if err != nil {
		return nil, err
	}
	return &slide, nil
}

func (r *Repository) CreateCarouselSlide(slide *CarouselSlide) error {
	return r.db.Create(slide).Error
}

func (r *Repository) UpdateCarouselSlide(id uint, updates map[string]interface{}) (*CarouselSlide, error) {
	slide, err := r.FindCarouselSlideByID(id)
	if err != nil {
		return nil, err
	}
	if err := r.db.Model(slide).Updates(updates).Error; err != nil {
		return nil, err
	}
	return r.FindCarouselSlideByID(id)
}

func (r *Repository) DeleteCarouselSlide(id uint) error {
	result := r.db.Delete(&CarouselSlide{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ReorderCarouselSlides(items []struct {
	ID    uint
	Order int
}) error {
	tx := r.db.Begin()
	for _, item := range items {
		if err := tx.Model(&CarouselSlide{}).Where("id = ?", item.ID).Update("order", item.Order).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (r *Repository) FindActivePublicNotifications() ([]PublicNotification, error) {
	var notifications []PublicNotification
	if err := r.db.Where("active = ?", true).
		Order("created_at desc").
		Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

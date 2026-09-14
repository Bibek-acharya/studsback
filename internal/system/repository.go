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

// UserIDByEmail resolves a registered user account from the inquiry email
// (owned by the auth module; raw SQL to avoid an import cycle). Guests
// resolve to 0 and skip the reply notification.
func (r *Repository) UserIDByEmail(email string) (uint, error) {
	var id uint
	err := r.db.Raw(`SELECT id FROM users WHERE email = ? AND deleted_at IS NULL LIMIT 1`, email).Scan(&id).Error
	return id, err
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

func (r *Repository) FindInstitutionInquiries(institutionID uint, page, limit int, status, inquiryType, search string) ([]ContactInquiry, int64, error) {
	var inquiries []ContactInquiry
	var total int64

	query := r.db.Model(&ContactInquiry{}).Where("institution_id = ?", institutionID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if inquiryType != "" {
		query = query.Where("type = ?", inquiryType)
	}
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
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

func (r *Repository) FindAds(page, limit int, pageFilter, positionFilter string, active *bool) ([]Ad, int64, error) {
	var ads []Ad
	var total int64

	query := r.db.Model(&Ad{})
	if pageFilter != "" {
		query = query.Where("page = ?", pageFilter)
	}
	if positionFilter != "" {
		query = query.Where("position = ?", positionFilter)
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

	r.resolveAdEntities(ads)
	return ads, total, nil
}

func (r *Repository) FindActiveAds(page, position string) ([]Ad, error) {
	var ads []Ad
	now := time.Now()
	var zeroTime time.Time

	query := r.db.Model(&Ad{}).
		Where("active = ?", true).
		Where("(start_date <= ? OR start_date = ?)", now, zeroTime).
		Where("(end_date >= ? OR end_date = ?)", now, zeroTime)

	if page != "" {
		query = query.Where("page = ?", page)
	}
	if position != "" {
		query = query.Where("position = ?", position)
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

	r.resolveAdEntities(ads)
	return ads, nil
}

func (r *Repository) FindAdByID(id uint) (*Ad, error) {
	var ad Ad
	err := r.db.First(&ad, id).Error
	if err != nil {
		return nil, err
	}
	r.resolveAdEntities([]Ad{ad})
	return &ad, nil
}

// resolveAdEntities batch-loads institution and course data for a slice of ads.
func (r *Repository) resolveAdEntities(ads []Ad) {
	if len(ads) == 0 {
		return
	}

	// Collect unique IDs
	collegeIDs := make(map[uint]bool)
	courseIDs := make(map[uint]bool)
	for i := range ads {
		if ads[i].CollegeID != nil {
			collegeIDs[*ads[i].CollegeID] = true
		}
		if ads[i].CourseID != nil {
			courseIDs[*ads[i].CourseID] = true
		}
	}

	// Batch-load institutions (entity picker uses institution_users IDs)
	collegeMap := make(map[uint]*AdCollege)
	if len(collegeIDs) > 0 {
		ids := make([]uint, 0, len(collegeIDs))
		for id := range collegeIDs {
			ids = append(ids, id)
		}
		type instRow struct {
			ID         uint    `gorm:"column:id"`
			Name       string  `gorm:"column:institution_name"`
			ImageURL   string  `gorm:"column:banner_url"`
			Location   string  `gorm:"column:district"`
			WebsiteURL string  `gorm:"column:website_url"`
			CollegeID  uint    `gorm:"column:college_id"`
		}
		var instRows []instRow
		r.db.Table("institution_users").Select("id, institution_name, banner_url, district, website_url, college_id").Where("id IN ? AND deleted_at IS NULL", ids).Find(&instRows)

		// Collect college_ids from institution_users to fetch rating from colleges table
		collegeTableIDs := make(map[uint]uint) // institution_users.id -> colleges.id
		for _, row := range instRows {
			if row.CollegeID > 0 {
				collegeTableIDs[row.ID] = row.CollegeID
			}
		}
		ratingMap := make(map[uint]float64)
		if len(collegeTableIDs) > 0 {
			cids := make([]uint, 0, len(collegeTableIDs))
			for _, cid := range collegeTableIDs {
				cids = append(cids, cid)
			}
			var cRows []struct {
				ID     uint    `gorm:"column:id"`
				Rating float64 `gorm:"column:rating"`
			}
			r.db.Table("colleges").Select("id, rating").Where("id IN ?", cids).Find(&cRows)
			for _, cr := range cRows {
				ratingMap[cr.ID] = cr.Rating
			}
		}

		for _, row := range instRows {
			var rating float64
			if cid, ok := collegeTableIDs[row.ID]; ok {
				rating = ratingMap[cid]
			}
			collegeMap[row.ID] = &AdCollege{
				ID:       row.ID,
				Name:     row.Name,
				ImageURL: row.ImageURL,
				Location: row.Location,
				Website:  row.WebsiteURL,
				Rating:   rating,
			}
		}
	}

	// Batch-load courses
	courseMap := make(map[uint]*AdCourse)
	if len(courseIDs) > 0 {
		ids := make([]uint, 0, len(courseIDs))
		for id := range courseIDs {
			ids = append(ids, id)
		}
		var rows []struct {
			ID         uint   `gorm:"column:id"`
			Title      string `gorm:"column:title"`
			Level      string `gorm:"column:level"`
			Duration   string `gorm:"column:duration"`
			FieldStudy string `gorm:"column:field_of_study"`
			BannerURL  string `gorm:"column:banner_url"`
		}
		r.db.Table("courses").Select("id, title, level, duration, field_of_study, banner_url").Where("id IN ?", ids).Find(&rows)
		for _, row := range rows {
			courseMap[row.ID] = &AdCourse{
				ID:         row.ID,
				Title:      row.Title,
				Level:      row.Level,
				Duration:   row.Duration,
				FieldStudy: row.FieldStudy,
				BannerURL:  row.BannerURL,
			}
		}
	}

	// Attach resolved entities
	for i := range ads {
		if ads[i].CollegeID != nil {
			ads[i].College = collegeMap[*ads[i].CollegeID]
		}
		if ads[i].CourseID != nil {
			ads[i].Course = courseMap[*ads[i].CourseID]
		}
	}
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

	if err := query.Order(`"order" asc, created_at desc`).Find(&slides).Error; err != nil {
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

func (r *Repository) CreatePublicNotification(n *PublicNotification) error {
	return r.db.Create(n).Error
}

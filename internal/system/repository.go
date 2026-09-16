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
		type courseRow struct {
			ID            uint   `gorm:"column:id"`
			Title         string `gorm:"column:title"`
			Level         string `gorm:"column:level"`
			Duration      string `gorm:"column:duration"`
			FieldStudy    string `gorm:"column:field_of_study"`
			BannerURL     string `gorm:"column:banner_url"`
			EstFee        string `gorm:"column:est_fee"`
			AffiliationID *uint  `gorm:"column:affiliation_id"`
		}
		var rows []courseRow
		r.db.Table("courses").Select("id, title, level, duration, field_of_study, banner_url, est_fee, affiliation_id").Where("id IN ?", ids).Find(&rows)

		// Resolve university names for affiliation
		uniIDs := make(map[uint]bool)
		for _, row := range rows {
			if row.AffiliationID != nil && *row.AffiliationID > 0 {
				uniIDs[*row.AffiliationID] = true
			}
		}
		uniMap := make(map[uint]string)
		if len(uniIDs) > 0 {
			uids := make([]uint, 0, len(uniIDs))
			for id := range uniIDs {
				uids = append(uids, id)
			}
			var uRows []struct {
				ID   uint   `gorm:"column:id"`
				Name string `gorm:"column:name"`
			}
			r.db.Table("universities").Select("id, name").Where("id IN ?", uids).Find(&uRows)
			for _, ur := range uRows {
				uniMap[ur.ID] = ur.Name
			}
		}

		for _, row := range rows {
			affiliation := ""
			if row.AffiliationID != nil && *row.AffiliationID > 0 {
				affiliation = uniMap[*row.AffiliationID]
			}
			courseMap[row.ID] = &AdCourse{
				ID:          row.ID,
				Title:       row.Title,
				Level:       row.Level,
				Duration:    row.Duration,
				FieldStudy:  row.FieldStudy,
				BannerURL:   row.BannerURL,
				EstFee:      row.EstFee,
				Affiliation: affiliation,
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

// Landing Course methods

func (r *Repository) FindActiveLandingFields() ([]LandingCourseField, error) {
	var fields []LandingCourseField
	err := r.db.Where("is_active = true").Order("display_order ASC, id ASC").Find(&fields).Error
	return fields, err
}

func (r *Repository) FindAllLandingFields() ([]LandingCourseField, error) {
	var fields []LandingCourseField
	err := r.db.Order("display_order ASC, id ASC").Find(&fields).Error
	return fields, err
}

func (r *Repository) FindLandingFieldByID(id uint) (*LandingCourseField, error) {
	var field LandingCourseField
	err := r.db.First(&field, id).Error
	return &field, err
}

func (r *Repository) UpdateLandingField(id uint, updates map[string]interface{}) error {
	return r.db.Model(&LandingCourseField{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) ReorderLandingFields(items []FieldReorderItem) error {
	tx := r.db.Begin()
	for _, item := range items {
		if err := tx.Model(&LandingCourseField{}).Where("id = ?", item.ID).Update("display_order", item.DisplayOrder).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (r *Repository) FindInstitutionsByFieldID(fieldID uint) ([]LandingCourseInstitution, error) {
	var institutions []LandingCourseInstitution
	err := r.db.Where("field_id = ?", fieldID).Order("order_index ASC, id ASC").Find(&institutions).Error
	return institutions, err
}

func (r *Repository) FindInstitutionsByFieldIDs(fieldIDs []uint) (map[uint][]LandingCourseInstitution, error) {
	var institutions []LandingCourseInstitution
	err := r.db.Where("field_id IN ?", fieldIDs).Order("order_index ASC, id ASC").Find(&institutions).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint][]LandingCourseInstitution)
	for _, inst := range institutions {
		result[inst.FieldID] = append(result[inst.FieldID], inst)
	}
	return result, nil
}

func (r *Repository) CreateLandingInstitution(inst *LandingCourseInstitution) error {
	return r.db.Create(inst).Error
}

func (r *Repository) DeleteLandingInstitution(id uint) error {
	return r.db.Delete(&LandingCourseInstitution{}, id).Error
}

func (r *Repository) ReorderLandingInstitutions(items []InstitutionReorderItem) error {
	tx := r.db.Begin()
	for _, item := range items {
		if err := tx.Model(&LandingCourseInstitution{}).Where("id = ?", item.ID).Update("order_index", item.OrderIndex).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (r *Repository) CountInstitutionsByField(fieldID uint) (int64, error) {
	var count int64
	err := r.db.Model(&LandingCourseInstitution{}).Where("field_id = ?", fieldID).Count(&count).Error
	return count, err
}

func (r *Repository) InstitutionLinkExists(fieldID, institutionID uint) (bool, error) {
	var count int64
	err := r.db.Model(&LandingCourseInstitution{}).Where("field_id = ? AND institution_id = ?", fieldID, institutionID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) SearchInstitutions(query string) ([]InstitutionSearchResult, error) {
	like := "%" + query + "%"

	// Search institution_users
	var instResults []InstitutionSearchResult
	err := r.db.Raw(`
		SELECT id, institution_name as name, logo_url, 'institution' as type,
		       LOWER(REPLACE(institution_name, ' ', '-')) as slug,
		       district as location
		FROM institution_users
		WHERE deleted_at IS NULL
		AND (institution_name ILIKE ? OR district ILIKE ?)
		AND status = 'approved'
		LIMIT 10
	`, like, like).Scan(&instResults).Error
	if err != nil {
		return nil, err
	}

	// Search legacy colleges
	var collegeResults []InstitutionSearchResult
	err = r.db.Raw(`
		SELECT id, name,
		       COALESCE(image_url, '') as logo_url,
		       'college' as type,
		       LOWER(REPLACE(name, ' ', '-')) as slug,
		       COALESCE(location, '') as location
		FROM colleges
		WHERE name ILIKE ?
		LIMIT 10
	`, like).Scan(&collegeResults).Error
	if err != nil {
		return nil, err
	}

	// Merge and limit to 10 total
	results := append(instResults, collegeResults...)
	if len(results) > 10 {
		results = results[:10]
	}
	return results, nil
}

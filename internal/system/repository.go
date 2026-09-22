package system

import (
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
			ID         uint   `gorm:"column:id"`
			Name       string `gorm:"column:institution_name"`
			ImageURL   string `gorm:"column:banner_url"`
			Location   string `gorm:"column:district"`
			WebsiteURL string `gorm:"column:website_url"`
			CollegeID  uint   `gorm:"column:college_id"`
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

	if err := query.Order(`"order" asc, created_at asc, id asc`).Find(&slides).Error; err != nil {
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

// MaxCarouselSlideOrder returns the highest slide order for a page (0 when none exist).
func (r *Repository) MaxCarouselSlideOrder(page string) (int, error) {
	var maxOrder int
	err := r.db.Model(&CarouselSlide{}).Where("page = ?", page).
		Select(`COALESCE(MAX("order"), 0)`).Scan(&maxOrder).Error
	return maxOrder, err
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

func (r *Repository) InstitutionLinkExists(fieldID, institutionID uint, institutionType string) (bool, error) {
	var count int64
	err := r.db.Model(&LandingCourseInstitution{}).
		Where("field_id = ? AND institution_id = ? AND institution_type = ?", fieldID, institutionID, institutionType).
		Count(&count).Error
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

// Course-finder ad card methods

func (r *Repository) CountCourseAdCards(position string) (int64, error) {
	var count int64
	err := r.db.Model(&CourseAdCard{}).Where("position = ?", position).Count(&count).Error
	return count, err
}

func (r *Repository) CreateCourseAdCard(card *CourseAdCard, institutions []CourseAdCardInstitution, mous []CourseAdCardMouCompany) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(card).Error; err != nil {
			return err
		}
		for i := range institutions {
			institutions[i].CardID = card.ID
			if err := tx.Create(&institutions[i]).Error; err != nil {
				return err
			}
		}
		for i := range mous {
			mous[i].CardID = card.ID
			if err := tx.Create(&mous[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// FindCourseAdCards lists cards for a position, children included. limit <= 0 means no cap.
func (r *Repository) FindCourseAdCards(position string, activeOnly bool, limit int) ([]CourseAdCard, error) {
	var cards []CourseAdCard
	query := r.db.Model(&CourseAdCard{}).Where("position = ?", position)
	if activeOnly {
		query = query.Where("active = ?", true)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Order("priority desc, id desc").Find(&cards).Error; err != nil {
		return nil, err
	}
	if err := r.loadCourseAdChildren(cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func (r *Repository) FindCourseAdCardByID(id uint) (*CourseAdCard, error) {
	var card CourseAdCard
	if err := r.db.First(&card, id).Error; err != nil {
		return nil, err
	}
	cards := []CourseAdCard{card}
	if err := r.loadCourseAdChildren(cards); err != nil {
		return nil, err
	}
	return &cards[0], nil
}

// loadCourseAdChildren batch-loads institution links and MOU companies for the cards.
func (r *Repository) loadCourseAdChildren(cards []CourseAdCard) error {
	if len(cards) == 0 {
		return nil
	}
	ids := make([]uint, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}

	var links []CourseAdCardInstitution
	if err := r.db.Where("card_id IN ?", ids).Order("order_index asc, id asc").Find(&links).Error; err != nil {
		return err
	}
	linksByCard := make(map[uint][]CourseAdCardInstitution)
	for _, l := range links {
		linksByCard[l.CardID] = append(linksByCard[l.CardID], l)
	}

	var mous []CourseAdCardMouCompany
	if err := r.db.Where("card_id IN ?", ids).Order("id asc").Find(&mous).Error; err != nil {
		return err
	}
	mousByCard := make(map[uint][]CourseAdCardMouCompany)
	for _, m := range mous {
		mousByCard[m.CardID] = append(mousByCard[m.CardID], m)
	}

	for i := range cards {
		cards[i].Institutions = linksByCard[cards[i].ID]
		if cards[i].Institutions == nil {
			cards[i].Institutions = []CourseAdCardInstitution{}
		}
		cards[i].MouCompanies = mousByCard[cards[i].ID]
		if cards[i].MouCompanies == nil {
			cards[i].MouCompanies = []CourseAdCardMouCompany{}
		}
	}
	return nil
}

// UpdateCourseAdCard applies column updates and replaces children lists when provided.
func (r *Repository) UpdateCourseAdCard(
	id uint,
	updates map[string]interface{},
	institutions *[]CourseAdCardInstitution,
	mous *[]CourseAdCardMouCompany,
) (*CourseAdCard, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&CourseAdCard{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		if institutions != nil {
			if err := tx.Where("card_id = ?", id).Delete(&CourseAdCardInstitution{}).Error; err != nil {
				return err
			}
			for i := range *institutions {
				(*institutions)[i].CardID = id
				if err := tx.Create(&(*institutions)[i]).Error; err != nil {
					return err
				}
			}
		}
		if mous != nil {
			if err := tx.Where("card_id = ?", id).Delete(&CourseAdCardMouCompany{}).Error; err != nil {
				return err
			}
			for i := range *mous {
				(*mous)[i].CardID = id
				if err := tx.Create(&(*mous)[i]).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindCourseAdCardByID(id)
}

func (r *Repository) DeleteCourseAdCard(id uint) error {
	result := r.db.Delete(&CourseAdCard{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) TrackCourseAdCardClick(id uint) error {
	result := r.db.Model(&CourseAdCard{}).Where("id = ?", id).
		Update("clicks", gorm.Expr("clicks + 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Advertise request methods

func (r *Repository) CreateAdvertiseRequest(req *AdvertiseRequest) error {
	return r.db.Create(req).Error
}

func (r *Repository) FindAdvertiseRequests(page, limit int, status string) ([]AdvertiseRequest, int64, error) {
	var requests []AdvertiseRequest
	var total int64

	query := r.db.Model(&AdvertiseRequest{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at desc, id desc").Offset(offset).Limit(limit).Find(&requests).Error; err != nil {
		return nil, 0, err
	}

	return requests, total, nil
}

func (r *Repository) FindAdvertiseRequestsByInstitution(institutionID uint) ([]AdvertiseRequest, error) {
	var requests []AdvertiseRequest
	err := r.db.Where("institution_id = ?", institutionID).
		Order("created_at desc, id desc").Find(&requests).Error
	return requests, err
}

func (r *Repository) FindAdvertiseRequestByID(id uint) (*AdvertiseRequest, error) {
	var req AdvertiseRequest
	if err := r.db.First(&req, id).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *Repository) UpdateAdvertiseRequestStatus(id uint, status string, note *string) (*AdvertiseRequest, error) {
	updates := map[string]interface{}{"status": status}
	if note != nil {
		updates["note"] = *note
	}
	result := r.db.Model(&AdvertiseRequest{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return r.FindAdvertiseRequestByID(id)
}

// InstitutionUserEmail resolves the advertiser's account email as the
// default contact email (owned by auth; raw SQL avoids an import cycle).
func (r *Repository) InstitutionUserEmail(id uint) (string, error) {
	var email string
	err := r.db.Raw(`SELECT email FROM institution_users WHERE id = ? AND deleted_at IS NULL LIMIT 1`, id).Scan(&email).Error
	return email, err
}

// ResolveCourseAdEntities batch-loads joined course and institution data for cards.
// Institution rating comes from the colleges table via college_id, mirroring
// resolveAdEntities; slug is derived from the institution name, matching
// SearchInstitutions.
func (r *Repository) ResolveCourseAdEntities(cards []CourseAdCard) error {
	if len(cards) == 0 {
		return nil
	}

	// Card institution IDs: link children plus the single_college institution_id.
	cardInstIDs := make(map[uint]bool)
	for i := range cards {
		if cards[i].InstitutionID != nil {
			cardInstIDs[*cards[i].InstitutionID] = true
		}
		for _, l := range cards[i].Institutions {
			cardInstIDs[l.InstitutionID] = true
		}
	}

	instMap := make(map[uint]*CourseAdInstitution)
	if len(cardInstIDs) > 0 {
		ids := make([]uint, 0, len(cardInstIDs))
		for id := range cardInstIDs {
			ids = append(ids, id)
		}

		type instRow struct {
			ID         uint   `gorm:"column:id"`
			Name       string `gorm:"column:institution_name"`
			ImageURL   string `gorm:"column:banner_url"`
			Location   string `gorm:"column:district"`
			WebsiteURL string `gorm:"column:website_url"`
			CollegeID  uint   `gorm:"column:college_id"`
		}
		var instRows []instRow
		if err := r.db.Table("institution_users").
			Select("id, institution_name, banner_url, district, website_url, college_id").
			Where("id IN ? AND deleted_at IS NULL", ids).Find(&instRows).Error; err != nil {
			return err
		}

		collegeTableIDs := make(map[uint]uint)
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
			if err := r.db.Table("colleges").Select("id, rating").Where("id IN ?", cids).Find(&cRows).Error; err != nil {
				return err
			}
			for _, cr := range cRows {
				ratingMap[cr.ID] = cr.Rating
			}
		}

		for _, row := range instRows {
			var rating float64
			if cid, ok := collegeTableIDs[row.ID]; ok {
				rating = ratingMap[cid]
			}
			instMap[row.ID] = &CourseAdInstitution{
				ID:       row.ID,
				Name:     row.Name,
				ImageURL: row.ImageURL,
				Rating:   rating,
				Location: row.Location,
				Website:  row.WebsiteURL,
				Slug:     strings.ToLower(strings.ReplaceAll(strings.TrimSpace(row.Name), " ", "-")),
			}
		}
	}

	// Courses
	courseIDs := make(map[uint]bool)
	for i := range cards {
		courseIDs[cards[i].CourseID] = true
	}
	courseMap := make(map[uint]*CourseAdCourse)
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
			Location      string `gorm:"column:location"`
			Description   string `gorm:"column:description"`
			AffiliationID *uint  `gorm:"column:affiliation_id"`
		}
		var rows []courseRow
		if err := r.db.Table("courses").
			Select("id, title, level, duration, field_of_study, banner_url, est_fee, location, description, affiliation_id").
			Where("id IN ? AND deleted_at IS NULL", ids).Find(&rows).Error; err != nil {
			return err
		}

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
			if err := r.db.Table("universities").Select("id, name").Where("id IN ?", uids).Find(&uRows).Error; err != nil {
				return err
			}
			for _, ur := range uRows {
				uniMap[ur.ID] = ur.Name
			}
		}

		for _, row := range rows {
			affiliation := ""
			if row.AffiliationID != nil && *row.AffiliationID > 0 {
				affiliation = uniMap[*row.AffiliationID]
			}
			courseMap[row.ID] = &CourseAdCourse{
				ID:          row.ID,
				Title:       row.Title,
				Level:       row.Level,
				Duration:    row.Duration,
				FieldStudy:  row.FieldStudy,
				BannerURL:   row.BannerURL,
				EstFee:      row.EstFee,
				Affiliation: affiliation,
				Location:    row.Location,
				Description: row.Description,
			}
		}
	}

	// Attach resolved entities
	for i := range cards {
		cards[i].Course = courseMap[cards[i].CourseID]
		infos := make([]CourseAdInstitution, 0, len(cards[i].Institutions))
		for _, l := range cards[i].Institutions {
			if inst, ok := instMap[l.InstitutionID]; ok {
				infos = append(infos, *inst)
			}
		}
		// Single_college: the lone institution lives on the card itself.
		if cards[i].Position == "single_college" && cards[i].InstitutionID != nil {
			if inst, ok := instMap[*cards[i].InstitutionID]; ok && len(infos) == 0 {
				infos = append(infos, *inst)
			}
		}
		cards[i].InstitutionInf = infos
	}
	return nil
}

// College-finder page ads

func (r *Repository) CountCollegeAdTrending(kind string) (int64, error) {
	var count int64
	err := r.db.Model(&CollegeAdTrendingItem{}).Where("kind = ?", kind).Count(&count).Error
	return count, err
}

func (r *Repository) CreateCollegeAdTrendingItem(item *CollegeAdTrendingItem) error {
	return r.db.Create(item).Error
}

// FindCollegeAdTrendingItems lists items, ordered priority desc, id desc.
// kind filters to one kind when non-empty; activeOnly filters to active rows.
func (r *Repository) FindCollegeAdTrendingItems(kind string, activeOnly bool) ([]CollegeAdTrendingItem, error) {
	var items []CollegeAdTrendingItem
	query := r.db.Model(&CollegeAdTrendingItem{})
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if activeOnly {
		query = query.Where("active = ?", true)
	}
	if err := query.Order("priority desc, id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FindCollegeAdTrendingItemByID(id uint) (*CollegeAdTrendingItem, error) {
	var item CollegeAdTrendingItem
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) UpdateCollegeAdTrendingItem(id uint, updates map[string]interface{}) (*CollegeAdTrendingItem, error) {
	result := r.db.Model(&CollegeAdTrendingItem{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return r.FindCollegeAdTrendingItemByID(id)
}

func (r *Repository) DeleteCollegeAdTrendingItem(id uint) error {
	result := r.db.Delete(&CollegeAdTrendingItem{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ResolveCollegeAdTrending batch-loads institution data from institution_users
// (the IDs stored on items are institution ids, mirroring
// ResolveCourseAdEntities), with rating and type joined from colleges via
// college_id. Items whose institution row is missing resolve to a nil
// College.
func (r *Repository) ResolveCollegeAdTrending(items []CollegeAdTrendingItem) error {
	if len(items) == 0 {
		return nil
	}

	// Item IDs: institution_users ids.
	collegeIDs := make(map[uint]bool)
	for i := range items {
		collegeIDs[items[i].CollegeID] = true
	}
	ids := make([]uint, 0, len(collegeIDs))
	for id := range collegeIDs {
		ids = append(ids, id)
	}

	type instRow struct {
		ID        uint   `gorm:"column:id"`
		Name      string `gorm:"column:institution_name"`
		ImageURL  string `gorm:"column:logo_url"`
		Location  string `gorm:"column:district"`
		CollegeID uint   `gorm:"column:college_id"`
	}
	var instRows []instRow
	if err := r.db.Table("institution_users").
		Select("id, institution_name, logo_url, district, college_id").
		Where("id IN ? AND deleted_at IS NULL", ids).Find(&instRows).Error; err != nil {
		return err
	}

	// rating/type from colleges via college_id.
	collegeTableIDs := make(map[uint]uint)
	for _, row := range instRows {
		if row.CollegeID > 0 {
			collegeTableIDs[row.ID] = row.CollegeID
		}
	}
	type collegeInfo struct {
		Rating      float64 `gorm:"column:rating"`
		CollegeType string  `gorm:"column:college_type"`
	}
	collegeInfoMap := make(map[uint]collegeInfo)
	if len(collegeTableIDs) > 0 {
		cids := make([]uint, 0, len(collegeTableIDs))
		for _, cid := range collegeTableIDs {
			cids = append(cids, cid)
		}
		var cRows []struct {
			ID          uint    `gorm:"column:id"`
			Rating      float64 `gorm:"column:rating"`
			CollegeType string  `gorm:"column:college_type"`
		}
		if err := r.db.Table("colleges").
			Select("id, rating, college_type").
			Where("id IN ? AND deleted_at IS NULL", cids).Find(&cRows).Error; err != nil {
			return err
		}
		for _, cr := range cRows {
			collegeInfoMap[cr.ID] = collegeInfo{Rating: cr.Rating, CollegeType: cr.CollegeType}
		}
	}

	collegeMap := make(map[uint]*CollegeAdCollege, len(instRows))
	for _, row := range instRows {
		var rating float64
		var collegeType string
		if cid, ok := collegeTableIDs[row.ID]; ok {
			info := collegeInfoMap[cid]
			rating = info.Rating
			collegeType = info.CollegeType
		}
		collegeMap[row.ID] = &CollegeAdCollege{
			ID:       row.ID,
			Name:     row.Name,
			ImageURL: row.ImageURL,
			Rating:   rating,
			Location: row.Location,
			Type:     collegeType,
		}
	}

	for i := range items {
		items[i].College = collegeMap[items[i].CollegeID]
	}
	return nil
}

// College recommendation feedback

func (r *Repository) CreateCollegeRecommendationFeedback(fb *CollegeRecommendationFeedback) error {
	return r.db.Create(fb).Error
}

// FindCollegeRecommendationFeedback returns the latest `limit` feedback rows.
func (r *Repository) FindCollegeRecommendationFeedback(limit int) ([]CollegeRecommendationFeedback, error) {
	var feedback []CollegeRecommendationFeedback
	if err := r.db.Order("created_at desc, id desc").Limit(limit).Find(&feedback).Error; err != nil {
		return nil, err
	}
	return feedback, nil
}

// CountCollegeRecommendationFeedback returns total, helpful and not-helpful counts.
func (r *Repository) CountCollegeRecommendationFeedback() (total, helpful, notHelpful int64, err error) {
	if err = r.db.Model(&CollegeRecommendationFeedback{}).Count(&total).Error; err != nil {
		return 0, 0, 0, err
	}
	if err = r.db.Model(&CollegeRecommendationFeedback{}).Where("helpful = ?", true).Count(&helpful).Error; err != nil {
		return 0, 0, 0, err
	}
	return total, helpful, total - helpful, nil
}

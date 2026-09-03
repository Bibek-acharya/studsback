package college

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

var feeNumberRegex = regexp.MustCompile(`\d[\d,]{3,}`)

var facetKeywordsByID = map[string][]string{
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
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(filters CollegeFilters) ([]College, int64, error) {
	var colleges []College
	var total int64

	query := r.db.Model(&College{})

	if filters.Location != "" {
		query = applyMultiILIKEFilter(query, "location", parseCSV(filters.Location))
	}

	if filters.Affiliation != "" {
		query = applyMultiILIKEFilter(query, "affiliation", parseCSV(filters.Affiliation))
	}

	if filters.UniversityID != "" {
		query = query.Where("university_affiliations @> ?::jsonb", fmt.Sprintf("[%s]", filters.UniversityID))
	}

	if len(filters.Academic) > 0 {
		query = applyFacetSearchFilter(query, filters.Academic)
	}

	if len(filters.Program) > 0 {
		query = applyFacetSearchFilter(query, filters.Program)
	}

	if len(filters.Province) > 0 {
		query = applyMultiILIKEFilter(query, "location", filters.Province)
	}

	if len(filters.District) > 0 {
		query = applyMultiILIKEFilter(query, "location", filters.District)
	}

	if len(filters.Local) > 0 {
		query = applyMultiILIKEFilter(query, "location", filters.Local)
	}

	if len(filters.Scholarship) > 0 {
		query = applyMultiILIKEFilter(query, "CAST(scholarships AS TEXT)", filters.Scholarship)
	}

	if len(filters.Facilities) > 0 {
		query = applyMultiILIKEFilter(query, "CAST(amenities AS TEXT)", filters.Facilities)
	}

	if filters.DirectAdmission {
		query = applyMultiILIKEFilter(query, "CAST(admissions AS TEXT) || ' ' || CAST(admission_cards AS TEXT)", []string{"direct admission", "direct apply", "direct"})
	}

	if filters.Type != "" {
		types := parseTypes(filters.Type)
		if len(types) == 1 {
			query = query.Where("college_type = ?", types[0])
		} else if len(types) > 1 {
			query = query.Where("college_type IN ?", types)
		}
	}

	if filters.Verified == "true" {
		query = query.Where("verified = ?", true)
	}

	if filters.Popular == "true" {
		query = query.Where("popular = ?", true)
	}

	if filters.MinRating != "" {
		if rating, err := parseFloat(filters.MinRating); err == nil {
			query = query.Where("rating >= ?", rating)
		}
	}

	if filters.Search != "" {
		searchLike := "%" + filters.Search + "%"
		query = query.Where(
			"name ILIKE ? OR full_name ILIKE ? OR affiliation ILIKE ? OR location ILIKE ? OR CAST(featured_programs AS TEXT) ILIKE ? OR CAST(courses AS TEXT) ILIKE ? OR CAST(programs_list AS TEXT) ILIKE ?",
			searchLike, searchLike, searchLike, searchLike, searchLike, searchLike, searchLike,
		)
	}

	if filters.CourseID != "" {
		query = query.
			Joins("LEFT JOIN college_university_courses ON college_university_courses.college_id = colleges.id AND college_university_courses.course_id = ?", filters.CourseID).
			Joins("LEFT JOIN institution_users iu_cuc ON iu_cuc.college_id = colleges.id AND iu_cuc.deleted_at IS NULL").
			Joins("LEFT JOIN institution_programs ip_cuc ON ip_cuc.institution_id = iu_cuc.id AND ip_cuc.global_course_id = ? AND ip_cuc.status = 'active' AND ip_cuc.deleted_at IS NULL", filters.CourseID).
			Select("colleges.*, (college_university_courses.course_id IS NOT NULL OR ip_cuc.id IS NOT NULL) AS course_offers")
	}

	sort := filters.Sort
	if sort != "rating" && sort != "name" && sort != "reviews" && sort != "verified" {
		sort = "rating"
	}

	order := filters.Order
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	if filters.FeeMax > 0 {
		var allColleges []College
		orderClause := "colleges.featured DESC, colleges.verified DESC, colleges.claimed DESC"
		if filters.CourseID != "" {
			orderClause = "course_offers DESC, " + orderClause
		}
		if err := query.Order(orderClause + ", " + sort + " " + order).
			Find(&allColleges).Error; err != nil {
			return nil, 0, err
		}

		filtered := make([]College, 0, len(allColleges))
		for _, college := range allColleges {
			if matchesFeeMax(college, filters.FeeMax) {
				filtered = append(filtered, college)
			}
		}

		total = int64(len(filtered))
		offset := (filters.Page - 1) * filters.PageSize
		if offset >= len(filtered) {
			return []College{}, total, nil
		}

		end := offset + filters.PageSize
		if end > len(filtered) {
			end = len(filtered)
		}

		return filtered[offset:end], total, nil
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (filters.Page - 1) * filters.PageSize

	orderClause := "colleges.featured DESC, colleges.verified DESC, colleges.claimed DESC"
	if filters.CourseID != "" {
		orderClause = "course_offers DESC, " + orderClause
	}

	if err := query.Order(orderClause + ", " + sort + " " + order).
		Offset(offset).
		Limit(filters.PageSize).
		Find(&colleges).Error; err != nil {
		return nil, 0, err
	}

	return colleges, total, nil
}

func (r *Repository) FindByID(id uint) (*College, error) {
	var college College
	err := r.db.First(&college, id).Error
	if err != nil {
		return nil, err
	}
	return &college, nil
}

// FindByIDOrInstitutionID resolves legacy comparison records that stored an
// institution_users ID instead of the corresponding colleges ID.
func (r *Repository) FindByIDOrInstitutionID(id uint) (*College, error) {
	college, err := r.FindByID(id)
	if err == nil {
		return college, nil
	}

	var institution struct {
		ID              uint
		CollegeID       uint
		InstitutionName string
		District        string
		Affiliation     string
		LogoURL         string
		About           string
	}
	if err := r.db.Table("institution_users").
		Select("id, college_id, institution_name, district, affiliation, logo_url, about").
		Where("id = ? AND deleted_at IS NULL", id).
		First(&institution).Error; err != nil {
		return nil, err
	}
	if institution.CollegeID > 0 {
		return r.FindByID(institution.CollegeID)
	}
	return &College{
		ID:          institution.ID,
		Name:        institution.InstitutionName,
		Location:    institution.District,
		Affiliation: institution.Affiliation,
		ImageURL:    institution.LogoURL,
		Description: institution.About,
	}, nil
}

func (r *Repository) Create(college *College) error {
	return r.db.Create(college).Error
}

func (r *Repository) Update(college *College) error {
	return r.db.Save(college).Error
}

func (r *Repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&College{}, id).Error
}

func (r *Repository) Approve(id uint) error {
	return r.db.Model(&College{}).Where("id = ?", id).Update("verified", true).Error
}

func (r *Repository) ToggleFeatured(id uint) error {
	var college College
	if err := r.db.First(&college, id).Error; err != nil {
		return err
	}
	return r.db.Model(&college).Update("featured", !college.Featured).Error
}

func (r *Repository) FindWithinBounds(north, south, east, west float64) ([]College, error) {
	var colleges []College
	err := r.db.Where("latitude IS NOT NULL AND longitude IS NOT NULL").
		Where("latitude BETWEEN ? AND ? AND longitude BETWEEN ? AND ?", south, north, west, east).
		Find(&colleges).Error
	return colleges, err
}

func (r *Repository) FindAllWithCoords() ([]College, error) {
	var colleges []College
	err := r.db.Where("latitude IS NOT NULL AND longitude IS NOT NULL").
		Find(&colleges).Error
	return colleges, err
}

// InstitutionBasic holds minimal institution data for map enrichment
type InstitutionBasic struct {
	CollegeID   uint
	LogoURL     string
	BannerURL   string
	ProfileData *string
}

// FindInstitutionsByCollegeIDs returns institution data keyed by college_id
func (r *Repository) FindInstitutionsByCollegeIDs(collegeIDs []uint) (map[uint]InstitutionBasic, error) {
	type row struct {
		CollegeID   uint   `gorm:"column:college_id"`
		LogoURL     string `gorm:"column:logo_url"`
		BannerURL   string `gorm:"column:banner_url"`
		ProfileData string `gorm:"column:profile_data"`
	}
	var rows []row
	err := r.db.Table("institution_users").
		Select("college_id, logo_url, banner_url, profile_data").
		Where("college_id IN ?", collegeIDs).
		Where("college_id > 0").
		Where("deleted_at IS NULL").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]InstitutionBasic, len(rows))
	for _, r := range rows {
		result[r.CollegeID] = InstitutionBasic{
			CollegeID:   r.CollegeID,
			LogoURL:     r.LogoURL,
			BannerURL:   r.BannerURL,
			ProfileData: &r.ProfileData,
		}
	}
	return result, nil
}

// ReviewAgg holds aggregated review data for a college
type ReviewAgg struct {
	CollegeID uint
	Rating    float64
	Reviews   int
}

// InstitutionPrefs holds preference data from an institution user
type InstitutionPrefs struct {
	CollegeID   uint
	Preferences map[string]interface{}
}

// FindInstitutionPreferencesByIDs returns institution preferences keyed by institution_users.id
func (r *Repository) FindInstitutionPreferencesByIDs(userIDs []uint) (map[uint]InstitutionPrefs, error) {
	type row struct {
		UserID      uint   `gorm:"column:id"`
		Preferences string `gorm:"column:preferences"`
	}
	var rows []row
	err := r.db.Table("institution_users").
		Select("id, preferences::text as preferences").
		Where("id IN ?", userIDs).
		Where("preferences IS NOT NULL").
		Where("deleted_at IS NULL").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]InstitutionPrefs, len(rows))
	for _, r := range rows {
		var prefs map[string]interface{}
		if err := json.Unmarshal([]byte(r.Preferences), &prefs); err == nil {
			result[r.UserID] = InstitutionPrefs{
				CollegeID:   r.UserID,
				Preferences: prefs,
			}
		}
	}
	return result, nil
}

// FindInstitutionPreferencesByCollegeIDs returns institution preferences keyed by college_id
func (r *Repository) FindInstitutionPreferencesByCollegeIDs(collegeIDs []uint) (map[uint]InstitutionPrefs, error) {
	type row struct {
		CollegeID   uint   `gorm:"column:college_id"`
		Preferences string `gorm:"column:preferences"`
	}
	var rows []row
	err := r.db.Table("institution_users").
		Select("college_id, preferences::text as preferences").
		Where("college_id IN ?", collegeIDs).
		Where("college_id > 0").
		Where("preferences IS NOT NULL").
		Where("deleted_at IS NULL").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]InstitutionPrefs, len(rows))
	for _, r := range rows {
		var prefs map[string]interface{}
		if err := json.Unmarshal([]byte(r.Preferences), &prefs); err != nil {
			continue
		}
		// The preferences are nested under "preferences" key in the JSONB
		if inner, ok := prefs["preferences"].(map[string]interface{}); ok {
			prefs = inner
		}
		result[r.CollegeID] = InstitutionPrefs{
			CollegeID:   r.CollegeID,
			Preferences: prefs,
		}
	}
	return result, nil
}

// FindReviewAggregations returns rating and review count for the given college IDs
func (r *Repository) FindReviewAggregations(collegeIDs []uint) (map[uint]ReviewAgg, error) {
	type row struct {
		CollegeID   uint    `gorm:"column:college_id"`
		ReviewCount int     `gorm:"column:review_count"`
		AvgRating   float64 `gorm:"column:avg_rating"`
	}
	var rows []row
	err := r.db.Table("reviews").
		Select("college_id, COUNT(*) as review_count, COALESCE(ROUND(AVG(COALESCE((ratings->>'academics')::numeric,0) + COALESCE((ratings->>'campus_life')::numeric,0) + COALESCE((ratings->>'career_support')::numeric,0) + COALESCE((ratings->>'value_for_money')::numeric,0)) / 4.0, 1), 0) as avg_rating").
		Where("college_id IN ?", collegeIDs).
		Where("college_id > 0").
		Where("deleted_at IS NULL").
		Group("college_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]ReviewAgg, len(rows))
	for _, r := range rows {
		result[r.CollegeID] = ReviewAgg{
			CollegeID: r.CollegeID,
			Rating:    r.AvgRating,
			Reviews:   r.ReviewCount,
		}
	}
	return result, nil
}

func (r *Repository) UpdateCollegeRating(collegeID uint) error {
	var result struct {
		AvgRating  float64
		TotalCount int64
	}
	err := r.db.Table("reviews").
		Select("COALESCE(AVG(ratings->>'Overall Experience')::numeric, 0) as avg_rating, COUNT(*) as total_count").
		Where("college_id = ? AND is_published = ?", collegeID, true).
		Scan(&result).Error
	if err != nil {
		return err
	}
	return r.db.Table("colleges").
		Where("id = ?", collegeID).
		Updates(map[string]interface{}{
			"rating":  result.AvgRating,
			"reviews": result.TotalCount,
		}).Error
}

type InstitutionMapDTO struct {
	ID        uint    `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	District  string  `json:"district,omitempty"`
	Province  string  `json:"province,omitempty"`
	Type      string  `json:"type,omitempty"`
	BannerURL string  `json:"banner_url,omitempty"`
	Phone     string  `json:"phone,omitempty"`
}

func (r *Repository) FindInstitutionsWithCoords() ([]InstitutionMapDTO, error) {
	var institutions []InstitutionMapDTO
	err := r.db.Table("institution_users").
		Select("id, institution_name as name, latitude, longitude, COALESCE(district,'') as district, COALESCE(province,'') as province, COALESCE(organization_type,'') as type, COALESCE(banner_url,'') as banner_url, COALESCE(contact_phone,'') as phone").
		Where("latitude IS NOT NULL AND longitude IS NOT NULL").
		Find(&institutions).Error
	return institutions, err
}

func (r *Repository) UpdateLocation(id uint, lat, lng float64) error {
	return r.db.Model(&College{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"latitude":  lat,
			"longitude": lng,
		}).Error
}

func (r *Repository) Count() (int64, error) {
	var total int64
	err := r.db.Model(&College{}).Count(&total).Error
	return total, err
}

func (r *Repository) FindFeatured(limit int) ([]College, error) {
	var colleges []College
	err := r.db.Where("featured = ?", true).
		Order("rating desc").
		Limit(limit).
		Find(&colleges).Error
	return colleges, err
}

func (r *Repository) FindAllForRecommendation(limit int) ([]College, error) {
	var colleges []College
	err := r.db.Model(&College{}).
		Where("deleted_at IS NULL").
		Order("rating DESC NULLS LAST, reviews DESC").
		Limit(limit).
		Find(&colleges).Error
	if err != nil {
		return nil, err
	}

	if len(colleges) > 0 {
		return colleges, nil
	}

	// Fallback: build College records from approved institution_users
	type instRow struct {
		ID               uint    `gorm:"column:id"`
		InstitutionName  string  `gorm:"column:institution_name"`
		District         string  `gorm:"column:district"`
		Province         string  `gorm:"column:province"`
		OrganizationType string  `gorm:"column:organization_type"`
		About            string  `gorm:"column:about"`
		LogoURL          string  `gorm:"column:logo_url"`
		ProfileData      *string `gorm:"column:profile_data"`
		WebsiteURL       string  `gorm:"column:website_url"`
		Affiliation      string  `gorm:"column:affiliation"`
		Featured         bool    `gorm:"column:featured"`
		Verified         bool    `gorm:"column:verified"`
	}

	var rows []instRow
	err = r.db.Table("institution_users").
		Select("id, institution_name, district, province, organization_type, about, logo_url, profile_data, website_url, affiliation, featured, verified").
		Where("status = ?", "approved").
		Where("deleted_at IS NULL").
		Order("featured DESC, id DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	colleges = make([]College, 0, len(rows))
	for _, row := range rows {
		loc := row.District
		if loc == "" {
			loc = row.Province
		}
		colType := row.OrganizationType
		if colType == "" {
			colType = "Private"
		}
		c := College{
			ID:               row.ID,
			Name:             row.InstitutionName,
			FullName:         row.InstitutionName,
			Location:         loc,
			Affiliation:      row.Affiliation,
			CollegeType:      colType,
			Verified:         row.Verified,
			Featured:         row.Featured,
			Description:      row.About,
			Website:          row.WebsiteURL,
			ImageURL:         row.LogoURL,
			AcademicFitScore: 5,
			CampusLifeScore:  5,
			CareerFitScore:   5,
			BalancedFitScore: 5,
		}
		colleges = append(colleges, c)
	}

	return colleges, nil
}

func parseTypes(typeStr string) []string {
	typeAlias := map[string]string{
		"ct_private":     "Private",
		"ct_public":      "Public / Govt",
		"ct_community":   "Community",
		"ct_constituent": "Constituent",
		"ct_foreign":     "Foreign Affiliated",
	}

	typesRaw := strings.Split(typeStr, ",")
	types := make([]string, 0, len(typesRaw))
	for _, t := range typesRaw {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			if mapped, ok := typeAlias[trimmed]; ok {
				types = append(types, mapped)
				continue
			}
			types = append(types, trimmed)
		}
	}
	return types
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	parsed := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parsed = append(parsed, trimmed)
		}
	}
	return parsed
}

func applyMultiILIKEFilter(query *gorm.DB, field string, values []string) *gorm.DB {
	if len(values) == 0 {
		return query
	}

	clauses := make([]string, 0, len(values))
	args := make([]interface{}, 0, len(values))
	for _, value := range values {
		clauses = append(clauses, field+" ILIKE ?")
		args = append(args, "%"+value+"%")
	}

	return query.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func applyFacetSearchFilter(query *gorm.DB, ids []string) *gorm.DB {
	keywords := make([]string, 0, len(ids))
	for _, id := range ids {
		if mapped, ok := facetKeywordsByID[id]; ok {
			keywords = append(keywords, mapped...)
		} else {
			keywords = append(keywords, id)
		}
	}

	if len(keywords) == 0 {
		return query
	}

	searchField := "CAST(featured_programs AS TEXT) || ' ' || CAST(courses AS TEXT) || ' ' || CAST(programs_list AS TEXT) || ' ' || CAST(admissions AS TEXT) || ' ' || description"
	return applyMultiILIKEFilter(query, searchField, keywords)
}

func matchesFeeMax(college College, feeMax int) bool {
	minFee, ok := extractCollegeMinFee(college)
	if !ok {
		return false
	}
	return minFee <= feeMax
}

func extractCollegeMinFee(college College) (int, bool) {
	feeTexts := []string{
		string(college.Courses),
		string(college.ProgramsList),
		string(college.Admissions),
		string(college.FeaturedPrograms),
		college.Description,
	}

	minFee := 0
	found := false

	for _, text := range feeTexts {
		for _, raw := range feeNumberRegex.FindAllString(text, -1) {
			normalized := strings.ReplaceAll(raw, ",", "")
			amount, err := strconv.Atoi(normalized)
			if err != nil {
				continue
			}

			if amount < 5000 {
				continue
			}

			if !found || amount < minFee {
				minFee = amount
				found = true
			}
		}
	}

	return minFee, found
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func (r *Repository) GetCollegeProfileForRecommendation(userID uint) (*CollegeProfileData, error) {
	type rawEntry struct {
		Level  string
		Stream string
		Grade  string
	}
	var entries []rawEntry
	if err := r.db.Table("education_entries").
		Where("user_id = ?", userID).
		Select("level, stream, grade").
		Find(&entries).Error; err != nil {
		return nil, err
	}

	type rawUser struct {
		Preferences []byte
	}
	var user rawUser
	if err := r.db.Table("users").
		Select("preferences").
		Where("id = ?", userID).
		First(&user).Error; err != nil {
		return nil, err
	}

	type bookmarkRow struct {
		Field string
	}
	var bookmarks []bookmarkRow
	r.db.Table("bookmarks").
		Select("DISTINCT field").
		Where("user_id = ? AND entity_type = 'college'", userID).
		Scan(&bookmarks)

	eduEntries := make([]CollegeEducationEntry, 0, len(entries))
	for _, e := range entries {
		eduEntries = append(eduEntries, CollegeEducationEntry{
			Level:  e.Level,
			Stream: e.Stream,
			Grade:  e.Grade,
		})
	}

	var prefs *CollegePreferences
	if len(user.Preferences) > 0 {
		var p CollegePreferences
		if err := json.Unmarshal(user.Preferences, &p); err == nil {
			prefs = &p
		}
	}

	fields := make([]string, 0, len(bookmarks))
	for _, b := range bookmarks {
		if b.Field != "" {
			fields = append(fields, b.Field)
		}
	}

	return &CollegeProfileData{
		EducationEntries: eduEntries,
		Preferences:      prefs,
		BookmarkedFields: fields,
	}, nil
}

func mapAdmissionLevel(level string) string {
	mapping := map[string]string{
		"high-school": "+2",
		"a-level":     "A-Level",
		"diploma":     "Diploma/CTEVT",
		"ctevt":       "Diploma/CTEVT",
		"bachelor":    "Bachelor",
		"master":      "Master",
	}
	if mapped, ok := mapping[level]; ok {
		return mapped
	}
	return level
}

func (r *Repository) GetFilterCounts(level string) (*CollegeFilterCountsResponse, error) {
	resp := &CollegeFilterCountsResponse{
		TypeCounts:      map[string]int64{},
		TypeCountsByID:  map[string]int64{},
		FacetCountsByID: map[string]int64{},
	}

	typeToID := map[string]string{
		"private":            "ct_private",
		"public / govt":      "ct_public",
		"community":          "ct_community",
		"constituent":        "ct_constituent",
		"foreign affiliated": "ct_foreign",
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

	totalQuery := r.db.Model(&College{})
	featuredQuery := r.db.Model(&College{}).Where("colleges.featured = ?", true)
	verifiedQuery := r.db.Model(&College{}).Where("colleges.verified = ?", true)
	popularQuery := r.db.Model(&College{}).Where("colleges.popular = ?", true)
	if level != "" {
		mappedLevel := mapAdmissionLevel(level)
		joinClause := "JOIN institution_users iu ON iu.college_id = colleges.id AND iu.deleted_at IS NULL JOIN admission_pages ap ON ap.institution_id = iu.id AND ap.status = 'published' AND ap.deleted_at IS NULL AND ap.data->'overview_data'->>'level' = ?"
		totalQuery = totalQuery.Joins(joinClause, mappedLevel)
		featuredQuery = featuredQuery.Joins(joinClause, mappedLevel)
		verifiedQuery = verifiedQuery.Joins(joinClause, mappedLevel)
		popularQuery = popularQuery.Joins(joinClause, mappedLevel)
	}

	if err := totalQuery.Count(&resp.Total).Error; err != nil {
		return nil, err
	}

	if err := featuredQuery.Count(&resp.Featured).Error; err != nil {
		return nil, err
	}

	if err := verifiedQuery.Count(&resp.Verified).Error; err != nil {
		return nil, err
	}

	if err := popularQuery.Count(&resp.Popular).Error; err != nil {
		return nil, err
	}

	type typeCountRow struct {
		CollegeType string
		Count       int64
	}

	var rows []typeCountRow
	typeQuery := r.db.Model(&College{}).
		Select("colleges.college_type, COUNT(*) as count").
		Group("colleges.college_type")
	if level != "" {
		mappedLevel := mapAdmissionLevel(level)
		typeQuery = typeQuery.Joins("JOIN institution_users iu ON iu.college_id = colleges.id AND iu.deleted_at IS NULL JOIN admission_pages ap ON ap.institution_id = iu.id AND ap.status = 'published' AND ap.deleted_at IS NULL AND ap.data->'overview_data'->>'level' = ?", mappedLevel)
	}
	if err := typeQuery.Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		key := strings.TrimSpace(row.CollegeType)
		if key == "" {
			continue
		}
		resp.TypeCounts[key] = row.Count

		if mappedID, ok := typeToID[strings.ToLower(key)]; ok {
			resp.TypeCountsByID[mappedID] = row.Count
		}
	}

	type collegeFacetRow struct {
		Location         string
		Affiliation      string
		CollegeType      string
		FeaturedPrograms []byte
		Courses          []byte
		ProgramsList     []byte
		ProgramsData     string
	}

	var facetRows []collegeFacetRow
	if level != "" {
		mappedLevel := mapAdmissionLevel(level)
		if err := r.db.Raw(`SELECT 
			c.location, c.affiliation, c.college_type, c.featured_programs, c.courses, c.programs_list,
			COALESCE(ap.data->>'programs_data', '[]') AS programs_data
			FROM admission_pages ap
			JOIN institution_users iu ON iu.id = ap.institution_id AND iu.deleted_at IS NULL
			JOIN colleges c ON c.id = iu.college_id AND c.deleted_at IS NULL
			WHERE ap.status = 'published' AND ap.deleted_at IS NULL
			AND ap.data->'overview_data'->>'level' = ?`, mappedLevel).Scan(&facetRows).Error; err != nil {
			return nil, err
		}
	} else {
		if err := r.db.Model(&College{}).
			Select("location, affiliation, college_type, featured_programs, courses, programs_list, '[]' AS programs_data").
			Scan(&facetRows).Error; err != nil {
			return nil, err
		}
	}

	for _, row := range facetRows {
		programTitles := ""
		var progData []struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(row.ProgramsData), &progData); err == nil {
			parts := make([]string, 0, len(progData))
			for _, p := range progData {
				if p.Title != "" {
					parts = append(parts, p.Title)
				}
			}
			programTitles = strings.Join(parts, " ")
		}

		searchText := strings.ToLower(strings.Join([]string{
			row.Location,
			row.Affiliation,
			row.CollegeType,
			string(row.FeaturedPrograms),
			string(row.Courses),
			string(row.ProgramsList),
			programTitles,
		}, " "))

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

// Comparison History methods

func (r *Repository) ValidateCollegeExists(collegeID uint) error {
	_, err := r.FindByIDOrInstitutionID(collegeID)
	return err
}

func (r *Repository) LogComparison(college1ID, college2ID uint, college1Name, college2Name string) error {
	// Normalize: always store smaller ID first to avoid duplicate pairs
	c1ID, c2ID := college1ID, college2ID
	c1Name, c2Name := college1Name, college2Name
	if college1ID > college2ID {
		c1ID, c2ID = college2ID, college1ID
		c1Name, c2Name = college2Name, college1Name
	}

	var history ComparisonHistory
	result := r.db.Where("college1_id = ? AND college2_id = ? AND deleted_at IS NULL", c1ID, c2ID).First(&history)
	if result.Error == gorm.ErrRecordNotFound {
		history = ComparisonHistory{
			College1ID:      c1ID,
			College2ID:      c2ID,
			College1Name:    c1Name,
			College2Name:    c2Name,
			ComparisonCount: 1,
		}
		return r.db.Create(&history).Error
	}
	if result.Error != nil {
		return result.Error
	}
	history.ComparisonCount++
	return r.db.Save(&history).Error
}

type PopularComparison struct {
	College1ID      uint   `json:"college1_id"`
	College1Name    string `json:"college1_name"`
	College1LogoURL string `json:"college1_logo_url"`
	College2ID      uint   `json:"college2_id"`
	College2Name    string `json:"college2_name"`
	College2LogoURL string `json:"college2_logo_url"`
	Count           int    `json:"count"`
}

func (r *Repository) GetPopularComparisons(limit int) ([]PopularComparison, error) {
	var results []PopularComparison
	err := r.db.
		Table("comparison_history ch").
		Select(`
			ch.college1_id,
			ch.college1_name,
			COALESCE(NULLIF(iu1.logo_url, ''), c1.image_url, '') as college1_logo_url,
			ch.college2_id,
			ch.college2_name,
			COALESCE(NULLIF(iu2.logo_url, ''), c2.image_url, '') as college2_logo_url,
			ch.comparison_count as count
		`).
		Joins("LEFT JOIN institution_users iu1 ON iu1.college_id = ch.college1_id AND iu1.deleted_at IS NULL").
		Joins("LEFT JOIN institution_users iu2 ON iu2.college_id = ch.college2_id AND iu2.deleted_at IS NULL").
		Joins("LEFT JOIN colleges c1 ON c1.id = ch.college1_id AND c1.deleted_at IS NULL").
		Joins("LEFT JOIN colleges c2 ON c2.id = ch.college2_id AND c2.deleted_at IS NULL").
		Where("ch.deleted_at IS NULL").
		Order("ch.comparison_count DESC").
		Limit(limit).
		Scan(&results).Error
	return results, err
}

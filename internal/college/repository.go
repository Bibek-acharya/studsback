package college

import (
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

var feeNumberRegex = regexp.MustCompile(`\d[\d,]{3,}`)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(filters CollegeFilters) ([]College, int64, error) {
	var colleges []College
	var total int64

	query := r.db.Model(&College{})

	if filters.UniversityID != "" {
		query = query.Where("university_id = ?", filters.UniversityID)
	}

	if filters.Location != "" {
		query = applyMultiILIKEFilter(query, "location", parseCSV(filters.Location))
	}

	if filters.Affiliation != "" {
		query = applyMultiILIKEFilter(query, "affiliation", parseCSV(filters.Affiliation))
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
		query = query.Joins("JOIN college_university_courses ON college_university_courses.college_id = colleges.id").
			Where("college_university_courses.course_id = ?", filters.CourseID)
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
		if err := query.Order(sort + " " + order).
			Preload("University").
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

	if err := query.Order(sort + " " + order).
		Offset(offset).
		Limit(filters.PageSize).
		Preload("University").
		Find(&colleges).Error; err != nil {
		return nil, 0, err
	}

	return colleges, total, nil
}

func (r *Repository) FindByID(id uint) (*College, error) {
	var college College
	err := r.db.Preload("University").First(&college, id).Error
	if err != nil {
		return nil, err
	}
	return &college, nil
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

func (r *Repository) UniversityExists(id uint) bool {
	var count int64
	r.db.Model(&University{}).Where("id = ?", id).Count(&count)
	return count > 0
}

func (r *Repository) FindUniversityByName(name string) (*University, error) {
	var university University
	err := r.db.Where("LOWER(name) = LOWER(?)", name).First(&university).Error
	return &university, err
}

func (r *Repository) FindUniversityByID(id uint) (*University, error) {
	var university University
	err := r.db.First(&university, id).Error
	return &university, err
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

func (r *Repository) GetFilterCounts() (*CollegeFilterCountsResponse, error) {
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

	if err := r.db.Model(&College{}).Count(&resp.Total).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&College{}).Where("featured = ?", true).Count(&resp.Featured).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&College{}).Where("verified = ?", true).Count(&resp.Verified).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&College{}).Where("popular = ?", true).Count(&resp.Popular).Error; err != nil {
		return nil, err
	}

	type typeCountRow struct {
		CollegeType string
		Count       int64
	}

	var rows []typeCountRow
	if err := r.db.Model(&College{}).
		Select("college_type, COUNT(*) as count").
		Group("college_type").
		Scan(&rows).Error; err != nil {
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
	}

	var facetRows []collegeFacetRow
	if err := r.db.Model(&College{}).
		Select("location, affiliation, college_type, featured_programs, courses, programs_list").
		Scan(&facetRows).Error; err != nil {
		return nil, err
	}

	for _, row := range facetRows {
		searchText := strings.ToLower(strings.Join([]string{
			row.Location,
			row.Affiliation,
			row.CollegeType,
			string(row.FeaturedPrograms),
			string(row.Courses),
			string(row.ProgramsList),
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

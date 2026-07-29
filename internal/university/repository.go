package university

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(search, uniType, status string, popular bool, isNepali string) ([]University, error) {
	var universities []University
	query := r.db.Model(&University{})

	if search != "" {
		query = query.Where("name ILIKE ? OR location ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if uniType != "" {
		query = query.Where("type = ?", uniType)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if popular {
		query = query.Where("popular = ?", true)
	}

	if isNepali == "true" {
		query = query.Where("is_nepali = ?", true)
	} else if isNepali == "false" {
		query = query.Where("is_nepali = ?", false)
	}

	err := query.Order("rank ASC").Find(&universities).Error
	return universities, err
}

func (r *Repository) FindByID(id uint) (*University, error) {
	var uni University
	err := r.db.Select("id, name, logo, location, type, is_nepali, rank, rating, review_count, verified, popular, description, established, students, chancellor, vice_chancellor, founder, website, cover, about, contact, quick, overview, leadership, courses, programs, scholarships, events, news, downloads, gallery, faculties, admissions, reviews").First(&uni, id).Error
	if err != nil {
		return nil, err
	}
	return &uni, nil
}

func (r *Repository) FindByIDFull(id uint) (*University, error) {
	var uni University
	err := r.db.First(&uni, id).Error
	if err != nil {
		return nil, err
	}
	return &uni, nil
}

func (r *Repository) FindCollegesByUniversityID(universityID uint) ([]College, error) {
	var mappingCollegeIDs []uint
	err := r.db.
		Model(&CollegeUniversityCourse{}).
		Where("university_id = ?", universityID).
		Distinct("college_id").
		Pluck("college_id", &mappingCollegeIDs).Error
	if err != nil {
		return nil, err
	}

	var colleges []College
	query := r.db.Model(&College{})
	if len(mappingCollegeIDs) > 0 {
		query = query.Where("id IN ?", mappingCollegeIDs)
	} else {
		query = query.Where("university_id = ?", universityID)
	}

	err = query.Find(&colleges).Error
	return colleges, err
}

func (r *Repository) Create(uni *University) error {
	return r.db.Create(uni).Error
}

func (r *Repository) Update(uni *University) error {
	return r.db.Save(uni).Error
}

func (r *Repository) FindDeletedByName(name string) (*University, error) {
	var uni University
	err := r.db.Unscoped().Where("name = ? AND deleted_at IS NOT NULL", name).First(&uni).Error
	if err != nil {
		return nil, err
	}
	return &uni, nil
}

func (r *Repository) Restore(id uint) error {
	return r.db.Unscoped().Model(&University{}).Where("id = ?", id).Update("deleted_at", nil).Error
}

func (r *Repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&University{}, id).Error
}

func (r *Repository) GetFilterCounts(isNepali string) (*UniversityFilterCountsResponse, error) {
	resp := &UniversityFilterCountsResponse{
		TypeCounts:     make(map[string]int64),
		TypeCountsByID: make(map[string]int64),
		RatingCounts:   make(map[string]int64),
		AcademicCounts: make(map[string]int64),
	}

	typeToID := map[string]string{
		"private":     "ut_private",
		"public":      "ut_public",
		"community":   "ut_community",
		"constituent": "ut_constituent",
	}

	base := r.db.Model(&University{})

	if isNepali == "true" {
		base = base.Where("is_nepali = ?", true)
	} else if isNepali == "false" {
		base = base.Where("is_nepali = ?", false)
	}

	if err := base.Count(&resp.Total).Error; err != nil {
		return nil, err
	}

	type typeCountRow struct {
		Type  string
		Count int64
	}

	var typeRows []typeCountRow
	if err := base.Select("type, COUNT(*) as count").Group("type").Scan(&typeRows).Error; err != nil {
		return nil, err
	}
	for _, row := range typeRows {
		key := row.Type
		resp.TypeCounts[key] = row.Count
		if mappedID, ok := typeToID[toLower(key)]; ok {
			resp.TypeCountsByID[mappedID] = row.Count
		}
	}

	ratingThresholds := []float64{4.5, 4.0, 3.5, 3.0}
	for _, t := range ratingThresholds {
		var count int64
		thresholdStr := strconv.FormatFloat(t, 'f', 1, 64)
		rater := r.db.Model(&University{})
		if isNepali == "true" {
			rater = rater.Where("is_nepali = ?", true)
		} else if isNepali == "false" {
			rater = rater.Where("is_nepali = ?", false)
		}
		if err := rater.Where("rating >= ?", t).Count(&count).Error; err != nil {
			return nil, err
		}
		resp.RatingCounts[thresholdStr] = count
	}

	academicKeywords := map[string][]string{
		"bachelors": {"bachelor", "bsc", "bba", "bbs", "bim", "mbbs", "bds", "bca", "bit", "b.ed", "bhm", "bachelor's", "undergraduate"},
		"masters":   {"master", "msc", "mba", "mbs", "mca", "mit", "m.ed", "mhm", "master's", "postgraduate", "graduate"},
	}

	searchField := "COALESCE(description, '') || ' ' || CAST(COALESCE(programs, '[]'::jsonb) AS TEXT) || ' ' || CAST(COALESCE(courses, '[]'::jsonb) AS TEXT)"
	for levelID, keywords := range academicKeywords {
		var count int64
		likeClauses := make([]string, 0, len(keywords))
		args := make([]interface{}, 0, len(keywords))
		for _, kw := range keywords {
			likeClauses = append(likeClauses, searchField+" ILIKE ?")
			args = append(args, "%"+kw+"%")
		}
		query := r.db.Model(&University{}).Where(strings.Join(likeClauses, " OR "), args...)
		if isNepali == "true" {
			query = query.Where("is_nepali = ?", true)
		} else if isNepali == "false" {
			query = query.Where("is_nepali = ?", false)
		}
		if err := query.Count(&count).Error; err != nil {
			return nil, err
		}
		resp.AcademicCounts[levelID] = count
	}

	return resp, nil
}

func toLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func (r *Repository) GetTabData(id uint, tab string) ([]byte, error) {
	var result struct {
		Data []byte `gorm:"column:data"`
	}

	err := r.db.Model(&University{}).
		Select(tab+" as data").
		Where("id = ?", id).
		Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

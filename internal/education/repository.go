package education

import (
	"errors"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindTopRatedColleges(limit int) ([]College, error) {
	var colleges []College
	err := r.db.Preload("University").Order("rating desc").Limit(limit).Find(&colleges).Error
	return colleges, err
}

func (r *Repository) FindExams() ([]Exam, error) {
	var exams []Exam
	err := r.db.Find(&exams).Error
	return exams, err
}

func (r *Repository) FindExamByID(id string) (*Exam, error) {
	var exam Exam
	err := r.db.Where("id = ? OR slug = ?", id, id).First(&exam).Error
	if err != nil {
		return nil, err
	}
	return &exam, nil
}

// CoursePaginationMeta for courses
type CoursePaginationMeta struct {
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Pages int64 `json:"pages"`
}

func (r *Repository) FindCoursesFiltered(page, limit int, search, level, field, affiliation string) ([]Course, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := r.db.Model(&Course{})

	if search != "" {
		query = query.Where("title ILIKE ? OR field ILIKE ? OR affiliation ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if level != "" {
		query = query.Where("level = ?", level)
	}
	if field != "" {
		query = query.Where("field = ?", field)
	}
	if affiliation != "" {
		query = query.Where("affiliation = ?", affiliation)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var courses []Course
	err := query.Order("id desc").Offset(offset).Limit(limit).Find(&courses).Error
	return courses, total, err
}

type InstitutionProgramEntry struct {
	ID                  uint   `json:"id"`
	ProgramName         string `json:"name"`
	Description         string `json:"description"`
	Duration            string `json:"duration"`
	Fee                 string `json:"fee"`
	BannerURL           string `json:"banner_url"`
	InstitutionName     string `json:"institution_name"`
	InstitutionLogo     string `json:"institution_logo"`
	InstitutionLocation string `gorm:"column:institution_location" json:"institution_location"`
}

func (r *Repository) FindPublishedInstitutionPrograms(search, level string) ([]InstitutionProgramEntry, error) {
	var entries []InstitutionProgramEntry
	query := r.db.Table("institution_programs").
		Select(`institution_programs.id, institution_programs.name as program_name, institution_programs.description,
			institution_programs.duration, institution_programs.fee, institution_programs.banner_url,
			iu.institution_name, iu.logo_url as institution_logo, iu.district as institution_location`).
		Joins("LEFT JOIN institution_users iu ON iu.id = institution_programs.institution_id").
		Where("institution_programs.status = ?", "active")
	if search != "" {
		query = query.Where("institution_programs.name ILIKE ?", "%"+search+"%")
	}
	err := query.Order("institution_programs.created_at DESC").Find(&entries).Error
	return entries, err
}

func (r *Repository) FindPublishedInstitutionProgramByID(id string) (*InstitutionProgramEntry, error) {
	var entry InstitutionProgramEntry
	err := r.db.Table("institution_programs").
		Select(`institution_programs.id, institution_programs.name as program_name, institution_programs.description,
			institution_programs.duration, institution_programs.fee, institution_programs.banner_url,
			iu.institution_name, iu.logo_url as institution_logo, iu.district as institution_location`).
		Joins("LEFT JOIN institution_users iu ON iu.id = institution_programs.institution_id").
		Where("institution_programs.id = ? AND institution_programs.status = ?", id, "active").
		First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *Repository) FindCourses() ([]Course, error) {
	var courses []Course
	err := r.db.Find(&courses).Error
	return courses, err
}

// CourseFilterCounts for filter sidebar
type CourseFilterCounts struct {
	LevelCount       map[string]int64 `json:"level_counts"`
	FieldCount       map[string]int64 `json:"field_counts"`
	AffiliationCount map[string]int64 `json:"affiliation_counts"`
	Total            int64            `json:"total"`
}

func (r *Repository) GetCourseFilterCounts() (*CourseFilterCounts, error) {
	counts := &CourseFilterCounts{
		LevelCount:       make(map[string]int64),
		FieldCount:       make(map[string]int64),
		AffiliationCount: make(map[string]int64),
	}

	// Level counts
	var levels []struct {
		Level string
		Count int64
	}
	r.db.Model(&Course{}).Select("level, COUNT(*) as count").Group("level").Find(&levels)
	for _, l := range levels {
		counts.LevelCount[l.Level] = l.Count
	}

	// Field counts
	var fields []struct {
		Field string
		Count int64
	}
	r.db.Model(&Course{}).Select("field, COUNT(*) as count").Group("field").Find(&fields)
	for _, f := range fields {
		counts.FieldCount[f.Field] = f.Count
	}

	// Affiliation counts
	var affils []struct {
		Affiliation string
		Count       int64
	}
	r.db.Model(&Course{}).Select("affiliation, COUNT(*) as count").Group("affiliation").Find(&affils)
	for _, a := range affils {
		counts.AffiliationCount[a.Affiliation] = a.Count
	}

	// Total
	r.db.Model(&Course{}).Count(&counts.Total)

	return counts, nil
}

func (r *Repository) FindCourseByID(id string) (*Course, error) {
	var course Course
	err := r.db.First(&course, id).Error
	if err != nil {
		return nil, err
	}
	return &course, nil
}

func (r *Repository) CountCourseOfferingColleges(courseID uint) (int64, error) {
	var count int64
	err := r.db.Model(&CollegeUniversityCourse{}).
		Distinct("college_id").
		Where("course_id = ?", courseID).
		Count(&count).Error
	return count, err
}

func (r *Repository) FindRelatedCourses(excludeID uint, field, level string, limit int) ([]Course, error) {
	var courses []Course
	err := r.db.
		Where("id <> ?", excludeID).
		Where("field = ? OR level = ?", field, level).
		Order("id asc").
		Limit(limit).
		Find(&courses).Error
	return courses, err
}

func (r *Repository) FindFallbackCourses(excludeID uint, limit int) ([]Course, error) {
	var courses []Course
	err := r.db.
		Where("id <> ?", excludeID).
		Order("id asc").
		Limit(limit).
		Find(&courses).Error
	return courses, err
}

func (r *Repository) FindCourseMappings(courseID uint) ([]CollegeUniversityCourse, error) {
	var mappings []CollegeUniversityCourse
	err := r.db.
		Where("course_id = ?", courseID).
		Order("college_id asc").
		Find(&mappings).Error
	return mappings, err
}

func (r *Repository) FindCollegesByIDs(ids []uint) ([]College, error) {
	var colleges []College
	if len(ids) == 0 {
		return colleges, nil
	}
	err := r.db.
		Where("id IN ?", ids).
		Order("rating desc").
		Find(&colleges).Error
	return colleges, err
}

func (r *Repository) FindUniversityByID(id uint) (*University, error) {
	var university University
	err := r.db.First(&university, id).Error
	if err != nil {
		return nil, err
	}
	return &university, nil
}

func (r *Repository) FindNews(limit int) ([]News, error) {
	var news []News
	err := r.db.Order("created_at desc").Limit(limit).Find(&news).Error
	return news, err
}

func (r *Repository) FindNewsFiltered(page, limit int, category, search, sort string) ([]News, int64, error) {
	var err error

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := r.db.Model(&News{})

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		query = query.Where("title ILIKE ? OR excerpt ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderClause := "created_at desc"
	if sort == "oldest" {
		orderClause = "created_at asc"
	} else if sort == "popular" {
		orderClause = "view_count desc"
	}

	var news []News
	err = query.Order(orderClause).Offset(offset).Limit(limit).Find(&news).Error
	return news, total, err
}

type NewsFilterCounts struct {
	CategoryCounts map[string]int64 `json:"category_counts"`
	Total          int64            `json:"total"`
}

func (r *Repository) GetNewsFilterCounts() (*NewsFilterCounts, error) {
	counts := &NewsFilterCounts{
		CategoryCounts: make(map[string]int64),
	}

	var results []struct {
		Category string
		Count    int64
	}

	r.db.Model(&News{}).
		Select("category, COUNT(*) as count").
		Group("category").
		Scan(&results)

	for _, res := range results {
		counts.CategoryCounts[res.Category] = res.Count
	}

	r.db.Model(&News{}).Count(&counts.Total)

	return counts, nil
}

func (r *Repository) FindNewsByID(id string) (*News, error) {
	var news News
	err := r.db.First(&news, id).Error
	if err != nil {
		return nil, err
	}
	return &news, nil
}

func (r *Repository) FindEvents() ([]Event, error) {
	var events []Event
	err := r.db.Order("date asc").Find(&events).Error
	return events, err
}

func (r *Repository) FindEventsFiltered(page, limit int, category, search, sort, featuredStr string) ([]Event, int64, error) {
	var events []Event
	var err error

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := r.db.Model(&Event{})

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		query = query.Where("title ILIKE ? OR description ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if featuredStr != "" {
		featured := featuredStr == "true"
		query = query.Where("featured = ?", featured)
	}

	var total int64
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderClause := "date asc"
	if sort == "oldest" {
		orderClause = "date desc"
	} else if sort == "popular" {
		orderClause = "interested_count desc"
	}

	err = query.Order(orderClause).Offset(offset).Limit(limit).Find(&events).Error
	return events, total, err
}

func (r *Repository) FindAllEvents(page, limit int) ([]Event, int64, error) {
	var events []Event
	var total int64

	query := r.db.Model(&Event{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&events).Error; err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

func (r *Repository) CreateEvent(event *Event) error {
	return r.db.Create(event).Error
}

func (r *Repository) UpdateEvent(id string, updates map[string]interface{}) (*Event, error) {
	var event Event
	if err := r.db.First(&event, id).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&event).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &event, nil
}

func (r *Repository) DeleteEvent(id string) error {
	result := r.db.Delete(&Event{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ToggleEventFeatured(id string) (*Event, error) {
	var event Event
	if err := r.db.First(&event, id).Error; err != nil {
		return nil, err
	}

	newFeatured := !event.Featured
	if err := r.db.Model(&event).Update("featured", newFeatured).Error; err != nil {
		return nil, err
	}

	event.Featured = newFeatured
	return &event, nil
}

type EventFilterCounts struct {
	CategoryCounts map[string]int64 `json:"category_counts"`
	Total          int64            `json:"total"`
}

func (r *Repository) GetEventFilterCounts() (*EventFilterCounts, error) {
	counts := &EventFilterCounts{
		CategoryCounts: make(map[string]int64),
	}

	var results []struct {
		Category string
		Count    int64
	}

	r.db.Model(&Event{}).
		Select("category, COUNT(*) as count").
		Group("category").
		Scan(&results)

	for _, res := range results {
		counts.CategoryCounts[res.Category] = res.Count
	}

	r.db.Model(&Event{}).Count(&counts.Total)

	return counts, nil
}

func (r *Repository) FindEventByID(id string) (*Event, error) {
	var event Event
	err := r.db.First(&event, id).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) FindBlogs(page, limit int, category, search, sort, tags string) ([]Blog, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := r.db.Model(&Blog{}).Where("published = ?", true)

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		query = query.Where("title ILIKE ? OR excerpt ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if tags != "" {
		query = query.Where("tags && ?", []string{tags})
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var blogs []Blog
	orderClause := getBlogSortOrder(sort)
	err := query.Order(orderClause).Offset(offset).Limit(limit).Find(&blogs).Error
	return blogs, total, err
}

func getBlogSortOrder(sort string) string {
	switch sort {
	case "oldest":
		return "featured desc, created_at asc"
	case "popular":
		return "featured desc, views desc"
	case "title":
		return "featured desc, title asc"
	default: // newest
		return "featured desc, created_at desc"
	}
}

func (r *Repository) FindBlogByID(id string) (*Blog, error) {
	var blog Blog
	err := r.db.Where("published = ?", true).First(&blog, id).Error
	if err != nil {
		return nil, err
	}
	return &blog, nil
}

func (r *Repository) IncrementBlogViews(blog *Blog) error {
	return r.db.Model(blog).Update("views", blog.Views+1).Error
}

func (r *Repository) FindRelatedBlogs(excludeID uint, category string, limit int) ([]Blog, error) {
	var blogs []Blog
	err := r.db.
		Where("published = ? AND id <> ? AND category = ?", true, excludeID, category).
		Order("created_at desc").
		Limit(limit).
		Find(&blogs).Error
	return blogs, err
}

// ─── Admin CRUD ──────────────────────────────────────────────────────────────

func (r *Repository) FindAllBlogsAdmin(page, limit int, category, search, sort string) ([]Blog, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := r.db.Model(&Blog{})

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		query = query.Where("title ILIKE ? OR excerpt ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var blogs []Blog
	orderClause := getBlogSortOrder(sort)
	err := query.Order(orderClause).Offset(offset).Limit(limit).Find(&blogs).Error
	return blogs, total, err
}

// BlogFilterCounts returns count of blogs by category
type BlogFilterCounts struct {
	CategoryCounts map[string]int64 `json:"category_counts"`
	Total          int64            `json:"total"`
}

func (r *Repository) GetBlogFilterCounts() (*BlogFilterCounts, error) {
	var counts BlogFilterCounts
	counts.CategoryCounts = make(map[string]int64)

	var results []struct {
		Category string
		Count    int64
	}

	err := r.db.Model(&Blog{}).Where("published = ?", true).
		Select("category, COUNT(*) as count").
		Group("category").
		Find(&results).Error
	if err != nil {
		return nil, err
	}

	var total int64
	r.db.Model(&Blog{}).Where("published = ?", true).Count(&total)
	counts.Total = total

	for _, r := range results {
		counts.CategoryCounts[r.Category] = r.Count
	}

	return &counts, nil
}
func (r *Repository) FindBlogByIDAdmin(id string) (*Blog, error) {
	var blog Blog
	err := r.db.First(&blog, id).Error
	if err != nil {
		return nil, err
	}
	return &blog, nil
}
func (r *Repository) CreateBlog(blog *Blog) error {
	return r.db.Create(blog).Error
}

func (r *Repository) UpdateBlog(blog *Blog) error {
	return r.db.Save(blog).Error
}

func (r *Repository) DeleteBlog(id uint) error {
	return r.db.Delete(&Blog{}, id).Error
}

func (r *Repository) CreateBlogComment(comment *BlogComment) error {
	return r.db.Create(comment).Error
}

func (r *Repository) GetBlogComments(blogID uint) ([]BlogComment, error) {
	var comments []BlogComment
	err := r.db.Where("blog_id = ?", blogID).Order("created_at desc").Find(&comments).Error
	return comments, err
}

func (r *Repository) IncrementCommentLikes(id uint) error {
	return r.db.Model(&BlogComment{}).Where("id = ?", id).Update("likes", gorm.Expr("likes + ?", 1)).Error
}

// ─── Admin News CRUD ─────────────────────────────────────────────────────────

func (r *Repository) FindAllNewsAdmin(page, limit int, category, search string) ([]News, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := r.db.Model(&News{})

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		query = query.Where("title ILIKE ? OR excerpt ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var news []News
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&news).Error
	return news, total, err
}

func (r *Repository) CreateNews(news *News) error {
	return r.db.Create(news).Error
}

func (r *Repository) FindNewsByIDAdmin(id string) (*News, error) {
	var news News
	err := r.db.First(&news, id).Error
	if err != nil {
		return nil, err
	}
	return &news, nil
}

func (r *Repository) UpdateNews(news *News) error {
	return r.db.Save(news).Error
}

func (r *Repository) DeleteNews(id string) error {
	result := r.db.Delete(&News{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

type InstitutionEntranceEntry struct {
	ID                  uint   `json:"id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	Date                string `json:"date"`
	HeroBanner          string `json:"hero_banner"`
	Status              string `json:"status"`
	Fee                 string `json:"application_fee"`
	InstitutionName     string `json:"institution_name"`
	InstitutionLogo     string `json:"institution_logo"`
	InstitutionLocation string `gorm:"column:institution_location" json:"institution_location"`
	InstitutionProvince string `gorm:"column:institution_province" json:"institution_province"`
	InstitutionWebsite  string `gorm:"column:institution_website" json:"institution_website"`
	InstitutionEmail    string        `json:"institution_email"`
	InstitutionPhone    string        `json:"institution_phone"`
	OverviewDetails     []byte        `gorm:"column:overview_details" json:"overview_details"`
	ExamDateSchedules   []byte        `gorm:"column:exam_date_schedules" json:"exam_date_schedules"`
	ApplicationLink     string        `json:"application_link"`
	NoticeFile          string        `json:"notice_file"`
}

func (r *Repository) GetAllExamEntries(search, level, stream, status string) ([]Exam, error) {
	var exams []Exam
	query := r.db.Model(&Exam{})
	if search != "" {
		query = query.Where("title ILIKE ? OR university ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if level != "" {
		query = query.Where("level = ?", level)
	}
	if stream != "" {
		query = query.Where("stream = ?", stream)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Order("created_at DESC").Find(&exams).Error
	return exams, err
}

func (r *Repository) GetPublishedInstitutionEntrances(search string) ([]InstitutionEntranceEntry, error) {
	var entries []InstitutionEntranceEntry
	query := r.db.Table("institution_entrances").
		Select(`institution_entrances.id, institution_entrances.title, institution_entrances.description, 
			institution_entrances.date, institution_entrances.hero_banner, institution_entrances.status,
			institution_entrances.application_fee as fee, institution_entrances.overview_details,
			institution_entrances.exam_date_schedules, institution_entrances.application_link,
			institution_entrances.notice_file,
			iu.institution_name, iu.logo_url as institution_logo, iu.district as institution_location,
			iu.province as institution_province, iu.website_url as institution_website,
			iu.contact_email as institution_email, iu.contact_phone as institution_phone`).
		Joins("LEFT JOIN institution_users iu ON iu.id = institution_entrances.institution_id").
		Where("institution_entrances.status = ?", "published")
	if search != "" {
		query = query.Where("institution_entrances.title ILIKE ?", "%"+search+"%")
	}
	err := query.Order("institution_entrances.created_at DESC").Find(&entries).Error
	return entries, err
}

func (r *Repository) GetEntranceFilterCounts() (FilterCounts, error) {
	var levels []string
	var streams []string
	var statuses []string

	r.db.Model(&Exam{}).Distinct("level").Pluck("level", &levels)
	r.db.Model(&Exam{}).Distinct("stream").Pluck("stream", &streams)
	r.db.Model(&Exam{}).Distinct("status").Pluck("status", &statuses)

	return FilterCounts{
		Levels:  levels,
		Streams: streams,
		Status:  statuses,
	}, nil
}

func (r *Repository) GetPublicEntranceByID(id string) (*Exam, error) {
	var exam Exam
	err := r.db.Where("id = ? OR slug = ?", id, id).First(&exam).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &exam, nil
}

func (r *Repository) GetInstitutionEntranceByID(id string) (*InstitutionEntranceEntry, error) {
	var entry InstitutionEntranceEntry
	err := r.db.Table("institution_entrances").
		Select(`institution_entrances.id, institution_entrances.title, institution_entrances.description, 
			institution_entrances.date, institution_entrances.hero_banner, institution_entrances.status,
			institution_entrances.application_fee as fee, institution_entrances.overview_details,
			institution_entrances.exam_date_schedules, institution_entrances.application_link,
			institution_entrances.notice_file,
			iu.institution_name, iu.logo_url as institution_logo, iu.district as institution_location,
			iu.province as institution_province, iu.website_url as institution_website,
			iu.contact_email as institution_email, iu.contact_phone as institution_phone`).
		Joins("LEFT JOIN institution_users iu ON iu.id = institution_entrances.institution_id").
		Where("institution_entrances.id = ? AND institution_entrances.status = ?", id, "published").
		First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

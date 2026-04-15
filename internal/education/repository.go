package education

import (
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

func (r *Repository) FindCourses() ([]Course, error) {
	var courses []Course
	err := r.db.Find(&courses).Error
	return courses, err
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

func (r *Repository) FindEventByID(id string) (*Event, error) {
	var event Event
	err := r.db.First(&event, id).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) FindBlogs(page, limit int, category, search string) ([]Blog, int64, error) {
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

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var blogs []Blog
	err := query.Order("featured desc, created_at desc").Offset(offset).Limit(limit).Find(&blogs).Error
	return blogs, total, err
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

func (r *Repository) FindAllBlogsAdmin(page, limit int, category, search string) ([]Blog, int64, error) {
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
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&blogs).Error
	return blogs, total, err
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

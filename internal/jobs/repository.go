package jobs

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateJob(job *Job) error {
	return r.db.Create(job).Error
}

func (r *Repository) FindJobByID(id uint) (*Job, error) {
	var job Job
	err := r.db.First(&job, id).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *Repository) UpdateJob(job *Job) error {
	return r.db.Save(job).Error
}

func (r *Repository) DeleteJob(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var applications []JobApplication
		if err := tx.Where("job_id = ?", id).Find(&applications).Error; err != nil {
			return err
		}
		if err := tx.Where("job_id = ?", id).Delete(&JobApplication{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Job{}, id).Error
	})
}

func (r *Repository) ListPublishedJobs(department, search string, page, limit int) ([]Job, int64, error) {
	var jobs []Job
	var total int64

	query := r.db.Where("status = ?", "published")
	if department != "" {
		query = query.Where("department = ?", department)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(title LIKE ? OR description LIKE ? OR location LIKE ?)", like, like, like)
	}

	if err := query.Model(&Job{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&jobs).Error
	return jobs, total, err
}

func (r *Repository) ListAllJobs(status, search string, page, limit int) ([]Job, int64, error) {
	var jobs []Job
	var total int64

	query := r.db.Model(&Job{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(title LIKE ? OR department LIKE ? OR location LIKE ?)", like, like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&jobs).Error
	return jobs, total, err
}

func (r *Repository) GetJobApplicationCount(jobID uint) (int64, error) {
	var count int64
	err := r.db.Model(&JobApplication{}).Where("job_id = ?", jobID).Count(&count).Error
	return count, err
}

func (r *Repository) GetJobApplicationCounts(jobIDs []uint) map[uint]int64 {
	counts := make(map[uint]int64)
	var results []struct {
		JobID uint
		Count int64
	}
	r.db.Model(&JobApplication{}).
		Select("job_id, count(*) as count").
		Where("job_id IN ?", jobIDs).
		Group("job_id").
		Scan(&results)
	for _, r := range results {
		counts[r.JobID] = r.Count
	}
	return counts
}

func (r *Repository) CreateApplication(app *JobApplication) error {
	return r.db.Create(app).Error
}

func (r *Repository) FindApplicationByID(id uint) (*JobApplication, error) {
	var app JobApplication
	err := r.db.Preload("Job").First(&app, id).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) UpdateApplication(app *JobApplication) error {
	return r.db.Save(app).Error
}

func (r *Repository) ListApplicationsByJob(jobID uint, status, search string, page, limit int) ([]JobApplication, int64, error) {
	var apps []JobApplication
	var total int64

	query := r.db.Where("job_id = ?", jobID).Preload("Job")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(full_name LIKE ? OR email LIKE ? OR phone LIKE ?)", like, like, like)
	}

	if err := query.Model(&JobApplication{}).Where("job_id = ?", jobID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&apps).Error
	return apps, total, err
}

func (r *Repository) ListApplicationsByJobForFiles(jobID uint) ([]JobApplication, error) {
	var apps []JobApplication
	err := r.db.Where("job_id = ?", jobID).Find(&apps).Error
	return apps, err
}

func (r *Repository) GetDepartments() ([]string, error) {
	var departments []string
	err := r.db.Model(&Job{}).
		Where("status = ?", "published").
		Distinct("department").
		Pluck("department", &departments).Error
	return departments, err
}

func (r *Repository) ApplicationExists(jobID uint, email string) bool {
	var count int64
	r.db.Model(&JobApplication{}).Where("job_id = ? AND email = ?", jobID, email).Count(&count)
	return count > 0
}

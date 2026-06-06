package university

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(search, uniType, status string, popular bool) ([]University, error) {
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

	err := query.Order("rank ASC").Find(&universities).Error
	return universities, err
}

func (r *Repository) FindByID(id uint) (*University, error) {
	var uni University
	err := r.db.Select("id, name, logo, location, type, rank, rating, review_count, verified, popular, description, established, students, chancellor, vice_chancellor, founder, website, cover").First(&uni, id).Error
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

func (r *Repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&University{}, id).Error
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

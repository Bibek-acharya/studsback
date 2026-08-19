package feedback

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(feedback *Feedback) error {
	return r.db.Create(feedback).Error
}

func (r *Repository) FindAll() ([]Feedback, error) {
	var feedbacks []Feedback
	err := r.db.Order("created_at DESC").Find(&feedbacks).Error
	return feedbacks, err
}

func (r *Repository) FindPublic(limit int) ([]Feedback, error) {
	var feedbacks []Feedback
	err := r.db.Where("experience != ''").Order("rating DESC, created_at DESC").Limit(limit).Find(&feedbacks).Error
	return feedbacks, err
}

func (r *Repository) FindByUserID(userID uint) ([]Feedback, error) {
	var feedbacks []Feedback
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&feedbacks).Error
	return feedbacks, err
}

func (r *Repository) HasUserSubmitted(userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&Feedback{}).Where("user_id = ?", userID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) Delete(id uint) error {
	return r.db.Delete(&Feedback{}, id).Error
}

type UserProfile struct {
	FirstName string
	LastName  string
	ImageURL  string
}

func (r *Repository) GetUserProfiles(userIDs []uint) (map[uint]UserProfile, error) {
	var users []struct {
		ID        uint
		FirstName string
		LastName  string
		ImageURL  string
	}
	if len(userIDs) == 0 {
		return map[uint]UserProfile{}, nil
	}
	if err := r.db.Table("users").Select("id, first_name, last_name, image_url").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]UserProfile, len(users))
	for _, u := range users {
		name := u.FirstName
		if u.LastName != "" {
			name += " " + u.LastName
		}
		result[u.ID] = UserProfile{FirstName: name, ImageURL: u.ImageURL}
	}
	return result, nil
}

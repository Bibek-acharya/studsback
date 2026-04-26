package auth

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindUserByEmail(email string) (*User, error) {
	var user User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateUser(user *User) error {
	return r.db.Create(user).Error
}

func (r *Repository) SaveUser(user *User) error {
	return r.db.Save(user).Error
}

func (r *Repository) FindUserByID(id uint) (*User, error) {
	var user User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) UpdatePreferences(user *User, prefs *Preferences) error {
	return r.db.Model(user).Update("preferences", prefs).Error
}

func (r *Repository) FindInstitutionUserByEmail(email string) (*InstitutionUser, error) {
	var user InstitutionUser
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindInstitutionUserByRegistrationNumber(reg string) (*InstitutionUser, error) {
	var user InstitutionUser
	err := r.db.Where("registration_number = ?", reg).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateInstitutionUser(user *InstitutionUser) error {
	return r.db.Create(user).Error
}

func (r *Repository) FindInstitutionUserByID(id uint) (*InstitutionUser, error) {
	var user InstitutionUser
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindScholarshipProviderUserByEmail(email string) (*ScholarshipProviderUser, error) {
	var user ScholarshipProviderUser
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindScholarshipProviderUserByRegistrationNumber(reg string) (*ScholarshipProviderUser, error) {
	var user ScholarshipProviderUser
	err := r.db.Where("registration_number = ?", reg).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateScholarshipProviderUser(user *ScholarshipProviderUser) error {
	return r.db.Create(user).Error
}

func (r *Repository) FindScholarshipProviderUserByID(id uint) (*ScholarshipProviderUser, error) {
	var user ScholarshipProviderUser
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindInstitutionUserByEmailOrGoogleID(email, googleID string) (*InstitutionUser, error) {
	var user InstitutionUser
	err := r.db.Where("email = ? OR google_id = ?", email, googleID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindScholarshipProviderUserByEmailOrGoogleID(email, googleID string) (*ScholarshipProviderUser, error) {
	var user ScholarshipProviderUser
	err := r.db.Where("email = ? OR google_id = ?", email, googleID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindScholarshipProvidersByStatus(status string) ([]ScholarshipProviderUser, error) {
	var users []ScholarshipProviderUser
	err := r.db.Where("status = ?", status).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *Repository) UpdateScholarshipProviderUser(user *ScholarshipProviderUser) error {
	return r.db.Save(user).Error
}

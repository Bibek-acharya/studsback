package auth

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
	user.Preferences = prefs
	return r.db.Save(user).Error
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

func (r *Repository) DeleteScholarshipProviderUser(id uint) error {
	result := r.db.Unscoped().Delete(&ScholarshipProviderUser{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("provider not found")
	}
	return nil
}

func (r *Repository) FindEducationEntriesByUserID(userID uint) ([]EducationEntry, error) {
	var entries []EducationEntry
	result := r.db.Where("user_id = ?", userID).Order("created_at desc").Find(&entries)
	return entries, result.Error
}

func (r *Repository) FindEducationEntryByID(id, userID uint) (*EducationEntry, error) {
	var entry EducationEntry
	result := r.db.Where("id = ? AND user_id = ?", id, userID).First(&entry)
	if result.Error != nil {
		return nil, result.Error
	}
	return &entry, nil
}

func (r *Repository) CreateEducationEntry(entry *EducationEntry) error {
	return r.db.Create(entry).Error
}

func (r *Repository) SaveEducationEntry(entry *EducationEntry) error {
	return r.db.Save(entry).Error
}

func (r *Repository) DeleteEducationEntry(id, userID uint) error {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&EducationEntry{})
	if result.RowsAffected == 0 {
		return errors.New("not found")
	}
	return result.Error
}

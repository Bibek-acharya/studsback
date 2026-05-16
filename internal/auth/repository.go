package auth

import (
	"errors"

	"studsphere/backend/internal/institution"
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

func (r *Repository) CreateInstitutionSettings(settings *institution.InstitutionSettings) error {
	return r.db.Create(settings).Error
}

func (r *Repository) FindInstitutionUserByID(id uint) (*InstitutionUser, error) {
	var user InstitutionUser
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) UpdateInstitutionUser(user *InstitutionUser) error {
	return r.db.Save(user).Error
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

func (r *Repository) FindInstitutionUsersByStatus(status string) ([]InstitutionUser, error) {
	var users []InstitutionUser
	err := r.db.Preload("Subscription").Where("status = ?", status).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *Repository) FindInstitutionUsersFiltered(status string, filter InstitutionFilter) ([]InstitutionUser, map[string]int64, error) {
	baseQuery := r.db.Table("institution_users").Where("institution_users.status = ?", status)

	if filter.Search != "" {
		s := "%" + filter.Search + "%"
		baseQuery = baseQuery.Where("institution_users.institution_name LIKE ? OR institution_users.registration_number LIKE ?", s, s)
	}
	if filter.Type != "" {
		baseQuery = baseQuery.Where("institution_users.organization_type = ?", filter.Type)
	}
	if filter.Province != "" {
		baseQuery = baseQuery.Where("institution_users.province = ?", filter.Province)
	}
	if filter.Verification == "verified" {
		baseQuery = baseQuery.Where("institution_users.verified = ?", true)
	} else if filter.Verification == "not_verified" {
		baseQuery = baseQuery.Where("institution_users.verified = ?", false)
	}
	if filter.Claim == "claimed" {
		baseQuery = baseQuery.Where("institution_users.claimed = ?", true)
	} else if filter.Claim == "unclaimed" {
		baseQuery = baseQuery.Where("institution_users.claimed = ?", false)
	}
	if filter.Level != "" {
		baseQuery = baseQuery.Where("institution_users.level = ?", filter.Level)
	}
	if filter.PaymentStatus != "" {
		baseQuery = baseQuery.Joins("LEFT JOIN institution_subscriptions ON institution_subscriptions.institution_id = institution_users.id").
			Where("institution_subscriptions.status = ?", filter.PaymentStatus)
	}

	var levelCounts []struct {
		Level string `gorm:"column:level"`
		Count int64  `gorm:"column:count"`
	}
	r.db.Table("institution_users").Select("level, COUNT(*) as count").
		Where("status = ?", status).
		Group("level").Scan(&levelCounts)

	countsMap := map[string]int64{}
	for _, lc := range levelCounts {
		if lc.Level != "" {
			countsMap[lc.Level] = lc.Count
		}
	}

	var users []InstitutionUser
	err := baseQuery.Preload("Subscription").Find(&users).Error
	if err != nil {
		return nil, nil, err
	}

	return users, countsMap, nil
}

func (r *Repository) FindAllInstitutions() ([]InstitutionUser, error) {
	var users []InstitutionUser
	err := r.db.Preload("Subscription").Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *Repository) FindSubscriptionByInstitutionID(institutionID uint) (*InstitutionSubscription, error) {
	var sub InstitutionSubscription
	err := r.db.Where("institution_id = ?", institutionID).First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *Repository) CreateOrUpdateSubscription(sub *InstitutionSubscription) error {
	var existing InstitutionSubscription
	result := r.db.Where("institution_id = ?", sub.InstitutionID).First(&existing)
	if result.Error != nil {
		return r.db.Create(sub).Error
	}
	sub.ID = existing.ID
	sub.CreatedAt = existing.CreatedAt
	return r.db.Save(sub).Error
}

func (r *Repository) DeleteInstitutionUser(id uint) error {
	result := r.db.Unscoped().Delete(&InstitutionUser{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("institution not found")
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

func (r *Repository) FindInstitutionUsersByStatusAndCollegeID(status string, collegeID uint, operators ...string) ([]InstitutionUser, error) {
	var users []InstitutionUser
	op := "="
	if len(operators) > 0 {
		op = operators[0]
	}
	err := r.db.Preload("Subscription").Where("status = ? AND college_id "+op+" ?", status, collegeID).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *Repository) UpdateCollegeClaimed(collegeID uint, claimed bool) error {
	return r.db.Table("colleges").Where("id = ?", collegeID).Update("claimed", claimed).Error
}

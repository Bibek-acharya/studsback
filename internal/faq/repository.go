package faq

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAllCategories() ([]FAQCategory, error) {
	var cats []FAQCategory
	err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("\"order\" ASC, id ASC")
	}).Order("\"order\" ASC, id ASC").Find(&cats).Error
	return cats, err
}

func (r *Repository) FindCategoryByID(id uint) (*FAQCategory, error) {
	var cat FAQCategory
	err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("\"order\" ASC, id ASC")
	}).First(&cat, id).Error
	if err != nil {
		return nil, err
	}
	return &cat, nil
}

func (r *Repository) CreateCategory(cat *FAQCategory) error {
	return r.db.Create(cat).Error
}

func (r *Repository) UpdateCategory(cat *FAQCategory) error {
	return r.db.Save(cat).Error
}

func (r *Repository) DeleteCategory(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("category_id = ?", id).Delete(&FAQItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&FAQCategory{}, id).Error
	})
}

func (r *Repository) CreateItem(item *FAQItem) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateItem(item *FAQItem) error {
	return r.db.Save(item).Error
}

func (r *Repository) DeleteItem(id uint) error {
	return r.db.Delete(&FAQItem{}, id).Error
}

func (r *Repository) FindItemByID(id uint) (*FAQItem, error) {
	var item FAQItem
	err := r.db.First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

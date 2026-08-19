package seeder

import (
	"studsphere/backend/internal/forum"

	"gorm.io/gorm"
)

func SeedForum(db *gorm.DB) error {
	var existing forum.ForumCommunity
	err := db.Where("name = ?", "General").First(&existing).Error
	if err == nil {
		db.Model(&existing).Updates(map[string]interface{}{
			"description": "A public feed for everyone on campus",
			"icon":        "globe",
			"bg_color":    "#4F46E5",
			"is_general":  true,
		})
		return nil
	}

	general := forum.ForumCommunity{
		Name:        "General",
		Description: "A public feed for everyone on campus",
		Icon:        "globe",
		BgColor:     "#4F46E5",
		IsGeneral:   true,
	}
	return db.Create(&general).Error
}

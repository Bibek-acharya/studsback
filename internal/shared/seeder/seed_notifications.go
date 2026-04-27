package seeder

import (
	"studsphere/backend/internal/system"

	"gorm.io/gorm"
)

func SeedPublicNotifications(db *gorm.DB) error {
	notifications := []system.PublicNotification{
		{
			Title:   "New Feature Added! 🎉",
			Message: "Dark mode is now available. Check your settings to toggle your theme.",
			Type:    "system",
			Active:  true,
			Icon:    "fa-sparkles",
			Color:   "text-indigo-500",
			BgColor: "bg-indigo-100",
		},
		{
			Title:   "College Match Tool Updated",
			Message: "Our college recommender now uses AI to find better matches based on your preferences.",
			Type:    "info",
			Active:  true,
			Icon:    "fa-wand-magic-sparkles",
			Color:   "text-blue-500",
			BgColor: "bg-blue-100",
		},
		{
			Title:   "Scholarship Application Deadline",
			Message: "National Merit Scholarship applications close on May 15. Apply now!",
			Type:    "alert",
			Active:  true,
			Icon:    "fa-award",
			Color:   "text-yellow-500",
			BgColor: "bg-yellow-100",
		},
		{
			Title:   "Campus Feed is Live",
			Message: "Connect with fellow students and share your campus experience on the new Campus Feed.",
			Type:    "info",
			Active:  true,
			Icon:    "fa-comments",
			Color:   "text-green-500",
			BgColor: "bg-green-100",
		},
		{
			Title:   "Configuration Complete",
			Message: "Congratulations! You have successfully created your Studsphere account.",
			Type:    "system",
			Active:  true,
			Icon:    "fa-settings",
			Color:   "text-gray-600",
			BgColor: "bg-gray-100",
		},
		{
			Title:   "Admission Deadlines Approaching",
			Message: "Many colleges have early admission deadlines in the coming weeks. Plan ahead!",
			Type:    "alert",
			Active:  true,
			Icon:    "fa-school",
			Color:   "text-orange-500",
			BgColor: "bg-orange-100",
		},
	}

	for _, n := range notifications {
		if err := db.Where("title = ?", n.Title).FirstOrCreate(&n).Error; err != nil {
			return err
		}
	}

	return nil
}

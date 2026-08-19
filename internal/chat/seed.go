package chat

import (
	"errors"
	"log"

	"gorm.io/gorm"
)

func SeedSitePages(db *gorm.DB) error {
	pages := []SitePage{
		{
			Slug:  "about-us",
			Title: "About StudSphere",
			Content: `StudSphere is Nepal's leading college discovery and scholarship platform. ` +
				`We help students find the perfect college, compare courses, and apply for scholarships. ` +
				`Our platform lists hundreds of colleges across Nepal, including Tribhuvan University affiliated colleges, ` +
				`Pokhara University, Kathmandu University, Purbanchal University, and more. ` +
				`We offer college finder tools, course finder, scholarship finder, college comparison, ` +
				`campus forum, and counselling services. ` +
				`Our partners include Project Shiksha (Sowers Action Nepal), Sowers Hong Kong, RONB, Ncell, ` +
				`Creating Opportunities, and Dari Club USA. ` +
				`We have scholarship programs like Project Shiksha that provide full financial support, ` +
				`accommodation, meals, and mentoring to deserving students.`,
		},
		{
			Slug:  "contact-us",
			Title: "Contact StudSphere",
			Content: `You can contact StudSphere through our website contact form. ` +
				`Visit the Contact Us page on StudSphere.com to submit inquiries. ` +
				`We handle inquiries about college admissions, scholarships, partnerships, and general support.` +
				`Our team is available to help students find the right college and scholarship opportunities in Nepal.`,
		},
		{
			Slug:  "faq",
			Title: "Frequently Asked Questions",
			Content: `StudSphere helps students discover colleges and scholarships in Nepal. ` +
				`Common topics include: how to find colleges using the college finder tool, ` +
				`how to compare colleges side by side, how to use the scholarship finder to discover financial aid, ` +
				`how to apply for scholarships through the platform, ` +
				`how to use the course finder to explore programs by field and level, ` +
				`how to book counselling sessions for personalized guidance, ` +
				`and how the college recommender works based on student preferences. ` +
				`Students can browse colleges by location (Kathmandu, Pokhara, Chitwan, Lalitpur, Bhaktapur, etc.), ` +
				`by type (private, public, community, constituent, foreign affiliated), ` +
				`and by program level (+2, bachelor, master, diploma, A Level).`,
		},
	}

	var lastErr error
	for _, page := range pages {
		var existing SitePage
		result := db.Where("slug = ?", page.Slug).First(&existing)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			if err := db.Create(&page).Error; err != nil {
				log.Printf("Failed to seed site_page %s: %v", page.Slug, err)
				lastErr = err
			} else {
				log.Printf("Seeded site_page: %s", page.Slug)
			}
		} else if result.Error != nil {
			log.Printf("Failed to check site_page %s: %v", page.Slug, result.Error)
			lastErr = result.Error
		} else {
			if err := db.Model(&existing).Updates(map[string]interface{}{
				"title":   page.Title,
				"content": page.Content,
			}).Error; err != nil {
				log.Printf("Failed to update site_page %s: %v", page.Slug, err)
				lastErr = err
			}
		}
	}
	return lastErr
}

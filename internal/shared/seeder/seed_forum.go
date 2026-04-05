package seeder

import (
	"log"

	"studsphere/backend/internal/forum"
	"studsphere/backend/internal/shared/config"
)

func SeedForum() error {
	db := config.GetDB()

	// 1. Seed Communities
	var commCount int64
	db.Model(&forum.ForumCommunity{}).Count(&commCount)
	if commCount == 0 {
		communities := []forum.ForumCommunity{
			{Name: "IOE Engineering Prep", Emoji: "📐", BgColor: "bg-orange-100"},
			{Name: "IT (CSIT/BCA/BIT)", Emoji: "💻", BgColor: "bg-blue-100"},
			{Name: "CEE Medical Prep", Emoji: "🩺", BgColor: "bg-green-100"},
			{Name: "Kathmandu University", Emoji: "🏛️", BgColor: "bg-purple-100"},
			{Name: "Tribhuvan University", Emoji: "🎒", BgColor: "bg-yellow-100"},
			{Name: "Academics", Emoji: "📚", BgColor: "bg-indigo-100"},
			{Name: "General Discussion", Emoji: "💬", BgColor: "bg-gray-100"},
		}

		for _, comm := range communities {
			if err := db.Create(&comm).Error; err != nil {
				log.Printf("Error seeding community %s: %v", comm.Name, err)
			}
		}
		log.Println("Forum communities seeded successfully")
	}

	// 2. Seed Posts
	var postCount int64
	db.Model(&forum.ForumPost{}).Count(&postCount)
	if postCount > 0 {
		return nil
	}

	// Find a user to assign posts to
	var user forum.User
	if err := db.First(&user).Error; err != nil {
		log.Println("No users found to seed forum posts")
		return nil
	}

	// Find communities to link
	var ioe forum.ForumCommunity
	db.Where("name = ?", "IOE Engineering Prep").First(&ioe)

	var it forum.ForumCommunity
	db.Where("name = ?", "IT (CSIT/BCA/BIT)").First(&it)

	var cee forum.ForumCommunity
	db.Where("name = ?", "CEE Medical Prep").First(&cee)

	posts := []forum.ForumPost{
		{
			UserID:       user.ID,
			CommunityID:  it.ID,
			Category:     "Discussion",
			Title:        "BSc. CSIT vs BCA: Which one is better for Software Engineering?",
			Content:      "I'm confused between CSIT and BCA. I want to become a full-stack developer. Which course offers better depth in programming and math?",
			Upvotes:      124,
			CommentCount: 45,
		},
		{
			UserID:       user.ID,
			CommunityID:  ioe.ID,
			Category:     "Exam Update",
			Title:        "IOE Entrance Exam 2081 expected dates?",
			Content:      "Has anyone heard anything about the IOE entrance exam dates for this year? Usually it happens in Bhadra but there are rumors it might be earlier.",
			Upvotes:      89,
			CommentCount: 22,
		},
		{
			UserID:       user.ID,
			CommunityID:  cee.ID,
			Category:     "Medical Prep",
			Title:        "Best Biology book for CEE?",
			Content:      "Is NCERT enough for CEE Biology or should I follow local books like Trueman or others?",
			Upvotes:      56,
			CommentCount: 15,
		},
	}

	for _, post := range posts {
		if err := db.Create(&post).Error; err != nil {
			return err
		}
	}

	log.Println("Forum posts seeded successfully")
	return nil
}

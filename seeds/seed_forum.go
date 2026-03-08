package seeds

import (
	"log"
	"studsphere/backend/config"
	"studsphere/backend/models"
)

func SeedForum() error {
	db := config.GetDB()

	// Check if we have any posts already
	var count int64
	db.Model(&models.ForumPost{}).Count(&count)
	if count > 0 {
		return nil
	}

	// Find a user to assign posts to (usually the first admin or student)
	var user models.User
	if err := db.First(&user).Error; err != nil {
		log.Println("No users found to seed forum posts")
		return nil
	}

	posts := []models.ForumPost{
		{
			UserID:       user.ID,
			Category:     "Scholarship",
			Title:        "Best resources for studying Data Structure in C for TU?",
			Content:      "I'm struggling with linked lists and trees in Data Structure (BIM 4th Sem, TU). Can anyone recommend the best Nepali authors or online courses that explain these topics clearly based on the TU syllabus?",
			Upvotes:      45,
			CommentCount: 12,
		},
		{
			UserID:       user.ID,
			Category:     "Academics",
			Title:        "How to prepare for IOM Entrance Exam?",
			Content:      "I'm planning to take the IOM entrance exam next year. What are the best coaching centers and books to follow?",
			Upvotes:      32,
			CommentCount: 8,
		},
		{
			UserID:       user.ID,
			Category:     "General",
			Title:        "Looking for study partners at Pulchowk Campus",
			Content:      "Hi everyone! I'm a first-year Civil Engineering student at Pulchowk. Looking for some study partners to go through Engineering Drawing and Math I. Anyone interested?",
			Upvotes:      15,
			CommentCount: 5,
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

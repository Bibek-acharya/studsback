package seeds

import (
	"log"

	"studsphere/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func SeedUniversities(db *gorm.DB) error {
	universities := []models.University{
		{
			Name:        "Tribhuvan University",
			Logo:        "https://ui-avatars.com/api/?name=TU&background=0A61EF&color=fff",
			Location:    "Kirtipur, Kathmandu",
			Type:        "Public",
			Rank:        1,
			Popular:     true,
			Description: "The first national institution of higher education in Nepal with broad constituent and affiliated colleges.",
			Established: "1959",
			Website:     "tu.edu.np",
		},
		{
			Name:        "Kathmandu University",
			Logo:        "https://ui-avatars.com/api/?name=KU&background=0284c7&color=fff",
			Location:    "Dhulikhel, Kavre",
			Type:        "Private",
			Rank:        2,
			Popular:     true,
			Description: "An autonomous, not-for-profit public university recognized for academic standards and research.",
			Established: "1991",
			Website:     "ku.edu.np",
		},
		{
			Name:        "Pokhara University",
			Logo:        "https://ui-avatars.com/api/?name=PU&background=ca8a04&color=fff",
			Location:    "Pokhara, Kaski",
			Type:        "Public",
			Rank:        3,
			Popular:     false,
			Description: "A major public university in western Nepal providing management, science, and technology programs.",
			Established: "1997",
			Website:     "pu.edu.np",
		},
	}

	for _, university := range universities {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"logo", "location", "type", "rank", "popular", "description", "established", "website", "updated_at"}),
		}).Create(&university).Error; err != nil {
			log.Printf("Error creating university %s: %v", university.Name, err)
			return err
		}
	}

	log.Println("Successfully seeded universities")
	return nil
}

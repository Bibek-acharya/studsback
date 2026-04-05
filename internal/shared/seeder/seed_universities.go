package seeder

import (
	"log"

	"studsphere/backend/internal/university"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func SeedUniversities(db *gorm.DB) error {
	universities := []university.University{
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
		{
			Name:        "University of London",
			Logo:        "https://ui-avatars.com/api/?name=UoL&background=6c1c1c&color=fff",
			Location:    "London, United Kingdom",
			Type:        "Public",
			Rank:        4,
			Popular:     true,
			Description: "A federal university in the UK with member institutions worldwide, offering globally recognized degrees.",
			Established: "1836",
			Website:     "london.ac.uk",
		},
		{
			Name:        "Coventry University",
			Logo:        "https://ui-avatars.com/api/?name=CU&background=cd1c2b&color=fff",
			Location:    "Coventry, United Kingdom",
			Type:        "Public",
			Rank:        5,
			Popular:     true,
			Description: "A modern university in the UK known for professional courses and industry connections.",
			Established: "1843",
			Website:     "coventry.ac.uk",
		},
		{
			Name:        "London Metropolitan University",
			Logo:        "https://ui-avatars.com/api/?name=LMU&background=2c3e50&color=fff",
			Location:    "London, United Kingdom",
			Type:        "Public",
			Rank:        6,
			Popular:     false,
			Description: "A UK university offering diverse programs with strong industry links in London.",
			Established: "1848",
			Website:     "londonmet.ac.uk",
		},
		{
			Name:        "University of the West of England",
			Logo:        "https://ui-avatars.com/api/?name=UWE&background=352e7e&color=fff",
			Location:    "Bristol, United Kingdom",
			Type:        "Public",
			Rank:        7,
			Popular:     false,
			Description: "A leading UK university known for professional and vocational programs.",
			Established: "1592",
			Website:     "uwe.ac.uk",
		},
		{
			Name:        "University of Wolverhampton",
			Logo:        "https://ui-avatars.com/api/?name=Wolv&background=622c88&color=fff",
			Location:    "Wolverhampton, United Kingdom",
			Type:        "Public",
			Rank:        8,
			Popular:     false,
			Description: "A modern university with strong career-focused programs and international reach.",
			Established: "1851",
			Website:     "wolverhampton.ac.uk",
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

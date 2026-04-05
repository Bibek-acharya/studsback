package seeder

import (
	"encoding/json"
	"log"
	"time"

	"studsphere/backend/internal/scholarship"

	"gorm.io/gorm"
)

func SeedScholarships(db *gorm.DB) error {
	// Check if scholarships already exist
	var count int64
	db.Model(&scholarship.Scholarship{}).Count(&count)
	if count > 0 {
		if err := db.Unscoped().Where("1 = 1").Delete(&scholarship.ScholarshipApplication{}).Error; err != nil {
			return err
		}
		if err := db.Unscoped().Where("1 = 1").Delete(&scholarship.Scholarship{}).Error; err != nil {
			return err
		}
	}

	fieldOfStudy, _ := json.Marshal([]string{
		"Computer Science & AI",
		"Environmental Science",
		"Public Health",
		"Business Administration",
	})

	selectionProcess, _ := json.Marshal([]map[string]string{
		{"stage": "Stage 1: Initial Screening", "description": "Applications are reviewed for eligibility and completeness. Incomplete applications are rejected immediately."},
		{"stage": "Stage 2: Academic Review", "description": "A panel of professors reviews academic transcripts, research proposals, and recommendation letters."},
		{"stage": "Stage 3: Interview", "description": "Shortlisted candidates are invited for a virtual interview with the scholarship committee in June."},
	})

	eligibilityCriteria, _ := json.Marshal([]map[string]string{
		{"criterion": "Nationality", "description": "Must be an international student from a non-EU country."},
		{"criterion": "Academic Merit", "description": "Must hold a First Class Honours degree or equivalent GPA (3.7/4.0)."},
		{"criterion": "Language Proficiency", "description": "IELTS score of 7.5 overall or TOEFL iBT score of 110."},
	})

	excludedRegions, _ := json.Marshal([]string{
		"United Kingdom",
		"Australia",
		"New Zealand",
		"USA (Specific state grants available instead)",
	})

	requiredDocuments, _ := json.Marshal([]map[string]string{
		{"name": "Academic Transcripts", "description": "Official copies from all universities attended."},
		{"name": "CV / Resume", "description": "Updated CV highlighting academic and leadership achievements."},
		{"name": "Recommendation Letters", "description": "Two academic references on official letterhead."},
		{"name": "Personal Statement", "description": "Max 1000 words outlining your goals and motivation."},
	})

	timeline, _ := json.Marshal([]map[string]string{
		{"date": "Jan 15, 2026", "event": "Applications Open"},
		{"date": "May 15, 2026", "event": "Submission Deadline"},
		{"date": "June 2026", "event": "Interview Stage"},
		{"date": "July 30, 2026", "event": "Results Announced"},
	})

	benefits, _ := json.Marshal([]map[string]string{
		{"title": "Tuition Coverage", "description": "100% of tuition fees covered for the duration of the 1-year Master's program."},
		{"title": "Living Stipend", "description": "Monthly living allowance of £1,400 to cover accommodation and expenses."},
		{"title": "Travel Grant", "description": "Round-trip economy airfare from home country to the UK."},
		{"title": "Health Insurance", "description": "Coverage for the NHS Immigration Health Surcharge."},
	})

	faqs, _ := json.Marshal([]map[string]string{
		{"question": "Is there an application fee?", "answer": "No, applying for the scholarship itself is free. However, there may be an application fee for the university admission process depending on the department."},
		{"question": "Can I apply if I am in my final year of Bachelor's?", "answer": "Yes, you can apply with your provisional transcripts. If selected, the offer will be conditional upon achieving the required final grades."},
		{"question": "Are part-time courses eligible?", "answer": "No, this specific scholarship is only available for full-time, on-campus Master's programs."},
	})

	parseDate := func(value string) time.Time {
		t, _ := time.Parse("2006-01-02", value)
		return t
	}

	scholarships := []scholarship.Scholarship{
		{
			Title:               "Global Future Leaders Scholarship 2026",
			Provider:            "Cambridge University, UK",
			Location:            "Cambridge, UK",
			Value:               "$30,000 / Year",
			Deadline:            parseDate("2026-05-15"),
			DegreeLevel:         "Masters",
			FundingType:         "Fully Funded",
			ScholarshipType:     "Merit Based",
			Description:         "Designed for high-achieving international students with leadership potential.",
			ImageURL:            "https://images.unsplash.com/photo-1523050854058-8df90110c9f1?ixlib=rb-1.2.1&auto=format&fit=crop&w=1200&q=80",
			FieldOfStudy:        fieldOfStudy,
			SelectionProcess:    selectionProcess,
			EligibilityCriteria: eligibilityCriteria,
			ExcludedRegions:     excludedRegions,
			RequiredDocuments:   requiredDocuments,
			Timeline:            timeline,
			Benefits:            benefits,
			FAQs:                faqs,
		},
		{
			Title:             "Nepal STEM Excellence Grant",
			Provider:          "Tech Nepal Foundation",
			Location:          "Kathmandu, Nepal",
			Value:             "NPR 400,000",
			Deadline:          parseDate("2026-04-10"),
			DegreeLevel:       "Bachelors",
			FundingType:       "Partial Tuition",
			ScholarshipType:   "Departmental",
			Description:       "Supports high-potential students entering computing, engineering, and AI programs in Nepal.",
			ImageURL:          "https://images.unsplash.com/photo-1519389950473-47ba0277781c?q=80&w=2070&auto=format&fit=crop",
			FieldOfStudy:      marshalJSON([]string{"Computer Science", "Engineering", "Data Science"}),
			RequiredDocuments: requiredDocuments,
		},
		{
			Title:             "Women in Research Scholarship",
			Provider:          "Global Science Alliance",
			Location:          "Pokhara, Nepal",
			Value:             "NPR 500,000",
			Deadline:          parseDate("2026-03-28"),
			DegreeLevel:       "Masters",
			FundingType:       "Need Based",
			ScholarshipType:   "Institutional Need",
			Description:       "Financial support for women pursuing research-oriented postgraduate study in STEM disciplines.",
			ImageURL:          "https://images.unsplash.com/photo-1573166368361-3f523199276e?q=80&w=2069&auto=format&fit=crop",
			FieldOfStudy:      marshalJSON([]string{"Biotechnology", "Physics", "Computer Science"}),
			RequiredDocuments: requiredDocuments,
		},
		{
			Title:             "TU Entrance Topper Scholarship",
			Provider:          "Tribhuvan University",
			Location:          "Kathmandu, Nepal",
			Value:             "Full Tuition Waiver",
			Deadline:          parseDate("2026-08-01"),
			DegreeLevel:       "Bachelors",
			FundingType:       "Fee Waiver",
			ScholarshipType:   "Entrance",
			Description:       "Awarded to top-ranked students in TU entrance exams across engineering and management faculties.",
			ImageURL:          "https://images.unsplash.com/photo-1541339907198-e08756dedf3f?q=80&w=2070&auto=format&fit=crop",
			FieldOfStudy:      marshalJSON([]string{"Engineering", "Management"}),
			RequiredDocuments: requiredDocuments,
		},
		{
			Title:             "Community Impact Scholarship",
			Provider:          "Youth Action Nepal",
			Location:          "Chitwan, Nepal",
			Value:             "NPR 250,000",
			Deadline:          parseDate("2026-06-05"),
			DegreeLevel:       "+2",
			FundingType:       "Grant",
			ScholarshipType:   "NGO / INGO",
			Description:       "For students with proven record in community service and social impact initiatives.",
			ImageURL:          "https://images.unsplash.com/photo-1488521787991-ed7bbaae773c?q=80&w=2070&auto=format&fit=crop",
			FieldOfStudy:      marshalJSON([]string{"Humanities", "Social Work", "Public Policy"}),
			RequiredDocuments: requiredDocuments,
		},
		{
			Title:             "PU School Partnership Scholarship",
			Provider:          "Pokhara University",
			Location:          "Pokhara, Nepal",
			Value:             "NPR 300,000",
			Deadline:          parseDate("2026-05-22"),
			DegreeLevel:       "Bachelors",
			FundingType:       "Partial Tuition",
			ScholarshipType:   "School-Based",
			Description:       "Scholarship for partner high school graduates enrolling in PU-affiliated bachelor programs.",
			ImageURL:          "https://images.unsplash.com/photo-1562774053-701939374585?q=80&w=2086&auto=format&fit=crop",
			FieldOfStudy:      marshalJSON([]string{"Business", "IT", "Health Sciences"}),
			RequiredDocuments: requiredDocuments,
		},
		{
			Title:             "College Merit Excellence Award",
			Provider:          "KIST College",
			Location:          "Lalitpur, Nepal",
			Value:             "NPR 200,000",
			Deadline:          parseDate("2026-04-30"),
			DegreeLevel:       "Bachelors",
			FundingType:       "Merit Scholarship",
			ScholarshipType:   "College-Based",
			Description:       "Merit scholarship for top-performing first-year students across all faculties.",
			ImageURL:          "https://images.unsplash.com/photo-1590012314607-cda9d9b699ae?q=80&w=2071&auto=format&fit=crop",
			FieldOfStudy:      marshalJSON([]string{"Business", "Computing", "Science"}),
			RequiredDocuments: requiredDocuments,
		},
		{
			Title:             "Higher Education Fee Relief Scheme",
			Provider:          "Education Ministry Nepal",
			Location:          "Nepal",
			Value:             "Up to 80% Fee Waiver",
			Deadline:          parseDate("2026-09-15"),
			DegreeLevel:       "Bachelors",
			FundingType:       "Fee Waiver",
			ScholarshipType:   "Institutional Need",
			Description:       "Need-based support for economically disadvantaged students joining higher education programs.",
			ImageURL:          "https://images.unsplash.com/photo-1524995997946-a1c2e315a42f?q=80&w=2070&auto=format&fit=crop",
			FieldOfStudy:      marshalJSON([]string{"Any"}),
			RequiredDocuments: requiredDocuments,
		},
	}

	for _, scholarship := range scholarships {
		entry := scholarship
		if err := db.Create(&entry).Error; err != nil {
			log.Printf("Error seeding scholarship %s: %v", scholarship.Title, err)
			return err
		}
	}

	log.Println("Successfully seeded scholarships")
	return nil
}

package seeder

import (
	"log"
	"time"

	"studsphere/backend/internal/education"

	"gorm.io/gorm"
)

type CurriculumSemester struct {
	Semester int      `json:"semester"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle"`
	Subjects []string `json:"subjects"`
}

type CareerOpportunity struct {
	Title string `json:"title"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

func SeedExams(db *gorm.DB) error {
	exams := []education.Exam{
		{
			Slug:         "neb-class-12",
			Title:        "NEB Class 12 Annual Examination 2081",
			Board:        "NEB (National Examination Board)",
			Badges:       marshalJSON([]string{"BOARD EXAM", "UPCOMING"}),
			Level:        "+2 / Intermediate",
			Type:         "Board Exam",
			ExamDate:     "Baishakh 14, 2082 (Apr 27, 2025)",
			ExamDateAD:   time.Date(2025, 4, 27, 0, 0, 0, 0, time.UTC),
			FormDeadline: "Magh 20, 2081 (Feb 3, 2025)",
			Fee:          "NPR 600",
			Highlights:   marshalJSON([]string{"Official routine published", "Form filling active", "Admit cards by Chaitra end"}),
			Description:  "Annual final examination for Grade 12 students across Nepal.",
			Status:       "active",
			ImageUrl:     "https://images.unsplash.com/photo-1434030216411-0b793f4b4173?q=80&w=800",
			University:   "National Board",
			Faculty:      "All Streams",
			NepaliDate:   "Asoj 01, 2082",
			Overview:     "The final gateway for secondary education in Nepal. Mandatory for higher studies.",
			Weightage:    marshalJSON([]map[string]interface{}{{"label": "Theory", "marks": 75, "color": "bg-brand-500", "width": "75%"}, {"label": "Practical", "marks": 25, "color": "bg-emerald-500", "width": "25%"}}),
		},
		{
			Slug:         "ioe-entrance",
			Title:        "IOE Entrance Examination 2081",
			Board:        "Institute of Engineering, TU",
			Badges:       marshalJSON([]string{"ENTRANCE", "POPULAR"}),
			Level:        "Undergraduate (Bachelor)",
			Type:         "Entrance Exam",
			ExamDate:     "Jestha 2082 (May/June 2025)",
			ExamDateAD:   time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			FormDeadline: "Chaitra 2081 (Mar/Apr 2025)",
			Fee:          "NPR 2000",
			Highlights:   marshalJSON([]string{"BE Computer, Civil, Arch", "140 MCQ Questions", "Pulchowk, Thapathali seats"}),
			Description:  "Entrance exam for BE programs at IOE constituent campuses.",
			Status:       "upcoming",
			ImageUrl:     "https://images.unsplash.com/photo-1517694712202-14dd9538aa97?q=80&w=800",
			University:   "Tribhuvan University",
			Faculty:      "IOE Pulchowk",
			NepaliDate:   "Mangsir 25, 2082",
			Overview:     "The gateway to Pulchowk Campus and other constituent engineering colleges. Get details on syllabus, shifts, and admit cards.",
			Weightage:    marshalJSON([]map[string]interface{}{{"label": "Mathematics", "marks": 50, "color": "bg-brand-500", "width": "35%"}, {"label": "Physics", "marks": 45, "color": "bg-blue-500", "width": "32%"}, {"label": "Chemistry", "marks": 25, "color": "bg-teal-500", "width": "18%"}, {"label": "English & Aptitude", "marks": 20, "color": "bg-purple-500", "width": "15%"}}),
		},
	}

	for _, exam := range exams {
		if err := db.Where(education.Exam{Slug: exam.Slug}).FirstOrCreate(&exam).Error; err != nil {
			log.Printf("Error seeding exam %s: %v", exam.Title, err)
		}
	}
	log.Println("Successfully seeded exams")
	return nil
}

func SeedCourses(db *gorm.DB) error {
	courses := []education.Course{
		{
			Title:         "B.Sc CSIT (Computer Science & IT)",
			ShortTitle:    "BSc CSIT",
			CollegesCount: 0,
			Affiliation:   "TU Affiliated",
			Badges:        marshalJSON([]string{"Top Choice", "High Growth"}),
			Level:         "Bachelor",
			Field:         "IT / Computing",
			Duration:      "4 Years",
			EstFee:        "NPR 4L - 8L",
			Highlights:    marshalJSON([]string{"Merit Scholarships (20 Seats)", "Internship Guaranteed", "Practical Based"}),
			CareerPath:    "Software Engineer, System Analyst, AI Researcher",
			Description:   "Build a strong foundation in software development, networking, databases, and modern IT systems.",
			Location:      "Available in Nepal",
			GovtFee:       "NPR 3,50,000",
			PrivateFee:    "NPR 8,50,000 - 12,00,000",
			Mode:          "On-Campus",
			DegreeLabel:   "Bachelor's Degree",
			About: marshalJSON([]string{
				"B.Sc CSIT is designed to build strong fundamentals in computing with equal focus on theory and practical implementation.",
				"Students learn software engineering, data structures, databases, networking, and modern development practices through labs and projects.",
			}),
			Curriculum: marshalJSON([]CurriculumSemester{
				{Semester: 1, Title: "Semester 1", Subtitle: "Foundations", Subjects: []string{"Introduction to IT", "C Programming", "Mathematics I"}},
				{Semester: 2, Title: "Semester 2", Subtitle: "Core Skills", Subjects: []string{"OOP in C++", "Discrete Structures", "Digital Logic"}},
				{Semester: 3, Title: "Semester 3", Subtitle: "Systems", Subjects: []string{"Data Structures", "Computer Architecture", "Statistics"}},
				{Semester: 4, Title: "Semester 4", Subtitle: "Software Core", Subjects: []string{"DBMS", "Operating Systems", "Microprocessor"}},
				{Semester: 5, Title: "Semester 5", Subtitle: "Applied Development", Subjects: []string{"Computer Networks", "Web Technology", "Software Engineering"}},
				{Semester: 6, Title: "Semester 6", Subtitle: "Advanced Topics", Subjects: []string{"Compiler Design", "E-Governance", "NET Centric Computing"}},
				{Semester: 7, Title: "Semester 7", Subtitle: "Specialization", Subjects: []string{"Artificial Intelligence", "Internship", "Elective I"}},
				{Semester: 8, Title: "Semester 8", Subtitle: "Capstone", Subjects: []string{"Advanced Java", "Project Work", "Elective II"}},
			}),
			Admissions: marshalJSON([]string{"Mark sheets / Transcripts", "Citizenship / ID proof", "Passport-size photos", "Migration and character certificate"}),
			Careers: marshalJSON([]CareerOpportunity{
				{Title: "Software Engineer", Icon: "cpu", Color: "emerald"},
				{Title: "Data Analyst", Icon: "chart", Color: "purple"},
				{Title: "Backend Developer", Icon: "database", Color: "blue"},
			}),
		},
		{
			Title:         "BIT (Bachelor in IT)",
			ShortTitle:    "BIT",
			CollegesCount: 0,
			Affiliation:   "Foreign Degree",
			Badges:        marshalJSON([]string{"Global Value", "Industry Ready"}),
			Level:         "Bachelor",
			Field:         "IT / Computing",
			Duration:      "4 Years",
			EstFee:        "NPR 6L - 10L",
			Highlights:    marshalJSON([]string{"Direct Entry", "Job Assistance", "Dual Certification"}),
			CareerPath:    "IT Consultant, Cloud Architect, Web Developer",
			Description:   "Comprehensive program designed to prepare you for a successful global tech career.",
			Location:      "Available in Nepal",
			GovtFee:       "NPR 3,50,000",
			PrivateFee:    "NPR 8,50,000 - 12,00,000",
			Mode:          "On-Campus",
			DegreeLabel:   "Bachelor's Degree",
			About: marshalJSON([]string{
				"BIT combines software development, cloud, cybersecurity, and product thinking to prepare graduates for modern IT roles.",
				"The curriculum emphasizes industry tools, problem-solving, and portfolio-based learning with internship exposure.",
			}),
			Curriculum: marshalJSON([]CurriculumSemester{
				{Semester: 1, Title: "Semester 1", Subtitle: "Computing Basics", Subjects: []string{"IT Fundamentals", "Programming Logic", "Business Communication"}},
				{Semester: 2, Title: "Semester 2", Subtitle: "Development", Subjects: []string{"Python Programming", "Web Development I", "Mathematics"}},
				{Semester: 3, Title: "Semester 3", Subtitle: "Data & Apps", Subjects: []string{"Database Systems", "Java Programming", "Human Computer Interaction"}},
				{Semester: 4, Title: "Semester 4", Subtitle: "Infrastructure", Subjects: []string{"Computer Networks", "Operating Systems", "Web Development II"}},
				{Semester: 5, Title: "Semester 5", Subtitle: "Enterprise", Subjects: []string{"Software Engineering", "Cloud Fundamentals", "Project Management"}},
				{Semester: 6, Title: "Semester 6", Subtitle: "Security", Subjects: []string{"Cyber Security", "Mobile Development", "DevOps Basics"}},
				{Semester: 7, Title: "Semester 7", Subtitle: "Industry Practice", Subjects: []string{"Internship", "Elective I", "System Integration"}},
				{Semester: 8, Title: "Semester 8", Subtitle: "Final Year", Subjects: []string{"Capstone Project", "Elective II", "Professional Ethics"}},
			}),
			Admissions: marshalJSON([]string{"+2 or equivalent transcript", "Valid ID document", "Recent passport-size photos", "Application form and fee receipt"}),
			Careers: marshalJSON([]CareerOpportunity{
				{Title: "Cloud Engineer", Icon: "cpu", Color: "emerald"},
				{Title: "Full Stack Developer", Icon: "database", Color: "blue"},
				{Title: "IT Consultant", Icon: "chart", Color: "purple"},
			}),
		},
	}

	for _, course := range courses {
		if err := db.Where("title = ?", course.Title).
			Assign(education.Course{
				ShortTitle:    course.ShortTitle,
				CollegesCount: course.CollegesCount,
				Affiliation:   course.Affiliation,
				Badges:        course.Badges,
				Level:         course.Level,
				Field:         course.Field,
				Duration:      course.Duration,
				EstFee:        course.EstFee,
				Highlights:    course.Highlights,
				CareerPath:    course.CareerPath,
				Description:   course.Description,
				Location:      course.Location,
				GovtFee:       course.GovtFee,
				PrivateFee:    course.PrivateFee,
				Mode:          course.Mode,
				DegreeLabel:   course.DegreeLabel,
				About:         course.About,
				Curriculum:    course.Curriculum,
				Admissions:    course.Admissions,
				Careers:       course.Careers,
			}).
			FirstOrCreate(&course).Error; err != nil {
			log.Printf("Error seeding course %s: %v", course.Title, err)
			return err
		}
	}
	log.Println("Successfully seeded courses")
	return nil
}

func SeedNews(db *gorm.DB) error {
	news := []education.News{
		{
			Category: "Academic",
			Title:    "NEB Class 12 Examination Routine 2081 Released",
			Excerpt:  "The National Examination Board (NEB) has officially published the examination schedule for the Grade 12 annual examinations.",
			Content:  "The National Examination Board (NEB) of Nepal has officially announced the routine for the Class 12 board examinations for the academic year 2081/82. The exams are scheduled to begin on Baishakh 14, 2082.",
			Image:    "https://images.unsplash.com/photo-1434030216411-0b793f4b4173?q=80&w=1200",
			Author:   "Academics Desk",
			Date:     "Jan 22, 2025",
			ReadTime: "4 min",
			Source:   "NEB Official",
			Tags:     marshalJSON([]string{"NEB", "Exam", "Class12", "Routine"}),
		},
		{
			Category: "Tech",
			Title:    "AI Integration in Nepali Higher Education",
			Excerpt:  "Top universities in Nepal are starting to integrate AI-driven tools into their curriculum and admission processes.",
			Content:  "A new wave of digital transformation is hitting the educational landscape of Nepal.",
			Image:    "https://images.unsplash.com/photo-1518770660439-4636190af475?q=80&w=1200",
			Author:   "Tech Reporter",
			Date:     "Jan 18, 2025",
			ReadTime: "6 min",
			Source:   "StudSphere Tech",
			Tags:     marshalJSON([]string{"AI", "Education", "TechTrends", "Nepal"}),
		},
	}

	for _, n := range news {
		if err := db.Where(education.News{Title: n.Title}).FirstOrCreate(&n).Error; err != nil {
			log.Printf("Error seeding news %s: %v", n.Title, err)
		}
	}
	log.Println("Successfully seeded news")
	return nil
}

func SeedEvents(db *gorm.DB) error {
	events := []education.Event{
		{
			Title:           "Virtual Open Day 2025",
			Excerpt:         "Join our virtual open day to explore campus life, scholarships, and program options.",
			Description:     "Register for a full virtual tour plus live Q&A with our admissions team.",
			Category:        "Seminar",
			Organizer:       "StudSphere Events",
			Location:        "Online / Zoom",
			Date:            "March 15, 2025",
			Time:            "10:00 AM - 2:00 PM",
			RegistrationFee: "Free",
			Image:           "https://images.unsplash.com/photo-1540575467063-178a50c2df87?q=80&w=800",
			Interested:      1200,
			Trending:        true,
		},
		{
			Title:           "Engineering Expo",
			Excerpt:         "Meet recruiters, attend workshops, and discover engineering careers.",
			Description:     "A hands-on expo for aspiring engineers with recruitment booths and live talks.",
			Category:        "Job Fair",
			Organizer:       "Engineering Hub Nepal",
			Location:        "Pulchowk Campus",
			Date:            "April 20, 2025",
			Time:            "9:00 AM - 5:00 PM",
			RegistrationFee: "Free",
			Image:           "https://images.unsplash.com/photo-1581094794329-c8112a89af12?q=80&w=800",
			Interested:      3500,
			Trending:        true,
		},
	}

	for _, event := range events {
		if err := db.Where(education.Event{Title: event.Title}).FirstOrCreate(&event).Error; err != nil {
			log.Printf("Error seeding event %s: %v", event.Title, err)
		}
	}
	log.Println("Successfully seeded events")
	return nil
}

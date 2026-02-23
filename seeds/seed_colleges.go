package seeds

import (
	"encoding/json"
	"log"

	"studsphere/backend/config"
	"studsphere/backend/models"

	"gorm.io/gorm"
)

// Course structure
type Course struct {
	Name     string `json:"name"`
	Level    string `json:"level"`
	Duration string `json:"duration"`
	Fees     string `json:"fees"`
	Focus    string `json:"focus"`
}

// Scholarship structure
type Scholarship struct {
	Title       string   `json:"title"`
	Percentage  string   `json:"percentage"`
	Description string   `json:"description"`
	Eligibility []string `json:"eligibility"`
	Color       string   `json:"color"`
}

// GalleryImage structure
type GalleryImage struct {
	URL     string `json:"url"`
	Caption string `json:"caption"`
}

// ProgramDetail structure
type ProgramDetail struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Duration string `json:"duration"`
	Color    string `json:"color"`
}

type AdmissionCard struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	University string `json:"university"`
	Faculty    string `json:"faculty"`
	Status     string `json:"status"`
	OpenDate   string `json:"openDate"`
	Deadline   string `json:"deadline"`
	Image      string `json:"image"`
}

type OfferedProgram struct {
	Name   string `json:"name"`
	Level  string `json:"level"`
	Status string `json:"status"`
}

type OfferedProgramCategory struct {
	ID       string           `json:"id"`
	Title    string           `json:"title"`
	Icon     string           `json:"icon"`
	Count    string           `json:"count"`
	Programs []OfferedProgram `json:"programs"`
}

type AlumniProfile struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Batch string `json:"batch"`
	Image string `json:"image"`
}

// AboutData structure
type AboutData struct {
	Vision         string `json:"vision"`
	Mission        string `json:"mission"`
	Accreditations string `json:"accreditations"`
	CampusLife     string `json:"campus_life"`
	PrincipalName  string `json:"principal_name"`
	PrincipalTitle string `json:"principal_title"`
	PrincipalMsg   string `json:"principal_message"`
}

// AdmissionEligibility structure
type AdmissionEligibility struct {
	Criteria []string `json:"criteria"`
}

// AdmissionDocument structure
type AdmissionDocument struct {
	Documents []string `json:"documents"`
}

// TimelineStep structure
type TimelineStep struct {
	Step  string `json:"step"`
	Title string `json:"title"`
	Sub   string `json:"sub"`
	Desc  string `json:"desc"`
}

// AdmissionInfo structure
type AdmissionInfo struct {
	Eligibility AdmissionEligibility `json:"eligibility"`
	Documents   AdmissionDocument    `json:"documents"`
	Timeline    []TimelineStep       `json:"timeline"`
}

// Department structure
type Department struct {
	Icon  string `json:"icon"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Color string `json:"color"`
}

// Review structure
type Review struct {
	Name        string `json:"name"`
	Initials    string `json:"initials"`
	Role        string `json:"role"`
	Time        string `json:"time"`
	Rating      int    `json:"rating"`
	Comment     string `json:"comment"`
	AvatarColor string `json:"avatar_color"`
}

func marshalJSON(data interface{}) []byte {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return []byte{}
	}
	return jsonData
}

func SeedColleges(db *gorm.DB) error {
	// Check if colleges already exist
	var count int64
	if err := db.Model(&models.College{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Printf("Recreating colleges seed data. Existing records: %d", count)
		if err := db.Unscoped().Where("1 = 1").Delete(&models.College{}).Error; err != nil {
			return err
		}
	}

	colleges := []models.College{
		{
			Name:             "Kathmandu University",
			FullName:         "Kathmandu University School of Engineering",
			Location:         "Kavre, Kathmandu Valley",
			Affiliation:      "Kathmandu University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.8,
			Reviews:          156,
			Programs:         45,
			Established:      "2000",
			Students:         "15k+",
			Description:      "Kathmandu University is a research university and leading educational institution in Nepal established in 2000 with a vision to provide world-class education.",
			Website:          "ku.edu.np",
			Email:            "info@ku.edu.np",
			Phone:            "+977-1-6680000",
			ImageURL:         "https://via.placeholder.com/300x200?text=Kathmandu+University",
			FeaturedPrograms: marshalJSON([]string{"BCA", "BBA", "BSc in Computer Science"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "Hostel", "Sports", "Cafeteria", "Wi-Fi"}),
			Courses: marshalJSON([]Course{
				{Name: "BCA (Bachelor of Computer Application)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 8,00,000", Focus: "Software Development, Networking, AI"},
				{Name: "BBA (Bachelor of Business Administration)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 6,50,000", Focus: "Management, Finance, Marketing"},
				{Name: "BSc in Computer Science", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 8,20,000", Focus: "Data Science, Systems, Security"},
				{Name: "MSc in Computer Science", Level: "Master", Duration: "2 Years", Fees: "NPR 10,00,000", Focus: "Advanced CS, Research"},
			}),
			Scholarships: marshalJSON([]Scholarship{
				{Title: "Merit Scholarship", Percentage: "Up to 100%", Description: "For exceptional academic performance", Eligibility: []string{"GPA 3.8+", "Top 5% in Entrance"}, Color: "yellow"},
				{Title: "Need-Based Aid", Percentage: "Up to 50%", Description: "For economically challenged students", Eligibility: []string{"Ward Office Verification", "GPA 2.8+"}, Color: "blue"},
				{Title: "Sports & Talent", Percentage: "Up to 75%", Description: "For sports and arts achievers", Eligibility: []string{"National Certificates", "Trial Success"}, Color: "green"},
			}),
			Gallery: marshalJSON([]GalleryImage{
				{URL: "https://images.unsplash.com/photo-1541339907198-e08756dedf3f?auto=format&fit=crop&w=800&q=60", Caption: "Graduation Day 2023"},
				{URL: "https://images.unsplash.com/photo-1524178232363-1fb2b075b655?auto=format&fit=crop&w=800&q=60", Caption: "Modern Classrooms"},
				{URL: "https://images.unsplash.com/photo-1599689018596-3d237199276e?auto=format&fit=crop&w=800&q=60", Caption: "E-Library Facility"},
				{URL: "https://images.unsplash.com/photo-1517245386807-bb43f82c33c4?auto=format&fit=crop&w=800&q=60", Caption: "IT Lab Session"},
				{URL: "https://images.unsplash.com/photo-1461896836934-ffe607ba8211?auto=format&fit=crop&w=800&q=60", Caption: "Annual Sports Meet"},
				{URL: "https://images.unsplash.com/photo-1523580494863-6f3031224c94?auto=format&fit=crop&w=800&q=60", Caption: "Guest Lecture Series"},
			}),
			ProgramsList: marshalJSON([]ProgramDetail{
				{Name: "Bachelor of Computer Application (BCA)", Category: "Science & Tech", Duration: "4 Years / 8 Semesters", Color: "blue"},
				{Name: "Bachelor of Business Administration (BBA)", Category: "Management", Duration: "4 Years / 8 Semesters", Color: "emerald"},
				{Name: "BSc Computer Science", Category: "Science & Tech", Duration: "4 Years / 8 Semesters", Color: "indigo"},
				{Name: "Master of Computer Science", Category: "Postgraduate", Duration: "2 Years / 4 Semesters", Color: "orange"},
			}),
			About: marshalJSON(AboutData{
				Vision:         "To be recognized as the epicenter for modern education, producing competent and globally minded leaders.",
				Mission:        "To provide world-class education combining rigorous academics with practical experience and ethical values.",
				Accreditations: "Affiliated with Kathmandu University and recognized by the Ministry of Education, Nepal. ISO 9001 certified campus.",
				CampusLife:     "Vibrant student life with 15+ clubs, regular events, international guest lectures, and career counseling.",
				PrincipalName:  "Dr. Ramesh Adhikari",
				PrincipalTitle: "Principal, PhD in Educational Leadership",
				PrincipalMsg:   "At Kathmandu University, we don't just teach; we inspire. Our holistic approach ensures that every student leaves our gates not just with a degree, but with a character built on integrity and a mind sharpened for the future.",
			}),
			Admissions: marshalJSON(AdmissionInfo{
				Eligibility: AdmissionEligibility{
					Criteria: []string{
						"Minimum GPA 2.4 in NEB +2 or equivalent (A-Levels, CBSE).",
						"CMAT/KUUMAT entrance score required for Management.",
						"Pass in College Internal Assessment (Written + Interview).",
						"English proficiency is mandatory for international students.",
					},
				},
				Documents: AdmissionDocument{
					Documents: []string{
						"Original Academic Transcripts (SEE & +2)",
						"Provisional & Migration Certificates",
						"Character Certificates",
						"Citizenship Copy / Passport",
						"2 Passport Size Photos",
					},
				},
				Timeline: []TimelineStep{
					{Step: "01", Title: "Application Submission", Sub: "May - June", Desc: "Fill out the official college application form online and upload all scanned academic documents."},
					{Step: "02", Title: "Entrance Exams", Sub: "July", Desc: "Appear for the mandatory university/college entrance exam (CMAT/IOST). Admit cards are issued 3 days prior."},
					{Step: "03", Title: "Interviews & Merit List", Sub: "August", Desc: "Shortlisted candidates face a personal interview. Final merit list is published within a week."},
					{Step: "04", Title: "Enrollment & Orientation", Sub: "September", Desc: "Selected students must pay admission fees to secure their seat. Orientation program follows shortly."},
				},
			}),
			AdmissionCards: marshalJSON([]AdmissionCard{
				{ID: 1, Name: "Bachelor in Information Technology", University: "Kathmandu University", Faculty: "School of Engineering", Status: "Ongoing", OpenDate: "20th, Dec, 2025", Deadline: "20th, Jan, 2026", Image: "https://images.unsplash.com/photo-1434030216411-0b793f4b4173?auto=format&fit=crop&w=800&q=60"},
				{ID: 2, Name: "Bachelor of Computer Application", University: "Kathmandu University", Faculty: "School of Science", Status: "Ongoing", OpenDate: "15th, Jan, 2026", Deadline: "15th, Feb, 2026", Image: "https://images.unsplash.com/photo-1454165833767-027eeef1596e?auto=format&fit=crop&w=800&q=60"},
				{ID: 3, Name: "BBA", University: "Kathmandu University", Faculty: "School of Management", Status: "Closed", OpenDate: "10th, Nov, 2025", Deadline: "30th, Nov, 2025", Image: "https://images.unsplash.com/photo-1523240715627-5d0b5114233c?auto=format&fit=crop&w=800&q=60"},
			}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{
				{ID: "undergrad", Title: "Undergraduate", Icon: "fa-graduation-cap", Count: "3 Programs", Programs: []OfferedProgram{{Name: "BE Computer Engineering", Level: "Bachelor", Status: "Ongoing"}, {Name: "BCA", Level: "Bachelor", Status: "Ongoing"}, {Name: "BBA", Level: "Bachelor", Status: "Closed"}}},
				{ID: "postgrad", Title: "Postgraduate", Icon: "fa-building-columns", Count: "2 Programs", Programs: []OfferedProgram{{Name: "MSc Computer Science", Level: "Master", Status: "Ongoing"}, {Name: "MBA", Level: "Master", Status: "Ongoing"}}},
			}),
			Alumni: marshalJSON([]AlumniProfile{
				{Name: "Sita Sharma", Role: "Product Manager @ Google", Batch: "Batch of 2015", Image: "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?auto=format&fit=crop&w=200&q=80"},
				{Name: "Rohan Karki", Role: "Software Engineer @ Microsoft", Batch: "Batch of 2017", Image: "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?auto=format&fit=crop&w=200&q=80"},
				{Name: "Nisha Rai", Role: "Data Scientist @ AWS", Batch: "Batch of 2018", Image: "https://images.unsplash.com/photo-1539571696357-5a69c17a67c6?auto=format&fit=crop&w=200&q=80"},
			}),
			Departments: marshalJSON([]Department{
				{Icon: "fa-laptop-code", Title: "IT & Computer Science", Color: "blue", Desc: "Cutting-edge technology, cloud computing, AI, and cybersecurity. Features dedicated coding labs and industry collaboration projects."},
				{Icon: "fa-briefcase", Title: "Management Studies", Color: "emerald", Desc: "Financial analysis, strategic marketing, organizational behavior, and entrepreneurship. Strong tie-ups with local businesses."},
				{Icon: "fa-book", Title: "Humanities & Social Science", Color: "pink", Desc: "Critical thinking, social work, mass communication, and psychology. Encourages community research and engagement."},
			}),
			CollegeReviews: marshalJSON([]Review{
				{Name: "Sushil Adhikari", Initials: "SA", Role: "BBA Student", Time: "2 months ago", Rating: 5, Comment: "The faculty here is extremely supportive. The blend of practical workshops and theory really helped me land my internship at a top bank. Highly recommend for Management students!", AvatarColor: "bg-blue-100 text-blue-600"},
				{Name: "Ananya Sharma", Initials: "AS", Role: "BCA Student", Time: "1 month ago", Rating: 5, Comment: "Best decision ever! The computer labs are world-class and the internship support is incredible. Already got placed before final semester.", AvatarColor: "bg-pink-100 text-pink-600"},
				{Name: "Rohit Poudel", Initials: "RP", Role: "MSc Student", Time: "3 weeks ago", Rating: 4, Comment: "The extracurricular activities and clubs are the best part. I joined the Robotics club and we won the national competition. It really balances study and fun.", AvatarColor: "bg-emerald-100 text-emerald-600"},
			}),
		},
		{
			Name:             "Tribhuvan University",
			FullName:         "Tribhuvan University Central Campus",
			Location:         "Kathmandu",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Public",
			Verified:         true,
			Popular:          true,
			Rating:           4.5,
			Reviews:          234,
			Programs:         120,
			Established:      "1959",
			Students:         "120k+",
			Description:      "Tribhuvan University is the oldest and largest university in Nepal, founded in 1959. It offers diverse programs across science, management, and humanities.",
			Website:          "tu.edu.np",
			Email:            "info@tu.edu.np",
			Phone:            "+977-1-4411980",
			ImageURL:         "https://via.placeholder.com/300x200?text=Tribhuvan+University",
			FeaturedPrograms: marshalJSON([]string{"Bachelor in Engineering", "Bachelor in Science", "Bachelor in Management"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "Canteen", "Parking", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "BE Civil Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,50,000", Focus: "Civil Infrastructure, Design"},
				{Name: "BSc Physics", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,00,000", Focus: "Physics, Research"},
				{Name: "BBS (Bachelor of Business Studies)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,50,000", Focus: "Business, Economics"},
			}),
			Scholarships: marshalJSON([]Scholarship{
				{Title: "Government Scholarship", Percentage: "Up to 100%", Description: "For government-endorsed students", Eligibility: []string{"Government Recommendation"}, Color: "blue"},
				{Title: "Excellent Performance", Percentage: "Up to 60%", Description: "For high performers", Eligibility: []string{"GPA 3.5+"}, Color: "green"},
			}),
			Gallery: marshalJSON([]GalleryImage{
				{URL: "https://images.unsplash.com/photo-1541339907198-e08756dedf3f?auto=format&fit=crop&w=800&q=60", Caption: "Main Campus"},
				{URL: "https://images.unsplash.com/photo-1524178232363-1fb2b075b655?auto=format&fit=crop&w=800&q=60", Caption: "Study Halls"},
			}),
			ProgramsList: marshalJSON([]ProgramDetail{
				{Name: "Bachelor of Engineering (Civil)", Category: "Engineering", Duration: "4 Years / 8 Semesters", Color: "indigo"},
				{Name: "Bachelor of Science (Physics)", Category: "Science", Duration: "4 Years / 8 Semesters", Color: "blue"},
				{Name: "Bachelor of Business Studies", Category: "Management", Duration: "4 Years / 8 Semesters", Color: "emerald"},
			}),
			About: marshalJSON(AboutData{
				Vision:         "To be a world-class university providing quality education and research opportunities.",
				Mission:        "To foster intellectual development and social responsibility through quality education.",
				Accreditations: "Oldest university in Nepal, government recognized, accredited by UGC.",
				CampusLife:     "Rich academic and cultural heritage with hundred plus student organizations.",
				PrincipalName:  "Prof. Dr. Ganesh Raj Joshi",
				PrincipalTitle: "Vice-Chancellor, PhD in Physics",
				PrincipalMsg:   "Tribhuvan University has been shaping minds for over 60 years. We believe in nurturing not just skilled professionals, but responsible citizens.",
			}),
			Admissions: marshalJSON(AdmissionInfo{
				Eligibility: AdmissionEligibility{
					Criteria: []string{
						"Minimum GPA 2.0 in NEB +2",
						"Entrance exam as per university guidelines",
						"Merit-based selection process",
					},
				},
				Documents: AdmissionDocument{
					Documents: []string{
						"Academic Transcripts",
						"Migration Certificate",
						"Character Certificate",
						"ID Proof",
						"Photographs",
					},
				},
				Timeline: []TimelineStep{
					{Step: "01", Title: "Notification", Sub: "April", Desc: "Admission notification is published"},
					{Step: "02", Title: "Application", Sub: "May", Desc: "Application submission period"},
					{Step: "03", Title: "Entrance Exam", Sub: "June", Desc: "Competitive entrance examination"},
					{Step: "04", Title: "Enrollment", Sub: "July", Desc: "Final enrollment and registration"},
				},
			}),
			AdmissionCards: marshalJSON([]AdmissionCard{
				{ID: 1, Name: "BE Civil Engineering", University: "Tribhuvan University", Faculty: "Institute of Engineering", Status: "Ongoing", OpenDate: "10th, Dec, 2025", Deadline: "10th, Jan, 2026", Image: "https://images.unsplash.com/photo-1541339907198-e08756dedf3f?auto=format&fit=crop&w=800&q=60"},
				{ID: 2, Name: "BSc Physics", University: "Tribhuvan University", Faculty: "Institute of Science", Status: "Ongoing", OpenDate: "5th, Jan, 2026", Deadline: "5th, Feb, 2026", Image: "https://images.unsplash.com/photo-1524178232363-1fb2b075b655?auto=format&fit=crop&w=800&q=60"},
			}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{
				{ID: "undergrad", Title: "Undergraduate", Icon: "fa-graduation-cap", Count: "3 Programs", Programs: []OfferedProgram{{Name: "BE Civil Engineering", Level: "Bachelor", Status: "Ongoing"}, {Name: "BSc Physics", Level: "Bachelor", Status: "Ongoing"}, {Name: "BBS", Level: "Bachelor", Status: "Closed"}}},
				{ID: "postgrad", Title: "Postgraduate", Icon: "fa-building-columns", Count: "1 Program", Programs: []OfferedProgram{{Name: "MSc Physics", Level: "Master", Status: "Ongoing"}}},
			}),
			Alumni: marshalJSON([]AlumniProfile{
				{Name: "Anil Regmi", Role: "Civil Engineer @ DoR", Batch: "Batch of 2012", Image: "https://images.unsplash.com/photo-1472099645785-5658abf4ff4e?auto=format&fit=crop&w=200&q=80"},
				{Name: "Priya Neupane", Role: "Researcher @ NAST", Batch: "Batch of 2014", Image: "https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=200&q=80"},
			}),
			Departments: marshalJSON([]Department{
				{Icon: "fa-tools", Title: "Engineering", Color: "indigo", Desc: "Comprehensive engineering programs with state-of-the-art facilities."},
				{Icon: "fa-atom", Title: "Science", Color: "blue", Desc: "Physics, Chemistry, Mathematics, and Biological Sciences programs."},
				{Icon: "fa-chart-pie", Title: "Management", Color: "emerald", Desc: "Business studies and management programs with practical focus."},
			}),
			CollegeReviews: marshalJSON([]Review{
				{Name: "Priya Neupane", Initials: "PN", Role: "Engineering Student", Time: "5 weeks ago", Rating: 4, Comment: "Great institution with a long legacy. Labs could be modernized but overall good learning environment.", AvatarColor: "bg-yellow-100 text-yellow-600"},
				{Name: "Bikram Singh", Initials: "BS", Role: "Science Student", Time: "1 month ago", Rating: 4, Comment: "Excellent for physics enthusiasts. Research opportunities are abundant. Faculty is dedicated and knowledgeable.", AvatarColor: "bg-orange-100 text-orange-600"},
			}),
		},
		{
			Name:             "Pokhara University",
			FullName:         "Pokhara University Main Campus",
			Location:         "Pokhara",
			Affiliation:      "Pokhara University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          false,
			Rating:           4.3,
			Reviews:          89,
			Programs:         35,
			Established:      "1997",
			Students:         "25k+",
			Description:      "Pokhara University is a leading private university in western Nepal offering quality education in engineering, management, and liberal arts.",
			Website:          "pu.edu.np",
			Email:            "admissions@pu.edu.np",
			Phone:            "+977-61-555555",
			ImageURL:         "https://via.placeholder.com/300x200?text=Pokhara+University",
			FeaturedPrograms: marshalJSON([]string{"BE Engineering", "BBA", "BSc"}),
			Amenities:        marshalJSON([]string{"Labs", "WiFi", "Cafeteria", "Sports", "Library"}),
			Courses: marshalJSON([]Course{
				{Name: "BE Computer Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 7,50,000", Focus: "Hardware, Software, Networks"},
				{Name: "BBA Tourism", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,50,000", Focus: "Tourism Management, Hospitality"},
				{Name: "BSc Environmental Science", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,80,000", Focus: "Environmental Studies, Conservation"},
			}),
			Scholarships: marshalJSON([]Scholarship{
				{Title: "Merit Scholarship", Percentage: "Up to 80%", Description: "For merit holders", Eligibility: []string{"GPA 3.6+"}, Color: "yellow"},
				{Title: "Regional Scholarship", Percentage: "Up to 40%", Description: "For western region students", Eligibility: []string{"From Western Nepal"}, Color: "blue"},
			}),
			Gallery: marshalJSON([]GalleryImage{
				{URL: "https://images.unsplash.com/photo-1541339907198-e08756dedf3f?auto=format&fit=crop&w=800&q=60", Caption: "Campus Entrance"},
				{URL: "https://images.unsplash.com/photo-1524178232363-1fb2b075b655?auto=format&fit=crop&w=800&q=60", Caption: "Academic Block"},
			}),
			ProgramsList: marshalJSON([]ProgramDetail{
				{Name: "Bachelor of Engineering (Computer)", Category: "Engineering", Duration: "4 Years / 8 Semesters", Color: "indigo"},
				{Name: "Bachelor of Business Administration", Category: "Management", Duration: "4 Years / 8 Semesters", Color: "emerald"},
				{Name: "Bachelor of Science (Environmental)", Category: "Science", Duration: "4 Years / 8 Semesters", Color: "green"},
			}),
			About: marshalJSON(AboutData{
				Vision:         "To empower students with knowledge and skills for the digital age.",
				Mission:        "Providing accessible quality education with modern infrastructure.",
				Accreditations: "Accredited by UGC, recognized by Nepal Government.",
				CampusLife:     "Active student engagement with various clubs and committees.",
				PrincipalName:  "Dr. Arjun Sharma",
				PrincipalTitle: "Vice-Chancellor, PhD in Engineering",
				PrincipalMsg:   "At Pokhara, we strive to offer education that prepares students for real-world challenges.",
			}),
			Admissions: marshalJSON(AdmissionInfo{
				Eligibility: AdmissionEligibility{
					Criteria: []string{
						"Minimum GPA 2.3 in +2",
						"Entrance test score",
						"Interview performance",
					},
				},
				Documents: AdmissionDocument{
					Documents: []string{
						"Academic Records",
						"Migration Documents",
						"Character Certificate",
						"Passport/ID",
						"Photos",
					},
				},
				Timeline: []TimelineStep{
					{Step: "01", Title: "Application Opens", Sub: "May", Desc: "Online application submission"},
					{Step: "02", Title: "Entrance Test", Sub: "June", Desc: "Competitive entrance examination"},
					{Step: "03", Title: "Results", Sub: "July", Desc: "Merit list announcement"},
					{Step: "04", Title: "Registration", Sub: "August", Desc: "Student enrollment and registration"},
				},
			}),
			AdmissionCards: marshalJSON([]AdmissionCard{
				{ID: 1, Name: "BE Computer Engineering", University: "Pokhara University", Faculty: "Engineering Faculty", Status: "Ongoing", OpenDate: "12th, Jan, 2026", Deadline: "12th, Feb, 2026", Image: "https://images.unsplash.com/photo-1517245386807-bb43f82c33c4?auto=format&fit=crop&w=800&q=60"},
				{ID: 2, Name: "BBA Tourism", University: "Pokhara University", Faculty: "Management Faculty", Status: "Closed", OpenDate: "2nd, Dec, 2025", Deadline: "2nd, Jan, 2026", Image: "https://images.unsplash.com/photo-1461896836934-ffe607ba8211?auto=format&fit=crop&w=800&q=60"},
			}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{
				{ID: "undergrad", Title: "Undergraduate", Icon: "fa-graduation-cap", Count: "3 Programs", Programs: []OfferedProgram{{Name: "BE Computer Engineering", Level: "Bachelor", Status: "Ongoing"}, {Name: "BBA Tourism", Level: "Bachelor", Status: "Closed"}, {Name: "BSc Environmental Science", Level: "Bachelor", Status: "Ongoing"}}},
				{ID: "postgrad", Title: "Postgraduate", Icon: "fa-building-columns", Count: "1 Program", Programs: []OfferedProgram{{Name: "MBA", Level: "Master", Status: "Ongoing"}}},
			}),
			Alumni: marshalJSON([]AlumniProfile{
				{Name: "Deepak Magar", Role: "Full Stack Developer", Batch: "Batch of 2019", Image: "https://images.unsplash.com/photo-1504593811423-6dd665756598?auto=format&fit=crop&w=200&q=80"},
				{Name: "Sneha Paudel", Role: "Tourism Consultant", Batch: "Batch of 2020", Image: "https://images.unsplash.com/photo-1487412720507-e7ab37603c6f?auto=format&fit=crop&w=200&q=80"},
			}),
			Departments: marshalJSON([]Department{
				{Icon: "fa-microchip", Title: "Engineering", Color: "indigo", Desc: "Computer and Civil Engineering programs."},
				{Icon: "fa-suitcase", Title: "Business", Color: "emerald", Desc: "Management and Business Administration."},
				{Icon: "fa-leaf", Title: "Science", Color: "green", Desc: "Environmental and Applied Sciences."},
			}),
			CollegeReviews: marshalJSON([]Review{
				{Name: "Deepak Magar", Initials: "DM", Role: "Engineering Student", Time: "2 months ago", Rating: 4, Comment: "Good infrastructure and supportive faculty. Located in beautiful Pokhara.", AvatarColor: "bg-purple-100 text-purple-600"},
				{Name: "Sneha Paudel", Initials: "SP", Role: "BBA Student", Time: "6 weeks ago", Rating: 4, Comment: "Great tourism program with practical approach. Industry connections are strong.", AvatarColor: "bg-red-100 text-red-600"},
			}),
		},
	}

	// Create colleges in database
	for _, college := range colleges {
		var university models.University
		if err := db.Where("name = ?", college.Affiliation).First(&university).Error; err != nil {
			log.Printf("Error finding university for college %s with affiliation %s: %v", college.Name, college.Affiliation, err)
			return err
		}

		college.UniversityID = university.ID
		college.Affiliation = university.Name

		if err := db.Create(&college).Error; err != nil {
			log.Printf("Error creating college %s: %v", college.Name, err)
			return err
		}
	}

	log.Println("Successfully seeded colleges with complete data")
	return nil
}

// Seed function that calls all seeding functions
func Seed() error {
	db := config.GetDB()
	if db == nil {
		return nil
	}
	if err := SeedUniversities(db); err != nil {
		return err
	}
	return SeedColleges(db)
}

package seeds

import (
	"encoding/json"
	"log"
	"strings"

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

func applyProfileFitDefaults(college *models.College) {
	name := strings.ToLower(college.Name)

	college.AcademicFitScore = 7
	college.CampusLifeScore = 7
	college.CareerFitScore = 7
	college.BalancedFitScore = 7
	college.ProfileTags = marshalJSON([]string{"balanced"})

	switch {
	case strings.Contains(name, "kusoe"):
		college.AcademicFitScore = 10
		college.CampusLifeScore = 7
		college.CareerFitScore = 9
		college.BalancedFitScore = 8
		college.ProfileTags = marshalJSON([]string{"academic", "career", "balanced"})
	case strings.Contains(name, "pulchowk"):
		college.AcademicFitScore = 9
		college.CampusLifeScore = 6
		college.CareerFitScore = 8
		college.BalancedFitScore = 7
		college.ProfileTags = marshalJSON([]string{"academic", "career"})
	case strings.Contains(name, "pokhara university school"):
		college.AcademicFitScore = 8
		college.CampusLifeScore = 9
		college.CareerFitScore = 8
		college.BalancedFitScore = 9
		college.ProfileTags = marshalJSON([]string{"campus", "balanced", "career"})
	}
}

func SeedColleges(db *gorm.DB) error {
	// Check if colleges already exist
	var count int64
	if err := db.Model(&models.College{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Printf("Recreating colleges seed data. Existing records: %d", count)
		if err := db.Unscoped().Where("1 = 1").Delete(&models.CollegeUniversityCourse{}).Error; err != nil {
			return err
		}
		if err := db.Unscoped().Where("1 = 1").Delete(&models.College{}).Error; err != nil {
			return err
		}
	}

	colleges := []models.College{
		{
			Name:             "KUSOE, Dhulikhel Campus",
			FullName:         "Kathmandu University School of Engineering (KUSOE), Dhulikhel Campus",
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
			Description:      "KUSOE Dhulikhel Campus is a leading engineering school under Kathmandu University known for rigorous academics, labs, and research-driven learning.",
			Website:          "soe.ku.edu.np",
			Email:            "info@soe.ku.edu.np",
			Phone:            "+977-1-6680000",
			ImageURL:         "https://via.placeholder.com/300x200?text=KUSOE+Dhulikhel",
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
			Name:             "Pulchowk Campus",
			FullName:         "Institute of Engineering, Pulchowk Campus",
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
			Description:      "Pulchowk Campus is the flagship engineering campus under Tribhuvan University, recognized for technical education and competitive programs.",
			Website:          "pcampus.edu.np",
			Email:            "info@pcampus.edu.np",
			Phone:            "+977-1-4411980",
			ImageURL:         "https://via.placeholder.com/300x200?text=Pulchowk+Campus",
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
			Name:             "Pokhara University School of Engineering",
			FullName:         "Pokhara University School of Engineering, Lekhnath Campus",
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
			Description:      "Pokhara University School of Engineering, Lekhnath Campus offers applied engineering and technology programs with industry-focused training.",
			Website:          "pu.edu.np",
			Email:            "admissions.soe@pu.edu.np",
			Phone:            "+977-61-555555",
			ImageURL:         "https://via.placeholder.com/300x200?text=PU+School+of+Engineering",
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
		{
			Name:             "Everest Engineering College",
			FullName:         "Everest Engineering College",
			Location:         "Lalitpur",
			Affiliation:      "Pokhara University",
			CollegeType:      "Private",
			Verified:         false,
			Popular:          false,
			Rating:           4.0,
			Reviews:          50,
			Programs:         20,
			Established:      "2001",
			Students:         "5k+",
			Description:      "Everest Engineering College is an engineering institution focused on diverse engineering degrees.",
			Website:          "eec.edu.np",
			Email:            "info@eec.edu.np",
			Phone:            "+977-1-555555",
			ImageURL:         "https://via.placeholder.com/300x200?text=Everest+Engineering",
			FeaturedPrograms: marshalJSON([]string{"BE Engineering", "BBA"}),
			Amenities:        marshalJSON([]string{"Labs", "WiFi", "Cafeteria"}),
			Courses:          marshalJSON([]Course{}),
			Scholarships:     marshalJSON([]Scholarship{}),
			Gallery:          marshalJSON([]GalleryImage{}),
			ProgramsList: marshalJSON([]ProgramDetail{
				{Name: "Bachelor of Engineering (Computer)", Category: "Engineering", Duration: "4 Years / 8 Semesters", Color: "indigo"},
			}),
			About: marshalJSON(AboutData{
				Vision: "To provide quality engineering education",
			}),
			Admissions: marshalJSON(AdmissionInfo{
				Eligibility: AdmissionEligibility{},
				Documents:   AdmissionDocument{},
				Timeline:    []TimelineStep{},
			}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Patan Multiple Campus",
			FullName:         "Patan Multiple Campus",
			Location:         "Lalitpur",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Public",
			Verified:         false,
			Popular:          true,
			Rating:           4.1,
			Reviews:          150,
			Programs:         50,
			Established:      "1954",
			Students:         "10k+",
			Description:      "A constituent campus of TU offering a wide variety of courses.",
			Website:          "pmc.edu.np",
			Email:            "info@pmc.edu.np",
			Phone:            "+977-1-444444",
			ImageURL:         "https://via.placeholder.com/300x200?text=Patan+Multiple+Campus",
			FeaturedPrograms: marshalJSON([]string{"BSc CSIT", "BCA"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "Cafeteria"}),
			Courses:          marshalJSON([]Course{}),
			Scholarships:     marshalJSON([]Scholarship{}),
			Gallery:          marshalJSON([]GalleryImage{}),
			ProgramsList:     marshalJSON([]ProgramDetail{}),
			About: marshalJSON(AboutData{
				Vision: "Leading constituent campus for modern education",
			}),
			Admissions: marshalJSON(AdmissionInfo{
				Eligibility: AdmissionEligibility{},
				Documents:   AdmissionDocument{},
				Timeline:    []TimelineStep{},
			}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Kathmandu Model College",
			FullName:         "Kathmandu Model College",
			Location:         "Kathmandu",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.3,
			Reviews:          120,
			Programs:         25,
			Established:      "1993",
			Students:         "5k+",
			Description:      "A premier college affiliated with Tribhuvan University offering management and computer applications programs.",
			Website:          "kmc.edu.np",
			Email:            "info@kmc.edu.np",
			Phone:            "+977-1-4423436",
			ImageURL:         "https://via.placeholder.com/300x200?text=Kathmandu+Model+College",
			FeaturedPrograms: marshalJSON([]string{"BBA", "BCA", "BBS"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Sports", "Cafeteria"}),
			Courses: marshalJSON([]Course{
				{Name: "BBA (Bachelor of Business Administration)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,50,000", Focus: "Management, Finance, Marketing"},
				{Name: "BCA (Bachelor of Computer Application)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,00,000", Focus: "Software Development, IT"},
				{Name: "BBS (Bachelor of Business Studies)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 3,80,000", Focus: "Business, Economics"},
			}),
			Scholarships: marshalJSON([]Scholarship{
				{Title: "Merit Scholarship", Percentage: "Up to 50%", Description: "For excellent academic performers", Eligibility: []string{"GPA 3.5+"}, Color: "blue"},
			}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "To provide quality education in management and technology"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Thames International College",
			FullName:         "Thames International College",
			Location:         "Kathmandu",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.2,
			Reviews:          95,
			Programs:         18,
			Established:      "1995",
			Students:         "3k+",
			Description:      "A well-known private college offering programs in management, social sciences, and hospitality.",
			Website:          "thames.edu.np",
			Email:            "info@thames.edu.np",
			Phone:            "+977-1-4441122",
			ImageURL:         "https://via.placeholder.com/300x200?text=Thames+International+College",
			FeaturedPrograms: marshalJSON([]string{"BBA", "BHM", "BSW"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "BBA (Bachelor of Business Administration)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,80,000", Focus: "Business Management"},
				{Name: "BHM (Bachelor of Hotel Management)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,20,000", Focus: "Hotel Management, Hospitality"},
				{Name: "BSW (Bachelor of Social Work)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 3,50,000", Focus: "Social Work"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Excellence in education and character development"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Kathmandu Engineering College",
			FullName:         "Kathmandu Engineering College",
			Location:         "Kathmandu",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.1,
			Reviews:          85,
			Programs:         12,
			Established:      "1998",
			Students:         "2.5k+",
			Description:      "A premier engineering college offering civil, computer, and electronics engineering programs.",
			Website:          "kec.edu.np",
			Email:            "info@kec.edu.np",
			Phone:            "+977-1-4489142",
			ImageURL:         "https://via.placeholder.com/300x200?text=Kathmandu+Engineering+College",
			FeaturedPrograms: marshalJSON([]string{"BE Civil", "BE Computer", "BE Electronics"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Workshop", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "BE Civil Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 7,50,000", Focus: "Civil Engineering"},
				{Name: "BE Computer Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 8,00,000", Focus: "Computer Engineering"},
				{Name: "BE Electronics Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 7,80,000", Focus: "Electronics Engineering"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Producing competent engineers for the nation"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Advanced College of Engineering and Management",
			FullName:         "Advanced College of Engineering and Management",
			Location:         "Kathmandu",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          false,
			Rating:           4.0,
			Reviews:          65,
			Programs:         8,
			Established:      "2000",
			Students:         "1.5k+",
			Description:      "An engineering college focused on practical education and industry connections.",
			Website:          "acem.edu.np",
			Email:            "info@acem.edu.np",
			Phone:            "+977-1-4465234",
			ImageURL:         "https://via.placeholder.com/300x200?text=Advanced+College+of+Engineering",
			FeaturedPrograms: marshalJSON([]string{"BE Computer", "BE Civil"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi"}),
			Courses: marshalJSON([]Course{
				{Name: "BE Computer Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 7,50,000", Focus: "Computer Engineering"},
				{Name: "BE Civil Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 7,20,000", Focus: "Civil Engineering"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Engineering excellence through innovation"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "GoldenGate International College",
			FullName:         "GoldenGate International College",
			Location:         "Kathmandu",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.2,
			Reviews:          110,
			Programs:         15,
			Established:      "1997",
			Students:         "3k+",
			Description:      "A reputed private college offering management and computer applications programs.",
			Website:          "ggic.edu.np",
			Email:            "info@ggic.edu.np",
			Phone:            "+977-1-4449876",
			ImageURL:         "https://via.placeholder.com/300x200?text=GoldenGate+International+College",
			FeaturedPrograms: marshalJSON([]string{"BBA", "BCA", "BBS"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "BBA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,60,000", Focus: "Business Administration"},
				{Name: "BCA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,10,000", Focus: "Computer Applications"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Quality education for future leaders"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Kathmandu National College",
			FullName:         "Kathmandu National College",
			Location:         "Kathmandu",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Private",
			Verified:         false,
			Popular:          false,
			Rating:           3.9,
			Reviews:          55,
			Programs:         10,
			Established:      "2001",
			Students:         "1.2k+",
			Description:      "A private college offering various undergraduate programs in management and humanities.",
			Website:          "knc.edu.np",
			Email:            "info@knc.edu.np",
			Phone:            "+977-1-4234567",
			ImageURL:         "https://via.placeholder.com/300x200?text=Kathmandu+National+College",
			FeaturedPrograms: marshalJSON([]string{"BBS", "BA"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi"}),
			Courses: marshalJSON([]Course{
				{Name: "BBS", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 3,20,000", Focus: "Business Studies"},
				{Name: "BA (Bachelor of Arts)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 2,80,000", Focus: "Humanities"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Affordable quality education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Whitefield International College",
			FullName:         "Whitefield International College",
			Location:         "Kathmandu",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Private",
			Verified:         false,
			Popular:          false,
			Rating:           3.8,
			Reviews:          45,
			Programs:         8,
			Established:      "2003",
			Students:         "800+",
			Description:      "A private college offering management and computer programs.",
			Website:          "whitefield.edu.np",
			Email:            "info@whitefield.edu.np",
			Phone:            "+977-1-4567890",
			ImageURL:         "https://via.placeholder.com/300x200?text=Whitefield+International+College",
			FeaturedPrograms: marshalJSON([]string{"BBA", "BCA"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi"}),
			Courses: marshalJSON([]Course{
				{Name: "BBA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,20,000", Focus: "Business"},
				{Name: "BCA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,80,000", Focus: "Computer Applications"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "International standard education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "College of Business Management",
			FullName:         "College of Business Management",
			Location:         "Kathmandu",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.4,
			Reviews:          140,
			Programs:         20,
			Established:      "1996",
			Students:         "4k+",
			Description:      "A leading management college offering MBA and other management programs.",
			Website:          "cbm.edu.np",
			Email:            "info@cbm.edu.np",
			Phone:            "+977-1-4445566",
			ImageURL:         "https://via.placeholder.com/300x200?text=College+of+Business+Management",
			FeaturedPrograms: marshalJSON([]string{"MBA", "BBA", "MBS"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports", "Auditorium"}),
			Courses: marshalJSON([]Course{
				{Name: "MBA (Master of Business Administration)", Level: "Postgraduate", Duration: "2 Years", Fees: "NPR 6,50,000", Focus: "Business Management"},
				{Name: "BBA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,00,000", Focus: "Business Administration"},
				{Name: "MBS (Master of Business Studies)", Level: "Postgraduate", Duration: "2 Years", Fees: "NPR 5,50,000", Focus: "Business Studies"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Premier management education in Nepal"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Prithvi Narayan Campus",
			FullName:         "Prithvi Narayan Campus",
			Location:         "Pokhara, Kaski",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Public",
			Verified:         true,
			Popular:          true,
			Rating:           4.2,
			Reviews:          180,
			Programs:         35,
			Established:      "1976",
			Students:         "8k+",
			Description:      "A major constituent campus of TU in Pokhara offering diverse programs in humanities, management, and science.",
			Website:          "pncampus.edu.np",
			Email:            "info@pncampus.edu.np",
			Phone:            "+977-61-521860",
			ImageURL:         "https://via.placeholder.com/300x200?text=Prithvi+Narayan+Campus",
			FeaturedPrograms: marshalJSON([]string{"BSc", "BA", "BBS", "MBS"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports Ground"}),
			Courses: marshalJSON([]Course{
				{Name: "BSc (Bachelor of Science)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 2,50,000", Focus: "Science"},
				{Name: "BA (Bachelor of Arts)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 1,80,000", Focus: "Humanities"},
				{Name: "BBS", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 2,20,000", Focus: "Business Studies"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Quality education for all"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Lalit Multiple Campus",
			FullName:         "Lalit Multiple Campus",
			Location:         "Lalitpur",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Public",
			Verified:         true,
			Popular:          true,
			Rating:           4.1,
			Reviews:          160,
			Programs:         30,
			Established:      "1973",
			Students:         "7k+",
			Description:      "A prominent constituent campus of TU in Lalitpur offering various undergraduate and postgraduate programs.",
			Website:          "lmc.edu.np",
			Email:            "info@lmc.edu.np",
			Phone:            "+977-1-5521260",
			ImageURL:         "https://via.placeholder.com/300x200?text=Lalit+Multiple+Campus",
			FeaturedPrograms: marshalJSON([]string{"BSc", "BA", "BBS", "BEd"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "BSc", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 2,40,000", Focus: "Science"},
				{Name: "BA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 1,70,000", Focus: "Humanities"},
				{Name: "B.Ed", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 2,80,000", Focus: "Education"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Excellence in higher education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Mahendra Multiple Campus Nepalgunj",
			FullName:         "Mahendra Multiple Campus Nepalgunj",
			Location:         "Nepalgunj, Banke",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Public",
			Verified:         true,
			Popular:          true,
			Rating:           4.0,
			Reviews:          145,
			Programs:         28,
			Established:      "1973",
			Students:         "6k+",
			Description:      "A major constituent campus of TU in western Nepal providing quality higher education.",
			Website:          "mmc.edu.np",
			Email:            "info@mmc.edu.np",
			Phone:            "+977-81-520100",
			ImageURL:         "https://via.placeholder.com/300x200?text=Mahendra+Multiple+Campus",
			FeaturedPrograms: marshalJSON([]string{"BSc", "BA", "BBS", "LLB"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel"}),
			Courses: marshalJSON([]Course{
				{Name: "BSc", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 2,20,000", Focus: "Science"},
				{Name: "BA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 1,60,000", Focus: "Humanities"},
				{Name: "LLB (Bachelor of Law)", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 3,50,000", Focus: "Law"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Education for regional development"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Birendra Multiple Campus",
			FullName:         "Birendra Multiple Campus",
			Location:         "Bharatpur, Chitwan",
			Affiliation:      "Tribhuvan University",
			CollegeType:      "Public",
			Verified:         true,
			Popular:          true,
			Rating:           4.1,
			Reviews:          130,
			Programs:         25,
			Established:      "1989",
			Students:         "5k+",
			Description:      "A constituent campus of TU in Chitwan offering diverse academic programs.",
			Website:          "bmc.edu.np",
			Email:            "info@bmc.edu.np",
			Phone:            "+977-56-521111",
			ImageURL:         "https://via.placeholder.com/300x200?text=Birendra+Multiple+Campus",
			FeaturedPrograms: marshalJSON([]string{"BSc", "BA", "BBS"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "BSc", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 2,30,000", Focus: "Science"},
				{Name: "BA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 1,65,000", Focus: "Humanities"},
				{Name: "BBS", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 2,10,000", Focus: "Business Studies"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Higher education for all"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Nepal Engineering College",
			FullName:         "Nepal Engineering College",
			Location:         "Pokhara, Kaski",
			Affiliation:      "Pokhara University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.2,
			Reviews:          90,
			Programs:         10,
			Established:      "2000",
			Students:         "2k+",
			Description:      "A premier engineering college affiliated with Pokhara University offering various engineering programs.",
			Website:          "nec.edu.np",
			Email:            "info@nec.edu.np",
			Phone:            "+977-61-520444",
			ImageURL:         "https://via.placeholder.com/300x200?text=Nepal+Engineering+College",
			FeaturedPrograms: marshalJSON([]string{"BE Civil", "BE Computer", "BE Electronics"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Workshop", "Hostel"}),
			Courses: marshalJSON([]Course{
				{Name: "BE Civil Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 7,80,000", Focus: "Civil Engineering"},
				{Name: "BE Computer Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 8,20,000", Focus: "Computer Engineering"},
				{Name: "BE Electronics Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 8,00,000", Focus: "Electronics Engineering"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Excellence in engineering education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Gandaki College of Engineering and Science",
			FullName:         "Gandaki College of Engineering and Science",
			Location:         "Pokhara, Kaski",
			Affiliation:      "Pokhara University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          false,
			Rating:           4.0,
			Reviews:          65,
			Programs:         8,
			Established:      "2001",
			Students:         "1.2k+",
			Description:      "An engineering and science college affiliated with Pokhara University.",
			Website:          "gces.edu.np",
			Email:            "info@gces.edu.np",
			Phone:            "+977-61-521111",
			ImageURL:         "https://via.placeholder.com/300x200?text=Gandaki+College+of+Engineering",
			FeaturedPrograms: marshalJSON([]string{"BE Computer", "BSc CSIT"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi"}),
			Courses: marshalJSON([]Course{
				{Name: "BE Computer Engineering", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 7,50,000", Focus: "Computer Engineering"},
				{Name: "BSc CSIT", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 6,80,000", Focus: "Computer Science & IT"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Quality technical education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "School of Environmental Science and Management",
			FullName:         "School of Environmental Science and Management (SchEMS)",
			Location:         "Kathmandu",
			Affiliation:      "Pokhara University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          false,
			Rating:           4.1,
			Reviews:          50,
			Programs:         6,
			Established:      "2003",
			Students:         "800+",
			Description:      "A specialized college offering environmental science and management programs.",
			Website:          "schems.edu.np",
			Email:            "info@schems.edu.np",
			Phone:            "+977-1-4466000",
			ImageURL:         "https://via.placeholder.com/300x200?text=School+of+Environmental+Science",
			FeaturedPrograms: marshalJSON([]string{"MSc Environmental Science", "BSc Environmental Science"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Research Center"}),
			Courses: marshalJSON([]Course{
				{Name: "MSc Environmental Science", Level: "Postgraduate", Duration: "2 Years", Fees: "NPR 5,50,000", Focus: "Environmental Science"},
				{Name: "BSc Environmental Science", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,80,000", Focus: "Environmental Studies"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Sustainability through education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Brihaspati College",
			FullName:         "Brihaspati College",
			Location:         "Kathmandu",
			Affiliation:      "Pokhara University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          false,
			Rating:           3.9,
			Reviews:          55,
			Programs:         8,
			Established:      "2004",
			Students:         "1k+",
			Description:      "A private college affiliated with Pokhara University offering management and IT programs.",
			Website:          "brihasptic.edu.np",
			Email:            "info@brihasptic.edu.np",
			Phone:            "+977-1-4012500",
			ImageURL:         "https://via.placeholder.com/300x200?text=Brihaspati+College",
			FeaturedPrograms: marshalJSON([]string{"BBA", "BCA", "BSc"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi"}),
			Courses: marshalJSON([]Course{
				{Name: "BBA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,40,000", Focus: "Business Administration"},
				{Name: "BCA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,90,000", Focus: "Computer Applications"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Quality education for brighter future"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Tilottama Campus",
			FullName:         "Tilottama Campus",
			Location:         "Butwal, Rupandehi",
			Affiliation:      "Pokhara University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.2,
			Reviews:          85,
			Programs:         12,
			Established:      "2002",
			Students:         "2k+",
			Description:      "A prominent campus in Butwal affiliated with Pokhara University offering various programs.",
			Website:          "tilottama.edu.np",
			Email:            "info@tilottama.edu.np",
			Phone:            "+977-71-520111",
			ImageURL:         "https://via.placeholder.com/300x200?text=Tilottama+Campus",
			FeaturedPrograms: marshalJSON([]string{"BBA", "BCA", "BSc"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "BBA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,30,000", Focus: "Business Administration"},
				{Name: "BCA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,80,000", Focus: "Computer Applications"},
				{Name: "BSc", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,50,000", Focus: "Science"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Excellence in education in western Nepal"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Ace Institute of Management",
			FullName:         "Ace Institute of Management",
			Location:         "Kathmandu",
			Affiliation:      "Pokhara University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.3,
			Reviews:          100,
			Programs:         15,
			Established:      "2001",
			Students:         "3k+",
			Description:      "A leading management institute affiliated with Pokhara University offering MBA and other programs.",
			Website:          "ace.edu.np",
			Email:            "info@ace.edu.np",
			Phone:            "+977-1-4444455",
			ImageURL:         "https://via.placeholder.com/300x200?text=Ace+Institute+of+Management",
			FeaturedPrograms: marshalJSON([]string{"MBA", "BBA", "BHM"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports", "Auditorium"}),
			Courses: marshalJSON([]Course{
				{Name: "MBA", Level: "Postgraduate", Duration: "2 Years", Fees: "NPR 6,80,000", Focus: "Business Management"},
				{Name: "BBA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,70,000", Focus: "Business Administration"},
				{Name: "BHM", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,00,000", Focus: "Hotel Management"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Premier management education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Nepal College of Management",
			FullName:         "Nepal College of Management",
			Location:         "Kathmandu",
			Affiliation:      "Kathmandu University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.3,
			Reviews:          110,
			Programs:         15,
			Established:      "1998",
			Students:         "2.5k+",
			Description:      "A premier management college affiliated with Kathmandu University offering business programs.",
			Website:          "ncm.edu.np",
			Email:            "info@ncm.edu.np",
			Phone:            "+977-1-4467234",
			ImageURL:         "https://via.placeholder.com/300x200?text=Nepal+College+of+Management",
			FeaturedPrograms: marshalJSON([]string{"MBA", "BBA", "BHM"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports", "Auditorium"}),
			Courses: marshalJSON([]Course{
				{Name: "MBA", Level: "Postgraduate", Duration: "2 Years", Fees: "NPR 7,00,000", Focus: "Business Management"},
				{Name: "BBA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,20,000", Focus: "Business Administration"},
				{Name: "BHM", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,50,000", Focus: "Hotel Management"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Excellence in management education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Little Angels College of Management",
			FullName:         "Little Angels College of Management",
			Location:         "Kathmandu",
			Affiliation:      "Kathmandu University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.2,
			Reviews:          95,
			Programs:         12,
			Established:      "2000",
			Students:         "2k+",
			Description:      "A reputed management college affiliated with Kathmandu University known for quality education.",
			Website:          "lacm.edu.np",
			Email:            "info@lacm.edu.np",
			Phone:            "+977-1-4442233",
			ImageURL:         "https://via.placeholder.com/300x200?text=Little+Angels+College+of+Management",
			FeaturedPrograms: marshalJSON([]string{"MBA", "BBA", "BCA"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "MBA", Level: "Postgraduate", Duration: "2 Years", Fees: "NPR 6,50,000", Focus: "Business Management"},
				{Name: "BBA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 4,90,000", Focus: "Business Administration"},
				{Name: "BCA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,30,000", Focus: "Computer Applications"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Nurturing future business leaders"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Kathmandu University School of Management",
			FullName:         "Kathmandu University School of Management",
			Location:         "Dhulikhel, Kavre",
			Affiliation:      "Kathmandu University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.5,
			Reviews:          130,
			Programs:         18,
			Established:      "1997",
			Students:         "3k+",
			Description:      "The business school of Kathmandu University offering world-class management education.",
			Website:          "kusom.edu.np",
			Email:            "info@kusom.edu.np",
			Phone:            "+977-1-6680011",
			ImageURL:         "https://via.placeholder.com/300x200?text=KU+School+of+Management",
			FeaturedPrograms: marshalJSON([]string{"MBA", "MBS", "BBA", "BSc"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports", "Auditorium", "Research Center"}),
			Courses: marshalJSON([]Course{
				{Name: "MBA", Level: "Postgraduate", Duration: "2 Years", Fees: "NPR 8,00,000", Focus: "Business Management"},
				{Name: "MBS", Level: "Postgraduate", Duration: "2 Years", Fees: "NPR 6,50,000", Focus: "Business Studies"},
				{Name: "BBA", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,50,000", Focus: "Business Administration"},
				{Name: "BSc", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 5,80,000", Focus: "Science"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Premier business school in Nepal"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Manipal College of Medical Sciences",
			FullName:         "Manipal College of Medical Sciences",
			Location:         "Pokhara, Kaski",
			Affiliation:      "Kathmandu University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.6,
			Reviews:          180,
			Programs:         10,
			Established:      "1994",
			Students:         "1.5k+",
			Description:      "A premier medical college affiliated with Kathmandu University offering MBBS and other medical programs.",
			Website:          "manipal.edu.np",
			Email:            "info@manipal.edu.np",
			Phone:            "+977-61-520111",
			ImageURL:         "https://via.placeholder.com/300x200?text=Manipal+College+of+Medical+Sciences",
			FeaturedPrograms: marshalJSON([]string{"MBBS", "BDS", "Nursing", "MD"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Hospital", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "MBBS", Level: "Undergraduate", Duration: "5.5 Years", Fees: "NPR 45,00,000", Focus: "Medicine & Surgery"},
				{Name: "BDS", Level: "Undergraduate", Duration: "5 Years", Fees: "NPR 22,00,000", Focus: "Dentistry"},
				{Name: "BSc Nursing", Level: "Undergraduate", Duration: "4 Years", Fees: "NPR 8,00,000", Focus: "Nursing"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Excellence in medical education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Islington College",
			FullName:         "Islington College",
			Location:         "Kathmandu",
			Affiliation:      "London Metropolitan University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.4,
			Reviews:          150,
			Programs:         12,
			Established:      "2000",
			Students:         "3k+",
			Description:      "A premier college affiliated with London Metropolitan University, UK, offering international degree programs.",
			Website:          "islington.edu.np",
			Email:            "info@islington.edu.np",
			Phone:            "+977-1-4443300",
			ImageURL:         "https://via.placeholder.com/300x200?text=Islington+College",
			FeaturedPrograms: marshalJSON([]string{"BSc Computing", "BA Business", "MBA"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports", "International Exchange"}),
			Courses: marshalJSON([]Course{
				{Name: "BSc (Hons) Computing", Level: "Undergraduate", Duration: "3 Years", Fees: "NPR 12,00,000", Focus: "Computing, Software Engineering"},
				{Name: "BA (Hons) Business Studies", Level: "Undergraduate", Duration: "3 Years", Fees: "NPR 10,50,000", Focus: "Business, Management"},
				{Name: "MBA", Level: "Postgraduate", Duration: "1.5 Years", Fees: "NPR 8,50,000", Focus: "Business Administration"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Global education for global careers"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "The British College",
			FullName:         "The British College",
			Location:         "Kathmandu",
			Affiliation:      "University of the West of England",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.5,
			Reviews:          140,
			Programs:         15,
			Established:      "2003",
			Students:         "2.5k+",
			Description:      "An exclusive college affiliated with University of the West of England (UWE Bristol), offering UK degrees in Nepal.",
			Website:          "thebritishcollege.edu.np",
			Email:            "admissions@thebritishcollege.edu.np",
			Phone:            "+977-1-4444500",
			ImageURL:         "https://via.placeholder.com/300x200?text=The+British+College",
			FeaturedPrograms: marshalJSON([]string{"BSc Computing", "BA Business Management", "MSc Engineering"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports", "UK Pathway"}),
			Courses: marshalJSON([]Course{
				{Name: "BSc (Hons) Computing", Level: "Undergraduate", Duration: "3 Years", Fees: "NPR 14,00,000", Focus: "Computing, AI"},
				{Name: "BA (Hons) Business Management", Level: "Undergraduate", Duration: "3 Years", Fees: "NPR 12,50,000", Focus: "Business, Marketing"},
				{Name: "MSc Engineering Management", Level: "Postgraduate", Duration: "1.5 Years", Fees: "NPR 9,00,000", Focus: "Engineering, Management"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "British quality education in Nepal"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Herald College Kathmandu",
			FullName:         "Herald College Kathmandu",
			Location:         "Kathmandu",
			Affiliation:      "University of Wolverhampton",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.3,
			Reviews:          120,
			Programs:         10,
			Established:      "2015",
			Students:         "2k+",
			Description:      "A modern college affiliated with University of Wolverhampton, UK, offering internationally recognized degrees.",
			Website:          "heraldcollege.edu.np",
			Email:            "info@heraldcollege.edu.np",
			Phone:            "+977-1-4444600",
			ImageURL:         "https://via.placeholder.com/300x200?text=Herald+College+Kathmandu",
			FeaturedPrograms: marshalJSON([]string{"BSc Computing", "BA Business", "MBA"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Sports"}),
			Courses: marshalJSON([]Course{
				{Name: "BSc (Hons) Computing", Level: "Undergraduate", Duration: "3 Years", Fees: "NPR 11,00,000", Focus: "Computing, Software"},
				{Name: "BA (Hons) Business", Level: "Undergraduate", Duration: "3 Years", Fees: "NPR 9,50,000", Focus: "Business, Management"},
				{Name: "MBA", Level: "Postgraduate", Duration: "1 Year", Fees: "NPR 7,50,000", Focus: "Business Administration"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "Excellence in international education"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
		},
		{
			Name:             "Softwarica College",
			FullName:         "Softwarica College of IT and E-Commerce",
			Location:         "Kathmandu",
			Affiliation:      "Coventry University",
			CollegeType:      "Private",
			Verified:         true,
			Popular:          true,
			Rating:           4.4,
			Reviews:          110,
			Programs:         8,
			Established:      "2008",
			Students:         "1.5k+",
			Description:      "A specialized IT college affiliated with Coventry University, UK, offering computing and technology programs.",
			Website:          "softwarica.edu.np",
			Email:            "info@softwarica.edu.np",
			Phone:            "+977-1-4444700",
			ImageURL:         "https://via.placeholder.com/300x200?text=Softwarica+College",
			FeaturedPrograms: marshalJSON([]string{"BSc Computing", "BSc Cyber Security", "MSc Computing"}),
			Amenities:        marshalJSON([]string{"Labs", "Library", "WiFi", "Hostel", "Tech Labs"}),
			Courses: marshalJSON([]Course{
				{Name: "BSc (Hons) Computing", Level: "Undergraduate", Duration: "3 Years", Fees: "NPR 13,00,000", Focus: "Computing, Development"},
				{Name: "BSc (Hons) Cyber Security", Level: "Undergraduate", Duration: "3 Years", Fees: "NPR 14,00,000", Focus: "Security, Networking"},
				{Name: "MSc Computing", Level: "Postgraduate", Duration: "1.5 Years", Fees: "NPR 8,00,000", Focus: "Advanced Computing"},
			}),
			Scholarships:    marshalJSON([]Scholarship{}),
			Gallery:         marshalJSON([]GalleryImage{}),
			ProgramsList:    marshalJSON([]ProgramDetail{}),
			About:           marshalJSON(AboutData{Vision: "IT education with international standards"}),
			Admissions:      marshalJSON(AdmissionInfo{}),
			AdmissionCards:  marshalJSON([]AdmissionCard{}),
			OfferedPrograms: marshalJSON([]OfferedProgramCategory{}),
			Alumni:          marshalJSON([]AlumniProfile{}),
			Departments:     marshalJSON([]Department{}),
			CollegeReviews:  marshalJSON([]Review{}),
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
		applyProfileFitDefaults(&college)

		if err := db.Create(&college).Error; err != nil {
			log.Printf("Error creating college %s: %v", college.Name, err)
			return err
		}
	}

	log.Println("Successfully seeded colleges with complete data")
	return nil
}

func SeedCollegeUniversityCourseMappings(db *gorm.DB) error {
	if err := db.Unscoped().Where("1 = 1").Delete(&models.CollegeUniversityCourse{}).Error; err != nil {
		return err
	}

	var colleges []models.College
	if err := db.Find(&colleges).Error; err != nil {
		return err
	}

	var universities []models.University
	if err := db.Find(&universities).Error; err != nil {
		return err
	}

	var courses []models.Course
	if err := db.Find(&courses).Error; err != nil {
		return err
	}

	collegeByName := map[string]uint{}
	for _, college := range colleges {
		collegeByName[college.Name] = college.ID
	}

	universityByName := map[string]uint{}
	for _, university := range universities {
		universityByName[university.Name] = university.ID
	}

	courseByTitle := map[string]uint{}
	for _, course := range courses {
		courseByTitle[course.Title] = course.ID
	}

	type mappingSeed struct {
		College    string
		University string
		Course     string
		Status     string
	}

	mappings := []mappingSeed{
		{College: "KUSOE, Dhulikhel Campus", University: "Kathmandu University", Course: "BIT (Bachelor in IT)", Status: "ongoing"},
		{College: "KUSOE, Dhulikhel Campus", University: "Kathmandu University", Course: "B.Sc in Data Science & Artificial Intelligence", Status: "ongoing"},
		{College: "KUSOE, Dhulikhel Campus", University: "Tribhuvan University", Course: "B.Sc CSIT (Computer Science & IT)", Status: "ongoing"},
		{College: "KUSOE, Dhulikhel Campus", University: "Kathmandu University", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "KUSOE, Dhulikhel Campus", University: "Kathmandu University", Course: "MBA", Status: "ongoing"},
		{College: "Pulchowk Campus", University: "Tribhuvan University", Course: "B.Sc CSIT (Computer Science & IT)", Status: "ongoing"},
		{College: "Pulchowk Campus", University: "Pokhara University", Course: "BIT (Bachelor in IT)", Status: "ongoing"},
		{College: "Pulchowk Campus", University: "Kathmandu University", Course: "B.Sc in Data Science & Artificial Intelligence", Status: "ongoing"},
		{College: "Pulchowk Campus", University: "Tribhuvan University", Course: "BE in Civil Engineering", Status: "ongoing"},
		{College: "Pulchowk Campus", University: "Tribhuvan University", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "Pulchowk Campus", University: "Tribhuvan University", Course: "BBA", Status: "ongoing"},
		{College: "Pokhara University School of Engineering", University: "Pokhara University", Course: "BIT (Bachelor in IT)", Status: "ongoing"},
		{College: "Pokhara University School of Engineering", University: "Tribhuvan University", Course: "B.Sc CSIT (Computer Science & IT)", Status: "ongoing"},
		{College: "Pokhara University School of Engineering", University: "Kathmandu University", Course: "B.Sc in Data Science & Artificial Intelligence", Status: "ongoing"},
		{College: "Pokhara University School of Engineering", University: "Pokhara University", Course: "BE in Civil Engineering", Status: "ongoing"},
		{College: "Pokhara University School of Engineering", University: "Pokhara University", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "Everest Engineering College", University: "Pokhara University", Course: "BIT (Bachelor in IT)", Status: "ongoing"},
		{College: "Everest Engineering College", University: "Pokhara University", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "Patan Multiple Campus", University: "Tribhuvan University", Course: "B.Sc CSIT (Computer Science & IT)", Status: "ongoing"},
		{College: "Patan Multiple Campus", University: "Tribhuvan University", Course: "BBA", Status: "ongoing"},
		{College: "Patan Multiple Campus", University: "Tribhuvan University", Course: "BBS", Status: "ongoing"},
		{College: "Patan Multiple Campus", University: "Tribhuvan University", Course: "BA", Status: "ongoing"},
		{College: "Kathmandu Model College", University: "Tribhuvan University", Course: "BBA", Status: "ongoing"},
		{College: "Kathmandu Model College", University: "Tribhuvan University", Course: "BCA", Status: "ongoing"},
		{College: "Kathmandu Model College", University: "Tribhuvan University", Course: "BBS", Status: "ongoing"},
		{College: "Thames International College", University: "Tribhuvan University", Course: "BBA", Status: "ongoing"},
		{College: "Thames International College", University: "Tribhuvan University", Course: "BHM", Status: "ongoing"},
		{College: "Thames International College", University: "Tribhuvan University", Course: "BSW", Status: "ongoing"},
		{College: "Kathmandu Engineering College", University: "Tribhuvan University", Course: "BE in Civil Engineering", Status: "ongoing"},
		{College: "Kathmandu Engineering College", University: "Tribhuvan University", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "Advanced College of Engineering and Management", University: "Tribhuvan University", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "Advanced College of Engineering and Management", University: "Tribhuvan University", Course: "BE in Civil Engineering", Status: "ongoing"},
		{College: "GoldenGate International College", University: "Tribhuvan University", Course: "BBA", Status: "ongoing"},
		{College: "GoldenGate International College", University: "Tribhuvan University", Course: "BCA", Status: "ongoing"},
		{College: "GoldenGate International College", University: "Tribhuvan University", Course: "BBS", Status: "ongoing"},
		{College: "Kathmandu National College", University: "Tribhuvan University", Course: "BBS", Status: "ongoing"},
		{College: "Kathmandu National College", University: "Tribhuvan University", Course: "BA", Status: "ongoing"},
		{College: "Whitefield International College", University: "Tribhuvan University", Course: "BBA", Status: "ongoing"},
		{College: "Whitefield International College", University: "Tribhuvan University", Course: "BCA", Status: "ongoing"},
		{College: "College of Business Management", University: "Tribhuvan University", Course: "MBA", Status: "ongoing"},
		{College: "College of Business Management", University: "Tribhuvan University", Course: "BBA", Status: "ongoing"},
		{College: "College of Business Management", University: "Tribhuvan University", Course: "MBS", Status: "ongoing"},
		{College: "Prithvi Narayan Campus", University: "Tribhuvan University", Course: "BSc CSIT (Computer Science & IT)", Status: "ongoing"},
		{College: "Prithvi Narayan Campus", University: "Tribhuvan University", Course: "BBA", Status: "ongoing"},
		{College: "Prithvi Narayan Campus", University: "Tribhuvan University", Course: "BBS", Status: "ongoing"},
		{College: "Prithvi Narayan Campus", University: "Tribhuvan University", Course: "BA", Status: "ongoing"},
		{College: "Lalit Multiple Campus", University: "Tribhuvan University", Course: "BSc CSIT (Computer Science & IT)", Status: "ongoing"},
		{College: "Lalit Multiple Campus", University: "Tribhuvan University", Course: "BA", Status: "ongoing"},
		{College: "Lalit Multiple Campus", University: "Tribhuvan University", Course: "BBS", Status: "ongoing"},
		{College: "Lalit Multiple Campus", University: "Tribhuvan University", Course: "B.Ed", Status: "ongoing"},
		{College: "Mahendra Multiple Campus Nepalgunj", University: "Tribhuvan University", Course: "BBS", Status: "ongoing"},
		{College: "Mahendra Multiple Campus Nepalgunj", University: "Tribhuvan University", Course: "BA", Status: "ongoing"},
		{College: "Mahendra Multiple Campus Nepalgunj", University: "Tribhuvan University", Course: "LLB", Status: "ongoing"},
		{College: "Birendra Multiple Campus", University: "Tribhuvan University", Course: "BSc CSIT (Computer Science & IT)", Status: "ongoing"},
		{College: "Birendra Multiple Campus", University: "Tribhuvan University", Course: "BBA", Status: "ongoing"},
		{College: "Birendra Multiple Campus", University: "Tribhuvan University", Course: "BBS", Status: "ongoing"},
		{College: "Nepal Engineering College", University: "Pokhara University", Course: "BE in Civil Engineering", Status: "ongoing"},
		{College: "Nepal Engineering College", University: "Pokhara University", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "Gandaki College of Engineering and Science", University: "Pokhara University", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "Gandaki College of Engineering and Science", University: "Pokhara University", Course: "B.Sc CSIT (Computer Science & IT)", Status: "ongoing"},
		{College: "School of Environmental Science and Management", University: "Pokhara University", Course: "B.Sc in Environmental Science", Status: "ongoing"},
		{College: "Brihaspati College", University: "Pokhara University", Course: "BBA", Status: "ongoing"},
		{College: "Brihaspati College", University: "Pokhara University", Course: "BCA", Status: "ongoing"},
		{College: "Tilottama Campus", University: "Pokhara University", Course: "BBA", Status: "ongoing"},
		{College: "Tilottama Campus", University: "Pokhara University", Course: "BCA", Status: "ongoing"},
		{College: "Ace Institute of Management", University: "Pokhara University", Course: "MBA", Status: "ongoing"},
		{College: "Ace Institute of Management", University: "Pokhara University", Course: "BBA", Status: "ongoing"},
		{College: "Ace Institute of Management", University: "Pokhara University", Course: "BHM", Status: "ongoing"},
		{College: "Nepal College of Management", University: "Kathmandu University", Course: "MBA", Status: "ongoing"},
		{College: "Nepal College of Management", University: "Kathmandu University", Course: "BBA", Status: "ongoing"},
		{College: "Nepal College of Management", University: "Kathmandu University", Course: "BHM", Status: "ongoing"},
		{College: "Little Angels College of Management", University: "Kathmandu University", Course: "MBA", Status: "ongoing"},
		{College: "Little Angels College of Management", University: "Kathmandu University", Course: "BBA", Status: "ongoing"},
		{College: "Little Angels College of Management", University: "Kathmandu University", Course: "BCA", Status: "ongoing"},
		{College: "Kathmandu University School of Management", University: "Kathmandu University", Course: "MBA", Status: "ongoing"},
		{College: "Kathmandu University School of Management", University: "Kathmandu University", Course: "MBS", Status: "ongoing"},
		{College: "Kathmandu University School of Management", University: "Kathmandu University", Course: "BBA", Status: "ongoing"},
		{College: "Manipal College of Medical Sciences", University: "Kathmandu University", Course: "MBBS", Status: "ongoing"},
		{College: "Manipal College of Medical Sciences", University: "Kathmandu University", Course: "BDS", Status: "ongoing"},
		{College: "Manipal College of Medical Sciences", University: "Kathmandu University", Course: "B.Sc Nursing", Status: "ongoing"},
		{College: "Islington College", University: "London Metropolitan University", Course: "BIT (Bachelor in IT)", Status: "ongoing"},
		{College: "Islington College", University: "London Metropolitan University", Course: "MBA", Status: "ongoing"},
		{College: "The British College", University: "University of the West of England", Course: "BIT (Bachelor in IT)", Status: "ongoing"},
		{College: "The British College", University: "University of the West of England", Course: "MBA", Status: "ongoing"},
		{College: "The British College", University: "University of the West of England", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "Herald College Kathmandu", University: "University of Wolverhampton", Course: "BIT (Bachelor in IT)", Status: "ongoing"},
		{College: "Herald College Kathmandu", University: "University of Wolverhampton", Course: "MBA", Status: "ongoing"},
		{College: "Softwarica College", University: "Coventry University", Course: "BIT (Bachelor in IT)", Status: "ongoing"},
		{College: "Softwarica College", University: "Coventry University", Course: "BE in Computer Engineering", Status: "ongoing"},
		{College: "Softwarica College", University: "Coventry University", Course: "MSc in Computer Science", Status: "ongoing"},
	}

	for _, item := range mappings {
		collegeID, collegeOk := collegeByName[item.College]
		universityID, universityOk := universityByName[item.University]
		courseID, courseOk := courseByTitle[item.Course]
		if !collegeOk || !universityOk || !courseOk {
			continue
		}

		relation := models.CollegeUniversityCourse{
			CollegeID:    collegeID,
			UniversityID: universityID,
			CourseID:     courseID,
			Status:       item.Status,
		}

		if err := db.Create(&relation).Error; err != nil {
			return err
		}
	}

	for _, course := range courses {
		var offeringCount int64
		if err := db.Model(&models.CollegeUniversityCourse{}).
			Distinct("college_id").
			Where("course_id = ?", course.ID).
			Count(&offeringCount).Error; err != nil {
			return err
		}

		if err := db.Model(&models.Course{}).
			Where("id = ?", course.ID).
			Update("colleges_count", int(offeringCount)).Error; err != nil {
			return err
		}
	}

	log.Println("Successfully seeded college-university-course mappings")
	return nil
}

// Seed function that calls all seeding functions
func Seed() error {
	db := config.GetDB()
	if db == nil {
		return nil
	}
	if err := SeedSuperAdmin(db); err != nil {
		return err
	}
	// if err := SeedUniversities(db); err != nil {
	// 	return err
	// }
	// if err := SeedCourses(db); err != nil {
	// 	return err
	// }
	// if err := SeedColleges(db); err != nil {
	// 	return err
	// }
	if err := SeedCollegeUniversityCourseMappings(db); err != nil {
		return err
	}
	if err := SeedExams(db); err != nil {
		return err
	}
	if err := SeedNews(db); err != nil {
		return err
	}
	if err := SeedEvents(db); err != nil {
		return err
	}
	if err := SeedForum(); err != nil {
		return err
	}
	return SeedScholarships(db)
}

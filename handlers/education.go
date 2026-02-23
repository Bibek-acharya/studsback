package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"studsphere/backend/utils"
)

type RankingCollege struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Location string   `json:"location"`
	Rank     int      `json:"rank"`
	Color    string   `json:"color"`
	Logo     string   `json:"logo"`
	Stats    any      `json:"stats"`
	Tags     []string `json:"tags"`
}

type EducationExam struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Board        string   `json:"board"`
	Badges       []string `json:"badges"`
	Level        string   `json:"level"`
	Type         string   `json:"type"`
	ExamDate     string   `json:"examDate"`
	FormDeadline string   `json:"formDeadline"`
	Fee          string   `json:"fee"`
	Highlights   []string `json:"highlights"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
}

type EducationExamDetails struct {
	Title      string   `json:"title"`
	ShortTitle string   `json:"shortTitle"`
	Board      string   `json:"board"`
	Location   string   `json:"location"`
	Badges     []string `json:"badges"`
	ExamDate   string   `json:"examDate"`
	Deadline   string   `json:"deadline"`
	Overview   string   `json:"overview"`
	Weightage  any      `json:"weightage"`
	Timeline   any      `json:"timeline"`
	Notices    any      `json:"notices"`
	Faqs       any      `json:"faqs"`
}

type EducationScholarship struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Provider    string   `json:"provider"`
	LogoColor   string   `json:"logoColor"`
	Initials    string   `json:"initials"`
	LogoText    string   `json:"logoText"`
	LogoBg      string   `json:"logoBg"`
	Location    string   `json:"location"`
	Type        string   `json:"type"`
	Amount      string   `json:"amount"`
	Deadline    string   `json:"deadline"`
	Status      string   `json:"status"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Image       string   `json:"image"`
	Eligibility string   `json:"eligibility"`
	Tags        []string `json:"tags"`
}

type EducationCourse struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	ShortTitle string   `json:"shortTitle"`
	Colleges   int      `json:"colleges"`
	Affiliation string  `json:"affiliation"`
	Badges     []string `json:"badges"`
	Level      string   `json:"level"`
	Field      string   `json:"field"`
	Duration   string   `json:"duration"`
	EstFee     string   `json:"estFee"`
	Highlights []string `json:"highlights"`
	CareerPath string   `json:"careerPath"`
	Description string  `json:"description"`
	Location   string   `json:"location"`
	GovtFee    string   `json:"govtFee"`
	PrivateFee string   `json:"privateFee"`
}

type EducationAdmissionCollege struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Location     string  `json:"location"`
	Logo         string  `json:"logo"`
	Rating       float64 `json:"rating"`
	University   string  `json:"university"`
	Description  string  `json:"description"`
	Facilities   []string `json:"facilities"`
	Programs     []gin.H `json:"programs"`
	PhoneNumber  string  `json:"phoneNumber"`
	ContactEmail string  `json:"contactEmail"`
	Website      string  `json:"website"`
}

func rankingsData() []RankingCollege {
	return []RankingCollege{
		{ID: 1, Name: "Pulchowk Engineering Campus", Location: "Lalitpur", Rank: 1, Color: "bg-rose-600", Logo: "P", Stats: gin.H{"year": "1972", "rating": 4.8}, Tags: []string{"Science & Tech"}},
		{ID: 2, Name: "Kathmandu University (SOE)", Location: "Dhulikhel", Rank: 2, Color: "bg-blue-600", Logo: "KU", Stats: gin.H{"year": "1994", "rating": 4.7}, Tags: []string{"Science & Tech", "Management"}},
		{ID: 3, Name: "Maharajgunj Medical Campus", Location: "Kathmandu", Rank: 3, Color: "bg-emerald-600", Logo: "M", Stats: gin.H{"year": "1972", "rating": 4.9}, Tags: []string{"Medical"}},
		{ID: 4, Name: "St. Xaviers College", Location: "Maitighar", Rank: 4, Color: "bg-indigo-600", Logo: "SX", Stats: gin.H{"year": "1988", "rating": 4.6}, Tags: []string{"Science & Tech", "Humanities"}},
		{ID: 5, Name: "Apex College", Location: "Baneshwor", Rank: 5, Color: "bg-orange-600", Logo: "A", Stats: gin.H{"year": "2000", "rating": 4.4}, Tags: []string{"Management"}},
		{ID: 6, Name: "Islington College", Location: "Kamalpokhari", Rank: 6, Color: "bg-cyan-600", Logo: "I", Stats: gin.H{"year": "1996", "rating": 4.5}, Tags: []string{"Science & Tech", "Management"}},
	}
}

func examsData() []EducationExam {
	return []EducationExam{
		{ID: "neb-class-12", Title: "NEB Class 12 Annual Examination 2081", Board: "NEB (National Examination Board)", Badges: []string{"BOARD EXAM", "UPCOMING"}, Level: "+2 / Intermediate", Type: "Board Exam", ExamDate: "Baishakh 14, 2082 (Apr 27, 2025)", FormDeadline: "Magh 20, 2081 (Feb 3, 2025)", Fee: "NPR 600", Highlights: []string{"Official routine published", "Form filling active", "Admit cards by Chaitra end"}, Description: "Annual final examination for Grade 12 students across Nepal.", Status: "active"},
		{ID: "ioe-entrance", Title: "IOE Entrance Examination 2081", Board: "Institute of Engineering, TU", Badges: []string{"ENTRANCE", "POPULAR"}, Level: "Undergraduate (Bachelor)", Type: "Entrance Exam", ExamDate: "Jestha 2082 (May/June 2025)", FormDeadline: "Chaitra 2081 (Mar/Apr 2025)", Fee: "NPR 2000", Highlights: []string{"BE Computer, Civil, Arch", "140 MCQ Questions", "Pulchowk, Thapathali seats"}, Description: "Entrance exam for BE programs at IOE constituent campuses.", Status: "upcoming"},
		{ID: "cee-medical", Title: "CEE Medical Entrance 2081", Board: "Institute of Medicine, TU", Badges: []string{"ENTRANCE", "COMPETITIVE"}, Level: "Undergraduate (Bachelor)", Type: "Entrance Exam", ExamDate: "Jestha 2082 (May/June 2025)", FormDeadline: "Chaitra 2081 (Mar/Apr 2025)", Fee: "NPR 2500", Highlights: []string{"MBBS, BDS, BSc Nursing", "200 MCQ Questions", "IOM, BPKIHS, PAHS seats"}, Description: "Common Entrance Examination for Medical and Health Sciences.", Status: "upcoming"},
		{ID: "cmat", Title: "CMAT (Common Management Admission Test)", Board: "Faculty of Management, TU", Badges: []string{"ENTRANCE", "POPULAR"}, Level: "Undergraduate (Bachelor)", Type: "Entrance Exam", ExamDate: "Jestha 2082 (May/June 2025)", FormDeadline: "Chaitra 2081 (Mar/Apr 2025)", Fee: "NPR 1500", Highlights: []string{"BBA, BHM programs", "Multiple test centers", "High demand"}, Description: "Entrance for management programs under TU affiliation.", Status: "upcoming"},
	}
}

func examDetailsData() map[string]EducationExamDetails {
	return map[string]EducationExamDetails{
		"ioe-entrance": {
			Title:      "IOE Entrance Examination 2081",
			ShortTitle: "IOE B.E./B.Arch",
			Board:      "Institute of Engineering, TU",
			Location:   "Chakupat, Lalitpur",
			Badges:     []string{"IOE / TU Affiliated", "Applications Open"},
			ExamDate:   "Falgun 25, 2081 (Mar 09, 2025)",
			Deadline:   "Falgun 10, 2081",
			Overview:   "The gateway to Pulchowk Campus and other constituent engineering colleges. Get details on syllabus, shifts, and admit cards.",
			Weightage:  []gin.H{{"label": "Mathematics", "marks": 50, "color": "bg-brand-500", "width": "35%"}, {"label": "Physics", "marks": 45, "color": "bg-blue-500", "width": "32%"}, {"label": "Chemistry", "marks": 25, "color": "bg-teal-500", "width": "18%"}, {"label": "English & Aptitude", "marks": 20, "color": "bg-purple-500", "width": "15%"}},
			Timeline:   []gin.H{{"event": "Application Opens", "bs": "Magh 20, 2081", "ad": "Feb 02, 2025", "status": "Active", "statusColor": "bg-green-100 text-green-700"}, {"event": "Deadline (Regular)", "bs": "Falgun 10, 2081", "ad": "Feb 22, 2025", "status": "Closing Soon", "statusColor": "bg-yellow-100 text-yellow-700"}, {"event": "Entrance Exam Start", "bs": "Falgun 25, 2081", "ad": "Mar 09, 2025", "status": "Upcoming", "statusColor": "bg-slate-100 text-slate-600"}},
			Notices:    []gin.H{{"id": 1, "title": "Call for Applications: BE/B.Arch Entrance 2081", "month": "Magh", "date": "20", "source": "Exam Board, Chakupat", "time": "2 days ago", "urgent": true}, {"id": 2, "title": "Update on Mathematics Syllabus (Vector & 3D)", "month": "Poush", "date": "05", "source": "Syllabus Committee", "time": "Jan 05, 2025", "urgent": false}},
			Faqs:       []gin.H{{"q": "Is there negative marking?", "a": "Yes. For every incorrect answer, 10% of the marks assigned to that question will be deducted."}, {"q": "Can I apply if my Grade 12 result is pending?", "a": "Yes. Students awaiting results can apply, but you must submit your transcript during the college admission process if you pass."}},
		},
		"neb-class-12": {
			Title:      "NEB Class 12 Board Examination 2081",
			ShortTitle: "NEB Class 12",
			Board:      "National Examination Board",
			Location:   "Sanothimi, Bhaktapur",
			Badges:     []string{"National Board", "Forms Active"},
			ExamDate:   "Baishakh 14, 2082",
			Deadline:   "Magh 20, 2081",
			Overview:   "The final gateway for secondary education in Nepal. Mandatory for higher studies.",
			Weightage:  []gin.H{{"label": "Theory", "marks": 75, "color": "bg-brand-500", "width": "75%"}, {"label": "Practical", "marks": 25, "color": "bg-emerald-500", "width": "25%"}},
			Timeline:   []gin.H{{"event": "Form Submission", "bs": "Magh 05, 2081", "ad": "Jan 18, 2025", "status": "Active", "statusColor": "bg-green-100 text-green-700"}, {"event": "Exam Date", "bs": "Baishakh 14, 2082", "ad": "Apr 27, 2025", "status": "Scheduled", "statusColor": "bg-amber-100 text-amber-700"}},
			Notices:    []gin.H{{"id": 1, "title": "Official Routine: Grade 12 Annual Exam 2081", "month": "Magh", "date": "02", "source": "NEB Office", "time": "5 days ago", "urgent": true}},
			Faqs:       []gin.H{{"q": "What is the pass criteria?", "a": "Minimum 35% in theory and 40% in practical for each subject individually."}},
		},
	}
}

func scholarshipsData() []EducationScholarship {
	return []EducationScholarship{
		{ID: 1, Title: "Future Tech Leaders Grant", Provider: "TechFoundation Global", LogoColor: "bg-blue-600", Initials: "TG", LogoText: "TG", LogoBg: "bg-blue-600", Location: "San Francisco, CA", Type: "MERIT-BASED", Amount: "$15,000", Deadline: "Oct 15, 2024", Status: "OPEN", Category: "Technology", Description: "Supports outstanding undergraduate students pursuing degrees in Computer Science, Engineering, or Data Science.", Image: "https://images.unsplash.com/photo-1519389950473-47ba0277781c?q=80&w=2070&auto=format&fit=crop", Eligibility: "GPA 3.5+", Tags: []string{"Technology", "GPA 3.5+"}},
		{ID: 2, Title: "Women in STEM Initiative", Provider: "Global Science Alliance", LogoColor: "bg-pink-600", Initials: "GS", LogoText: "GS", LogoBg: "bg-pink-600", Location: "London, UK", Type: "FULL TUITION", Amount: "$25,000", Deadline: "Mar 01, 2024", Status: "CLOSING SOON", Category: "Science", Description: "Dedicated to empowering women in science and mathematics. This scholarship covers full tuition for one academic year.", Image: "https://images.unsplash.com/photo-1573166368361-3f5231646f25?q=80&w=2069&auto=format&fit=crop", Eligibility: "Female Undergrad", Tags: []string{"Science", "Female Undergrad"}},
		{ID: 3, Title: "Community Arts Fund", Provider: "National Arts Council", LogoColor: "bg-purple-600", Initials: "NA", LogoText: "NA", LogoBg: "bg-purple-600", Location: "New York, NY", Type: "GRANT", Amount: "$5,000", Deadline: "Jan 15, 2024", Status: "CLOSED", Category: "Arts", Description: "For students demonstrating exceptional talent in visual or performing arts who have contributed significantly.", Image: "https://images.unsplash.com/photo-1460661419201-fd4cecdf8a8b?q=80&w=2080&auto=format&fit=crop", Eligibility: "Portfolio Required", Tags: []string{"Arts", "Portfolio Required"}},
		{ID: 4, Title: "Global Business Merit", Provider: "Enterprise Corp", LogoColor: "bg-emerald-600", Initials: "EC", LogoText: "EC", LogoBg: "bg-emerald-600", Location: "Remote / Online", Type: "MERIT-BASED", Amount: "$10,000", Deadline: "Nov 30, 2024", Status: "OPEN", Category: "Business", Description: "Awarded to MBA students with a strong track record of entrepreneurial spirit and business leadership.", Image: "https://images.unsplash.com/photo-1556761175-5973dc0f32e7?q=80&w=2032&auto=format&fit=crop", Eligibility: "MBA Students", Tags: []string{"Business", "MBA"}},
		{ID: 5, Title: "Medical Research Fellow", Provider: "HealthFirst Institute", LogoColor: "bg-red-600", Initials: "HF", LogoText: "HF", LogoBg: "bg-red-600", Location: "Boston, MA", Type: "FELLOWSHIP", Amount: "$50,000", Deadline: "Feb 28, 2024", Status: "CLOSING SOON", Category: "Medical", Description: "A prestigious grant for postgraduate students conducting breakthrough research in immunology.", Image: "https://images.unsplash.com/photo-1532938911079-1b06ac7ceec7?q=80&w=1932&auto=format&fit=crop", Eligibility: "PhD Candidates", Tags: []string{"Medical", "Post-Grad"}},
		{ID: 6, Title: "Athletic Excellence Award", Provider: "Sports United", LogoColor: "bg-orange-600", Initials: "SU", LogoText: "SU", LogoBg: "bg-orange-600", Location: "Chicago, IL", Type: "PERFORMANCE", Amount: "$8,000", Deadline: "Sep 01, 2024", Status: "OPEN", Category: "Sports", Description: "Recognizing student-athletes who balance high performance in sports with academic excellence.", Image: "https://images.unsplash.com/photo-1517649763962-0c623066013b?q=80&w=2070&auto=format&fit=crop", Eligibility: "Varsity Athletes", Tags: []string{"Sports", "Athletes"}},
	}
}

func scholarshipCategoriesData() []gin.H {
	return []gin.H{
		{"id": "college", "name": "College Based", "title": "College-Based", "count": 150, "subtitle": "12 Scholarships Open", "desc": "Direct aid from universities for enrolled students.", "icon": "fa-building-columns", "color": "blue"},
		{"id": "school", "name": "School Based", "title": "School-Based", "count": 85, "subtitle": "25 Scholarships Open", "desc": "For students excelling in secondary education.", "icon": "fa-graduation-cap", "color": "indigo"},
		{"id": "institutional", "name": "Institutional Merit", "title": "Institutional Merit", "count": 210, "subtitle": "50+ Awards Available", "desc": "Awarded to students with outstanding academic achievements.", "icon": "fa-medal", "color": "emerald"},
		{"id": "need", "name": "Institutional Need", "title": "Institutional Need", "count": 190, "subtitle": "100+ Grants Open", "desc": "Financial aid for students demonstrating significant financial need.", "icon": "fa-hand-holding-heart", "color": "amber"},
		{"id": "entrance", "name": "Entrance", "title": "Entrance", "count": 45, "subtitle": "10 Top Ranker Awards", "desc": "Scholarships for top rankers in IOE, IOM, and exams.", "icon": "fa-pencil", "color": "purple"},
		{"id": "ngo", "name": "NGO / INGO", "title": "NGO / INGO", "count": 60, "subtitle": "8 Partner Programs", "desc": "Supported by international and national organizations.", "icon": "fa-globe", "color": "rose"},
	}
}

func coursesData() []EducationCourse {
	return []EducationCourse{
		{ID: "1", Title: "B.Sc CSIT (Computer Science & IT)", ShortTitle: "BSc CSIT", Colleges: 34, Affiliation: "TU Affiliated", Badges: []string{"Top Choice", "High Growth"}, Level: "Bachelor", Field: "IT / Computing", Duration: "4 Years", EstFee: "NPR 4L - 8L", Highlights: []string{"Merit Scholarships (20 Seats)", "Internship Guaranteed", "Practical Based"}, CareerPath: "Software Engineer, System Analyst, AI Researcher", Description: "Build a strong foundation in software development, networking, databases, and modern IT systems.", Location: "Available in Nepal", GovtFee: "NPR 3,50,000", PrivateFee: "NPR 8,50,000 - 12,00,000"},
		{ID: "2", Title: "BIT (Bachelor in IT)", ShortTitle: "BIT", Colleges: 15, Affiliation: "Foreign Degree", Badges: []string{"Global Value", "Industry Ready"}, Level: "Bachelor", Field: "IT / Computing", Duration: "4 Years", EstFee: "NPR 6L - 10L", Highlights: []string{"Direct Entry", "Job Assistance", "Dual Certification"}, CareerPath: "IT Consultant, Cloud Architect, Web Developer", Description: "Comprehensive program designed to prepare you for a successful global tech career.", Location: "Available in Nepal", GovtFee: "NPR 3,50,000", PrivateFee: "NPR 8,50,000 - 12,00,000"},
		{ID: "3", Title: "MBA (Business Administration)", ShortTitle: "MBA", Colleges: 25, Affiliation: "Pokhara University", Badges: []string{"Executive", "Networking"}, Level: "Master", Field: "Management", Duration: "2 Years", EstFee: "NPR 3L - 6L", Highlights: []string{"Corporate Guest Sessions", "Leadership Focus", "Case Study Method"}, CareerPath: "Operations Manager, Entrepreneur, HR Director", Description: "Leadership-oriented management degree focused on practical business execution.", Location: "Available in Nepal", GovtFee: "NPR 2,50,000", PrivateFee: "NPR 5,50,000 - 9,00,000"},
	}
}

func admissionsData() []EducationAdmissionCollege {
	return []EducationAdmissionCollege{
		{ID: "1", Name: "Goldenagete International College", Location: "Kamal Pokhari, Kathmandu", Logo: "https://ui-avatars.com/api/?name=GIC&background=0A61EF&color=fff", Rating: 4.8, University: "TU", Description: "A premier institution focused on excellence in Management and Technology.", Facilities: []string{"IT Lab", "Library", "Cafeteria", "Sports"}, Programs: []gin.H{{"name": "BBA", "level": "Bachelor", "status": "Ongoing"}, {"name": "BSc CSIT", "level": "Bachelor", "status": "Ongoing"}, {"name": "MBA", "level": "Master", "status": "Closed"}}, PhoneNumber: "01-4433221", ContactEmail: "admission@goldenagete.edu.np", Website: "goldenagete.edu.np"},
		{ID: "2", Name: "KIST College & SS", Location: "Kamalpokhari, Kathmandu", Logo: "https://ui-avatars.com/api/?name=KIST&background=2563EB&color=fff", Rating: 4.5, University: "TU", Description: "KIST offers top-notch facilities and a dedicated faculty team.", Facilities: []string{"E-Library", "Physics Lab", "Hostel"}, Programs: []gin.H{{"name": "BIT", "level": "Bachelor", "status": "Ongoing"}, {"name": "BIM", "level": "Bachelor", "status": "Ongoing"}, {"name": "MBS", "level": "Master", "status": "Ongoing"}}, PhoneNumber: "01-4422334", ContactEmail: "info@kist.edu.np", Website: "kist.edu.np"},
		{ID: "3", Name: "St. Xavier's College", Location: "Maitighar, Kathmandu", Logo: "https://ui-avatars.com/api/?name=SX&background=0f172a&color=fff", Rating: 4.9, University: "TU", Description: "Renowned for its academic rigour and Jesuit values.", Facilities: []string{"Research Center", "Auditorium", "Chapel"}, Programs: []gin.H{{"name": "BSc Physics", "level": "Bachelor", "status": "Ongoing"}, {"name": "BSc Microbiology", "level": "Bachelor", "status": "Ongoing"}, {"name": "BA", "level": "Bachelor", "status": "Ongoing"}}, PhoneNumber: "01-4221311", ContactEmail: "sxm@sxc.edu.np", Website: "sxc.edu.np"},
	}
}

func GetEducationRankings(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Education rankings retrieved successfully", gin.H{"colleges": rankingsData()})
}

func GetEducationExams(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Education exams retrieved successfully", gin.H{"exams": examsData()})
}

func GetEducationExamByID(c *gin.Context) {
	examID := c.Param("id")
	details := examDetailsData()
	exam, ok := details[examID]
	if !ok {
		utils.ErrorResponse(c, http.StatusNotFound, "Exam not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Education exam retrieved successfully", exam)
}

func GetEducationScholarships(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Education scholarships retrieved successfully", gin.H{
		"scholarships": scholarshipsData(),
		"categories":   scholarshipCategoriesData(),
	})
}

func GetEducationCourses(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Education courses retrieved successfully", gin.H{"courses": coursesData()})
}

func GetEducationCourseByID(c *gin.Context) {
	courseID := c.Param("id")
	for _, course := range coursesData() {
		if course.ID == courseID {
			utils.SuccessResponse(c, http.StatusOK, "Education course retrieved successfully", course)
			return
		}
	}
	utils.ErrorResponse(c, http.StatusNotFound, "Course not found")
}

func GetEducationAdmissions(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Education admissions retrieved successfully", gin.H{"colleges": admissionsData()})
}

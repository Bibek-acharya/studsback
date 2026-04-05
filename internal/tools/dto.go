package tools

type ScholarshipFinderRequest struct {
	EducationLevel string   `json:"education_level" binding:"required"`
	StudyMode      string   `json:"study_mode"`
	AcademicScore  string   `json:"academic_score"`
	TargetCountry  string   `json:"target_country"`
	NeedType       string   `json:"need_type"`
	Skills         []string `json:"skills"`
	Achievements   []string `json:"achievements"`
	Involvements   []string `json:"involvements"`
}

type CollegeRecommenderRequest struct {
	StudentType        string `json:"student_type" binding:"required,oneof=academic campus career balanced"`
	ProgramInterest    string `json:"program_interest"`
	PreferredLocation  string `json:"preferred_location"`
	BudgetPreference   string `json:"budget_preference"`
	CampusLifePriority string `json:"campus_life_priority"`
	CareerGoal         string `json:"career_goal"`
	NeedScholarship    bool   `json:"need_scholarship"`
	PreferredMode      string `json:"preferred_mode"`
	CollegeType        string `json:"college_type"`
	FinalPriority      string `json:"final_priority"`
}

type ScholarshipRecommendation struct {
	ID          uint     `json:"id"`
	Title       string   `json:"title"`
	Provider    string   `json:"provider"`
	Location    string   `json:"location"`
	Value       string   `json:"value"`
	Deadline    string   `json:"deadline"`
	DegreeLevel string   `json:"degree_level"`
	FundingType string   `json:"funding_type"`
	ScholarType string   `json:"scholarship_type"`
	Description string   `json:"description"`
	ImageURL    string   `json:"image_url"`
	MatchScore  int      `json:"match_score"`
	Reasons     []string `json:"reasons"`
}

type CollegeRecommendation struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Location    string   `json:"location"`
	Affiliation string   `json:"affiliation"`
	Type        string   `json:"type"`
	Rating      float64  `json:"rating"`
	Reviews     int      `json:"reviews"`
	ImageURL    string   `json:"image_url"`
	Website     string   `json:"website"`
	MatchScore  int      `json:"match_score"`
	Reasons     []string `json:"reasons"`
}

type ScholarshipRecommendationsResponse struct {
	Recommendations []ScholarshipRecommendation `json:"recommendations"`
}

type CollegeRecommendationsResponse struct {
	Recommendations []CollegeRecommendation `json:"recommendations"`
}

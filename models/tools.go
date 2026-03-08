package models

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

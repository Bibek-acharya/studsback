package admission

type CreateAdmissionRequest struct {
	CollegeID         uint   `json:"college_id" binding:"required"`
	ProgramName       string `json:"program_name" binding:"required"`
	ProgramLevel      string `json:"program_level" binding:"required"`
	StudentName       string `json:"student_name" binding:"required"`
	StudentEmail      string `json:"student_email" binding:"required,email"`
	StudentPhone      string `json:"student_phone" binding:"required"`
	DateOfBirth       string `json:"date_of_birth"`
	Gender            string `json:"gender"`
	Address           string `json:"address"`
	City              string `json:"city"`
	LastQualification string `json:"last_qualification"`
	Institution       string `json:"institution"`
	GPA               string `json:"gpa"`
	EntranceScore     string `json:"entrance_score"`
	Statement         string `json:"statement"`
}

type UpdateAdmissionRequest struct {
	ProgramName       *string `json:"program_name"`
	ProgramLevel      *string `json:"program_level"`
	StudentName       *string `json:"student_name"`
	StudentEmail      *string `json:"student_email"`
	StudentPhone      *string `json:"student_phone"`
	DateOfBirth       *string `json:"date_of_birth"`
	Gender            *string `json:"gender"`
	Address           *string `json:"address"`
	City              *string `json:"city"`
	LastQualification *string `json:"last_qualification"`
	Institution       *string `json:"institution"`
	GPA               *string `json:"gpa"`
	EntranceScore     *string `json:"entrance_score"`
	Statement         *string `json:"statement"`
}

type UpdateAdmissionStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending under_review approved rejected waitlisted"`
	Notes  string `json:"notes"`
}

type AdmissionResponse struct {
	ID                uint        `json:"id"`
	CreatedAt         string      `json:"created_at"`
	UpdatedAt         string      `json:"updated_at"`
	UserID            *uint       `json:"user_id,omitempty"`
	CollegeID         uint        `json:"college_id"`
	ProgramName       string      `json:"program_name"`
	ProgramLevel      string      `json:"program_level"`
	StudentName       string      `json:"student_name"`
	StudentEmail      string      `json:"student_email"`
	StudentPhone      string      `json:"student_phone"`
	DateOfBirth       *string     `json:"date_of_birth,omitempty"`
	Gender            string      `json:"gender,omitempty"`
	Address           string      `json:"address,omitempty"`
	City              string      `json:"city,omitempty"`
	LastQualification string      `json:"last_qualification,omitempty"`
	Institution       string      `json:"institution,omitempty"`
	GPA               string      `json:"gpa,omitempty"`
	EntranceScore     string      `json:"entrance_score,omitempty"`
	Statement         string      `json:"statement,omitempty"`
	Status            string      `json:"status"`
	Notes             string      `json:"notes,omitempty"`
	ReviewedBy        *uint       `json:"reviewed_by,omitempty"`
	ReviewedAt        *string     `json:"reviewed_at,omitempty"`
	College           *CollegeDTO `json:"college,omitempty"`
	User              *UserDTO    `json:"user,omitempty"`
}

type CollegeDTO struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type UserDTO struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
}

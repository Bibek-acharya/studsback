package counselling

type CreateCounsellingBookingRequest struct {
	College          string `json:"college" binding:"required"`
	ProgramLevel     string `json:"program_level" binding:"required"`
	InterestedCourse string `json:"interested_course" binding:"required"`
	SessionMode      string `json:"session_mode" binding:"required"`
	SessionDate      string `json:"session_date" binding:"required"`
	SessionTime      string `json:"session_time" binding:"required"`
	StudentName      string `json:"student_name" binding:"required"`
	StudentPhone     string `json:"student_phone" binding:"required"`
	StudentEmail     string `json:"student_email" binding:"required,email"`
	StudentNotes     string `json:"student_notes"`
}

type CounsellingBookingResponse struct {
	ID               uint   `json:"id"`
	College          string `json:"college"`
	ProgramLevel     string `json:"program_level"`
	InterestedCourse string `json:"interested_course"`
	SessionMode      string `json:"session_mode"`
	SessionDate      string `json:"session_date"`
	SessionTime      string `json:"session_time"`
	StudentName      string `json:"student_name"`
	StudentPhone     string `json:"student_phone"`
	StudentEmail     string `json:"student_email"`
	StudentNotes     string `json:"student_notes"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
}

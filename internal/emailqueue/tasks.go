package emailqueue

const (
	TypeSendOTPEmail     = "send_otp_email"
	TypeSendWelcomeEmail = "send_welcome_email"
	TypeSendReviewEmail  = "send_review_email"
	TypeSendGenericHTML  = "send_generic_html"
	TypeSendAdmitCard    = "send_admit_card"
)

type Payload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	From    string `json:"from,omitempty"`
}

type OTPEmailPayload struct {
	To        string `json:"to"`
	OTP       string `json:"otp"`
	ExpiresIn int    `json:"expires_in,omitempty"`
}

type WelcomeEmailPayload struct {
	To          string `json:"to"`
	FirstName   string `json:"first_name"`
	VerifyToken string `json:"verify_token,omitempty"`
}

type ReviewEmailPayload struct {
	To          string `json:"to"`
	CollegeName string `json:"college_name"`
	ReviewLink  string `json:"review_link,omitempty"`
}

// AdmitCardPayload holds all data needed to generate an admit card PDF
// and send it via email in the async worker.
type AdmitCardPayload struct {
	Email            string `json:"email"`
	CandidateName    string `json:"candidate_name"`
	DateOfBirth      string `json:"date_of_birth"`
	Gender           string `json:"gender"`
	RollNumber       string `json:"roll_number"`
	ExamCentre       string `json:"exam_centre"`
	Stream           string `json:"stream"`
	PhotoURL         string `json:"photo_url"`
	ScholarshipTitle string `json:"scholarship_title"`
	Provider         string `json:"provider"`
	ExamDate         string `json:"exam_date"`
	ExamTime         string `json:"exam_time"`
	Shift            string `json:"shift"`
	SubjectName      string `json:"subject_name"`
}

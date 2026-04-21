package emailqueue

const (
	TypeSendOTPEmail     = "send_otp_email"
	TypeSendWelcomeEmail = "send_welcome_email"
	TypeSendReviewEmail  = "send_review_email"
	TypeSendGenericHTML  = "send_generic_html"
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

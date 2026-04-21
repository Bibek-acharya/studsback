package utils

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/smtp"

	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/shared/config"
)

func GenerateOTP() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func SendOTPEmail(to, otp string) error {
	if emailqueue.Queue != nil {
		log.Printf("Enqueueing OTP email to %s via Asynq", to)
		return emailqueue.EnqueueSendOTPEmail(to, otp, 10)
	}

	log.Printf("Asynq not available, sending OTP directly to %s", to)
	return sendOTPDirect(to, otp)
}

func sendOTPDirect(to, otp string) error {
	cfg := config.AppConfig
	smtpHost := cfg.SMTPHost
	smtpPort := cfg.SMTPPort
	smtpUser := cfg.SMTPUser
	smtpPass := cfg.SMTPPass
	fromName := "StudSphere"

	log.Printf("SMTP Config: host=%s, port=%s, user=%s, pass_set=%t", smtpHost, smtpPort, smtpUser, smtpPass != "")

	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}
	if smtpPort == "" {
		smtpPort = "587"
	}

	subject := "Your StudSphere Verification Code"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Inter, Arial, sans-serif; background: #f8fafc; padding: 40px 0;">
  <div style="max-width: 480px; margin: 0 auto; background: #fff; border-radius: 16px; border: 1px solid #e2e8f0; overflow: hidden;">
    <div style="background: #2563eb; padding: 32px; text-align: center;">
      <h1 style="color: #fff; font-size: 24px; font-weight: 900; letter-spacing: -0.5px; margin: 0;">StudSphere</h1>
    </div>
    <div style="padding: 40px 32px;">
      <h2 style="color: #1e293b; font-size: 20px; font-weight: 700; margin: 0 0 8px;">Verify your email address</h2>
      <p style="color: #64748b; font-size: 15px; margin: 0 0 32px;">Enter the code below to complete your registration.</p>
      <div style="background: #f1f5f9; border-radius: 12px; padding: 24px; text-align: center; letter-spacing: 12px; font-size: 36px; font-weight: 900; color: #1e293b; margin-bottom: 32px;">
        %s
      </div>
      <p style="color: #94a3b8; font-size: 13px; text-align: center; margin: 0;">This code expires in <strong>10 minutes</strong>. Do not share it with anyone.</p>
    </div>
    <div style="background: #f8fafb; padding: 24px 32px; text-align: center; border-top: 1px solid #f3f4f6;">
      <div style="margin-bottom: 16px;">
        <a href="https://www.facebook.com/share/1CEcyRH9ZZ/" style="text-decoration: none; margin: 0 6px; display: inline-block;">
          <img src="https://upload.wikimedia.org/wikipedia/commons/5/51/Facebook_f_logo_%282019%29.svg" alt="Facebook" width="28" height="28" style="display: block;">
        </a>
        <a href="https://www.instagram.com/stud.sphere?igsh=NDM5Z29nc2ZqMmc=" style="text-decoration: none; margin: 0 6px; display: inline-block;">
          <img src="https://upload.wikimedia.org/wikipedia/commons/e/e7/Instagram_logo_2016.svg" alt="Instagram" width="28" height="28" style="display: block;">
        </a>
        <a href="https://www.tiktok.com/@stud.sphere?_r=1&_t=ZS-95OYyC0vodM" style="text-decoration: none; margin: 0 6px; display: inline-block;">
          <img src="https://cdn-icons-png.flaticon.com/512/2111/2111468.png" alt="TikTok" width="28" height="28" style="display: block;">
        </a>
        <a href="https://wa.me/9779800000000" style="text-decoration: none; margin: 0 6px; display: inline-block;">
          <img src="https://upload.wikimedia.org/wikipedia/commons/6/6b/WhatsApp.svg" alt="WhatsApp" width="28" height="28" style="display: block;">
        </a>
      </div>
      <p style="color: #9ca3af; font-size: 12px; margin: 0;">This email can't receive replies. For more information, visit the <a href="https://studsphere.com/help" style="color: #2563eb; text-decoration: none;">Studsphere Help Center</a>.</p>
      <p style="color: #9ca3af; font-size: 12px; margin: 8px 0 0;">&copy; 2026 Studsphere Inc.</p>
    </div>
  </div>
</body>
</html>`, otp)

	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg := []byte(fmt.Sprintf("From: %s <%s>\nTo: %s\nSubject: %s\n%s%s", fromName, smtpUser, to, subject, mime, body))

	var auth smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	addr := smtpHost + ":" + smtpPort
	log.Printf("Attempting to send OTP email to %s via SMTP %s (user: %s)", to, addr, smtpUser)
	if err := smtp.SendMail(addr, auth, smtpUser, []string{to}, msg); err != nil {
		log.Printf("SMTP send error: %v", err)
		return fmt.Errorf("failed to send email: %w", err)
	}
	log.Printf("OTP email sent successfully to %s", to)

	return nil
}

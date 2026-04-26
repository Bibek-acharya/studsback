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

func GenerateRandomPassword(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$"
	if length < 8 {
		length = 12
	}
	password := make([]byte, length)
	for i := range password {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		password[i] = charset[n.Int64()]
	}
	return string(password), nil
}

func SendApprovalEmail(to, orgName, password string) error {
	subject := "Your StudSphere Scholarship Provider Account – Approved!"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Inter, Arial, sans-serif; background: #f8fafc; padding: 40px 0;">
  <div style="max-width: 480px; margin: 0 auto; background: #fff; border-radius: 16px; border: 1px solid #e2e8f0; overflow: hidden;">
    <div style="background: #2563eb; padding: 32px; text-align: center;">
      <h1 style="color: #fff; font-size: 24px; font-weight: 900; letter-spacing: -0.5px; margin: 0;">StudSphere</h1>
    </div>
    <div style="padding: 40px 32px;">
      <h2 style="color: #1e293b; font-size: 20px; font-weight: 700; margin: 0 0 8px;">Welcome, %s!</h2>
      <p style="color: #64748b; font-size: 15px; margin: 0 0 24px;">Your scholarship provider account has been approved. You can now log in using the credentials below.</p>
      <div style="background: #f1f5f9; border-radius: 12px; padding: 24px;">
        <p style="margin: 0 0 12px; font-size: 14px; color: #475569;"><strong>Email:</strong> %s</p>
        <p style="margin: 0 0 0; font-size: 14px; color: #475569;"><strong>Password:</strong> <span style="font-family: monospace; font-size: 16px; background: #e2e8f0; padding: 2px 8px; border-radius: 4px;">%s</span></p>
      </div>
      <p style="color: #94a3b8; font-size: 13px; text-align: center; margin: 24px 0 0;">Please change your password after logging in for security purposes.</p>
    </div>
    <div style="background: #f8fafb; padding: 24px 32px; text-align: center; border-top: 1px solid #f3f4f6;">
      <p style="color: #9ca3af; font-size: 12px; margin: 0;">&copy; 2026 Studsphere Inc.</p>
    </div>
  </div>
</body>
</html>`, orgName, to, password)

	return sendGenericHTMLEmail(to, subject, body)
}

func SendRejectionEmail(to, orgName string) error {
	subject := "Your StudSphere Scholarship Provider Registration – Update"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Inter, Arial, sans-serif; background: #f8fafc; padding: 40px 0;">
  <div style="max-width: 480px; margin: 0 auto; background: #fff; border-radius: 16px; border: 1px solid #e2e8f0; overflow: hidden;">
    <div style="background: #2563eb; padding: 32px; text-align: center;">
      <h1 style="color: #fff; font-size: 24px; font-weight: 900; letter-spacing: -0.5px; margin: 0;">StudSphere</h1>
    </div>
    <div style="padding: 40px 32px;">
      <h2 style="color: #1e293b; font-size: 20px; font-weight: 700; margin: 0 0 8px;">Registration Update</h2>
      <p style="color: #64748b; font-size: 15px; margin: 0 0 24px;">Dear %s,</p>
      <p style="color: #64748b; font-size: 15px; margin: 0 0 24px;">After reviewing your application, we regret to inform you that your scholarship provider registration has not been approved at this time.</p>
      <p style="color: #64748b; font-size: 15px; margin: 0 0 0;">If you have any questions, please contact our support team for further assistance.</p>
    </div>
    <div style="background: #f8fafb; padding: 24px 32px; text-align: center; border-top: 1px solid #f3f4f6;">
      <p style="color: #9ca3af; font-size: 12px; margin: 0;">&copy; 2026 Studsphere Inc.</p>
    </div>
  </div>
</body>
</html>`, orgName)

	return sendGenericHTMLEmail(to, subject, body)
}

func sendGenericHTMLEmail(to, subject, htmlBody string) error {
	if emailqueue.Queue != nil {
		log.Printf("Enqueueing email to %s via Asynq", to)
		return emailqueue.EnqueueGenericEmail(to, subject, htmlBody)
	}

	log.Printf("Asynq not available, sending email directly to %s", to)
	return sendEmailDirect(to, subject, htmlBody)
}

func sendEmailDirect(to, subject, htmlBody string) error {
	cfg := config.AppConfig
	smtpHost := cfg.SMTPHost
	smtpPort := cfg.SMTPPort
	smtpUser := cfg.SMTPUser
	smtpPass := cfg.SMTPPass
	fromName := "StudSphere"

	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}
	if smtpPort == "" {
		smtpPort = "587"
	}

	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg := []byte(fmt.Sprintf("From: %s <%s>\nTo: %s\nSubject: %s\n%s%s", fromName, smtpUser, to, subject, mime, htmlBody))

	var auth smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	addr := smtpHost + ":" + smtpPort
	log.Printf("Attempting to send email to %s via SMTP %s", to, addr)
	if err := smtp.SendMail(addr, auth, smtpUser, []string{to}, msg); err != nil {
		log.Printf("SMTP send error: %v", err)
		return fmt.Errorf("failed to send email: %w", err)
	}
	log.Printf("Email sent successfully to %s", to)

	return nil
}



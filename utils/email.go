package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/smtp"
	"os"
)

// GenerateOTP generates a random 6-digit numeric OTP
func GenerateOTP() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// SendOTPEmail sends an OTP to the given email address
func SendOTPEmail(to, otp string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromName := "StudSphere"

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
	if err := smtp.SendMail(addr, auth, smtpUser, []string{to}, msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

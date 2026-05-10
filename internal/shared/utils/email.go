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
	body := renderAuthCodeStyleEmail(
		"Verify your email address",
		"Enter the code below to complete your registration.",
		otp,
		"This code expires in <strong>10 minutes</strong>. Do not share it with anyone.",
	)

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
	body := renderAuthCodeStyleEmail(
		fmt.Sprintf("Welcome, %s!", orgName),
		"Your scholarship provider account has been approved. You can now log in using the credentials below.",
		fmt.Sprintf("<strong>Email:</strong> %s<br><strong>Password:</strong> <span style=\"font-family: monospace; font-size: 16px; background: #e2e8f0; padding: 2px 8px; border-radius: 4px;\">%s</span>", to, password),
		"Please change your password after logging in for security purposes.",
	)

	return sendGenericHTMLEmail(to, subject, body)
}

func SendRejectionEmail(to, orgName string) error {
	subject := "Your StudSphere Scholarship Provider Registration – Update"
	body := renderAuthCodeStyleEmail(
		fmt.Sprintf("Registration Update for %s", orgName),
		"After reviewing your application, we regret to inform you that your scholarship provider registration has not been approved at this time.",
		"If you have any questions, please contact our support team for further assistance.",
		"",
	)

	return sendGenericHTMLEmail(to, subject, body)
}

func renderAuthCodeStyleEmail(heading, message, codeHTML, footerNote string) string {
	footer := ""
	if footerNote != "" {
		footer = fmt.Sprintf(`<p style="color: #94a3b8; font-size: 13px; text-align: left; margin: 16px 0 0;">%s</p>`, footerNote)
	}

	content := fmt.Sprintf(`
<p>%s</p>
<div style="background: #f1f5f9; border-radius: 12px; padding: 24px; text-align: center; color: #1e293b; margin-bottom: 16px;">
    %s
</div>
%s
<div class="signature">
    <p>Best Regards,</p>
    <p>Team Studsphere</p>
</div>`, message, codeHTML, footer)

	return fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Studsphere</title>
    <style>
        body {
            margin: 0;
            padding: 40px 20px;
            background-color: #f9fbfb;
            font-family: 'Plus Jakarta Sans', sans-serif;
            -webkit-font-smoothing: antialiased;
            color: #333333;
        }
        .email-container {
            max-width: 540px;
            margin: 0 auto;
            background-color: #ffffff;
            border-radius: 12px;
            border: 1px solid #eaeaea;
            box-shadow: 0 8px 24px rgba(0,0,0,0.04);
            overflow: hidden;
        }
        .header {
            padding: 40px 24px 20px;
            text-align: center;
        }
        .brand-logo { display: inline-block; margin-bottom: 15px; }
        .brand-logo img { height: 40px; width: auto; }
        .welcome-title {
            color: #111827;
            font-size: 24px;
            font-weight: 500;
            margin: 0;
            font-family: 'Plus Jakarta Sans', sans-serif;
        }
        .content {
            padding: 20px 40px;
            color: #4b5563;
            font-size: 16px;
            line-height: 1.6;
        }
        .content p { margin: 0 0 16px 0; }
        .signature { margin-top: 35px; }
        .signature p { margin: 0 0 4px 0; color: #374151; }
        .footer {
            background-color: #f9fafb;
            padding: 30px 40px;
            text-align: center;
            border-top: 1px solid #f3f4f6;
        }
        .social-links { margin-bottom: 20px; }
        .social-links a {
            display: inline-block;
            margin: 0 8px;
            text-decoration: none;
            color: #6b7280;
        }
        .social-links svg { width: 24px; height: 24px; fill: currentColor; }
        .footer-text { color: #6b7280; font-size: 13px; line-height: 1.6; }
        .footer-text p { margin: 0 0 8px 0; }
        .footer-text a { color: #0000ff; text-decoration: none; font-weight: 500; }
        @media (max-width: 480px) {
            body { padding: 15px 10px; }
            .content { padding: 20px 24px; }
            .footer { padding: 24px; }
        }
    </style>
</head>
<body>
    <div class="email-container">
        <div class="header">
            <div class="brand-logo">
                <img src="https://storage.studsphere.com/uploads/studsphere.png" alt="Studsphere">
            </div>
            <h1 class="welcome-title">%s</h1>
        </div>
        <div class="content">
            %s
        </div>
        <div class="footer">
            <div class="social-links">
                <a href="https://www.facebook.com/share/1CEcyRH9ZZ/" aria-label="Facebook">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M22 12c0-5.52-4.48-10-10-10S2 6.48 2 12c0 4.84 3.44 8.87 8 9.8V15H8v-3h2V9.5C10 7.57 11.57 6 13.5 6H16v3h-2c-.55 0-1 .45-1 1v2h3v3h-3v6.95c5.05-.5 9-4.76 9-9.95z"/></svg>
                </a>
                <a href="https://www.instagram.com/stud.sphere?igsh=NDM5Z29nc2ZqMmc=" aria-label="Instagram">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M12 2.16c3.2 0 3.58.01 4.85.07 3.25.15 4.77 1.69 4.92 4.92.06 1.27.07 1.65.07 4.85s-.01 3.58-.07 4.85c-.15 3.23-1.66 4.77-4.92 4.92-1.27.06-1.64.07-4.85.07s-3.58-.01-4.85-.07c-3.26-.15-4.77-1.7-4.92-4.92-.06-1.27-.07-1.64-.07-4.85s.01-3.58.07-4.85C2.38 3.85 3.9 2.31 7.15 2.23c1.27-.06 1.64-.07 4.85-.07m0-2.16C8.74 0 8.33.01 7.05.07c-4.26.19-6.78 2.71-6.98 6.98C0 8.33 0 8.74 0 12s.01 3.67.07 4.95c.2 4.27 2.72 6.79 6.98 6.98 1.28.06 1.69.07 4.95.07s3.67-.01 4.95-.07c4.27-.2 6.79-2.72 6.98-6.98.06-1.28.07-1.69.07-4.95s-.01-3.67-.07-4.95c-.2-4.27-2.72-6.79-6.98-6.98C15.67.01 15.26 0 12 0zm0 5.84A6.16 6.16 0 1 0 18.16 12 6.16 6.16 0 0 0 12 5.84zm0 10.16A4 4 0 1 1 16 12a4 4 0 0 1-4 4zm5.4-9.56a1.44 1.44 0 1 1-2.88 0 1.44 1.44 0 0 1 2.88 0z"/></svg>
                </a>
                <a href="https://www.tiktok.com/@stud.sphere?_r=1&_t=ZS-95OYyC0vodM" aria-label="TikTok">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M19.32 5.33a8.67 8.67 0 0 1-4.27-1.29v8.22a2.67 2.67 0 1 1-2.67-2.67c.14 0 .28 0 .41.05v-2.52a5.2 5.2 0 0 0-.41-.05 5.2 5.2 0 1 0 5.2 5.2V7.68a10.95 10.95 0 0 0 4.27.74V5.33z"/></svg>
                </a>
                <a href="https://wa.me/9779800000000" aria-label="WhatsApp">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 0 1-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 0 1-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 0 1 2.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0 0 12.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 0 0 5.683 1.448h.005c6.554 0 11.890-5.335 11.893-11.893A11.821 11.821 0 0 0 20.885 3.488"/></svg>
                </a>
            </div>
            <div class="footer-text">
                <p>This email can't receive replies. For more information, visit the <a href="https://studsphere.com/help">Studsphere Help Center</a>.</p>
                <p>&copy; 2026 Studsphere Inc., Sallyan House, Baghbajar Kathmandu, Nepal</p>
            </div>
        </div>
    </div>
</body>
</html>`, heading, content)
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

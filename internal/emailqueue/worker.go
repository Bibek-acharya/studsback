package emailqueue

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/hibiken/asynq"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/logger"
)

var (
	Mux              *asynq.Server
	extraHandlers    map[string]func(context.Context, *asynq.Task) error
)

const concurrency = 10

func RegisterHandler(pattern string, handler func(context.Context, *asynq.Task) error) {
	if extraHandlers == nil {
		extraHandlers = make(map[string]func(context.Context, *asynq.Task) error)
	}
	extraHandlers[pattern] = handler
}

func StartWorker() error {
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisDB := 6

	srv := asynq.NewServer(
		&asynq.RedisClientOpt{
			Addr:     redisAddr,
			Username: "default",
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		asynq.Config{
			Concurrency: concurrency,
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeSendOTPEmail, handleOTPTask)
	mux.HandleFunc(TypeSendWelcomeEmail, handleWelcomeTask)
	mux.HandleFunc(TypeSendGenericHTML, handleGenericTask)
	mux.HandleFunc(TypeSendReviewEmail, handleReviewTask)

	for pattern, handler := range extraHandlers {
		mux.HandleFunc(pattern, handler)
	}

	if err := srv.Start(mux); err != nil {
		return fmt.Errorf("failed to start asynq worker: %w", err)
	}

	Mux = srv
	logger.Info("Asynq worker started", "concurrency", concurrency)
	return nil
}

func StopWorker() {
	if Mux != nil {
		Mux.Shutdown()
	}
}

func handleOTPTask(ctx context.Context, task *asynq.Task) error {
	var payload OTPEmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	html := renderOTPTemplate(payload.OTP, payload.ExpiresIn)
	return sendEmail(payload.To, "Your StudSphere Verification Code", html)
}

func handleWelcomeTask(ctx context.Context, task *asynq.Task) error {
	var payload WelcomeEmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	html := renderWelcomeTemplate(payload.FirstName, payload.VerifyToken)
	return sendEmail(payload.To, "Welcome to StudSphere!", html)
}

func handleGenericTask(ctx context.Context, task *asynq.Task) error {
	var payload Payload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	return sendEmail(payload.To, payload.Subject, payload.HTML)
}

func handleReviewTask(ctx context.Context, task *asynq.Task) error {
	var payload ReviewEmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	html := renderReviewTemplate(payload.CollegeName, payload.ReviewLink)
	return sendEmail(payload.To, "New Review for "+payload.CollegeName, html)
}

func sendEmail(to, subject, htmlBody string) error {
	cfg := config.AppConfig
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	smtpHost := cfg.SMTPHost
	smtpPort := cfg.SMTPPort
	smtpUser := cfg.SMTPUser
	smtpPass := cfg.SMTPPass

	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}
	if smtpPort == "" {
		smtpPort = "587"
	}

	fromName := "StudSphere"

	var fullBody string
	if strings.Contains(strings.ToLower(htmlBody), "<!doctype html>") || strings.Contains(strings.ToLower(htmlBody), "<html") {
		fullBody = htmlBody
	} else {
		fullBody = renderEmailWrapper("Studsphere", plainTextToHTML(htmlBody))
	}

	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg := fmt.Appendf(nil, "From: %s <%s>\nTo: %s\nSubject: %s\n%s%s",
		fromName, smtpUser, to, subject, mime, fullBody)

	var auth smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	addr := smtpHost + ":" + smtpPort
	log.Printf("Sending email to %s via %s", to, addr)

	if err := smtp.SendMail(addr, auth, smtpUser, []string{to}, msg); err != nil {
		logger.Error("Failed to send email", "to", to, "error", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	logger.Info("Email sent successfully", "to", to, "subject", subject)
	return nil
}

// SendAdmitCardEmail sends an email with a PDF admit card attached.
// This bypasses the async queue since PDF bytes cannot be easily serialised.
func SendAdmitCardEmail(to, candidateName, scholarshipTitle string, pdfBytes []byte) error {
	cfg := config.AppConfig
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	smtpHost := cfg.SMTPHost
	smtpPort := cfg.SMTPPort
	smtpUser := cfg.SMTPUser
	smtpPass := cfg.SMTPPass

	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}
	if smtpPort == "" {
		smtpPort = "587"
	}

	fromName := "Studsphere"
	subject := fmt.Sprintf("Admit Card - %s", scholarshipTitle)

	boundary := "==MimeBoundary=="

	content := fmt.Sprintf(`
<p>Dear %s,</p>
<p>Congratulations! Your application for <strong>%s</strong> has been confirmed and your payment has been verified.</p>
<p>Please find your <strong>Admit Card</strong> attached to this email as a PDF. Print and bring it to the examination centre.</p>
<div class="signature">
    <p>Best Regards,</p>
    <p>Team Studsphere</p>
</div>`, candidateName, scholarshipTitle)

	htmlBody := renderEmailWrapper("Admit Card", content)

	// Build multipart MIME message
	var msgBuf bytes.Buffer
	msgBuf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, smtpUser))
	msgBuf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msgBuf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msgBuf.WriteString("MIME-Version: 1.0\r\n")
	msgBuf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	msgBuf.WriteString("\r\n")

	// HTML part
	msgBuf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msgBuf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msgBuf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	msgBuf.WriteString("\r\n")
	msgBuf.WriteString(htmlBody)
	msgBuf.WriteString("\r\n")

	// PDF attachment
	msgBuf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msgBuf.WriteString("Content-Type: application/pdf\r\n")
	msgBuf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"Admit_Card_%s.pdf\"\r\n", strings.ReplaceAll(scholarshipTitle, " ", "_")))
	msgBuf.WriteString("Content-Transfer-Encoding: base64\r\n")
	msgBuf.WriteString("\r\n")

	// Base64-encode the PDF in 76-char lines
	encoded := base64.StdEncoding.EncodeToString(pdfBytes)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		msgBuf.WriteString(encoded[i:end])
		msgBuf.WriteString("\r\n")
	}

	msgBuf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	var auth smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	addr := smtpHost + ":" + smtpPort
	log.Printf("Sending admit card email to %s via %s", to, addr)

	if err := smtp.SendMail(addr, auth, smtpUser, []string{to}, msgBuf.Bytes()); err != nil {
		logger.Error("Failed to send admit card email", "to", to, "error", err)
		return fmt.Errorf("failed to send admit card email: %w", err)
	}

	logger.Info("Admit card email sent", "to", to)
	return nil
}

// SendAdmitCardEmailHTML sends an email with the admit card rendered as HTML in the body.
// Used as fallback when PDF generation fails.
func SendAdmitCardEmailHTML(to, candidateName, scholarshipTitle, provider, rollNumber, examCentre, stream, examDate, examTime, gender, dob string) error {
	cfg := config.AppConfig
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	smtpHost := cfg.SMTPHost
	smtpPort := cfg.SMTPPort
	smtpUser := cfg.SMTPUser
	smtpPass := cfg.SMTPPass

	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}
	if smtpPort == "" {
		smtpPort = "587"
	}

	fromName := "Studsphere"
	subject := fmt.Sprintf("Admit Card - %s", scholarshipTitle)

	content := fmt.Sprintf(`
<p>Dear %s,</p>
<p>Congratulations! Your application for <strong>%s</strong> has been confirmed and your payment has been verified.</p>
<p>Please find your admit card details below:</p>

<table style="width:100%%;border-collapse:collapse;margin:20px 0;font-size:14px;">
	<tr>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;background:#f9fafb;font-weight:600;width:140px;">Candidate Name</td>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;">%s</td>
	</tr>
	<tr>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;background:#f9fafb;font-weight:600;">Date of Birth</td>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;">%s</td>
	</tr>
	<tr>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;background:#f9fafb;font-weight:600;">Gender</td>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;">%s</td>
	</tr>
	<tr>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;background:#f9fafb;font-weight:600;">Roll Number</td>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;">%s</td>
	</tr>
	<tr>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;background:#f9fafb;font-weight:600;">Exam Centre</td>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;">%s</td>
	</tr>
	<tr>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;background:#f9fafb;font-weight:600;">Stream</td>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;">%s</td>
	</tr>
	<tr>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;background:#f9fafb;font-weight:600;">Exam Date</td>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;">%s</td>
	</tr>
	<tr>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;background:#f9fafb;font-weight:600;">Exam Time</td>
		<td style="padding:10px 12px;border:1px solid #e5e7eb;">%s</td>
	</tr>
</table>

<p style="color:#dc2626;font-weight:600;">Note: Please print this email and bring it to the examination centre along with a valid photo ID. We are unable to generate the PDF admit card at this time.</p>

<div class="signature">
	<p>Best Regards,</p>
	<p>Team Studsphere</p>
</div>`, candidateName, scholarshipTitle, candidateName, dob, gender, rollNumber, examCentre, stream, examDate, examTime)

	htmlBody := renderEmailWrapper("Admit Card", content)

	var msgBuf bytes.Buffer
	msgBuf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, smtpUser))
	msgBuf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msgBuf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msgBuf.WriteString("MIME-Version: 1.0\r\n")
	msgBuf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msgBuf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	msgBuf.WriteString("\r\n")
	msgBuf.WriteString(htmlBody)
	msgBuf.WriteString("\r\n")

	var auth smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	addr := smtpHost + ":" + smtpPort
	log.Printf("Sending admit card HTML email to %s via %s", to, addr)

	if err := smtp.SendMail(addr, auth, smtpUser, []string{to}, msgBuf.Bytes()); err != nil {
		logger.Error("Failed to send admit card HTML email", "to", to, "error", err)
		return fmt.Errorf("failed to send admit card HTML email: %w", err)
	}

	logger.Info("Admit card HTML email sent", "to", to)
	return nil
}

func renderOTPTemplate(otp string, expiresIn int) string {
	if expiresIn == 0 {
		expiresIn = 10
	}

	content := fmt.Sprintf(`
<p>Hi,</p>
<p>Greetings!</p>
<p>You are just a step away from accessing your Studsphere account.</p>
<p>The code is valid for only 10 minutes and can be used only once.</p>
<div style="margin-top: 20px; margin-bottom: 30px;">
    <p style="margin: 0 0 6px 0; color: #111827;">Your OTP Code</p>
    <p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; font-family: 'Plus Jakarta Sans', sans-serif; color: #111827; text-align: left; margin: 20px 0;">%s</p>
    <p>Expires in: %d minutes</p>
</div>
<div class="signature">
    <p>Best Regards,</p>
    <p>Team Studsphere</p>
</div>`, otp, expiresIn)

	return renderEmailWrapper("Welcome to Studsphere!", content)
}

func renderWelcomeTemplate(firstName, verifyToken string) string {
	var verifyLink string
	if verifyToken != "" {
		verifyLink = fmt.Sprintf(`
<div style="text-align: center; margin: 24px 0;">
  <a href="%s" style="display: inline-block; background: #2563eb; color: #fff; font-weight: 600; padding: 14px 28px; border-radius: 10px; text-decoration: none;">Verify Email</a>
</div>`, verifyToken)
	}

	content := fmt.Sprintf(`
<p>Hi %s,</p>
<p>Thank you for joining StudSphere! We're excited to help you find your ideal college in Nepal.</p>
%s
<p style="color: #94a3b8; font-size: 13px; margin: 16px 0 0;">If you didn't create this account, please ignore this email.</p>
<div class="signature">
    <p>Best Regards,</p>
    <p>Team Studsphere</p>
</div>`, firstName, verifyLink)

	return renderEmailWrapper("Welcome to Studsphere!", content)
}

func renderReviewTemplate(collegeName, reviewLink string) string {
	linkHTML := ""
	if reviewLink != "" {
		linkHTML = fmt.Sprintf(`
<div style="text-align: center; margin: 24px 0;">
  <a href="%s" style="display: inline-block; background: #2563eb; color: #fff; font-weight: 600; padding: 14px 28px; border-radius: 10px; text-decoration: none;">Read Reviews</a>
</div>`, reviewLink)
	}

	content := fmt.Sprintf(`
<p>Hi,</p>
<p>New student reviews have been submitted for <strong>%s</strong>. Help other students make informed decisions.</p>
%s
<p style="color: #94a3b8; font-size: 13px; margin: 16px 0 0;">Thank you for being part of StudSphere!</p>
<div class="signature">
    <p>Best Regards,</p>
    <p>Team Studsphere</p>
</div>`, collegeName, linkHTML)

	return renderEmailWrapper("New Reviews for "+collegeName, content)
}

// Utility functions for queue management (simplified for this version)
func CleanOldJobs() error {
	logger.Info("Clean old jobs not implemented in this version")
	return nil
}

func RetryFailedJob(queue, taskID string) error {
	logger.Info("Retry job not implemented in this version", "queue", queue, "task_id", taskID)
	return nil
}

func GetFailedJobs() ([]*asynq.TaskInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func GetPendingJobs() ([]*asynq.TaskInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetStudSphereLogoURL returns the URL for the StudSphere logo
func GetStudSphereLogoURL() string {
	return "https://storage.studsphere.com/uploads/studsphere.png"
}

func plainTextToHTML(text string) string {
	paragraphs := strings.Split(text, "\n\n")
	var htmlParagraphs []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		escaped := strings.ReplaceAll(p, "&", "&amp;")
		escaped = strings.ReplaceAll(escaped, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		escaped = strings.ReplaceAll(escaped, "\n", "<br>")
		htmlParagraphs = append(htmlParagraphs, "<p>"+escaped+"</p>")
	}
	return strings.Join(htmlParagraphs, "")
}

func renderEmailWrapper(title, content string) string {
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
        .brand-logo {
            display: inline-block;
            margin-bottom: 15px;
        }
        .brand-logo img {
            height: 40px;
            width: auto;
        }
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
        .content p {
            margin: 0 0 16px 0;
        }
        .signature {
            margin-top: 35px;
        }
        .signature p {
            margin: 0 0 4px 0;
            color: #374151;
        }
        .footer {
            background-color: #f9fafb;
            padding: 30px 40px;
            text-align: center;
            border-top: 1px solid #f3f4f6;
        }
        .social-links {
            margin-bottom: 20px;
        }
        .social-links a {
            display: inline-block;
            margin: 0 8px;
            text-decoration: none;
            color: #6b7280;
        }
        .social-links img {
            width: 24px;
            height: 24px;
        }
        .footer-text {
            color: #6b7280;
            font-size: 13px;
            line-height: 1.6;
        }
        .footer-text p {
            margin: 0 0 8px 0;
        }
        .footer-text a {
            color: #0000ff;
            text-decoration: none;
            font-weight: 500;
        }
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
                    <img src="https://cdn-icons-png.flaticon.com/512/5968/5968764.png" alt="Facebook" width="24" height="24">
                </a>
                <a href="https://www.instagram.com/stud.sphere?igsh=NDM5Z29nc2ZqMmc=" aria-label="Instagram">
                    <img src="https://cdn-icons-png.flaticon.com/512/2111/2111463.png" alt="Instagram" width="24" height="24">
                </a>
                <a href="https://www.tiktok.com/@stud.sphere?_r=1&_t=ZS-95OYyC0vodM" aria-label="TikTok">
                    <img src="https://cdn-icons-png.flaticon.com/512/3046/3046121.png" alt="TikTok" width="24" height="24">
                </a>
                <a href="https://wa.me/9779800000000" aria-label="WhatsApp">
                    <img src="https://cdn-icons-png.flaticon.com/512/4462/4462466.png" alt="WhatsApp" width="24" height="24">
                </a>
            </div>
            <div class="footer-text">
                <p>This email can't receive replies. For more information, visit the <a href="https://studsphere.com/help">Studsphere Help Center</a>.</p>
                <p>&copy; 2026 Stud Sphere Pvt. Ltd., Sallyan House, Bagbazar Kathmandu, Nepal</p>
            </div>
        </div>
    </div>
</body>
</html>`, title, content)
}

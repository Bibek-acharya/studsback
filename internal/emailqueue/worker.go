package emailqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/hibiken/asynq"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/logger"
)

var Mux *asynq.Server

const concurrency = 10

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
		// Full HTML document provided, use as-is
		fullBody = htmlBody
	} else {
		// Partial HTML content, wrap in template
		fullBody = fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Inter, Arial, sans-serif; background: #f8fafc; padding: 40px 0;">
  <div style="max-width: 480px; margin: 0 auto; background: #fff; border-radius: 16px; border: 1px solid #e2e8f0; overflow: hidden;">
    <div style="background: linear-gradient(135deg, #2563eb 0%, #7c3aed 100%); padding: 32px; text-align: center;">
      <h1 style="color: #fff; font-size: 24px; font-weight: 900; letter-spacing: -0.5px; margin: 0;">StudSphere</h1>
    </div>
    <div style="padding: 40px 32px;">
      %s
    </div>
    <div style="background: #f8fafc; padding: 20px 32px; text-align: center; border-top: 1px solid #e2e8f0;">
      <p style="color: #94a3b8; font-size: 12px; margin: 0;">&copy; 2026 StudSphere. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`, htmlBody)
	}

	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg := []byte(fmt.Sprintf("From: %s <%s>\nTo: %s\nSubject: %s\n%s%s",
		fromName, smtpUser, to, subject, mime, fullBody))

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

func renderOTPTemplate(otp string, expiresIn int) string {
	if expiresIn == 0 {
		expiresIn = 10
	}

	logoURL := GetStudSphereLogoURL()

	return fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Studsphere OTP Email</title>
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

        .otp-section {
            margin-top: 20px;
            margin-bottom: 30px;
        }

        .otp-section p {
            margin: 0 0 6px 0;
            color: #111827;
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
            transition: color 0.2s ease;
        }

        .social-links a:hover {
            color: #2563eb;
        }

        .social-links svg {
            width: 24px;
            height: 24px;
            fill: currentColor;
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

        .footer-text a:hover {
            text-decoration: underline;
        }

        @media (max-width: 480px) {
            body {
                padding: 15px 10px;
            }
            .content {
                padding: 20px 24px;
            }
            .footer {
                padding: 24px;
            }
        }
    </style>
</head>
<body>
    <div class="email-container">
        <div class="header">
            <div class="brand-logo">
                <img src="%s" alt="Studsphere">
            </div>
            <h1 class="welcome-title">Welcome to Studsphere!</h1>
        </div>

        <div class="content">
            <p>Hi,</p>
            <p>Greetings!</p>
            <p>You are just a step away from accessing your Studsphere account.</p>
            <p>The code is valid for only 10 minutes and can be used only once.</p>

            <div class="otp-section">
                <p>Your OTP Code</p>
                <p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; font-family: 'Plus Jakarta Sans', sans-serif; color: #111827; text-align: left; margin: 20px 0;">%s</p>
                <p>Expires in: %d minutes</p>
            </div>

            <div class="signature">
                <p>Best Regards,</p>
                <p>Team Studsphere</p>
            </div>
        </div>

        <div class="footer">
            <div class="social-links">
                <a href="https://www.facebook.com/share/1CEcyRH9ZZ/" aria-label="Facebook">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">
                        <path d="M22 12c0-5.52-4.48-10-10-10S2 6.48 2 12c0 4.84 3.44 8.87 8 9.8V15H8v-3h2V9.5C10 7.57 11.57 6 13.5 6H16v3h-2c-.55 0-1 .45-1 1v2h3v3h-3v6.95c5.05-.5 9-4.76 9-9.95z"/>
                    </svg>
                </a>
                <a href="https://www.instagram.com/stud.sphere?igsh=NDM5Z29nc2ZqMmc=" aria-label="Instagram">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">
                        <path d="M12 2.16c3.2 0 3.58.01 4.85.07 3.25.15 4.77 1.69 4.92 4.92.06 1.27.07 1.65.07 4.85s-.01 3.58-.07 4.85c-.15 3.23-1.66 4.77-4.92 4.92-1.27.06-1.64.07-4.85.07s-3.58-.01-4.85-.07c-3.26-.15-4.77-1.7-4.92-4.92-.06-1.27-.07-1.64-.07-4.85s.01-3.58.07-4.85C2.38 3.85 3.9 2.31 7.15 2.23c1.27-.06 1.64-.07 4.85-.07m0-2.16C8.74 0 8.33.01 7.05.07c-4.26.19-6.78 2.71-6.98 6.98C0 8.33 0 8.74 0 12s.01 3.67.07 4.95c.2 4.27 2.72 6.79 6.98 6.98 1.28.06 1.69.07 4.95.07s3.67-.01 4.95-.07c4.27-.2 6.79-2.72 6.98-6.98.06-1.28.07-1.69.07-4.95s-.01-3.67-.07-4.95c-.2-4.27-2.72-6.79-6.98-6.98C15.67.01 15.26 0 12 0zm0 5.84A6.16 6.16 0 1 0 18.16 12 6.16 6.16 0 0 0 12 5.84zm0 10.16A4 4 0 1 1 16 12a4 4 0 0 1-4 4zm5.4-9.56a1.44 1.44 0 1 1-2.88 0 1.44 1.44 0 0 1 2.88 0z"/>
                    </svg>
                </a>
                <a href="https://www.tiktok.com/@stud.sphere?_r=1&_t=ZS-95OYyC0vodM" aria-label="TikTok">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">
                        <path d="M19.32 5.33a8.67 8.67 0 0 1-4.27-1.29v8.22a2.67 2.67 0 1 1-2.67-2.67c.14 0 .28 0 .41.05v-2.52a5.2 5.2 0 0 0-.41-.05 5.2 5.2 0 1 0 5.2 5.2V7.68a10.95 10.95 0 0 0 4.27.74V5.33z"/>
                    </svg>
                </a>
                <a href="https://wa.me/9779800000000" aria-label="WhatsApp">
                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">
                        <path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 0 1-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 0 1-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 0 1 2.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0 0 12.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 0 0 5.683 1.448h.005c6.554 0 11.890-5.335 11.893-11.893A11.821 11.821 0 0 0 20.885 3.488"/>
                    </svg>
                </a>
            </div>

            <div class="footer-text">
                <p>This email can't receive replies. For more information, visit the <a href="https://studsphere.com/help">Studsphere Help Center</a>.</p>
                <p>&copy; 2026 Studsphere Inc., Sallyan House, Baghbajar Kathmandu, Nepal</p>
            </div>
        </div>
        </div>
    </div>
</body>
</html>`, logoURL, otp, expiresIn)
}

func renderWelcomeTemplate(firstName, verifyToken string) string {
	var verifyLink string
	if verifyToken != "" {
		verifyLink = fmt.Sprintf(`
<div style="text-align: center; margin: 24px 0;">
  <a href="%s" style="display: inline-block; background: #2563eb; color: #fff; font-weight: 600; padding: 14px 28px; border-radius: 10px; text-decoration: none;">Verify Email</a>
</div>`, verifyToken)
	}

	return fmt.Sprintf(`
<h2 style="color: #1e293b; font-size: 20px; font-weight: 700; margin: 0 0 8px;">Welcome to StudSphere, %s!</h2>
<p style="color: #64748b; font-size: 15px; margin: 0 0 16px;">Thank you for joining. We're excited to help you find your ideal college in Nepal.</p>
%s
<p style="color: #94a3b8; font-size: 13px; margin: 0;">If you didn't create this account, please ignore this email.</p>`, firstName, verifyLink)
}

func renderReviewTemplate(collegeName, reviewLink string) string {
	linkHTML := ""
	if reviewLink != "" {
		linkHTML = fmt.Sprintf(`
<div style="text-align: center; margin: 24px 0;">
  <a href="%s" style="display: inline-block; background: #2563eb; color: #fff; font-weight: 600; padding: 14px 28px; border-radius: 10px; text-decoration: none;">Read Reviews</a>
</div>`, reviewLink)
	}

	return fmt.Sprintf(`
<h2 style="color: #1e293b; font-size: 20px; font-weight: 700; margin: 0 0 8px;">New Reviews for %s</h2>
<p style="color: #64748b; font-size: 15px; margin: 0 0 16px;">New student reviews have been submitted for %s. Help other students make informed decisions.</p>
%s
<p style="color: #94a3b8; font-size: 13px; margin: 0;">Thank you for being part of StudSphere!</p>`, collegeName, collegeName, linkHTML)
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
	cfg := config.AppConfig
	if cfg == nil {
		return "https://studsphere.com/assets/studsphere-logo.png" // fallback
	}

	// Return the internal API endpoint that serves the logo
	// This hides the MinIO URL and provides better control
	// For production, use the backend URL since emails need to access the API
	backendURL := "https://api.studsphere.com" // TODO: Make this configurable from env

	return backendURL + "/api/v1/tools/logo/serve"
}

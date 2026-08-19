package scholarship

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/shared/logger"

	"github.com/hibiken/asynq"
)

// HandleAdmitCardTask processes an admit card generation request from the async queue.
// It generates the PDF (with retries), then sends the email with the PDF attached.
// On final failure, it sends an HTML fallback email instead.
func HandleAdmitCardTask(ctx context.Context, task *asynq.Task) error {
	var payload emailqueue.AdmitCardPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal admit card payload: %w", err)
	}

	cardData := AdmitCardData{
		CandidateName:    payload.CandidateName,
		DateOfBirth:      payload.DateOfBirth,
		Gender:           payload.Gender,
		RollNumber:       payload.RollNumber,
		ExamCentre:       payload.ExamCentre,
		Stream:           payload.Stream,
		PhotoURL:         PhotoToBase64(payload.PhotoURL),
		ScholarshipTitle: payload.ScholarshipTitle,
		Provider:         payload.Provider,
		ExamDate:         payload.ExamDate,
		ExamTime:         payload.ExamTime,
		Shift:            payload.Shift,
		SubjectName:      payload.SubjectName,
	}

	var pdfBytes []byte
	var lastErr error
	for i := 0; i < 3; i++ {
		pdfBytes, lastErr = GenerateAdmitCardPDF(cardData, nil)
		if lastErr == nil {
			break
		}
		logger.Warn("HandleAdmitCardTask: PDF generation attempt failed",
			"attempt", i+1, "to", payload.Email, "roll", payload.RollNumber,
			"error", lastErr)
		time.Sleep(1 * time.Second)
	}

	if lastErr != nil {
		logger.Error("HandleAdmitCardTask: PDF generation failed after retries, sending HTML fallback",
			"to", payload.Email, "roll", payload.RollNumber, "error", lastErr)
		return emailqueue.SendAdmitCardEmailHTML(
			payload.Email, payload.CandidateName, payload.ScholarshipTitle,
			payload.Provider, payload.RollNumber, payload.ExamCentre,
			payload.Stream, payload.ExamDate, payload.ExamTime,
			payload.Gender, payload.DateOfBirth,
		)
	}

	logger.Info("HandleAdmitCardTask: PDF generated, sending email",
		"to", payload.Email, "roll", payload.RollNumber, "size", len(pdfBytes))
	return emailqueue.SendAdmitCardEmail(payload.Email, payload.CandidateName, payload.ScholarshipTitle, pdfBytes)
}

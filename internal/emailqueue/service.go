package emailqueue

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/logger"

	"github.com/hibiken/asynq"
)

var (
	RedisAddr string
	RedisPass string
	RedisDB   int
	Queue     *asynq.Client
	Inspector *asynq.Inspector
)

func InitAsynq() error {
	RedisDB = 6
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPass := getEnv("REDIS_PASSWORD", "")
	RedisAddr = redisAddr
	RedisPass = redisPass

	r := &asynq.RedisClientOpt{
		Addr:     redisAddr,
		Username: "default",
		Password: redisPass,
		DB:       RedisDB,
	}

	Queue = asynq.NewClient(r)
	Inspector = asynq.NewInspector(r)

	logger.Info("Asynq initialized", "redis", redisAddr, "db", RedisDB)
	return nil
}

func CloseAsynq() {
	if Queue != nil {
		Queue.Close()
	}
	if Inspector != nil {
		Inspector.Close()
	}
}

func EnqueueSendOTPEmail(to, otp string, expiresIn int) error {
	if Queue == nil {
		return fmt.Errorf("asynq not initialized")
	}

	payload, err := json.Marshal(OTPEmailPayload{
		To:        to,
		OTP:       otp,
		ExpiresIn: expiresIn,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(TypeSendOTPEmail, payload)
	_, err = Queue.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(5*time.Minute))
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	logger.Info("OTP email enqueued", "to", to)
	return nil
}

func EnqueueWelcomeEmail(to, firstName string, verifyToken string) error {
	if Queue == nil {
		return fmt.Errorf("asynq not initialized")
	}

	payload, err := json.Marshal(WelcomeEmailPayload{
		To:          to,
		FirstName:   firstName,
		VerifyToken: verifyToken,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(TypeSendWelcomeEmail, payload)
	_, err = Queue.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(5*time.Minute))
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	logger.Info("Welcome email enqueued", "to", to)
	return nil
}

func EnqueueGenericEmail(to, subject, html string) error {
	if Queue == nil {
		return fmt.Errorf("asynq not initialized")
	}

	var from string
	if config.AppConfig != nil {
		from = config.AppConfig.SMTPUser
	}

	payload, err := json.Marshal(Payload{
		To:      to,
		Subject: subject,
		HTML:    html,
		From:    from,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(TypeSendGenericHTML, payload)
	_, err = Queue.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(5*time.Minute))
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	logger.Info("Generic email enqueued", "to", to, "subject", subject)
	return nil
}

func EnqueueReviewEmail(to, collegeName, reviewLink string) error {
	if Queue == nil {
		return fmt.Errorf("asynq not initialized")
	}

	payload, err := json.Marshal(ReviewEmailPayload{
		To:          to,
		CollegeName: collegeName,
		ReviewLink:  reviewLink,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(TypeSendReviewEmail, payload)
	_, err = Queue.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(5*time.Minute))
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	logger.Info("Review email enqueued", "to", to)
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

type QueueStats struct {
	Pending  int `json:"pending"`
	Progress int `json:"progress"`
	Failed   int `json:"failed"`
}

func GetQueueStats() (*QueueStats, error) {
	if Inspector == nil {
		return nil, fmt.Errorf("asynq not initialized")
	}

	stats := &QueueStats{}

	queues, err := Inspector.Queues()
	if err == nil {
		for _, qname := range queues {
			info, err := Inspector.GetQueueInfo(qname)
			if err == nil {
				stats.Pending = info.Pending
				stats.Progress = info.Active
				stats.Failed = info.Failed
				break // Just get stats from first queue
			}
		}
	}

	return stats, nil
}

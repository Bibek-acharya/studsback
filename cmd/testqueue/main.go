package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"studsphere/backend/internal/emailqueue"
	"studsphere/backend/internal/shared/config"
)

func main() {
	// Load config
	config.Load()

	fmt.Println("Testing Asynq email queue...")

	// Initialize Asynq
	if err := emailqueue.InitAsynq(); err != nil {
		log.Printf("Failed to initialize Asynq: %v", err)
		log.Println("Make sure Redis is running on localhost:6379")
		os.Exit(1)
	}

	// Start worker in background
	go func() {
		if err := emailqueue.StartWorker(); err != nil {
			log.Printf("Failed to start worker: %v", err)
		}
	}()

	// Give worker time to start
	time.Sleep(2 * time.Second)

	// Test enqueue OTP email
	fmt.Println("Enqueueing test OTP email...")
	err := emailqueue.EnqueueSendOTPEmail("test@example.com", "123456", 10)
	if err != nil {
		log.Printf("Failed to enqueue email: %v", err)
		os.Exit(1)
	}

	fmt.Println("Email enqueued successfully!")
	fmt.Println("Check your email and logs for delivery confirmation.")
	fmt.Println("Press Ctrl+C to exit...")

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down...")
	emailqueue.StopWorker()
	emailqueue.CloseAsynq()
}

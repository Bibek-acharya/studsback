package utils

import (
	"sync"
	"time"
)

type otpEntry struct {
	OTP       string
	ExpiresAt time.Time
	Data      interface{}
}

var (
	otpStore   = make(map[string]otpEntry) // key: email
	otpStoreMu sync.Mutex
)

// StoreOTP stores an OTP and associated data (e.g. registration details) for a given email
func StoreOTP(email, otp string, data interface{}) {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	otpStore[email] = otpEntry{
		OTP:       otp,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Data:      data,
	}
}

// VerifyOTP checks if the OTP for a given email is valid and not expired
func VerifyOTP(email, otp string) (bool, interface{}) {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	entry, exists := otpStore[email]
	if !exists {
		return false, nil
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(otpStore, email)
		return false, nil
	}
	if entry.OTP != otp {
		return false, nil
	}
	delete(otpStore, email) // single use
	return true, entry.Data
}

// GetOTPData returns the data associated with an email's OTP without deleting it
func GetOTPData(email string) interface{} {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	entry, exists := otpStore[email]
	if !exists {
		return nil
	}
	return entry.Data
}

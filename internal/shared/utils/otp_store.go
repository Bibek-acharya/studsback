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
	otpStore   = make(map[string]otpEntry)
	otpStoreMu sync.Mutex
)

func StoreOTP(email, otp string, data interface{}) {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	otpStore[email] = otpEntry{
		OTP:       otp,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Data:      data,
	}
}

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
	delete(otpStore, email)
	return true, entry.Data
}

func GetOTPData(email string) interface{} {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	entry, exists := otpStore[email]
	if !exists {
		return nil
	}
	return entry.Data
}

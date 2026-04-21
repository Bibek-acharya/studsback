package utils

import (
	"sync"
	"time"
)

type otpEntry struct {
	OTP       string
	ExpiresAt time.Time
	Type      string
	Data      interface{}
}

var (
	otpStore   = make(map[string]otpEntry)
	otpStoreMu sync.Mutex
)

func StoreOTP(email, otp string, data interface{}) {
	StoreOTPWithType(email, otp, "", data)
}

func StoreOTPWithType(email, otp, otpType string, data interface{}) {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	otpStore[email] = otpEntry{
		OTP:       otp,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Type:      otpType,
		Data:      data,
	}
}

func VerifyOTP(email, otp string) (bool, string, interface{}) {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	entry, exists := otpStore[email]
	if !exists {
		return false, "", nil
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(otpStore, email)
		return false, "", nil
	}
	if entry.OTP != otp {
		return false, "", nil
	}
	otpType := entry.Type
	delete(otpStore, email)
	return true, otpType, entry.Data
}

func GetOTPData(email string) (string, interface{}) {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	entry, exists := otpStore[email]
	if !exists {
		return "", nil
	}
	return entry.Type, entry.Data
}

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
	// Referral is the code the person arrived with, captured on the invite link
	// and sent on RegisterRequest. It is a FIELD OF THE ENTRY rather than part of
	// Data because Data is one of three unrelated structs (auth.User,
	// auth.InstitutionUser, auth.ScholarshipProviderUser) and none of them should
	// grow a column for something that is not theirs — Data is what gets written
	// to a table on VerifyOTP, and this must not.
	//
	// It has to live in the store at all, because the OTP is the gap between
	// "Register received the code" and "the account exists". VerifyOTP is where
	// the users row is created and therefore where attribution has to happen, and
	// by then the request that carried the code is long gone. The store is the
	// only thing that spans the gap.
	Referral string
}

var (
	otpStore   = make(map[string]otpEntry)
	otpStoreMu sync.Mutex
)

func StoreOTP(email, otp string, data interface{}) {
	StoreOTPWithType(email, otp, "", data)
}

func StoreOTPWithType(email, otp, otpType string, data interface{}) {
	StoreOTPWithReferral(email, otp, otpType, data, "")
}

// StoreOTPWithReferral stages a pending account together with the referral code
// the invitee arrived with.
func StoreOTPWithReferral(email, otp, otpType string, data interface{}, referral string) {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	otpStore[email] = otpEntry{
		OTP:       otp,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		Type:      otpType,
		Data:      data,
		Referral:  referral,
	}
}

func VerifyOTP(email, otp string) (bool, string, interface{}) {
	valid, otpType, data, _ := VerifyOTPWithReferral(email, otp)
	return valid, otpType, data
}

// VerifyOTPWithReferral consumes the entry and returns the referral code with it.
//
// The code is returned FROM the consume rather than read afterwards by a second
// call. VerifyOTP deletes the entry on success (one attempt per issued code), so
// a follow-up getter would find nothing — and a getter that ran before the
// consume would have to race it. Returning it from the same locked read is the
// only version where the two cannot disagree.
//
// VerifyOTP itself is kept as a three-value wrapper rather than changed in place.
// There are callers that have no interest in a referral code — password reset is
// the obvious one — and widening their signature would push the concept into
// modules that must not know about it.
func VerifyOTPWithReferral(email, otp string) (bool, string, interface{}, string) {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	entry, exists := otpStore[email]
	if !exists {
		return false, "", nil, ""
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(otpStore, email)
		return false, "", nil, ""
	}
	if entry.OTP != otp {
		return false, "", nil, ""
	}
	otpType := entry.Type
	referral := entry.Referral
	delete(otpStore, email)
	return true, otpType, entry.Data, referral
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

// GetOTPStaged is the whole staged entry, for the one caller that has to re-stage
// it.
//
// SendOTP re-stores the pending account under a freshly generated code when the
// student clicks "send code again" — and SendOTP is on the MAINLINE path, not an
// edge case: Register deliberately does not mail the OTP, the frontend calls
// /send-otp afterwards. A re-store that carried back only (type, data) would drop
// the referral code, and the referral would then be lost for precisely the
// students who took the ordinary route through registration. So the entry is read
// and written whole.
//
// This is the same shape as the harness in
// internal/auth/registration_role_test.go, which re-stages under a known code to
// drive the real verification flow.
func GetOTPStaged(email string) (otpType string, data interface{}, referral string, found bool) {
	otpStoreMu.Lock()
	defer otpStoreMu.Unlock()
	entry, exists := otpStore[email]
	if !exists {
		return "", nil, "", false
	}
	return entry.Type, entry.Data, entry.Referral, true
}

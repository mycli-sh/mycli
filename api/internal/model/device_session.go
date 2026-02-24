package model

import "time"

type DeviceSession struct {
	ID          string    `json:"id"`
	DeviceCode  string    `json:"device_code"`
	UserCode    string    `json:"user_code"`
	Email       string    `json:"email"`
	ExpiresAt   time.Time `json:"expires_at"`
	Authorized  bool      `json:"authorized"`
	UserID      *string   `json:"user_id,omitempty"`
	OTPAttempts int       `json:"otp_attempts"`
	CreatedAt   time.Time `json:"created_at"`
}

package model

import "time"

type MagicLink struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	TokenHash  string     `json:"token_hash"`
	DeviceCode string     `json:"device_code"`
	OTPHash    *string    `json:"otp_hash,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	UsedAt     *time.Time `json:"used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

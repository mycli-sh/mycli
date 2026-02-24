package model

import (
	"time"

	"github.com/google/uuid"
)

type MagicLink struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	TokenHash   string     `json:"token_hash"`
	DeviceCode  string     `json:"device_code"`
	OTPHash     *string    `json:"otp_hash,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
	UsedAt      *time.Time `json:"used_at"`
	CreatedAt   time.Time  `json:"created_at"`
	Authorized  bool       `json:"authorized"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	OTPAttempts int        `json:"otp_attempts"`
}

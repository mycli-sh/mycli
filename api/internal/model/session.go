package model

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	RefreshTokenHash string     `json:"-"`
	UserAgent        string     `json:"user_agent"`
	IPAddress        string     `json:"ip_address"`
	DeviceID         string     `json:"device_id"`
	DeviceName       string     `json:"device_name"`
	LastUsedAt       time.Time  `json:"last_used_at"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

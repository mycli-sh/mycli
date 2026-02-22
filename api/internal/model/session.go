package model

import "time"

type Session struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	RefreshTokenHash string     `json:"-"`
	UserAgent        string     `json:"user_agent"`
	IPAddress        string     `json:"ip_address"`
	LastUsedAt       time.Time  `json:"last_used_at"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

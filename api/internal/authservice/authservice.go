package authservice

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"mycli.sh/api/internal/model"
	"mycli.sh/api/internal/store"
)

// Store defines the subset of store operations needed by the auth service.
type Store interface {
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	CreateUser(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	CreateSession(ctx context.Context, userID uuid.UUID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName string, expiresAt time.Time) (*model.Session, error)
	RevokeSessionByDeviceID(ctx context.Context, userID uuid.UUID, deviceID string) error
}

const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 30 * 24 * time.Hour
	AccessTokenTTL       = 900            // seconds, matches AccessTokenDuration
	RefreshTokenTTL      = 30 * 24 * 3600 // seconds, matches RefreshTokenDuration
)

// Service centralises auth business logic shared across handlers.
type Service struct {
	jwtSecret string
	store     Store
}

// New creates a new auth service.
func New(jwtSecret string, s Store) *Service {
	return &Service{jwtSecret: jwtSecret, store: s}
}

// TokenResult holds the result of IssueTokens.
type TokenResult struct {
	AccessToken   string
	RefreshToken  string
	SessionID     string
	NeedsUsername bool
}

// FindOrCreateUser looks up a user by email. If the user does not exist, it
// creates one.
func (s *Service) FindOrCreateUser(ctx context.Context, email string) (*model.User, error) {
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return s.store.CreateUser(ctx, email)
		}
		return nil, err
	}
	return user, nil
}

// IssueTokens generates JWT tokens (access 15m + refresh 30d), revokes any
// existing session for the same device, creates a new session, and checks
// whether the user still needs to set a username.
func (s *Service) IssueTokens(ctx context.Context, userID uuid.UUID, r *http.Request) (*TokenResult, error) {
	accessToken, refreshToken, err := s.GenerateTokenPair(userID)
	if err != nil {
		return nil, err
	}

	meta := ExtractRequestMeta(r)
	refreshTokenHash := HashToken(refreshToken)

	// Revoke any existing session for this device before creating a new one
	if meta.DeviceID != "" {
		_ = s.store.RevokeSessionByDeviceID(ctx, userID, meta.DeviceID)
	}

	var sessionID string
	if sess, err := s.store.CreateSession(ctx, userID, refreshTokenHash, meta.UserAgent, meta.IPAddress, meta.DeviceID, meta.DeviceName, time.Now().Add(RefreshTokenDuration)); err == nil {
		sessionID = sess.ID.String()
	}

	needsUsername := false
	if user, err := s.store.GetUserByID(ctx, userID); err == nil {
		needsUsername = user.Username == nil
	}

	return &TokenResult{
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		SessionID:     sessionID,
		NeedsUsername: needsUsername,
	}, nil
}

// GenerateTokenPair creates a matched access + refresh JWT pair for the given user.
func (s *Service) GenerateTokenPair(userID uuid.UUID) (accessToken, refreshToken string, err error) {
	sub := userID.String()
	accessToken, err = GenerateJWTToken(s.jwtSecret, sub, "access", AccessTokenDuration)
	if err != nil {
		return "", "", err
	}
	refreshToken, err = GenerateJWTToken(s.jwtSecret, sub, "refresh", RefreshTokenDuration)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

// RequestMeta holds request metadata extracted from HTTP headers.
type RequestMeta struct {
	UserAgent  string
	IPAddress  string
	DeviceID   string
	DeviceName string
}

// ExtractRequestMeta extracts common request metadata (IP, device ID, user agent, device name)
// from an HTTP request.
func ExtractRequestMeta(r *http.Request) RequestMeta {
	ipAddress := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ipAddress = strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
	}
	deviceID := r.Header.Get("X-Device-ID")
	if !ValidDeviceID(deviceID) {
		deviceID = ""
	}
	return RequestMeta{
		UserAgent:  r.UserAgent(),
		IPAddress:  ipAddress,
		DeviceID:   deviceID,
		DeviceName: r.Header.Get("X-Device-Name"),
	}
}

// --- Utility functions (moved from handler package) ---

// GenerateJWTToken creates a signed JWT with the given parameters.
func GenerateJWTToken(secret, userID, tokenType string, duration time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"type": tokenType,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(duration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateCode returns a cryptographically random hex string of the given byte length.
func GenerateCode(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateOTP returns a 6-digit random OTP code.
func GenerateOTP() string {
	const digits = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		code[i] = digits[n.Int64()]
	}
	return string(code)
}

// HashToken returns the SHA-256 hex digest of a token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ValidDeviceID(id string) bool {
	return uuidRe.MatchString(id)
}

// ValidEmail performs a basic email validation check.
func ValidEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return false
	}
	domain := email[at+1:]
	if len(domain) < 3 || !strings.Contains(domain, ".") {
		return false
	}
	return true
}

package middleware

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"mycli.sh/api/internal/model"
)

type contextKey string

const (
	UserIDKey     contextKey = "user_id"
	AuthMethodKey contextKey = "auth_method"
	ProfileIDKey  contextKey = "profile_id"
	TokenIDKey    contextKey = "token_id"
)

// APITokenLookup is the minimal interface needed to validate API tokens.
type APITokenLookup interface {
	GetAPITokenByHash(ctx context.Context, tokenHash string) (*model.APIToken, error)
	UpdateAPITokenLastUsed(ctx context.Context, id uuid.UUID) error
}

func GetUserID(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(UserIDKey).(uuid.UUID)
	return v
}

func GetAuthMethod(ctx context.Context) string {
	v, _ := ctx.Value(AuthMethodKey).(string)
	return v
}

func GetProfileID(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(ProfileIDKey).(uuid.UUID)
	return v
}

// parseAccessToken validates a JWT string, checks that it uses HMAC signing,
// is an "access" type token, and returns the subject (user ID) as a uuid.UUID.
func parseAccessToken(jwtSecret, tokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, errors.New("invalid claims")
	}

	if tokenType, _ := claims["type"].(string); tokenType != "access" {
		return uuid.Nil, errors.New("invalid token type")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return uuid.Nil, errors.New("missing subject")
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, errors.New("invalid subject")
	}

	return userID, nil
}

// isAPIToken returns true if the token starts with the "myc_" prefix.
func isAPIToken(token string) bool {
	return strings.HasPrefix(token, "myc_")
}

// hashAPIToken returns the SHA-256 hex hash of the raw API token.
func hashAPIToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

// authenticateToken resolves a Bearer token to a user ID and sets auth context values.
// Returns the user ID and a context with auth metadata, or an error.
func authenticateToken(ctx context.Context, jwtSecret string, tokenLookup APITokenLookup, tokenStr string) (context.Context, uuid.UUID, error) {
	if isAPIToken(tokenStr) {
		if tokenLookup == nil {
			return ctx, uuid.Nil, errors.New("api tokens not supported")
		}
		hash := hashAPIToken(tokenStr)
		apiToken, err := tokenLookup.GetAPITokenByHash(ctx, hash)
		if err != nil {
			return ctx, uuid.Nil, errors.New("invalid or revoked api token")
		}
		ctx = context.WithValue(ctx, UserIDKey, apiToken.UserID)
		ctx = context.WithValue(ctx, AuthMethodKey, "api_token")
		ctx = context.WithValue(ctx, TokenIDKey, apiToken.ID)
		if apiToken.ProfileID != nil {
			ctx = context.WithValue(ctx, ProfileIDKey, *apiToken.ProfileID)
		}
		// Update last_used_at asynchronously (best-effort)
		go func() {
			_ = tokenLookup.UpdateAPITokenLastUsed(context.Background(), apiToken.ID)
		}()
		return ctx, apiToken.UserID, nil
	}

	// JWT path
	userID, err := parseAccessToken(jwtSecret, tokenStr)
	if err != nil {
		return ctx, uuid.Nil, err
	}
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, AuthMethodKey, "jwt")
	return ctx, userID, nil
}

// OptionalAuth parses the JWT or API token if present but does not reject
// unauthenticated requests.
func OptionalAuth(jwtSecret string, tokenLookup ...APITokenLookup) func(http.Handler) http.Handler {
	var lookup APITokenLookup
	if len(tokenLookup) > 0 {
		lookup = tokenLookup[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				ctx, _, err := authenticateToken(r.Context(), jwtSecret, lookup, tokenStr)
				if err == nil {
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Auth(jwtSecret string, tokenLookup ...APITokenLookup) func(http.Handler) http.Handler {
	var lookup APITokenLookup
	if len(tokenLookup) > 0 {
		lookup = tokenLookup[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED","message":"missing or invalid authorization header"}}`)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			ctx, _, err := authenticateToken(r.Context(), jwtSecret, lookup, tokenStr)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED","message":"`+err.Error()+`"}}`)
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireJWT is middleware that rejects requests authenticated with API tokens.
// Used for sensitive endpoints like token management (tokens can't manage tokens).
func RequireJWT() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if GetAuthMethod(r.Context()) != "jwt" {
				writeJSONError(w, http.StatusForbidden, `{"error":{"code":"JWT_REQUIRED","message":"this endpoint requires interactive authentication"}}`)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

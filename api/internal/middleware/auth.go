package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}

// parseAccessToken validates a JWT string, checks that it uses HMAC signing,
// is an "access" type token, and returns the subject (user ID).
func parseAccessToken(jwtSecret, tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
	}

	if tokenType, _ := claims["type"].(string); tokenType != "access" {
		return "", errors.New("invalid token type")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", errors.New("missing subject")
	}

	return sub, nil
}

// OptionalAuth parses the JWT if present but does not reject unauthenticated
// requests. If the token is valid the user ID is placed into context; otherwise
// the request continues with no user context.
func OptionalAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				if userID, err := parseAccessToken(jwtSecret, tokenStr); err == nil {
					ctx := context.WithValue(r.Context(), UserIDKey, userID)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED","message":"missing or invalid authorization header"}}`)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := parseAccessToken(jwtSecret, tokenStr)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED","message":"`+err.Error()+`"}}`)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

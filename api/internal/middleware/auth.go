package middleware

import (
	"context"
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

// OptionalAuth parses the JWT if present but does not reject unauthenticated
// requests. If the token is valid the user ID is placed into context; otherwise
// the request continues with no user context.
func OptionalAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, jwt.ErrSignatureInvalid
					}
					return []byte(jwtSecret), nil
				})
				if err == nil && token.Valid {
					if claims, ok := token.Claims.(jwt.MapClaims); ok {
						if sub, _ := claims["sub"].(string); sub != "" {
							if tokenType, _ := claims["type"].(string); tokenType == "access" {
								ctx := context.WithValue(r.Context(), UserIDKey, sub)
								r = r.WithContext(ctx)
							}
						}
					}
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
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				writeJSONError(w, http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED","message":"invalid token"}}`)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED","message":"invalid claims"}}`)
				return
			}

			sub, _ := claims["sub"].(string)
			if sub == "" {
				writeJSONError(w, http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED","message":"missing subject"}}`)
				return
			}

			tokenType, _ := claims["type"].(string)
			if tokenType != "access" {
				writeJSONError(w, http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED","message":"invalid token type"}}`)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

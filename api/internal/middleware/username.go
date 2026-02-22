package middleware

import (
	"context"
	"net/http"

	"mycli.sh/api/internal/model"
)

// UserLookup is the minimal interface needed by RequireUsername.
type UserLookup interface {
	GetUserByID(ctx context.Context, id string) (*model.User, error)
}

// RequireUsername returns middleware that blocks requests from users who
// haven't set a username yet, returning 403 USERNAME_REQUIRED.
func RequireUsername(lookup UserLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" {
				writeJSONError(w, http.StatusUnauthorized, `{"error":{"code":"UNAUTHORIZED","message":"missing user context"}}`)
				return
			}

			user, err := lookup.GetUserByID(r.Context(), userID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, `{"error":{"code":"INTERNAL_ERROR","message":"failed to look up user"}}`)
				return
			}

			if user.Username == nil {
				writeJSONError(w, http.StatusForbidden, `{"error":{"code":"USERNAME_REQUIRED","message":"you must set a username before using this endpoint"}}`)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

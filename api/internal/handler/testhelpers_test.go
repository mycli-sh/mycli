package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mycli.sh/api/internal/middleware"
)

// requestWithUser creates an HTTP request with userID injected into context
// (simulating the auth middleware).
func requestWithUser(method, target string, body any, userID string) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, target, &buf)
	r.Header.Set("Content-Type", "application/json")
	if userID != "" {
		ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
		r = r.WithContext(ctx)
	}
	return r
}

// decodeJSON decodes the response body into v and fails the test on error.
func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response body: %v (body=%s)", err, rec.Body.String())
	}
}

// errorResponse mirrors the error envelope returned by writeError.
type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

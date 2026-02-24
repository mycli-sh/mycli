package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"mycli.sh/api/internal/middleware"
)

// Test UUIDs used across handler tests.
var (
	testUser1      = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	testUser2      = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	testUserOther  = uuid.MustParse("00000000-0000-4000-8000-000000000003")
	testUserNew    = uuid.MustParse("00000000-0000-4000-8000-000000000004")
	testLibOwner   = uuid.MustParse("00000000-0000-4000-8000-000000000005")
	testCmd1       = uuid.MustParse("00000000-0000-4000-8000-000000000010")
	testCmd2       = uuid.MustParse("00000000-0000-4000-8000-000000000011")
	testCmd123     = uuid.MustParse("00000000-0000-4000-8000-000000000012")
	testCmdLib1    = uuid.MustParse("00000000-0000-4000-8000-000000000015")
	testSes1       = uuid.MustParse("00000000-0000-4000-8000-000000000020")
	testSes2       = uuid.MustParse("00000000-0000-4000-8000-000000000021")
	testSesCurrent = uuid.MustParse("00000000-0000-4000-8000-000000000022")
	testML1        = uuid.MustParse("00000000-0000-4000-8000-000000000030")
	testCV1        = uuid.MustParse("00000000-0000-4000-8000-000000000040")
	testLib1       = uuid.MustParse("00000000-0000-4000-8000-000000000050")
	testDeviceUUID = "11111111-1111-4111-8111-111111111111"
)

// requestWithUser creates an HTTP request with userID injected into context
// (simulating the auth middleware).
func requestWithUser(method, target string, body any, userID uuid.UUID) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, target, &buf)
	r.Header.Set("Content-Type", "application/json")
	if userID != uuid.Nil {
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

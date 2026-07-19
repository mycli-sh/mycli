package middleware

import (
	"fmt"
	"net/http"
)

// Body-size limits for incoming requests.
//
// Defaults match the largest realistic payload for the route group:
//   - DefaultBodyLimitBytes covers token/profile/command bodies (KB at most).
//   - ReleaseBodyLimitBytes covers POST /v1/releases which bundles many
//     libraries, each with many command specs.
//
// Keep CLI-side constants (see cli/internal/client/client.go) in sync.
const (
	DefaultBodyLimitBytes int64 = 256 * 1024
	ReleaseBodyLimitBytes int64 = 4 * 1024 * 1024
)

// BodyLimit rejects requests larger than max bytes with 413. Fast-fails on
// declared Content-Length, otherwise wraps the body in MaxBytesReader so a
// chunked oversize payload is cut off during Read and bubbles up as a decode
// error (which handlers already surface as 400).
func BodyLimit(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > max {
				writeJSONError(w, http.StatusRequestEntityTooLarge,
					fmt.Sprintf(`{"error":{"code":"PAYLOAD_TOO_LARGE","message":"request body exceeds %d bytes"}}`, max))
				return
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, max)
			}
			next.ServeHTTP(w, r)
		})
	}
}

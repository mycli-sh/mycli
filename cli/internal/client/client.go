package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofrs/flock"
	"resty.dev/v3"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/config"
)

// Version is set at build time via -ldflags. Defaults to "dev".
var Version = "dev"

// InstallMethod is set at build time via -ldflags. Tracks how the CLI was installed.
// Possible values: "source" (default), "github" (goreleaser/install script), "brew" (Homebrew).
var InstallMethod = "source"

// globalRefreshMu serializes refresh across every *Client in the process. The
// background keepalive and a command run on separate clients; without it they
// could both POST the same single-use refresh token and the loser would 401.
var globalRefreshMu sync.Mutex

// authRefreshPath must not itself trigger a refresh — that would recurse into
// globalRefreshMu on the same goroutine. Guarding by path (not a global flag)
// lets concurrent requests wait for the in-flight refresh and use its new token.
const authRefreshPath = "/v1/auth/refresh"

func isRefreshRequest(url string) bool {
	return strings.HasSuffix(url, authRefreshPath)
}

// ErrSessionExpired is returned when the JWT session is definitively dead
// (refresh rejected) and credentials have been cleared. Match with errors.Is.
var ErrSessionExpired = errors.New(`your session has expired — run "my cli login" to sign in again`)

type Client struct {
	rc *resty.Client
	// sessionCleared is set when a 401 triggered a definitive credential clear,
	// so do()/GetCatalog() can translate the 401 into ErrSessionExpired.
	sessionCleared atomic.Bool
}

func New(baseURL string) *Client {
	c := &Client{}

	rc := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(30*time.Second).
		SetHeader("Content-Type", "application/json")

	// Request middleware: inject common headers and auth token on every request
	rc.AddRequestMiddleware(func(_ *resty.Client, req *resty.Request) error {
		req.SetHeader("User-Agent", "mycli/"+Version)
		req.SetHeader("X-Device-ID", config.DeviceID())
		if hostname, err := os.Hostname(); err == nil {
			req.SetHeader("X-Device-Name", hostname)
		}
		if tokens, err := auth.LoadTokens(); err == nil && tokens.AccessToken != "" {
			// Skip refresh for API tokens (myc_ prefix) — they don't expire via JWT
			// and for the refresh request itself (would recurse).
			if !auth.IsAPIToken(tokens.AccessToken) && !isRefreshRequest(req.URL) {
				// Proactively refresh once the token has expired. tryRefresh is
				// throttled to minRefreshInterval, so refreshing earlier would
				// just be skipped — this keeps refreshes at least that far apart.
				if !tokens.ExpiresAt.IsZero() && time.Now().After(tokens.ExpiresAt) {
					if ok, _ := c.tryRefresh(); ok {
						tokens, _ = auth.LoadTokens() // reload after refresh
					}
				}
			}
			if tokens.AccessToken != "" {
				req.SetHeader("Authorization", "Bearer "+tokens.AccessToken)
			}
		}
		return nil
	})

	// Retry once on 401 after refreshing the token (JWT only, not API tokens)
	rc.SetRetryCount(1).
		SetRetryDefaultConditions(false).
		AddRetryConditions(resty.RetryConditionFunc(func(resp *resty.Response, err error) bool {
			if err != nil || resp == nil {
				return false
			}
			if resp.StatusCode() != http.StatusUnauthorized {
				return false
			}
			// Don't retry refresh for API tokens
			if tokens, loadErr := auth.LoadTokens(); loadErr == nil && auth.IsAPIToken(tokens.AccessToken) {
				return false
			}
			// The refresh request itself must not recurse into a refresh.
			if isRefreshRequest(resp.Request.URL) {
				return false
			}
			ok, refreshErr := c.tryRefresh()
			if ok {
				return true // retry — middleware will pick up the new token
			}
			// Only wipe credentials on a definitive rejection. Transient failures
			// keep the tokens so the next invocation can retry.
			if isDefinitiveAuthFailure(refreshErr) {
				_ = auth.ClearTokens()
				c.sessionCleared.Store(true)
			}
			return false
		}))

	c.rc = rc
	return c
}

// Close releases resources held by the underlying Resty client.
func (c *Client) Close() {
	_ = c.rc.Close()
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type apiErrorEnvelope struct {
	Error APIError `json:"error"`
}

func (c *Client) do(method, path string, reqBody any, out any) error {
	c.sessionCleared.Store(false)
	req := c.rc.R()
	if reqBody != nil {
		req.SetBody(reqBody)
	}
	var errEnv apiErrorEnvelope
	req.SetResultError(&errEnv)
	if out != nil {
		req.SetResult(out)
	}

	resp, err := req.Execute(method, path)
	if err != nil {
		return err
	}
	if resp.IsStatusFailure() {
		if resp.StatusCode() == http.StatusUnauthorized && c.sessionCleared.Load() {
			return ErrSessionExpired
		}
		if errEnv.Error.Code != "" {
			return &errEnv.Error
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
	}
	return nil
}

// minRefreshInterval is the floor between token rotations. The access token
// lives 15 minutes, so a refresh within that window means the current token is
// still valid — reuse it instead of rotating again. This both throttles refresh
// traffic and dedupes concurrent refreshers (they see the winner's fresh
// LastRefreshedAt and skip their own POST).
const minRefreshInterval = 15 * time.Minute

// refreshLockTimeout bounds how long we wait for the cross-process refresh lock
// before proceeding without it, so a stuck peer can never wedge refresh.
const refreshLockTimeout = 10 * time.Second

func refreshedRecently(t *auth.Tokens) bool {
	return t != nil && !t.LastRefreshedAt.IsZero() && time.Since(t.LastRefreshedAt) < minRefreshInterval
}

// acquireRefreshLock takes a cross-process advisory lock so parallel `my`
// processes serialize their refreshes over the shared credential file. It
// returns a release func that is always safe to call, and degrades to a no-op
// if the lock can't be acquired within refreshLockTimeout.
func acquireRefreshLock() func() {
	dir := config.DefaultDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return func() {}
	}
	fl := flock.New(filepath.Join(dir, "refresh.lock"))
	ctx, cancel := context.WithTimeout(context.Background(), refreshLockTimeout)
	defer cancel()
	if locked, err := fl.TryLockContext(ctx, 50*time.Millisecond); err != nil || !locked {
		return func() {}
	}
	return func() { _ = fl.Unlock() }
}

// tryRefresh refreshes the access token using the stored refresh token,
// serialized in-process (globalRefreshMu) and across processes (file lock). If a
// refresh happened within minRefreshInterval — by this or any parallel process —
// it reuses the current token instead of rotating. Returns (true, nil) when the
// stored tokens are valid afterward; (false, err) on a genuine rejection, with
// err carrying the reason so callers can tell a dead session from a transient
// failure.
func (c *Client) tryRefresh() (bool, error) {
	globalRefreshMu.Lock()
	defer globalRefreshMu.Unlock()

	unlock := acquireRefreshLock()
	defer unlock()

	tokens, err := auth.LoadTokens()
	if err != nil || tokens.RefreshToken == "" {
		return false, nil
	}

	// Throttle / dedup: a recent rotation means the token is still valid.
	if refreshedRecently(tokens) {
		return true, nil
	}

	refreshResp, err := c.RefreshToken(tokens.RefreshToken)
	if err != nil || refreshResp.AccessToken == "" {
		// A parallel refresher may have rotated the session out from under us.
		// If disk shows a fresh rotation, treat as success so we don't wipe it.
		if fresh, loadErr := auth.LoadTokens(); loadErr == nil && refreshedRecently(fresh) {
			return true, nil
		}
		return false, err
	}

	saveRefreshedTokens(tokens, refreshResp)
	return true, nil
}

// RefreshNow forces a refresh, used by the background keepalive to keep the
// session alive. It shares tryRefresh's coordination and throttle; returns true
// when the stored tokens are valid afterward.
func (c *Client) RefreshNow() bool {
	ok, _ := c.tryRefresh()
	return ok
}

// saveRefreshedTokens applies a successful refresh response onto the loaded
// tokens and persists them. The caller must hold globalRefreshMu.
func saveRefreshedTokens(tokens *auth.Tokens, refreshResp *auth.TokenResponse) {
	now := time.Now()
	tokens.AccessToken = refreshResp.AccessToken
	tokens.ExpiresAt = now.Add(time.Duration(refreshResp.ExpiresIn) * time.Second)
	tokens.LastRefreshedAt = now
	if refreshResp.RefreshToken != "" {
		tokens.RefreshToken = refreshResp.RefreshToken
	}
	_ = auth.SaveTokens(tokens)
}

// isDefinitiveAuthFailure reports whether a refresh error means the session is
// genuinely dead (an explicit auth rejection from /v1/auth/refresh), so the
// caller may clear credentials. Transport errors and 5xx do not qualify.
func isDefinitiveAuthFailure(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Code {
	case "INVALID_TOKEN", "SESSION_REVOKED", "SESSION_EXPIRED":
		return true
	default:
		return false
	}
}

// Auth endpoints

func (c *Client) StartDeviceFlow(email string) (*auth.DeviceCodeResponse, error) {
	var resp auth.DeviceCodeResponse
	body := map[string]string{"email": email}
	if err := c.do("POST", "/v1/auth/device/start", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) PollDeviceToken(deviceCode string) (*auth.TokenResponse, error) {
	var resp auth.TokenResponse
	err := c.do("POST", "/v1/auth/device/token", map[string]string{"device_code": deviceCode}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) VerifyOTP(deviceCode, code string) error {
	var resp struct {
		Authorized bool `json:"authorized"`
	}
	return c.do("POST", "/v1/auth/verify-code", map[string]string{
		"device_code": deviceCode,
		"code":        code,
	}, &resp)
}

func (c *Client) ResendVerification(deviceCode, email string) (int, error) {
	var resp struct {
		ExpiresIn int `json:"expires_in"`
	}
	err := c.do("POST", "/v1/auth/device/resend", map[string]string{
		"device_code": deviceCode,
		"email":       email,
	}, &resp)
	return resp.ExpiresIn, err
}

func (c *Client) Logout(refreshToken string) error {
	var body map[string]string
	if refreshToken != "" {
		body = map[string]string{"refresh_token": refreshToken}
	}
	return c.do("POST", "/v1/auth/logout", body, nil)
}

func (c *Client) RefreshToken(refreshToken string) (*auth.TokenResponse, error) {
	var resp auth.TokenResponse
	err := c.do("POST", "/v1/auth/refresh", map[string]string{"refresh_token": refreshToken}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// User endpoints

type UserInfo struct {
	ID            string  `json:"id"`
	Email         string  `json:"email"`
	Username      *string `json:"username,omitempty"`
	NeedsUsername bool    `json:"needs_username"`
}

func (c *Client) GetMe() (*UserInfo, error) {
	var resp UserInfo
	if err := c.do("GET", "/v1/me", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Username endpoints

type UsernameAvailability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (c *Client) CheckUsernameAvailable(username string) (*UsernameAvailability, error) {
	var resp UsernameAvailability
	if err := c.do("GET", "/v1/usernames/"+username+"/available", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) SetUsername(username string) error {
	return c.do("PATCH", "/v1/me/username", map[string]string{"username": username}, nil)
}

// Command endpoints

type CreateCommandRequest struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type CommandResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	LatestVersion int    `json:"latest_version,omitempty"`
}

type CommandListResponse struct {
	Commands   []CommandResponse `json:"commands"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

func (c *Client) CreateCommand(req *CreateCommandRequest) (*CommandResponse, error) {
	var resp CommandResponse
	if err := c.do("POST", "/v1/commands", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListCommands(cursor string, limit int) (*CommandListResponse, error) {
	path := fmt.Sprintf("/v1/commands?limit=%d", limit)
	if cursor != "" {
		path += "&cursor=" + cursor
	}
	var resp CommandListResponse
	if err := c.do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetCommand(id string) (*CommandResponse, error) {
	var resp CommandResponse
	if err := c.do("GET", "/v1/commands/"+id, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetCommandBySlug(slug string) (*CommandResponse, error) {
	var resp CommandListResponse
	if err := c.do("GET", "/v1/commands?slug="+slug, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Commands) == 0 {
		return nil, nil
	}
	return &resp.Commands[0], nil
}

type PublishVersionRequest struct {
	SpecJSON json.RawMessage `json:"spec_json"`
	Message  string          `json:"message,omitempty"`
}

type VersionResponse struct {
	ID       string `json:"id"`
	Version  int    `json:"version"`
	SpecHash string `json:"spec_hash"`
}

func (c *Client) PublishVersion(commandID string, req *PublishVersionRequest) (*VersionResponse, error) {
	var resp VersionResponse
	if err := c.do("POST", "/v1/commands/"+commandID+"/versions", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetVersionSpec(commandID string, version int) (json.RawMessage, error) {
	var resp struct {
		SpecJSON json.RawMessage `json:"spec_json"`
	}
	path := fmt.Sprintf("/v1/commands/%s/versions/%d", commandID, version)
	if err := c.do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.SpecJSON, nil
}

func (c *Client) DeleteCommand(id string) error {
	return c.do("DELETE", "/v1/commands/"+id, nil, nil)
}

type CatalogItem struct {
	CommandID      string   `json:"command_id"`
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Version        int      `json:"version"`
	SpecHash       string   `json:"spec_hash"`
	UpdatedAt      string   `json:"updated_at"`
	Library        string   `json:"library,omitempty"`
	LibraryOwner   string   `json:"library_owner,omitempty"`
	Aliases        []string `json:"aliases,omitempty"`
	LibraryAliases []string `json:"library_aliases,omitempty"`
}

type CatalogResponse struct {
	Items []CatalogItem `json:"items"`
	ETag  string        `json:"-"`
}

func (c *Client) GetCatalog(etag string, profile ...string) (*CatalogResponse, error) {
	c.sessionCleared.Store(false)
	req := c.rc.R()
	if etag != "" {
		req.SetHeader("If-None-Match", etag)
	}
	var catalog CatalogResponse
	var errEnv apiErrorEnvelope
	req.SetResult(&catalog).SetResultError(&errEnv)

	path := "/v1/catalog"
	if len(profile) > 0 && profile[0] != "" {
		path += "?profile=" + url.QueryEscape(profile[0])
	}

	resp, err := req.Execute("GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == http.StatusNotModified {
		return nil, nil
	}
	if resp.IsStatusFailure() {
		if resp.StatusCode() == http.StatusUnauthorized && c.sessionCleared.Load() {
			return nil, ErrSessionExpired
		}
		if errEnv.Error.Code != "" {
			return nil, &errEnv.Error
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
	}

	catalog.ETag = resp.Header().Get("ETag")
	return &catalog, nil
}

// Library endpoints

type PublicLibrary struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Owner        string `json:"owner"`
	IsPublic     bool   `json:"is_public"`
	InstallCount int    `json:"install_count"`
}

type SearchLibrariesResponse struct {
	Libraries []PublicLibrary `json:"libraries"`
	Total     int             `json:"total"`
}

type PublicLibraryDetail struct {
	Library  PublicLibrary    `json:"library"`
	Owner    string           `json:"owner"`
	Commands []LibraryCommand `json:"commands"`
}

type LibraryCommand struct {
	CommandID   string `json:"command_id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (c *Client) SearchPublicLibraries(query string, limit, offset int) (*SearchLibrariesResponse, error) {
	path := fmt.Sprintf("/v1/libraries?q=%s&limit=%d&offset=%d", query, limit, offset)
	var resp SearchLibrariesResponse
	if err := c.do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetPublicLibrary(owner, slug string) (*PublicLibraryDetail, error) {
	var resp PublicLibraryDetail
	path := fmt.Sprintf("/v1/libraries/%s/%s", owner, slug)
	if err := c.do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type CreateReleaseRequest struct {
	Tag         string            `json:"tag"`
	CommitHash  string            `json:"commit_hash"`
	Namespace   string            `json:"namespace,omitempty"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	GitURL      string            `json:"git_url,omitempty"`
	Aliases     []string          `json:"aliases,omitempty"`
	Commands    []json.RawMessage `json:"commands"`
}

type CreateReleaseResponse struct {
	Release   LibraryReleaseInfo `json:"release"`
	Published int                `json:"published"`
}

type LibraryReleaseInfo struct {
	ID           string `json:"id"`
	Version      string `json:"version"`
	Tag          string `json:"tag"`
	CommitHash   string `json:"commit_hash"`
	CommandCount int    `json:"command_count"`
	ReleasedAt   string `json:"released_at"`
}

// MaxReleaseBodyBytes mirrors api/internal/middleware.ReleaseBodyLimitBytes.
// Keep in sync; the CLI rejects oversized payloads before transmitting so
// users get a clear message instead of a wire-level 413.
const MaxReleaseBodyBytes = 4 * 1024 * 1024

func (c *Client) CreateRelease(slug string, req *CreateReleaseRequest) (*CreateReleaseResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal release body: %w", err)
	}
	if int64(len(body)) > MaxReleaseBodyBytes {
		return nil, fmt.Errorf(
			"release payload is %d bytes, exceeds server limit of %d bytes; reduce the number of commands or the size of individual specs",
			len(body), MaxReleaseBodyBytes,
		)
	}
	var resp CreateReleaseResponse
	if err := c.do("POST", "/v1/libraries/"+slug+"/releases", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type ListReleasesResponse struct {
	Releases []LibraryReleaseInfo `json:"releases"`
}

func (c *Client) ListReleases(owner, slug string) ([]LibraryReleaseInfo, error) {
	var resp ListReleasesResponse
	path := fmt.Sprintf("/v1/libraries/%s/%s/releases", owner, slug)
	if err := c.do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Releases, nil
}

// Token endpoints

type CreateTokenRequest struct {
	Name      string `json:"name"`
	ExpiresIn string `json:"expires_in,omitempty"`
	ProfileID string `json:"profile_id,omitempty"`
}

type CreateTokenResponse struct {
	Token       string  `json:"token"`
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	TokenPrefix string  `json:"token_prefix"`
	ProfileID   *string `json:"profile_id,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

type APITokenInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	TokenPrefix string  `json:"token_prefix"`
	ProfileID   *string `json:"profile_id,omitempty"`
	LastUsedAt  *string `json:"last_used_at,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

type ListTokensResponse struct {
	Tokens []APITokenInfo `json:"tokens"`
}

func (c *Client) CreateToken(req *CreateTokenRequest) (*CreateTokenResponse, error) {
	body := map[string]any{"name": req.Name}
	if req.ExpiresIn != "" {
		body["expires_in"] = req.ExpiresIn
	}
	if req.ProfileID != "" {
		body["profile_id"] = req.ProfileID
	}
	var resp CreateTokenResponse
	if err := c.do("POST", "/v1/tokens", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListTokens() (*ListTokensResponse, error) {
	var resp ListTokensResponse
	if err := c.do("GET", "/v1/tokens", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) RevokeToken(id string) error {
	return c.do("DELETE", "/v1/tokens/"+id, nil, nil)
}

// Profile endpoints

type ProfileInfo struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ListProfilesResponse struct {
	Profiles []ProfileInfo `json:"profiles"`
}

type ProfileDetailResponse struct {
	Profile   ProfileInfo     `json:"profile"`
	Libraries json.RawMessage `json:"libraries"`
}

func (c *Client) CreateProfile(slug, name, description string) (*ProfileInfo, error) {
	var resp ProfileInfo
	body := map[string]string{
		"slug":        slug,
		"name":        name,
		"description": description,
	}
	if err := c.do("POST", "/v1/profiles", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListProfiles() (*ListProfilesResponse, error) {
	var resp ListProfilesResponse
	if err := c.do("GET", "/v1/profiles", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetProfile(slug string) (*ProfileDetailResponse, error) {
	var resp ProfileDetailResponse
	if err := c.do("GET", "/v1/profiles/"+slug, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteProfile(slug string, force bool) error {
	path := "/v1/profiles/" + slug
	if force {
		path += "?force=true"
	}
	return c.do("DELETE", path, nil, nil)
}

func (c *Client) AddLibraryToProfile(profileSlug, library string) error {
	body := map[string]string{"library": library}
	return c.do("POST", "/v1/profiles/"+profileSlug+"/libraries", body, nil)
}

func (c *Client) RemoveLibraryFromProfile(profileSlug, owner, libSlug string) error {
	return c.do("DELETE", fmt.Sprintf("/v1/profiles/%s/libraries/%s/%s", profileSlug, owner, libSlug), nil, nil)
}

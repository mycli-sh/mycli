package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"resty.dev/v3"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/config"
)

// Version is set at build time via -ldflags. Defaults to "dev".
var Version = "dev"

// InstallMethod is set at build time via -ldflags. Tracks how the CLI was installed.
// Possible values: "source" (default), "github" (goreleaser/install script), "brew" (Homebrew).
var InstallMethod = "source"

type Client struct {
	rc         *resty.Client
	refreshMu  sync.Mutex  // serializes refresh attempts across goroutines
	refreshing atomic.Bool // true while a refresh is in flight; prevents middleware re-entry on the refresh request itself
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
			if !auth.IsAPIToken(tokens.AccessToken) {
				// Proactively refresh if token is expired or near-expiry (30s buffer).
				// Skip if a refresh is already in flight (e.g., the /v1/auth/refresh request itself).
				if !c.refreshing.Load() && !tokens.ExpiresAt.IsZero() && time.Now().After(tokens.ExpiresAt.Add(-30*time.Second)) {
					if c.tryRefresh() {
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
		DisableRetryDefaultConditions().
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
			if c.refreshing.Load() {
				return false
			}
			if c.tryRefresh() {
				return true // retry — middleware will pick up the new token
			}
			// Refresh failed — tokens are invalid, clear them so user can re-login
			_ = auth.ClearTokens()
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
	req := c.rc.R()
	if reqBody != nil {
		req.SetBody(reqBody)
	}
	var errEnv apiErrorEnvelope
	req.SetError(&errEnv)
	if out != nil {
		req.SetResult(out)
	}

	resp, err := req.Execute(method, path)
	if err != nil {
		return err
	}
	if resp.IsError() {
		if errEnv.Error.Code != "" {
			return &errEnv.Error
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), resp.String())
	}
	return nil
}

// recentRefreshDedup is how recently another goroutine must have refreshed
// for a queued caller to consider its rotation redundant. Five seconds is long
// enough to absorb the round-trip of a concurrent refresh but short enough
// that a server-side rejection (401) still forces a fresh refresh attempt.
const recentRefreshDedup = 5 * time.Second

// tryRefresh attempts to refresh the access token using the stored refresh token.
// Returns true if the refresh succeeded and tokens were updated. Safe to call
// concurrently — refresh attempts are serialized; if another caller already
// refreshed the access token within recentRefreshDedup while this one waited
// for the lock, it returns true without making a redundant network call.
func (c *Client) tryRefresh() bool {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	tokens, err := auth.LoadTokens()
	if err != nil || tokens.RefreshToken == "" {
		return false
	}

	// If another goroutine just rotated the token, the refresh token we hold
	// is already stale. Treat as success — the caller will reload tokens.
	if !tokens.LastRefreshedAt.IsZero() && time.Since(tokens.LastRefreshedAt) < recentRefreshDedup {
		return true
	}

	c.refreshing.Store(true)
	defer c.refreshing.Store(false)

	refreshResp, err := c.RefreshToken(tokens.RefreshToken)
	if err != nil || refreshResp.AccessToken == "" {
		return false
	}

	now := time.Now()
	tokens.AccessToken = refreshResp.AccessToken
	tokens.ExpiresAt = now.Add(time.Duration(refreshResp.ExpiresIn) * time.Second)
	tokens.LastRefreshedAt = now
	if refreshResp.RefreshToken != "" {
		tokens.RefreshToken = refreshResp.RefreshToken
	}
	_ = auth.SaveTokens(tokens)
	return true
}

// RefreshNow forces a refresh of the access token using the stored refresh
// token, bypassing the near-expiry check. Intended for background/periodic
// refreshes that keep the server-side session alive. Concurrent-safe; returns
// true on success.
func (c *Client) RefreshNow() bool {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	tokens, err := auth.LoadTokens()
	if err != nil || tokens.RefreshToken == "" {
		return false
	}

	c.refreshing.Store(true)
	defer c.refreshing.Store(false)

	refreshResp, err := c.RefreshToken(tokens.RefreshToken)
	if err != nil || refreshResp.AccessToken == "" {
		return false
	}

	now := time.Now()
	tokens.AccessToken = refreshResp.AccessToken
	tokens.ExpiresAt = now.Add(time.Duration(refreshResp.ExpiresIn) * time.Second)
	tokens.LastRefreshedAt = now
	if refreshResp.RefreshToken != "" {
		tokens.RefreshToken = refreshResp.RefreshToken
	}
	_ = auth.SaveTokens(tokens)
	return true
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
	req := c.rc.R()
	if etag != "" {
		req.SetHeader("If-None-Match", etag)
	}
	var catalog CatalogResponse
	var errEnv apiErrorEnvelope
	req.SetResult(&catalog).SetError(&errEnv)

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
	if resp.IsError() {
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

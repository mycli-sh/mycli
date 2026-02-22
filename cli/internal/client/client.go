package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"mycli.sh/cli/internal/auth"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (c *Client) do(method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Inject auth header if tokens available
	if tokens, err := auth.LoadTokens(); err == nil && tokens.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error APIError `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Code != "" {
			return &errResp.Error
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return nil
}

// DoRaw performs an HTTP request and returns the raw response. Used for polling/device flow.
func (c *Client) DoRaw(method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
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
	CommandID    string   `json:"command_id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      int      `json:"version"`
	SpecHash     string   `json:"spec_hash"`
	UpdatedAt    string   `json:"updated_at"`
	Library      string   `json:"library,omitempty"`
	LibraryOwner string   `json:"library_owner,omitempty"`
	Aliases      []string `json:"aliases,omitempty"`
}

type CatalogResponse struct {
	Items []CatalogItem `json:"items"`
	ETag  string        `json:"-"`
}

func (c *Client) GetCatalog(etag string) (*CatalogResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/v1/catalog", nil)
	if err != nil {
		return nil, err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if tokens, err := auth.LoadTokens(); err == nil && tokens.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, nil // no changes
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var catalog CatalogResponse
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, err
	}
	catalog.ETag = resp.Header.Get("ETag")
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

func (c *Client) CreateRelease(slug string, req *CreateReleaseRequest) (*CreateReleaseResponse, error) {
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

func (c *Client) InstallLibrary(owner, slug string) error {
	return c.do("POST", fmt.Sprintf("/v1/libraries/%s/%s/install", owner, slug), nil, nil)
}

func (c *Client) UninstallLibrary(owner, slug string) error {
	return c.do("DELETE", fmt.Sprintf("/v1/libraries/%s/%s/install", owner, slug), nil, nil)
}

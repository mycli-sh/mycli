package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
)

const (
	checkInterval = 24 * time.Hour
	checkTimeout  = 5 * time.Second
	cacheFile     = "update_check.json"
	releaseURL    = "https://api.github.com/repos/mycli-sh/mycli/releases/latest"
)

type checkResult struct {
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
}

var (
	backgroundDone sync.WaitGroup
)

// CheckInBackground spawns a goroutine that checks for a new release if the
// cached result is older than 24 hours.
func CheckInBackground() {
	backgroundDone.Add(1)
	go func() {
		defer backgroundDone.Done()
		doCheck()
	}()
}

// NotifyIfAvailable reads the cached check result and prints an upgrade notice
// to stderr if a newer version is available.
func NotifyIfAvailable() {
	backgroundDone.Wait()

	cached, err := readCache()
	if err != nil || cached.LatestVersion == "" {
		return
	}

	current := normalizeVersion(client.Version)
	latest := normalizeVersion(cached.LatestVersion)
	if current == "" || latest == "" || current == "dev" {
		return
	}

	if compareVersions(latest, current) > 0 {
		fmt.Fprintf(os.Stderr, "\nA new version of mycli is available: v%s → v%s\n", current, latest)
		switch client.InstallMethod {
		case "brew":
			fmt.Fprintln(os.Stderr, "Run 'brew upgrade mycli' to upgrade.")
		default:
			fmt.Fprintln(os.Stderr, "Run 'my cli update' to upgrade.")
		}
	}
}

func doCheck() {
	cached, _ := readCache()
	if !cached.CheckedAt.IsZero() && time.Since(cached.CheckedAt) < checkInterval {
		return
	}

	httpClient := &http.Client{Timeout: checkTimeout}
	resp, err := httpClient.Get(releaseURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil || release.TagName == "" {
		return
	}

	writeCache(checkResult{
		LatestVersion: release.TagName,
		CheckedAt:     time.Now(),
	})
}

func cachePath() string {
	return filepath.Join(config.DefaultDir(), cacheFile)
}

func readCache() (checkResult, error) {
	var result checkResult
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

func writeCache(r checkResult) {
	dir := config.DefaultDir()
	_ = os.MkdirAll(dir, 0700)
	data, err := json.Marshal(r)
	if err != nil {
		return
	}
	_ = os.WriteFile(cachePath(), data, 0600)
}

// normalizeVersion strips a leading "v" from a version string.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// compareVersions compares two dotted version strings (e.g. "0.5.0" vs "0.6.0").
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")

	maxLen := len(pa)
	if len(pb) > maxLen {
		maxLen = len(pb)
	}

	for i := 0; i < maxLen; i++ {
		var na, nb int
		if i < len(pa) {
			na = parseSegment(pa[i])
		}
		if i < len(pb) {
			nb = parseSegment(pb[i])
		}
		if na != nb {
			return na - nb
		}
	}
	return 0
}

func parseSegment(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

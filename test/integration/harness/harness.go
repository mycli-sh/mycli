//go:build integration

// Package harness provides integration test infrastructure for mycli.
//
// It spins up Postgres and the API in Docker containers via testcontainers-go,
// builds the CLI binary, and exposes helpers for seeding users, issuing API
// tokens, and shelling out to the CLI.
//
// All file-level operations require the "integration" build tag — this package
// is excluded from `go test ./...` by default.
package harness

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
)

// Harness holds the running infrastructure for a single test session.
type Harness struct {
	APIURL string
	DB     *pgxpool.Pool

	cliPath string
	cleanup []func()
	mu      sync.Mutex
}

// Start brings up Postgres + the API and returns a ready-to-use Harness.
// Containers are shared across tests in the same package via TestMain.
func Start(t *testing.T) *Harness {
	t.Helper()
	return getShared(t)
}

// CLIPath returns the path to the compiled `my` binary.
func (h *Harness) CLIPath() string { return h.cliPath }

func (h *Harness) register(fn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cleanup = append(h.cleanup, fn)
}

func (h *Harness) teardown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.cleanup) - 1; i >= 0; i-- {
		h.cleanup[i]()
	}
}

// --- shared singleton ------------------------------------------------------

var (
	sharedOnce    sync.Once
	sharedHarness *Harness
	sharedErr     error
)

func initShared() (*Harness, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	h := &Harness{}

	net, err := network.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create network: %w", err)
	}
	h.register(func() { _ = net.Remove(context.Background()) })

	pg, err := startPostgres(ctx, net)
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	h.register(func() { _ = tc.TerminateContainer(pg.container) })

	// The API container runs migrations at startup (api/cmd/api/main.go), so
	// by the time /health succeeds the schema is in place.
	api, err := startAPI(ctx, net, pg.containerDSN)
	if err != nil {
		return nil, fmt.Errorf("start api: %w", err)
	}
	h.register(func() { _ = tc.TerminateContainer(api.container) })
	h.APIURL = api.url

	if err := waitForHealth(ctx, h.APIURL); err != nil {
		return nil, fmt.Errorf("api never reported healthy: %w", err)
	}

	pool, err := pgxpool.New(ctx, pg.hostDSN)
	if err != nil {
		return nil, fmt.Errorf("connect host pgxpool: %w", err)
	}
	h.register(func() { pool.Close() })
	h.DB = pool

	cli, err := buildCLI(ctx)
	if err != nil {
		return nil, fmt.Errorf("build cli: %w", err)
	}
	h.register(func() { _ = os.Remove(cli) })
	h.cliPath = cli

	return h, nil
}

func getShared(t *testing.T) *Harness {
	t.Helper()
	sharedOnce.Do(func() {
		sharedHarness, sharedErr = initShared()
	})
	if sharedErr != nil {
		t.Fatalf("integration harness failed to start: %v", sharedErr)
	}
	return sharedHarness
}

// Shutdown tears down the shared harness. Call from TestMain after m.Run().
func Shutdown() {
	if sharedHarness != nil {
		sharedHarness.teardown()
	}
}

// waitForHealth polls /health until 200 OK or context deadline.
func waitForHealth(ctx context.Context, baseURL string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(60 * time.Second)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("timed out waiting for %s/health", baseURL)
}

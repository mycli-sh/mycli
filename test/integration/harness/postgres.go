//go:build integration

package harness

import (
	"context"
	"fmt"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	pgDB       = "mycli_test"
	pgUser     = "mycli"
	pgPassword = "mycli"
	pgImage    = "postgres:18-alpine"
	pgNetAlias = "postgres-test"
)

type pgInfo struct {
	container    tc.Container
	hostDSN      string // for tests running on the host (seeding etc.)
	containerDSN string // for the API container, reaches postgres via the docker-network alias
}

func startPostgres(ctx context.Context, net *tc.DockerNetwork) (*pgInfo, error) {
	c, err := postgres.Run(ctx, pgImage,
		postgres.WithDatabase(pgDB),
		postgres.WithUsername(pgUser),
		postgres.WithPassword(pgPassword),
		tc.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
		tcnetwork.WithNetwork([]string{pgNetAlias}, net),
	)
	if err != nil {
		return nil, fmt.Errorf("postgres.Run: %w", err)
	}

	hostDSN, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("postgres ConnectionString: %w", err)
	}

	containerDSN := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable",
		pgUser, pgPassword, pgNetAlias, pgDB)

	return &pgInfo{
		container:    c.Container,
		hostDSN:      hostDSN,
		containerDSN: containerDSN,
	}, nil
}

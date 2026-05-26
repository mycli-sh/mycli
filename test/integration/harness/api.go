//go:build integration

package harness

import (
	"context"
	"fmt"
	"os"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	apiJWTSecret    = "integration-test-secret"
	apiNetAlias     = "api-test"
	apiInternalPort = "8080/tcp"
	defaultAPIImage = "" // empty => build from docker/api.Dockerfile each run
)

type apiInfo struct {
	container tc.Container
	url       string
}

// startAPI brings up the API container attached to the shared docker network.
// When MYCLI_API_IMAGE is set the image is reused; otherwise the API is built
// from docker/api.Dockerfile rooted at the repo top level.
func startAPI(ctx context.Context, net *tc.DockerNetwork, dsn string) (*apiInfo, error) {
	req := tc.ContainerRequest{
		ExposedPorts: []string{apiInternalPort},
		Env: map[string]string{
			"DATABASE_URL": dsn,
			"JWT_SECRET":   apiJWTSecret,
			"BASE_URL":     "http://" + apiNetAlias + ":8080",
			"PORT":         "8080",
		},
		WaitingFor:     wait.ForHTTP("/health").WithPort(apiInternalPort),
		Networks:       []string{net.Name},
		NetworkAliases: map[string][]string{net.Name: {apiNetAlias}},
	}

	if image := os.Getenv("MYCLI_API_IMAGE"); image != "" {
		req.Image = image
	} else {
		root, err := repoRoot()
		if err != nil {
			return nil, err
		}
		req.FromDockerfile = tc.FromDockerfile{
			Context:    root,
			Dockerfile: "docker/api.Dockerfile",
			KeepImage:  true,
		}
	}

	c, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            false,
	})
	if err != nil {
		return nil, fmt.Errorf("start api container: %w", err)
	}

	host, err := c.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("api host: %w", err)
	}
	port, err := c.MappedPort(ctx, apiInternalPort)
	if err != nil {
		return nil, fmt.Errorf("api mapped port: %w", err)
	}

	return &apiInfo{
		container: c,
		url:       fmt.Sprintf("http://%s:%s", host, port.Port()),
	}, nil
}

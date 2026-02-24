# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

mycli (module `mycli.sh`) is a CLI tool (`my`) and API server for defining, publishing, and running shell-based command specs. Users author commands as YAML or JSON specs (validated against a JSON Schema), push them to the API, and run them. Users can also add git-backed **libraries** — repositories of command libraries that work without an account. Authentication uses a device-flow with email-verified magic links.

## Build & Dev Commands

```bash
make build-cli          # builds CLI → bin/my
make build-api          # builds API → bin/api
make test               # go test ./...
go test ./pkg/spec/...  # run tests for a single package
make lint               # golangci-lint run ./...
make dev                # builds and runs API server on :8080
make migrate            # runs DB migrations
docker compose up -d    # starts PostgreSQL (mycli:mycli@localhost:5432/mycli_dev)
```

## Architecture

Two binaries in one Go module:

- **CLI** (`cli/cmd/my/main.go`) — Cobra-based CLI. All subcommands live under `my cli` (defined in `cli/internal/commands/cli.go`). Subcommand implementations in `cli/internal/commands/`. The CLI stores config/credentials/cache under `~/.my/`.
- **API** (`api/cmd/api/main.go`) — Chi-based HTTP server. Routes defined inline in main.go. Handlers in `api/internal/handler/`, stores in `api/internal/store/` using pgx pool directly (no ORM).
- **`api/internal/email`** — `Sender` interface for sending magic-link verification emails. `ResendSender` (Resend HTTP API) for production; `LogSender` (prints to stdout) when `RESEND_API_KEY` is unset.
- **`cli/internal/shelf/`** — Shelf management: registry (shelves.json), manifest parsing (shelf.yaml), git operations, spec discovery from cloned repos.

Shared package:

- **`pkg/spec`** — `CommandSpec` types, JSON Schema validation (embedded via `//go:embed`), parsing, and hashing. The JSON Schema lives at `pkg/spec/schema/command-v1.schema.json`.

Key data flow:
1. `my cli init` scaffolds a `command.yaml` file. With a name argument it creates a subdirectory (`deploy/command.yaml`). Errors if file exists; `--force` overrides.
2. `my cli push` validates the spec via `pkg/spec`, then creates/updates the command and publishes a version through the API. `--dir` batch-pushes all spec files found in a directory tree.
3. Syncing is automatic — it happens during `my cli login` and library operations (e.g., `my library add`, `my library update`). There is no standalone sync command. The catalog is cached locally under `~/.my/cache/` with ETag support.
4. `my cli run <slug>` loads the cached spec, parses args (positional + flags), renders Go templates (`{{.args.X}}`, `{{.env.X}}`, `{{.cwd}}`, `{{.home}}`), and executes steps via shell. `my cli run -f <file>` runs directly from a spec file without push/sync.
5. `my library add <url>` clones a library repo, validates specs, and registers it. Library commands are available as `my <library> <slug>`.
6. `my cli set-api-url <url>` sets and persists a custom API URL for the CLI to use.

## API Configuration

The API reads all config from environment variables: `DATABASE_URL`, `PORT`, `JWT_SECRET`, `BASE_URL`, `RESEND_API_KEY`, `EMAIL_FROM`. See `api/internal/config/config.go` for defaults.

## Conventions

- Database IDs use native PostgreSQL UUIDs (UUIDv7 via `uuidv7()`, Go type `uuid.UUID` from `google/uuid`)
- Soft deletes on commands (`deleted_at` column)
- Auth tokens stored in OS keyring with file fallback (`~/.my/credentials.json`)
- CLI local history stored as JSONL at `~/.my/history.jsonl`
- Command slugs must match `^[a-z][a-z0-9-]*$`
- Library repos cloned under `~/.my/shelves/repos/` (path derived from URL)
- Library registry at `~/.my/shelves/shelves.json`

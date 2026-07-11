# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

mycli (module `mycli.sh`) is a CLI tool (`my`) and API server for defining, publishing, and running shell-based command specs. Users author commands as YAML or JSON specs (validated against a JSON Schema), push them to the API, and run them. Users can also add git-backed **sources** — repositories of command libraries that work without an account — or install **libraries** from the public registry. Authentication uses a device-flow with email-verified magic links, or a `myc_…` API token for non-interactive use (e.g. CI).

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
- **`cli/internal/library/`** — Source and library management: source registry (`sources.json`), manifest parsing (`mycli.yaml`), git operations, spec discovery from cloned repos.

Shared package:

- **`pkg/spec`** — `CommandSpec` types, JSON Schema validation (embedded via `//go:embed`), parsing, and hashing. The JSON Schema lives at `pkg/spec/schema/command-v1.schema.json`.

Key data flow:
1. `my cli init` scaffolds a `command.yaml` file. With a name argument it creates a subdirectory (`deploy/command.yaml`). Errors if file exists; `--force` overrides.
2. `my cli push` validates the spec via `pkg/spec`, then creates/updates the command and publishes a version through the API. `--dir` batch-pushes all spec files found in a directory tree.
3. Every user always has a `default` profile (created in migration 003 for existing users; created transactionally on signup for new users). The "active profile" is a CLI-local concept stored in `~/.my/config.json`; `cli/internal/config.GetActiveProfile()` falls back to `"default"` when unset and is overridable via `MY_PROFILE`. The `default` profile slug is immutable and cannot be deleted server-side.
4. Syncing pulls the catalog for the active profile from the API and caches it under `~/.my/cache/profiles/<slug>/catalog.json`; specs are content-addressable at `~/.my/cache/specs/<commandID>/<version>.json` and shared across profiles. Sync is automatic during `my cli login` (default profile) and `my library install`/`uninstall` (target profile); explicit refresh is `my library sync [--profile <slug>] [--all]`. The catalog endpoint supports ETag / `If-None-Match`. A legacy `~/.my/cache/catalog.json` is migrated to the default profile slot on first run.
5. `my cli run <slug>` loads the spec from the active profile's cache, parses args (positional + flags), renders Go templates (`{{.args.X}}`, `{{.env.X}}`, `{{.cwd}}`, `{{.home}}`), and executes steps via shell. `my cli run -f <file>` runs directly from a spec file without push/sync.
6. `my source add <url>` clones a source repo, validates specs, and registers it. `my library install <name>` adds the library to the **active profile** on the server (via `POST /v1/profiles/{slug}/libraries`) and syncs that profile's cache. There is no standalone library-install endpoint — all library mutations go through profiles. Library commands are available as `my <library> <slug>`.
7. `my cli token create <name>` mints an API token (`myc_<40 hex>`, SHA-256-hashed at rest, max 10 per user, name ≤ 100 chars). Setting `MY_API_TOKEN=myc_…` makes the CLI skip JWT refresh entirely and use the token as a Bearer credential. Token-management routes (`/v1/tokens/*`) are JWT-only — API tokens can't manage other tokens.
8. `my cli set-api-url <url>` sets and persists a custom API URL for the CLI to use.

## API Configuration

The API reads all config from environment variables: `DATABASE_URL`, `PORT`, `JWT_SECRET`, `BASE_URL`, `RESEND_API_KEY`, `EMAIL_FROM`, `ALLOWED_ORIGINS`, `WEB_BASE_URL`, `SYSTEM_ADMIN_EMAILS`. See `api/internal/config/config.go` for defaults.

## Conventions

- **Use `bun` as the JavaScript package manager.** Do not use npm or yarn. Use `bun install`, `bun run`, etc.
- Database IDs use native PostgreSQL UUIDs (UUIDv7 via `uuidv7()`, Go type `uuid.UUID` from `google/uuid`)
- Soft deletes on commands (`deleted_at` column)
- JWT credentials stored in OS keyring with file fallback (`~/.my/credentials.json`); API tokens are read from `MY_API_TOKEN`
- JWT sessions use a sliding 60-day window: `/v1/auth/refresh` rotates the refresh token AND bumps `sessions.expires_at` to `now() + 60d`. Refresh tokens are single-use; the immediately-previous token stays valid for a short reuse grace (`authservice.RefreshTokenGrace`) so a concurrent/duplicate refresh isn't rejected. Refresh across all CLI clients in a process is serialized by a process-global lock (`cli/internal/client`), and the reactive 401 handler only clears credentials on a definitive auth rejection. The CLI fires a silent background refresh from root `PersistentPreRun` (`cli/internal/auth/background.go`) when `LastRefreshedAt` is older than 7 days, with a 30-min post-login grace; API tokens skip this path entirely
- CLI local history stored as JSONL at `~/.my/history.jsonl`
- Command slugs must match `^[a-z][a-z0-9-]*$` (also enforced on profile slugs and the cache directory layout)
- Source repos cloned under `~/.my/sources/repos/` (path derived from URL)
- Source registry at `~/.my/sources/sources.json`
- Per-profile catalog at `~/.my/cache/profiles/<slug>/catalog.json`; shared spec cache at `~/.my/cache/specs/<commandID>/<version>.json`
- Authenticated request bodies are capped at 256 KiB by default, 4 MiB on `POST /v1/libraries/{slug}/releases` (`api/internal/middleware/bodylimit.go`). CLI pre-validates release payloads against the same limit (`cli/internal/client/client.MaxReleaseBodyBytes`) — keep the two in sync
- TUI components use **Bubble Tea v2** (`charm.land/bubbletea/v2`) and **Lipgloss v2** (`charm.land/lipgloss/v2`). Key patterns live in `cli/internal/commands/explore.go` (library explorer) and `cli/internal/commands/otpui.go` (OTP verification). Reuse existing styles and color vars from explore.go when building new TUI components.

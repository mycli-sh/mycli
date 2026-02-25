# mycli

A CLI tool and API server for defining, publishing, and running shell-based command specs. Author commands as YAML or JSON specs, push them to a server, sync them locally, and run them from anywhere.

## Quick Start

**Prerequisites:** Go 1.25+, Docker (for PostgreSQL)

```bash
# Start the database
docker compose up -d

# Build
make build-cli
make build-api

# Run the API server (dev mode)
make dev

# Log in and create your first command
./bin/my cli login
./bin/my cli init hello
# edit command.yaml
./bin/my cli push
./bin/my cli run hello
# or run directly from a file
./bin/my cli run -f command.yaml
```

**Or use git-backed sources without an account:**

```bash
make build-cli
./bin/my source add https://github.com/user/example-library.git
./bin/my <library> <command>
```

## CLI Reference

All management subcommands live under `cli`:

### Authentication

| Command | Description |
|---------|-------------|
| `cli login` | Log in via email magic link (auto-syncs after login) |
| `cli logout` | Clear stored credentials |
| `cli whoami` | Show current user info (ID, email) |

### Commands

| Command | Description |
|---------|-------------|
| `cli init [name]` | Scaffold a `command.yaml` file (with name: creates subdirectory) |
| `cli push` | Validate and publish a command spec to the server |
| `cli show <slug>` | Show details of a cached command |
| `cli run [-f file \| slug] [args...]` | Run a cached command (or directly from a spec file with `-f`) |
| `cli status` | Show API URL, login state, last sync time, cached command count |
| `cli history` | Show run history |

### Sources (git-backed)

| Command | Description |
|---------|-------------|
| `source add <git-url>` | Clone a git source repository |
| `source remove <name>` | Remove a git source |
| `source list` | List installed git sources |
| `source update [name]` | Update all or a specific git source (git pull) |

### Libraries

| Command | Description |
|---------|-------------|
| `library install <name>` | Install a library from the registry |
| `library uninstall <name>` | Remove a registry-installed library |
| `library list` | List all libraries (from any source) |
| `library search <query>` | Search public libraries |
| `library explore` | Interactive TUI for browsing libraries |
| `library info <identifier>` | Show library details |
| `library release <tag>` | Create a versioned release from a git tag |

Installed library commands are available as top-level subcommands: `my <library> <command>`.

### Flags

**Global flags:**
- `--api-url` — Override the API server URL
- `--config` — Override the config file path

**Command-specific flags:**

| Command | Flag | Description |
|---------|------|-------------|
| `cli push` | `-f, --file` | Spec file to push (default: `command.yaml`, auto-detects) |
| `cli push` | `-m, --message` | Version message |
| `cli push` | `--dir` | Push all spec files found in directory tree |
| `cli show` | `--raw` | Output raw JSON spec |
| `cli run` | `-y, --yes` | Skip confirmation prompts |
| `cli run` | `-f, --file` | Run directly from a spec file |
| `cli history` | `-n, --last` | Number of entries to show (default: 20) |
| `cli init` | `--force` | Overwrite existing spec file |
| `source add` | `--ref` | Git branch or tag to checkout |
| `source add` | `--name` | Alias for the source (defaults to repo name) |
| `source list` | `--json` | Output as JSON |
| `library list` | `--json` | Output as JSON |

## Command Spec Format

Commands are defined as YAML or JSON files validated against a [JSON Schema](pkg/spec/schema/command-v1.schema.json).

```json
{
  "schemaVersion": 1,
  "kind": "command",
  "metadata": {
    "name": "greet",
    "slug": "greet",
    "description": "Greet someone",
    "tags": ["example"],
    "aliases": ["hi"]
  },
  "dependencies": ["curl"],
  "args": {
    "positional": [
      { "name": "name", "description": "Who to greet", "required": true }
    ],
    "flags": [
      { "name": "loud", "short": "l", "type": "bool", "description": "Shout it" }
    ]
  },
  "defaults": {
    "shell": "/bin/bash",
    "timeout": "30s",
    "env": { "GREETING": "Hello" }
  },
  "steps": [
    {
      "name": "greet",
      "run": "echo '{{.env.GREETING}}, {{.args.name}}!'"
    }
  ],
  "policy": {
    "requireConfirmation": true,
    "allowedExecutables": ["/bin/echo"]
  }
}
```

### Top-Level Fields

| Field | Required | Description |
|-------|----------|-------------|
| `schemaVersion` | yes | Must be `1` |
| `kind` | yes | Must be `"command"` |
| `metadata` | yes | `name`, `slug` (required); `description`, `tags`, `aliases` (optional). Slugs must match `^[a-z][a-z0-9-]*$` |
| `dependencies` | no | Array of required binary dependencies (e.g., `["curl", "jq"]`). Checked before execution |
| `args` | no | `positional` (ordered list) and `flags` (named options with optional `short`, `type`, `default`) |
| `defaults` | no | Default `shell`, `timeout`, and `env` vars applied to all steps |
| `steps` | yes | Array of steps (min 1). Each has `name`, `run` (required); `env`, `timeout`, `shell`, `continueOnError` (optional) |
| `policy` | no | `requireConfirmation` prompts before running; `allowedExecutables` restricts which binaries can be invoked |

### Flag Types

Flags support `string` (default), `bool`, and `int` types.

### Template Variables

Step `run` lines and `env` values are rendered as Go templates:

| Variable | Description |
|----------|-------------|
| `{{.args.X}}` | Argument value (positional or flag) by name |
| `{{.env.X}}` | Environment variable value |
| `{{.cwd}}` | Current working directory |
| `{{.home}}` | User's home directory |

## Source Repos

A **source repo** is a git repository that provides one or more command **libraries**. Source repos let you share and use curated sets of commands without needing an account or API server — just a git URL.

### How It Works

1. `my source add <url>` clones the repo and reads its manifest (`mycli.yaml`)
2. Each library in the manifest maps to a directory of command spec files
3. Commands become available as `my <library> <slug>` (e.g., `my ops deploy`)
4. `my source update` pulls the latest changes from all source repos

### Manifest Format (`mycli.yaml`)

Every source repo must have a `mycli.yaml` at its root (`mycli.yml`, `mycli.json`, and the older `shelf.yaml`/`shelf.yml`/`shelf.json` are also accepted):

```yaml
schemaVersion: 1
name: my-library
description: A collection of useful commands
libraries:
  ops:
    name: Operations
    description: Deployment and operations commands
    path: ops
  k8s:
    name: Kubernetes
    description: Kubernetes helper commands
    path: k8s
```

- `schemaVersion` must be `1`
- Each key in `libraries` is the library slug (must match `^[a-z][a-z0-9-]*$`)
- `path` points to a directory containing command spec files
- Each `.yaml`, `.yml`, or `.json` file in a library directory must be a valid command spec whose `metadata.slug` matches the filename (minus extension)

### Directory Layout

```
my-library/
  mycli.yaml            # Manifest
  ops/                  # Library directory
    deploy.yaml         # Command spec (slug: "deploy")
    status.yaml         # Command spec (slug: "status")
  k8s/
    logs.yaml           # Command spec (slug: "logs")
```

### Storage

- Source repos are cloned to `~/.my/sources/repos/` (path derived from URL)
- The source registry lives at `~/.my/sources/sources.json`

See [`examples/library/`](examples/library/) for a complete working example.

## API Server

### Setup

```bash
docker compose up -d          # PostgreSQL on localhost:5432
make migrate                  # Run database migrations
make dev                      # Build and start on :8080
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://mycli:mycli@localhost:5432/mycli_dev?sslmode=disable` | PostgreSQL connection string |
| `PORT` | `8080` | Server listen port |
| `JWT_SECRET` | `dev-secret-change-me` | Secret for signing JWTs |
| `BASE_URL` | `http://localhost:8080` | Public base URL (used in emails) |
| `RESEND_API_KEY` | _(empty)_ | Resend API key for sending emails. When unset, emails are printed to stdout |
| `EMAIL_FROM` | `mycli@updates.mycli.sh` | Sender address for emails |

### Endpoints

**Public (no auth):**

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/auth/device/start` | Start device auth flow |
| `POST` | `/v1/auth/device/token` | Poll for device token |
| `POST` | `/v1/auth/device/resend` | Resend OTP |
| `POST` | `/v1/auth/verify-code` | Verify OTP code |
| `POST` | `/v1/auth/refresh` | Refresh access token |
| `GET` | `/v1/auth/verify` | Verify magic link |
| `POST` | `/v1/auth/web/login` | Start web auth flow |
| `POST` | `/v1/auth/web/verify` | Verify web auth |
| `GET` | `/v1/usernames/{username}/available` | Check username availability |
| `GET` | `/device` | Device verification page |
| `POST` | `/device` | Device verification submit |
| `GET` | `/health` | Health check |

**Library browsing (optional auth):**

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/libraries` | Search libraries |
| `GET` | `/v1/libraries/{owner}/{slug}` | Library detail |
| `GET` | `/v1/libraries/{owner}/{slug}/releases` | List releases |
| `GET` | `/v1/libraries/{owner}/{slug}/releases/{version}` | Get a release |
| `GET` | `/v1/libraries/{owner}/{slug}/commands/{commandSlug}` | Get a command |
| `GET` | `/v1/libraries/{owner}/{slug}/commands/{commandSlug}/versions` | List command versions |

**Authenticated (no username required):**

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/me` | Current user info |
| `PATCH` | `/v1/me/username` | Set username |
| `GET` | `/v1/sessions` | List sessions |
| `DELETE` | `/v1/sessions/{id}` | Revoke a session |
| `DELETE` | `/v1/sessions` | Revoke all sessions |

**Authenticated (username required):**

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/me/sync-summary` | Sync summary |
| `POST` | `/v1/commands` | Create a command |
| `GET` | `/v1/commands` | List commands |
| `GET` | `/v1/commands/{id}` | Get a command |
| `DELETE` | `/v1/commands/{id}` | Delete a command (soft delete) |
| `POST` | `/v1/commands/{id}/versions` | Publish a version |
| `GET` | `/v1/commands/{id}/versions/{version}` | Get a specific version |
| `GET` | `/v1/catalog` | Get synced catalog (supports ETag) |
| `POST` | `/v1/libraries/{slug}/releases` | Create a release |
| `POST` | `/v1/libraries/{owner}/{slug}/install` | Install a library |
| `DELETE` | `/v1/libraries/{owner}/{slug}/install` | Uninstall a library |

## Development

```bash
make build-cli          # Build CLI to bin/my
make build-api          # Build API to bin/api
make build-web          # Build web frontend
make build-site         # Build marketing site
make build-docs         # Build documentation site
make test               # go test ./...
make lint               # golangci-lint run ./...
make dev                # Build and run API on :8080
make dev-web            # Run web frontend dev server
make dev-site           # Run marketing site dev server
make dev-docs           # Run docs dev server
make migrate            # Run DB migrations
make sqlc-generate      # Regenerate sqlc query code
make dev-reset          # Reset dev environment
```

Run tests for a single package:

```bash
go test ./pkg/spec/...
```

### Database

```bash
docker compose up -d   # PostgreSQL 18 (mycli:mycli@localhost:5432/mycli_dev)
docker compose down    # Stop
```

## Architecture

Two binaries sharing one Go module (`mycli.sh`):

```
cli/cmd/my/main.go          CLI entry point (Cobra)
cli/internal/commands/       Subcommand implementations
cli/internal/engine/         Template rendering + shell execution
cli/internal/cache/          Local spec cache (~/.my/cache/)
cli/internal/auth/           Token storage (OS keyring + file fallback)
cli/internal/history/        Run history (JSONL at ~/.my/history.jsonl)
cli/internal/library/        Library management: registry, git ops, manifest parsing, spec discovery
cli/internal/client/         API client
cli/internal/config/         Config loading
cli/internal/termui/         Terminal UI helpers (Bubble Tea)

api/cmd/api/main.go          API entry point (Chi)
api/internal/handler/         HTTP handlers
api/internal/store/           PostgreSQL stores (pgx, no ORM)
api/internal/middleware/       Auth + rate limiting
api/internal/config/          Env-based configuration
api/internal/database/        Connection + migrations
api/internal/email/           Email sender (Resend / dev log)

pkg/spec/                    Shared: CommandSpec types, JSON Schema validation, parsing, hashing
```

Config and credentials are stored under `~/.my/`. Database IDs use prefixed UUIDs (`usr_`, `cmd_`, `cv_`, `ml_`, `lib_`).

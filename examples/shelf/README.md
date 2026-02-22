# Example Shelf

This directory is an example **shelf** — a git-backed repository of command libraries for `my`.

## Structure

```
shelf.yaml          # Shelf manifest (required at repo root)
ops/                # "ops" library
  deploy.yaml       # my ops deploy
  status.yaml       # my ops status
k8s/                # "k8s" library
  logs.yaml         # my k8s logs
```

## Manifest

The `shelf.yaml` file defines the shelf and its libraries:

```yaml
shelfVersion: 1
name: example-shelf
description: An example shelf with ops and k8s command libraries
libraries:
  ops:
    name: Operations
    description: Deployment and operations commands
    path: ops
```

Each library key (e.g., `ops`) becomes a top-level subcommand group in the CLI. The `path` field points to a directory containing command spec files.

The manifest can also be named `shelf.yml` or `shelf.json` for backward compatibility.

## Command Specs

Each `.yaml`, `.yml`, or `.json` file in a library directory must be a valid [command spec](../../pkg/spec/schema/command-v1.schema.json). The filename (minus extension) must match the spec's `metadata.slug`.

## Usage

To use this shelf with `my`:

```bash
# If this were a git repo, you'd add it with:
my shelf add https://github.com/user/example-shelf.git

# Then run commands:
my ops deploy my-service --env production
my ops status my-service
my k8s logs my-pod -n kube-system
```

## Creating Your Own Shelf

1. Create a git repository
2. Add a `shelf.yaml` manifest at the root
3. Create directories for each library
4. Add command spec YAML files (one per command, filename = slug)
5. Push to a git host
6. Share the URL — users add it with `my shelf add <url>`

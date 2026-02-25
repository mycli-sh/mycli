# Example Library

This directory is an example **source** — a git-backed repository of command libraries for `my`.

## Structure

```
mycli.yaml          # Library manifest (required at repo root)
ops/                # "ops" library
  deploy.yaml       # my ops deploy
  status.yaml       # my ops status
k8s/                # "k8s" library
  logs.yaml         # my k8s logs
```

## Manifest

The `mycli.yaml` file defines the source and its libraries:

```yaml
schemaVersion: 1
name: example-library
description: An example library with ops and k8s command libraries
libraries:
  ops:
    name: Operations
    description: Deployment and operations commands
    path: ops
```

Each library key (e.g., `ops`) becomes a top-level subcommand group in the CLI. The `path` field points to a directory containing command spec files.

The manifest can also be named `mycli.yml`, `mycli.json`, or `shelf.yaml`/`shelf.yml`/`shelf.json` for backward compatibility.

## Command Specs

Each `.yaml`, `.yml`, or `.json` file in a library directory must be a valid [command spec](../../pkg/spec/schema/command-v1.schema.json). The filename (minus extension) must match the spec's `metadata.slug`.

## Usage

To use this source with `my`:

```bash
# If this were a git repo, you'd add it with:
my source add https://github.com/user/example-library.git

# Then run commands:
my ops deploy my-service --env production
my ops status my-service
my k8s logs my-pod -n kube-system
```

## Creating Your Own Source

1. Create a git repository
2. Add a `mycli.yaml` manifest at the root
3. Create directories for each library
4. Add command spec YAML files (one per command, filename = slug)
5. Push to a git host
6. Share the URL — users add it with `my source add <url>`

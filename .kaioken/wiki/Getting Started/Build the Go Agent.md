# Build the Go Agent

This chapter covers downloading Go module dependencies, building the `airfoil` binary from source, and verifying the CLI works via its help commands. The agent lives in the `agent/` directory and is a standalone Go module (`github.com/papitsho/airfoil`).

## Table of Contents

- [Prerequisites](#prerequisites)
- [Module Overview](#module-overview)
- [Download Dependencies](#download-dependencies)
- [Build the Binary](#build-the-binary)
- [Verify the CLI](#verify-the-cli)
- [Project Layout](#project-layout)
- [Referenced Files](#referenced-files)

---

## Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Go | **1.26.5** (or compatible) | Declared in `go.mod`; the toolchain directive pins this version. |
| Git | Any recent | Required for `go mod download` to fetch VCS dependencies. |

The repository uses Go modules (no `vendor/` directory). All dependencies are declared in `agent/go.mod`.

---

## Module Overview

The agent module is defined at `agent/go.mod`. It declares three direct dependencies and several indirect ones pulled in by Cobra and gofeed.

`agent/go.mod:1-17`

```go
module github.com/papitsho/airfoil

go 1.26.5

require (
	github.com/mmcdole/gofeed v1.4.1
	github.com/spf13/cobra v1.10.2
	golang.org/x/sync v0.22.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mmcdole/goxpp/v2 v2.0.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)
```

**Key points:**
- **Module path**: `github.com/papitsho/airfoil` — import this path in any Go code that consumes internal packages.
- **Direct deps**:
  - `gofeed` — RSS/Atom parsing for ingestion adapters.
  - `cobra` — CLI framework (commands: `run`, `version`, `doctor`).
  - `golang.org/x/sync` — Concurrency primitives (e.g., `errgroup` for parallel fetches).
- **Go version**: `1.26.5` — the `go` directive also acts as a toolchain hint; `go build` will auto-install this version if not present.

---

## Download Dependencies

From the `agent/` directory, run:

```bash
cd agent
go mod download
```

This fetches all modules listed in `go.mod` and `go.sum` into the module cache (`$GOPATH/pkg/mod` or `~/go/pkg/mod`). It also verifies checksums against `go.sum`.

**Expected output** (truncated):

```
go: downloading github.com/spf13/cobra v1.10.2
go: downloading github.com/mmcdole/gofeed v1.4.1
go: downloading golang.org/x/sync v0.22.0
...
```

If you add or upgrade dependencies later, run `go mod tidy` to synchronize `go.mod` and `go.sum`.

---

## Build the Binary

The main package is `cmd/airfoil`. Build it with:

```bash
cd agent
go build -o airfoil ./cmd/airfoil
```

- `-o airfoil` — writes the binary as `./airfoil` in the current directory (`agent/`).
- `./cmd/airfoil` — the main package path (contains `main.go`).

**Alternative**: install into `$GOBIN`/`$GOPATH/bin`:

```bash
go install ./cmd/airfoil
```

This places `airfoil` on your `PATH` (assuming `$GOBIN` is in `PATH`).

### Build Tags / Flags

No build tags are required. The module uses only standard library and declared dependencies. For a reproducible build with version metadata, you can inject `ldflags` (the `version` command reads these):

```bash
go build -ldflags "-X github.com/papitsho/airfoil/internal/config.Version=dev -X github.com/papitsho/airfoil/internal/config.Commit=$(git rev-parse --short HEAD)" -o airfoil ./cmd/airfoil
```

> The `internal/config` package (not shown in the provided source) defines `Version` and `Commit` variables populated at link time. The `version` subcommand prints them.

---

## Verify the CLI

Run the binary with no arguments or with `--help` to see the top-level usage:

```bash
./airfoil --help
```

**Expected output** (abridged):

```
Command airfoil runs the Airfoil pipeline: ingest, normalize, embed, cluster,
score, summarize, write, publish.

Usage:
  airfoil [command]

Available Commands:
  doctor      Check environment and configuration
  run         Run the full pipeline once
  version     Print version information

Flags:
  -h, --help   help for airfoil

Use "airfoil [command] --help" for more information about a command.
```

### Subcommand Help

| Command | Purpose | Verify with |
|---------|---------|-------------|
| `run` | Execute the full ingestion → summarization → publish pipeline | `./airfoil run --help` |
| `doctor` | Validate config, env vars, connectivity, and write permissions | `./airfoil doctor --help` |
| `version` | Print version, commit, and build metadata | `./airfoil version` |

Example — `doctor` help:

```bash
./airfoil doctor --help
```

```
Check environment, configuration, and connectivity before running the pipeline.

Usage:
  airfoil doctor [flags]

Flags:
  -h, --help   help for doctor
```

Example — `version`:

```bash
./airfoil version
```

```
airfoil version dev (commit abc1234)
```

> If `version` prints `dev` and no commit, the binary was built without `ldflags` (see [Build the Binary](#build-the-binary)).

---

## Project Layout

The `agent/` directory follows the standard Go project layout with `internal/` for private packages and `cmd/` for entry points.

```
agent/
├── go.mod
├── go.sum
├── cmd/
│   └── airfoil/
│       ├── main.go          # Thin entry: newRootCmd().Execute()
│       └── root.go          # Cobra root command + subcommands (run, version, doctor)
├── internal/
│   ├── config/              # Config loading, validation, derived paths
│   ├── ingest/              # Adapter interface, Runner, hnAdapter, rssAdapter
│   ├── normalize/           # Build, Dedupe, CanonicalURL, Excerpt (≤300 chars)
│   ├── store/               # ReadJSON, WriteJSON (atomic)
│   └── model/               # Item, Cluster, Story, Index, State, Tier constants
└── config/                  # JSON configs (sources.json, scoring.json, keywords.json)
```

**Entry point** (`cmd/airfoil/main.go`) is intentionally minimal:

`agent/cmd/airfoil/main.go:12-17`

```go
func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "airfoil:", err)
		os.Exit(1)
	}
```

All CLI logic (command definitions, flags, `run`/`doctor`/`version` implementations) lives in `root.go` and the `internal/` packages. This keeps `main.go` stable and testable.

---

## Referenced Files

| File | Lines | Purpose |
|------|-------|---------|
| `agent/go.mod` | 1–17 | Module declaration, Go version, direct/indirect dependencies |
| `agent/cmd/airfoil/main.go` | 12–17 | Thin main entry point; delegates to Cobra root command |

<!-- kaioken:files agent/go.mod,agent/cmd/airfoil/main.go -->

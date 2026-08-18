# Prerequisites & Toolchain Versions

This chapter lists the required toolchain versions and system dependencies needed before setting up the Airfoil development environment. It covers the Go agent (`agent/`) and the React + Vite site (`site/`), plus optional local services for embeddings and LLM fallback.

## Table of Contents

- [Go Toolchain](#go-toolchain)
- [Node.js Toolchain](#nodejs-toolchain)
- [System Dependencies](#system-dependencies)
- [Optional Local Services](#optional-local-services)
- [Version Verification](#version-verification)

---

## Go Toolchain

The Go agent is built with **Go 1.26.5** as declared in `agent/go.mod`.

`agent/go.mod:1-7`

```
module github.com/papitsho/airfoil

go 1.26.5
```

### Direct Dependencies (from `go.mod`)

| Module | Version | Purpose |
|--------|---------|---------|
| `github.com/mmcdole/gofeed` | v1.4.1 | RSS/Atom feed parsing for ingestion adapters |
| `github.com/spf13/cobra` | v1.10.2 | CLI framework (`cmd/airfoil`) |
| `golang.org/x/sync` | v0.22.0 | Concurrency primitives (errgroup, semaphore) |

### Indirect Dependencies (notable)

| Module | Version | Notes |
|--------|---------|-------|
| `golang.org/x/net` | v0.57.0 | HTTP/2, DNS, networking internals |
| `golang.org/x/text` | v0.40.0 | Text processing, encoding, Unicode |
| `github.com/spf13/pflag` | v1.0.9 | POSIX/GNU-style flags (Cobra dependency) |

### Go Version Policy

- **Minimum**: Go 1.26.5 (as pinned in `go.mod`)
- **Recommended**: Latest Go 1.26.x patch release
- **Not supported**: Go 1.25 or earlier (uses features from 1.26)

> **Note**: The `go` directive in `go.mod` controls the minimum version accepted by the toolchain. CI and local builds should use 1.26.5 or a compatible newer patch.

---

## Node.js Toolchain

The site (`site/`) uses **React 19**, **Vite 8**, and **TypeScript 6**, which require a modern Node.js runtime.

`site/package.json:1-35`

```json
{
  "name": "site",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "lint": "oxlint",
    "preview": "vite preview"
  },
  "dependencies": {
    "@fontsource/inter": "^5.3.0",
    "@fontsource/jetbrains-mono": "^5.3.0",
    "react": "^19.2.8",
    "react-dom": "^19.2.8",
    "react-router-dom": "^7.18.2"
  },
  "devDependencies": {
    "@types/node": "^24.13.3",
    "@types/react": "^19.2.17",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^6.0.4",
    "oxlint": "^1.75.0",
    "typescript": "~6.0.2",
    "vite": "^8.2.0"
  }
}
```

### Required Node.js Version

| Tool | Version Constraint | Inferred Minimum Node.js |
|------|-------------------|--------------------------|
| React 19 | `^19.2.8` | Node 18.17+ (officially 18.17+ for React 19) |
| Vite 8 | `^8.2.0` | Node 20.12+ (Vite 8 drops Node 18) |
| TypeScript 6 | `~6.0.2` | Node 18+ |
| Oxlint | `^1.75.0` | Node 18+ |

**Effective minimum: Node.js 20.12+** (driven by Vite 8).  
**Recommended: Node.js 22 LTS (Jod)** or latest Node.js 20 LTS (Iron).

### Package Manager

- **npm** (bundled with Node.js) — used by `package.json` scripts
- **pnpm** or **yarn** also work but are not required; the repo uses plain `npm` in CI

### Key Dev Dependencies

| Package | Version | Role |
|---------|---------|------|
| `typescript` | `~6.0.2` | Type checking (`tsc -b`) |
| `vite` | `^8.2.0` | Dev server + production bundler |
| `@vitejs/plugin-react` | `^6.0.4` | React Fast Refresh, JSX transform |
| `oxlint` | `^1.75.0` | Fast linting (replaces ESLint) |
| `@types/node` | `^24.13.3` | Node.js type definitions |
| `@types/react` | `^19.2.17` | React 19 type definitions |
| `@types/react-dom` | `^19.2.3` | React DOM type definitions |

---

## System Dependencies

These must be installed on the host machine (or in the CI runner) before running any build or development commands.

| Dependency | Minimum Version | Purpose | Install Hint |
|------------|----------------|---------|--------------|
| **Git** | 2.30+ | Version control; agent commits/pushes generated content | `apt install git` / `brew install git` / winget |
| **Go** | 1.26.5 | Build the `airfoil` CLI agent | [go.dev/dl](https://go.dev/dl/) |
| **Node.js** | 20.12+ | Run Vite dev server, build site, type-check | [nodejs.org](https://nodejs.org/) / nvm / fnm / volta |
| **npm** | 10+ (bundled) | Install site dependencies | Comes with Node.js |

### Platform-Specific Notes

| Platform | Notes |
|----------|-------|
| **Linux** | Install `build-essential` (or equivalent) for any native Go modules (none currently, but good practice). |
| **macOS** | Xcode Command Line Tools required: `xcode-select --install`. |
| **Windows** | Use WSL2 for best compatibility; native Windows works but Git line endings (`core.autocrlf`) must be configured. |

---

## Optional Local Services

These are **not required** for basic development (the agent falls back gracefully), but enable full local pipeline execution without external API keys.

### Ollama (Local Embeddings + LLM Fallback)

| Detail | Value |
|--------|-------|
| **Purpose** | Default embedder (`AIRFOIL_EMBEDDER=ollama`) and final LLM fallback in summarization chain |
| **Minimum Version** | 0.3.x (any recent release) |
| **Models to Pull** | `nomic-embed-text` (embeddings), `llama3.1:8b` or `qwen2.5:7b` (summarization) |
| **Install** | `curl -fsSL https://ollama.com/install.sh \| sh` |
| **Verify** | `ollama list` shows pulled models |

> The agent reads `AIRFOIL_EMBEDDER=ollama` and `OLLAMA_EMBED_MODEL` / `OLLAMA_LLM_MODEL` env vars. See [Configuration System](../configuration-system/) for details.

### GitHub CLI (`gh`) — Optional

| Detail | Value |
|--------|-------|
| **Purpose** | Authenticate `git push` in CI / local testing; manage secrets |
| **Install** | `brew install gh` / `winget install GitHub.cli` / [github.com/cli/cli](https://github.com/cli/cli) |

---

## Version Verification

Run these commands to confirm your toolchain matches the requirements:

```bash
# Go
go version
# Expected: go version go1.26.5 ...

# Node.js + npm
node --version
# Expected: v20.12.0 or higher (v22.x recommended)
npm --version
# Expected: 10.x or higher

# Git
git --version
# Expected: 2.30+

# Optional: Ollama
ollama --version
# Expected: 0.3.x or higher
ollama list
# Should show nomic-embed-text and a chat model (llama3.1:8b, qwen2.5:7b, etc.)
```

### Quick Environment Check Script

Save as `check-env.sh` and run `bash check-env.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "=== Toolchain Versions ==="
printf "Go:      "; go version | awk '{print $3}'
printf "Node:    "; node --version
printf "npm:     "; npm --version
printf "Git:     "; git --version | awk '{print $3}'

echo ""
echo "=== Go Module Check ==="
cd agent && go mod verify && echo "go.mod checksums OK"

echo ""
echo "=== Site Dependencies ==="
cd ../site && npm ci --prefer-offline --no-audit 2>/dev/null && echo "npm ci OK"

echo ""
echo "=== Optional: Ollama ==="
if command -v ollama >/dev/null 2>&1; then
  ollama --version
  ollama list | grep -E 'nomic-embed-text|llama3|qwen' || echo "  (models not pulled yet)"
else
  echo "  Ollama not installed (optional)"
fi
```

---

## Referenced Files

- `agent/go.mod` — Go module declaration, version, and dependencies
- `site/package.json` — Node.js project manifest, scripts, dependencies, devDependencies

---

*End of Prerequisites & Toolchain Versions. Next: [Environment Variables & Configuration](../configuration-system/).*

<!-- kaioken:files agent/go.mod,site/package.json -->

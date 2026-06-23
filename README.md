# skillpack

An MCP server that serves [Agent Skills](https://agentskills.io) (agentskills.io format) and parameterized commands from embedded Markdown trees — giving any MCP-compatible agent progressive-disclosure access to domain knowledge.

[![CI](https://github.com/marstid/skillpack/actions/workflows/build-and-push-image.yaml/badge.svg)](https://github.com/marstid/skillpack/actions/workflows/build-and-push-image.yaml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## What is skillpack

A **skill** is a self-contained Markdown guide that teaches an agent how to use a related cluster of tools for one product area — the right attributes, query syntax, time formats, and common pitfalls. Skills are *content* delivered just-in-time, so the model only pays the token cost for the guidance it actually needs. A skill is **not code** — it's a `SKILL.md` file with YAML frontmatter plus optional bundled resources (`references/`, `scripts/`, `assets/`).

skillpack delivers skills through progressive disclosure, loading the cheapest signal first and escalating only when the task warrants it:

| Tier | Surface | Typical tokens |
|------|---------|---------------|
| 0 | `list_skills` — catalog (name + description) | ~50–100 / skill |
| 1 | `activate_skill` with `header_only=true` — frontmatter + resource manifest, no body | small preview |
| 2 | `activate_skill` — full body wrapped in `<skill_content>` | the actual guide |
| 3 | `skill://<name>/<path>` resources — bundled files on demand | deep-dive references |

A **command** is a parameterized prompt template (`COMMAND.md`) that maps 1:1 to an [MCP prompt](https://modelcontextprotocol.io/docs/concepts/prompts). Clients render them as slash commands with argument inputs.

skillpack ships as a single static Go binary with the example `skills/` and `commands/` trees embedded via `//go:embed`. You can serve the embedded trees, override them with on-disk directories at runtime, or build a custom image from your own skills repo (see [Use case: Custom skills image](#use-case-custom-skills-image-via-your-own-skills-repo)).

---

## Quick start

Run the prebuilt image and connect an MCP client over HTTP:

```sh
podman run --rm -p 8080:8080 ghcr.io/marstid/skillpack:latest
# Server now listening at http://localhost:8080/mcp
```

Or with Docker:

```sh
docker run --rm -p 8080:8080 ghcr.io/marstid/skillpack:latest
```

Build from source instead:

```sh
git clone https://github.com/marstid/skillpack.git
cd skillpack
make run   # builds ./skillpack and runs it on :8080
```

---

## Configuring an MCP client

### Claude Desktop (stdio)

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or the equivalent on your OS:

```json
{
  "mcpServers": {
    "skillpack": {
      "command": "podman",
      "args": ["run", "--rm", "-i", "ghcr.io/marstid/skillpack:latest", "--transport", "stdio"]
    }
  }
}
```

If you built from source, replace the `command`/`args` with the binary directly:

```json
{
  "mcpServers": {
    "skillpack": {
      "command": "/path/to/skillpack",
      "args": ["--transport", "stdio"]
    }
  }
}
```

### Claude Desktop (HTTP)

```json
{
  "mcpServers": {
    "skillpack": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

Start the server separately (`podman run --rm -p 8080:8080 ghcr.io/marstid/skillpack:latest`).

### Cursor

Add to `.cursor/mcp.json` in your project or `~/.cursor/mcp.json` globally:

```json
{
  "mcpServers": {
    "skillpack": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

> **Read-only.** skillpack only exposes discovery, activation, and resource reads — it never writes to your filesystem or executes code. Skills are guidance for the agent, not a tool execution surface.

---

## CLI reference

| Flag | Default | Description |
|------|---------|-------------|
| `--transport` | `http` | MCP transport: `http` (Streamable HTTP) or `stdio` |
| `--addr` | `:8080` | HTTP listen address (ignored when `--transport stdio`) |
| `--log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `--skills-dir` | *embedded* | External skills directory (fully replaces the embedded `skills/` tree) |
| `--commands-dir` | *embedded* | External commands directory (fully replaces the embedded `commands/` tree) |
| `--merge-skills` | `false` | Merge `--skills-dir` *on top of* the embedded tree; user entries win on name collisions |
| `--merge-commands` | `false` | Merge `--commands-dir` on top of the embedded tree; user entries win on collisions |

**Merge vs. replace.** By default `--skills-dir` / `--commands-dir` fully replaces the embedded tree. Add `--merge-skills` / `--merge-commands` to walk the embedded tree first and the external one second, so your external skills add to (or shadow) the embedded set rather than replacing it. This lets you ship a curated baseline alongside team-specific overrides.

**Logging.** All logs go to stderr (so stdio mode never corrupts the MCP transport on stdout). Validation failures — a skill missing its `description`, a name not matching its parent directory, a malformed `SKILL.md` — are logged at WARN level with `skill_dir` and `reason`; the offending skill is skipped and never appears in `list_skills`.

---

## Skills format (`SKILL.md`)

Each skill lives in its own directory under `skills/`. The entry point is `SKILL.md`:

```markdown
---
name: markdown-lint
description: Lint markdown files for common issues. Use when the user asks to check or validate markdown documentation.
license: Apache-2.0
compatibility: Requires no external tools
metadata:
  author: skillpack
  version: "1.0"
allowed-tools: Read Grep Glob
---

# Markdown Lint

When to use this skill: the user asks to lint or validate markdown documentation.

## Steps
1. Scan headings for structure issues.
2. Check for trailing whitespace.
3. See [rules](references/rules.md) for the full list.
```

### Frontmatter fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Must match the parent directory name. `[a-z0-9-]` only, ≤64 chars, no leading/trailing/double hyphens. |
| `description` | Yes | Drives discovery — shown in the `list_skills` catalog. |
| `license` | No | SPDX license identifier. |
| `compatibility` | No | Free-text note on external tool requirements. |
| `metadata` | No | `map[string]string` of arbitrary key-value pairs (not nested YAML). |
| `allowed-tools` | No | Space-separated list of tools the skill expects to use. |

### Bundled resources

Every non-`SKILL.md` file under the skill directory is automatically discovered as a resource — not just conventional `references/`, `scripts/`, `assets/` folders. Resources are read on demand via the `skill://<name>/<path>` MCP resource URI and are *not* returned on skill activation (keeping the first hit small while allowing arbitrarily deep material).

See the example `skills/markdown-lint/` (with `references/rules.md`) shipped in this repo.

---

## Commands format (`COMMAND.md`)

Each command lives in its own directory under `commands/`. Commands map 1:1 to MCP prompts — clients render them as slash commands with argument inputs.

```markdown
---
name: commit
description: Create a conventional-commit message from staged changes.
arguments:
  - name: scope
    description: Optional scope for the commit.
    required: false
---

Write a commit message{{#if scope}} in scope '{{scope}}'{{/if}}.
Analyze staged changes.
```

### Template directives

| Directive | Meaning |
|-----------|---------|
| `{{arg}}` | Substituted with the arg value (empty if missing). |
| `{{#if arg}}...{{/if}}` | Body kept only if `arg` is present and non-empty. |

---

## MCP tools reference

| Tool | Description |
|------|-------------|
| `list_skills` | Returns the catalog (name + description) as JSON. Pass an optional `query` for case-insensitive fuzzy subsequence ranking over name + description. |
| `activate_skill` | Loads a skill by `name`. Returns the full body in `<skill_content>` plus a `<skill_resources>` listing. Set `header_only=true` for a cheap tier-1 preview — the `<skill_header>` block with frontmatter metadata + resource manifest, no body. |
| `resources` (`prompts/list`) | MCP resources at `skill://<name>/<path>` for any bundled file, plus a static `skill://<name>/SKILL.md` per skill. MCP prompts expose one per command — invoke with arguments to get a rendered user message. |

---

## Building from source

Requires Go 1.26 and optionally [golangci-lint](https://golangci-lint.run/).

```sh
make build      # Build ./skillpack
make install    # Install into $GOPATH/bin
make run        # Build + run on :8080
make run-stdio  # Build + run over stdio (for MCP clients)
make check      # fmt + vet + lint + test
```

---

## Building the container image locally

```sh
make image           # podman build -t skillpack:latest .
make image-run       # Run HTTP on :8080
make image-run-stdio # Run over stdio (interactive)
make clean-all       # Remove binary + local image
```

The `Dockerfile` is multi-stage: a `golang:1.26.4-alpine` builder produces a static, stripped binary, which is shipped on a `distroless/static-debian12:nonroot` runtime (~11 MB total, non-root, no shell). Build layer caching is available via a `:buildcache` image ref when using the GitHub Actions workflow.

---

## Use case: Custom skills image via your own skills repo

> A **runnable version** of this recipe — with sample skills and commands — lives in [`examples/custom-skills-image/`](examples/custom-skills-image/). Clone it and `podman build .` to try it end-to-end.

The recommended pattern for a team or organization is to maintain a **separate skills repo** (e.g. `github.com/yourorg/skills`) containing your `skills/` and `commands/` trees. On every push to `main`, CI builds a custom image `FROM` the published skillpack image, copies your trees in, and pushes to your registry namespace. Users then point their MCP client at the custom image — no rebuilding skillpack itself.

### 1. Your skills repo layout

```
skills/
  logs-triage/
    SKILL.md
    references/
      query-syntax.md
  metrics-query/
    SKILL.md
commands/
  incident-report/
    COMMAND.md
Dockerfile
.github/workflows/build-and-push.yaml
```

### 2. Dockerfile (in your skills repo)

```dockerfile
# Build on the published skillpack runtime image.
FROM ghcr.io/marstid/skillpack:latest

# Replace the embedded trees entirely with your own.
COPY skills/   /skills/
COPY commands/ /commands/

CMD ["--skills-dir", "/skills", "--commands-dir", "/commands", "--transport", "http", "--addr", ":8080", "--log-level", "info"]
```

> **Merge instead of replace.** If you want the embedded example skills to remain available alongside your own, replace the bare `--skills-dir`/`--commands-dir` flags in `CMD` with `--merge-skills --merge-commands` (pointing the flags at your `/skills` and `/commands` paths). Your skills will shadow any embedded name collisions.

### 3. GitHub Actions workflow (in your skills repo)

`.github/workflows/build-and-push.yaml`:

```yaml
name: build-and-push-image

on:
  push:
    branches: [main]

permissions:
  contents: read
  packages: write

env:
  IMAGE: ghcr.io/yourorg/skillpack-skills

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: docker/setup-buildx-action@v3

      - name: Log in to ghcr.io
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Compute tags
        id: tags
        run: |
          sha="$(git rev-parse --short=8 HEAD)"
          echo "tags=$IMAGE:latest,$IMAGE:sha-$sha" >> "$GITHUB_OUTPUT"

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: ${{ steps.tags.outputs.tags }}
          cache-from: type=registry,ref=ghcr.io/yourorg/skillpack-skills:buildcache
          cache-to: type=registry,ref=ghcr.io/yourorg/skillpack-skills:buildcache,mode=max
```

Replace `yourorg` with your GitHub owner name. The workflow runs `go test` is not needed here since your repo contains only Markdown — the build just copies trees onto the skillpack base.

### 4. Connect clients to the custom image

**stdio (Claude Desktop):**

```json
{
  "mcpServers": {
    "skills": {
      "command": "podman",
      "args": ["run", "--rm", "-i", "ghcr.io/yourorg/skillpack-skills:latest", "--transport", "stdio"]
    }
  }
}
```

**HTTP:**

```sh
podman run --rm -p 8080:8080 ghcr.io/yourorg/skillpack-skills:latest
```
Then connect your client to `http://localhost:8080/mcp`.

### 5. Update flow

When a teammate updates a skill in the `skills/` repo and merges the PR to `main`, CI rebuilds and pushes a new `:latest` + `:sha-<commit>` image. Users running the `:latest` tag get the update on their next container pull; users pinning a `:sha-<commit>` tag stay on that exact version until they explicitly bump.

---

## Development

```sh
make test           # Run all tests
make test-race      # Run tests with the race detector
make test-verbose   # Verbose test output
make vet            # go vet
make lint           # golangci-lint
make fmt            # go fmt
make tidy           # go mod tidy + verify
make smoke          # Build + --help smoke test
make clean          # Remove built artifacts
make clean-all      # Remove built artifacts + local image
```

Requires Go 1.26.4 (see `go.mod`). Tests cover skill/command parsing, name validation, multi-FS merge semantics, fuzzy search ranking, and MCP tool integration via an in-memory transport.

---

## License

[MIT](LICENSE) — Copyright (c) 2026 Martin Stidelius
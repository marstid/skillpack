# Example: Custom skills image

This directory is the runnable counterpart of the **"Use case: Custom skills
image via your own skills repo"** section in the [top-level README](../../README.md).
It shows how to build a custom skillpack container image that serves *your*
skills and commands instead of — or alongside — the embedded example ones.

## Contents

```
examples/custom-skills-image/
├── Dockerfile          # Builds on ghcr.io/marstid/skillpack:latest
├── workflow.yaml       # Sample GitHub Actions pipeline (copy to .github/workflows/)
├── skills/
│   ├── chess/
│   │   ├── SKILL.md
│   │   └── references/
│   │       └── rules.md
│   ├── logs-triage/
│   │   ├── SKILL.md
│   │   └── references/
│   │       └── query-syntax.md
│   └── metrics-query/
│       └── SKILL.md
├── commands/
│   └── incident-report/
│       └── COMMAND.md
└── README.md           # this file
```

The sample skills (`chess`, `logs-triage`, `metrics-query`) and command
(`incident-report`) are intentionally small but follow the agentskills.io
format with all frontmatter fields exercised.

### Validate a skill was picked up

The easiest skill to validate end-to-end is **`chess`**. Its description
contains plain, recognizable trigger terms ("play chess", "make a chess
move"), so when you ask any connected agent "Let's play chess", the agent
should call `activate_skill` with `name: "chess"` and pull the guide (and the
bundled `references/rules.md` when a rule question comes up). If the skill is
missing, `list_skills` returns nothing for `query: "chess"` instead.

Quick boot-time check:
```sh
podman run --rm -p 8080:8080 skillpack-custom
# In the stderr log you should see:
#   level=INFO msg=loaded skills=3 commands=1 skills_source=/skills ...
# `(logs-triage, metrics-query, chess) plus the incident-report command.
```

## Build the image locally

From this directory:

```sh
podman build -t skillpack-custom .
```

This pulls the published `ghcr.io/marstid/skillpack:latest` base image, copies
the local `skills/` and `commands/` trees in (replacing the embedded example
skills), and defaults the container to serve them over HTTP.

## Run it

```sh
# HTTP
podman run --rm -p 8080:8080 skillpack-custom

# stdio (for Claude Desktop, Cursor, etc.)
podman run --rm -i skillpack-custom --transport stdio
```

Then connect an MCP client to `http://localhost:8080/mcp` — see the top-level
README for client configuration snippets.

## Wire CI in your own skills repo

1. Create a new git repo for your skills (or copy this directory into an
   existing repo's root).
2. Move `workflow.yaml` to `.github/workflows/build-and-push.yaml`.
3. Replace `yourorg` with your GitHub owner name in the workflow and (if you
   want to rename the image) update `IMAGE:` accordingly.
4. Push to `main`. On every PR merge, CI rebuilds and pushes a new
   `:latest` plus an immutable `:sha-<commit>` tag to your ghcr.io namespace.

## Merge instead of replace

The Dockerfile here uses `--skills-dir` / `--commands-dir` which **fully
replaces** the embedded trees. If you want the embedded example skills
(`markdown-lint`, the `commit` command) to remain available alongside your
own, switch the `CMD` in the Dockerfile to:

```dockerfile
CMD ["--skills-dir", "/skills", "--commands-dir", "/commands", "--merge-skills", "--merge-commands", "--transport", "http", "--addr", ":8080", "--log-level", "info"]
```

User-supplied entries shadow any embedded name collisions (walked last →
last wins). See the top-level README for the full merge-vs-replace reference.
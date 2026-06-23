# syntax=docker/dockerfile:1
#
# Multi-stage Dockerfile for skillpack, the MCP skill server.
#
# Stage 1 builds a static, stripped binary from source. Stage 2 ships it on a
# distroless non-root base (~3 MB total) suitable for both HTTP and stdio MCP
# transports. The embedded skills/ and commands/ trees are compiled in via
# //go:embed, so overriding them at runtime is done with --skills-dir /
# --commands-dir pointing at a mounted volume.
#
# Build:
#   podman build -t skillpack:latest .
#   podman build --build-arg VERSION=v0.1.0 -t skillpack:v0.1.0 .
# Run (HTTP):
#   podman run --rm -p 8080:8080 skillpack:latest
# Run (stdio):
#   podman run --rm -i skillpack:latest --transport stdio
# Run with custom skills/commands (read-only mounts):
#   podman run --rm -p 8080:8080 \
#     -v "$PWD/my-skills:/skills:ro" \
#     -v "$PWD/my-commands:/commands:ro" \
#     skillpack:latest --skills-dir /skills --commands-dir /commands

# ---- builder ----
# Pin the minor Go release to match go.mod (go 1.26.4). alpine keeps the image
# small while still providing a shell and apk for the builder stage.
FROM golang:1.26.4-alpine AS builder

# VERSION is stamped into the binary via ldflags; defaults to "dev".
ARG VERSION=dev

WORKDIR /src

# Cache dependency downloads across image rebuilds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the rest of the source, including the embedded skills/ and commands/ trees.
COPY . .

# Build a static, stripped, reproducible binary. -trimpath removes host paths;
# -s -w strips the symbol table and DWARF; Version is injected for MCP init.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
      -trimpath \
      -ldflags "-s -w -X github.com/marstid/skillpack/internal/mcp.Version=${VERSION}" \
      -o /out/skillpack \
      ./cmd/skillpack

# ---- runtime ----
# distroless static-nonroot ships a static-binary-friendly filesystem with no
# shell, no package manager, and a non-root UID/GID 65532. Ideal for a static
# Go server that needs neither at runtime.
FROM gcr.io/distroless/static-debian12:nonroot

# OCI image labels for tooling (skopeo, podman inspect, registries).
LABEL org.opencontainers.image.title="skillpack" \
      org.opencontainers.image.description="MCP server serving Agent Skills (agentskills.io) and skillpack commands" \
      org.opencontainers.image.source="https://github.com/marstid/skillpack" \
      org.opencontainers.image.licenses="MIT"

COPY --from=builder /out/skillpack /skillpack

# HTTP transport listens on :8080 by default. The binary supports --transport
# stdio as well, in which case EXPOSE is informational only.
EXPOSE 8080

# Non-root user is baked into the base image (UID 65532). No USER directive
# needed; distroless:nonroot activates it by default.

ENTRYPOINT ["/skillpack"]
# Defaults match `skillpack --transport http --addr :8080 --log-level info`.
CMD ["--transport", "http", "--addr", ":8080", "--log-level", "info"]
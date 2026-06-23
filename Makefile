# skillpack Makefile
#
# Common development tasks for the skillpack MCP server. All targets are
# self-documenting via `make help`. Targets are phony unless they produce a
# file named the same as the target.
#
# Variables can be overridden from the environment or the command line:
#   make test TEST_FLAGS="-run TestSearch -v"
#   make run TRANSPORT=stdio
#   make run ADDR=:9090

BINARY   ?= skillpack
CMD      := ./cmd/skillpack
PKG      := ./...
ADDR     ?= :8080
TRANSPORT ?= http
LOGLEVEL ?= info
GOFLAGS  ?= -trimpath
BUILD_FLAGS ?= $(GOFLAGS)
TEST_FLAGS ?=

# Container image build. Podman is the default; override with CONTAINER=docker.
CONTAINER ?= podman
IMAGE     ?= skillpack
TAG       ?= latest
VERSION   ?= dev
CONTAINER_FLAGS ?=

# Tooling binaries; prefer local installs, fall back to `go run` wrappers.
GO        := go
GOLANGCI  := golangci-lint

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} \
	/^[a-zA-Z_-]+:.*?##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the skillpack binary into the project root
	$(GO) build $(BUILD_FLAGS) -o $(BINARY) $(CMD)

.PHONY: install
install: ## Install the skillpack binary into $$GOPATH/bin
	$(GO) install $(BUILD_FLAGS) $(CMD)

.PHONY: run
run: build ## Run the server (TRANSPORT=http|stdio, ADDR=:8080, LOGLEVEL=info)
	./$(BINARY) --transport $(TRANSPORT) --addr $(ADDR) --log-level $(LOGLEVEL)

.PHONY: run-stdio
run-stdio: ## Run the server over stdio (for MCP clients)
	$(MAKE) run TRANSPORT=stdio

.PHONY: test
test: ## Run all tests
	$(GO) test $(TEST_FLAGS) $(PKG)

.PHONY: test-race
test-race: ## Run tests with the race detector
	$(GO) test -race $(TEST_FLAGS) $(PKG)

.PHONY: test-verbose
test-verbose: ## Run tests verbosely
	$(GO) test -v $(TEST_FLAGS) $(PKG)

.PHONY: vet
vet: ## Run go vet
	$(GO) vet $(PKG)

.PHONY: lint
lint: ## Run golangci-lint (skips tests dirs)
	$(GOLANGCI) run ./...

.PHONY: fmt
fmt: ## Format all Go files
	$(GO) fmt $(PKG)

.PHONY: tidy
tidy: ## Tidy and verify go.mod/go.sum
	$(GO) mod tidy
	$(GO) mod verify

.PHONY: check
check: fmt vet lint test ## Run fmt + vet + lint + test

.PHONY: clean
clean: ## Remove built artifacts
	rm -f $(BINARY)

.PHONY: smoke
smoke: build ## Quick build + --help smoke test
	./$(BINARY) --help >/dev/null && echo "smoke: OK"

# ----------------------------------------------------------------------------
# Container image targets (use podman by default; CONTAINER=docker to switch)
# ----------------------------------------------------------------------------

.PHONY: image
image: ## Build the container image (IMAGE=skillpack, TAG=latest, VERSION=dev)
	$(CONTAINER) build $(CONTAINER_FLAGS) \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE):$(TAG) \
		.

.PHONY: image-run
image-run: ## Run the image (TRANSPORT, ADDR, LOGLEVEL override server flags)
	$(CONTAINER) run --rm -p 8080:8080 $(IMAGE):$(TAG) \
		--transport $(TRANSPORT) --addr $(ADDR) --log-level $(LOGLEVEL)

.PHONY: image-run-stdio
image-run-stdio: ## Run the image over stdio (interactive; for MCP clients)
	$(CONTAINER) run --rm -i $(IMAGE):$(TAG) --transport stdio

# Example mount-based override; point SKILLS_DIR/CMD_DIR at host dirs to use.
SKILLS_DIR ?=
COMMANDS_DIR ?=
.PHONY: image-run-mounted
image-run-mounted: ## Run the image with --skills-dir / --commands-dir mounted (SKILLS_DIR=, COMMANDS_DIR=)
	@vols=""; \
	[ -n "$(SKILLS_DIR)" ]    && vols="$$vols -v $$PWD/$(SKILLS_DIR):/skills:ro"; \
	[ -n "$(COMMANDS_DIR)" ]  && vols="$$vols -v $$PWD/$(COMMANDS_DIR):/commands:ro"; \
	args="--transport $(TRANSPORT) --addr $(ADDR) --log-level $(LOGLEVEL)"; \
	[ -n "$(SKILLS_DIR)" ]    && args="$$args --skills-dir /skills"; \
	[ -n "$(COMMANDS_DIR)" ]  && args="$$args --commands-dir /commands"; \
	$(CONTAINER) run --rm -p 8080:8080 $$vols $(IMAGE):$(TAG) $$args

.PHONY: image-shell
image-shell: ## Open a shell in a debug shell based on the builder stage
	$(CONTAINER) run --rm -it --entrypoint /bin/sh $(IMAGE):$(TAG) || \
	echo "runtime image has no shell; use 'make image-shell-builder' instead"

.PHONY: image-shell-builder
image-shell-builder: ## Open a shell in the builder stage for debugging
	$(CONTAINER) run --rm -it --entrypoint /bin/sh $(IMAGE):$(TAG)

.PHONY: image-rm
image-rm: ## Remove the local image
	$(CONTAINER) rmi $(IMAGE):$(TAG) 2>/dev/null || true

.PHONY: clean-all
clean-all: clean image-rm ## Remove built artifacts and local image
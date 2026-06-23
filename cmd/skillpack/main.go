// Command skillpack is an MCP server that serves Agent Skills (agentskills.io) and
// skillpack commands from embedded markdown trees. Skills map to MCP tools and a
// resource template; commands map to MCP prompts.
//
// Usage:
//
//	skillpack [--transport http|stdio] [--addr :8080] [--skills-dir DIR] [--commands-dir DIR] [--merge-skills] [--merge-commands]
//
// By default the embedded skills/ and commands/ trees are served. Passing
// --skills-dir or --commands-dir fully replaces the corresponding embedded
// tree with the on-disk directory, enabling markdown-only edits without
// rebuilding. Add --merge-skills or --merge-commands to merge the on-disk
// tree on top of the embedded one instead of replacing it; on name
// collisions the user-supplied entry wins.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marstid/skillpack"
	"github.com/marstid/skillpack/internal/command"
	skillpackmcp "github.com/marstid/skillpack/internal/mcp"
	"github.com/marstid/skillpack/internal/skill"
)

// skillpackmcp aliases the internal/mcp package, which would otherwise shadow
// the go-sdk mcp import.

func main() {
	transport := flag.String("transport", "http", "MCP transport: \"http\" (Streamable HTTP) or \"stdio\"")
	addr := flag.String("addr", ":8080", "HTTP listen address (ignored when --transport stdio)")
	skillsDir := flag.String("skills-dir", "", "external skills directory that fully replaces the embedded skills/ tree (use --merge-skills to merge instead)")
	mergeSkills := flag.Bool("merge-skills", false, "merge --skills-dir on top of the embedded skills/ tree (user entries win on name collisions); otherwise --skills-dir fully replaces the embedded tree")
	commandsDir := flag.String("commands-dir", "", "external commands directory that fully replaces the embedded commands/ tree (use --merge-commands to merge instead)")
	mergeCommands := flag.Bool("merge-commands", false, "merge --commands-dir on top of the embedded commands/ tree (user entries win on name collisions); otherwise --commands-dir fully replaces the embedded tree")
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	flag.Parse()

	// In stdio mode, stdout is the MCP transport — all logging must go to
	// stderr to avoid corrupting the protocol channel.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLogLevel(*logLevel),
	}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	skillFSes, skillsSourceLabel, skillsMerge, err := resolveSourceList(skillpack.SkillsFS, "skills", *skillsDir, *mergeSkills)
	if err != nil {
		logger.Error("resolving skills source failed", "error", err)
		os.Exit(1)
	}
	commandFSes, commandsSourceLabel, commandsMerge, err := resolveSourceList(skillpack.CommandsFS, "commands", *commandsDir, *mergeCommands)
	if err != nil {
		logger.Error("resolving commands source failed", "error", err)
		os.Exit(1)
	}

	skStore, err := skill.New(ctx, logger, skillFSes...)
	if err != nil {
		logger.Error("loading skills failed", "error", err)
		os.Exit(1)
	}
	cmdStore, err := command.New(ctx, logger, commandFSes...)
	if err != nil {
		logger.Error("loading commands failed", "error", err)
		os.Exit(1)
	}
	logger.Info("loaded", "skills", len(skStore.List()), "commands", len(cmdStore.List()),
		"skills_source", skillsSourceLabel, "skills_merge", skillsMerge,
		"commands_source", commandsSourceLabel, "commands_merge", commandsMerge)

	srv := skillpackmcp.New(skStore, cmdStore)

	switch *transport {
	case "http":
		runHTTP(ctx, srv, *addr, logger)
	case "stdio":
		runStdio(ctx, srv, logger)
	default:
		logger.Error("unknown --transport", "value", *transport)
		os.Exit(2)
	}
}

// resolveSourceList returns the ordered list of fs.FS sources for a content
// kind, plus a human-readable source label and the merge flag in effect. When
// override is empty the embedded tree alone is used. When override is set and
// merge is false, override fully replaces the embedded tree. When merge is
// true, the embedded tree is walked first and the override second so user
// entries win on name collisions.
func resolveSourceList(embedded fs.FS, sub, override string, merge bool) ([]fs.FS, string, bool, error) {
	embeddedSub, err := fs.Sub(embedded, sub)
	if err != nil {
		return nil, "", false, fmt.Errorf("root embedded %s fs: %w", sub, err)
	}
	if override == "" {
		return []fs.FS{embeddedSub}, "embedded", false, nil
	}
	info, err := os.Stat(override)
	if err != nil {
		return nil, "", false, fmt.Errorf("stat %s dir %q: %w", sub, override, err)
	}
	if !info.IsDir() {
		return nil, "", false, fmt.Errorf("--%s-dir %q is not a directory", sub, override)
	}
	userFS := os.DirFS(override)
	if !merge {
		return []fs.FS{userFS}, override, false, nil
	}
	return []fs.FS{embeddedSub, userFS}, override, true, nil
}

func runHTTP(ctx context.Context, srv *mcp.Server, addr string, logger *slog.Logger) {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	httpHandler := http.NewServeMux()
	httpHandler.Handle("/mcp", handler)
	// Also accept requests at "/" for convenience in some clients.
	httpHandler.Handle("/", handler)

	server := &http.Server{Addr: addr, Handler: httpHandler}
	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		_ = server.Shutdown(context.Background())
	}()
	logger.Info("skillpack MCP server listening (Streamable HTTP)", "addr", addr, "endpoint", "http://localhost"+addr+"/mcp")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

func runStdio(ctx context.Context, srv *mcp.Server, logger *slog.Logger) {
	logger.Info("skillpack MCP server listening (stdio on stdin/stdout)")
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		logger.Error("stdio transport failed", "error", err)
		os.Exit(1)
	}
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

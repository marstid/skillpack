// Package mcp wires the skillpack skill and command stores into an MCP server
// (tools, prompts, resources) and serves it over stdio or Streamable HTTP.
package mcp

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marstid/skillpack/internal/command"
	"github.com/marstid/skillpack/internal/skill"
)

// New constructs an *mcp.Server wired with all skillpack features: the
// activate_skill / list_skills / read_resource tools, one prompt per command,
// the skill://{name}/{path} resource template, and per-skill static SKILL.md
// resources. The logger is used for SDK-internal diagnostics and access
// logging at DEBUG level. If logger is nil, logging is discarded.
func New(skStore *skill.Store, cmdStore *command.Store, logger *slog.Logger) *mcp.Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    "skillpack",
			Version: Version,
		},
		&mcp.ServerOptions{
			Instructions: serverInstructions(skStore, cmdStore),
			Logger:        logger,
			Capabilities: &mcp.ServerCapabilities{
				Tools:     &mcp.ToolCapabilities{ListChanged: false},
				Prompts:   &mcp.PromptCapabilities{ListChanged: false},
				Resources: &mcp.ResourceCapabilities{ListChanged: false, Subscribe: false},
			},
		},
	)
	srv.AddReceivingMiddleware(accessLogMiddleware(logger))
	registerTools(srv, skStore)
	registerPrompts(srv, cmdStore)
	registerResources(srv, skStore)
	return srv
}

// accessLogMiddleware wraps every incoming MCP method (tools/call,
// resources/read, prompts/get, initialize, ping, list calls, etc.) and logs
// at DEBUG level: the method, session ID, relevant request details, duration,
// and any error. In stdio mode the logger writes to stderr, so this never
// corrupts the protocol channel on stdout.
func accessLogMiddleware(logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			start := time.Now()

			result, err := next(ctx, method, req)
			duration := time.Since(start)

			attrs := []any{
				slog.String("method", method),
				slog.Duration("duration", duration),
			}
			if sess := req.GetSession(); sess != nil {
				attrs = append(attrs, slog.String("session", sess.ID()))
			}

			switch r := req.(type) {
			case *mcp.CallToolRequest:
				if r.Params != nil {
					attrs = append(attrs, slog.String("tool", r.Params.Name))
				}
			case *mcp.ReadResourceRequest:
				if r.Params != nil {
					attrs = append(attrs, slog.String("uri", r.Params.URI))
				}
			case *mcp.GetPromptRequest:
				if r.Params != nil {
					attrs = append(attrs, slog.String("prompt", r.Params.Name))
				}
			}

			if err != nil {
				attrs = append(attrs, slog.Any("error", err))
				logger.Debug("MCP request failed", attrs...)
			} else {
				logger.Debug("MCP request completed", attrs...)
			}

			return result, err
		}
	}
}

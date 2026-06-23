// Package mcp wires the skillpack skill and command stores into an MCP server
// (tools, prompts, resources) and serves it over stdio or Streamable HTTP.
package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marstid/skillpack/internal/command"
	"github.com/marstid/skillpack/internal/skill"
)

// New constructs an *mcp.Server wired with all skillpack features: the
// activate_skill / list_skills tools, one prompt per command, the
// skill://{name}/{path} resource template, and per-skill static SKILL.md
// resources.
func New(skStore *skill.Store, cmdStore *command.Store) *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    "skillpack",
			Version: Version,
		},
		&mcp.ServerOptions{
			Instructions: serverInstructions(skStore, cmdStore),
			Capabilities: &mcp.ServerCapabilities{
				Tools:     &mcp.ToolCapabilities{ListChanged: false},
				Prompts:   &mcp.PromptCapabilities{ListChanged: false},
				Resources: &mcp.ResourceCapabilities{ListChanged: false, Subscribe: false},
			},
		},
	)
	registerTools(srv, skStore)
	registerPrompts(srv, cmdStore)
	registerResources(srv, skStore)
	return srv
}

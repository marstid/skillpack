package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marstid/skillpack/internal/command"
)

// registerPrompts installs one MCP prompt per skillpack command. Each command's
// frontmatter arguments become MCP prompt arguments, so clients can render
// them as slash commands with argument inputs.
func registerPrompts(srv *mcp.Server, store *command.Store) {
	for _, c := range store.List() {
		c := c // capture for closure
		args := make([]*mcp.PromptArgument, 0, len(c.Arguments))
		for _, a := range c.Arguments {
			args = append(args, &mcp.PromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			})
		}
		srv.AddPrompt(&mcp.Prompt{
			Name:        c.Name,
			Description: c.Description,
			Arguments:   args,
		}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			text, err := store.Render(c.Name, req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			return &mcp.GetPromptResult{
				Description: c.Description,
				Messages: []*mcp.PromptMessage{
					{
						Role:    mcp.Role("user"),
						Content: &mcp.TextContent{Text: text},
					},
				},
			}, nil
		})
	}
}

package mcp

import (
	"strings"

	"github.com/marstid/skillpack/internal/command"
	"github.com/marstid/skillpack/internal/skill"
)

// serverInstructions is the MCP server-level instructions advertised to
// clients during initialize. It is intentionally short; the detailed usage
// guidance lives in tool and prompt descriptions.
func serverInstructions(sk *skill.Store, cmd *command.Store) string {
	var b strings.Builder
	b.WriteString("# skillpack — Agent Skills & Commands over MCP\n\n")
	b.WriteString("This server serves Agent Skills (agentskills.io format) and skillpack commands.\n\n")
	b.WriteString("## Skills\n")
	b.WriteString("- `list_skills` — list available skills (name + description).\n")
	b.WriteString("- `activate_skill` — load a skill's full instructions into context.\n")
	b.WriteString("- Resources `skill://<name>/<path>` — read bundled files (scripts/, references/, assets/) on demand.\n\n")
	b.WriteString("## Commands\n")
	b.WriteString("Prompts (`prompts/list`) expose skillpack commands; invoke with arguments to get a rendered user message.\n\n")
	b.WriteString("Use progressive disclosure: call `list_skills`, then `activate_skill` for the relevant one, and read bundled resources only when the instructions reference them.")
	return b.String()
}

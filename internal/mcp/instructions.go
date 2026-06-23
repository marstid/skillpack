package mcp

import (
	"fmt"
	"strings"

	"github.com/marstid/skillpack/internal/command"
	"github.com/marstid/skillpack/internal/skill"
)

// serverInstructions is the MCP server-level instructions advertised to
// clients during initialize. It embeds the full skill and command catalog so
// that harnesses (e.g. OpenCode, Claude Desktop, Cursor) surface skills in the
// agent's initial context just like native skills — without requiring a
// separate tools/list round trip. Proactive guidance tells the model to call
// activate_skill when a task matches.
func serverInstructions(sk *skill.Store, cmd *command.Store) string {
	var b strings.Builder
	b.WriteString("# skillpack — Agent Skills & Commands over MCP\n\n")
	b.WriteString("This server serves Agent Skills (agentskills.io format) and custom commands.\n\n")
	b.WriteString("## Skills\n\n")
	b.WriteString("When the user's task matches one of the skills below, proactively call `activate_skill` to load that skill's full instructions into context. The `activate_skill` tool is the skill-loading mechanism — equivalent to a native skill loader.\n\n")

	skills := sk.List()
	if len(skills) == 0 {
		b.WriteString("(No skills are currently loaded.)\n\n")
	} else {
		b.WriteString("Available skills:\n")
		for _, e := range skills {
			fmt.Fprintf(&b, "- **%s**: %s\n", e.Name, e.Description)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Skill tools\n")
	b.WriteString("- `activate_skill` — load a skill's full instructions into context (supports `header_only` for a cheap preview).\n")
	b.WriteString("- `read_resource` — call this after `activate_skill` to read any bundled file referenced in the skill instructions. Pass the skill name and the relative file path (e.g. `references/rules.md`). Do NOT read bundled skill files from the local filesystem.\n")
	b.WriteString("- `list_skills` — list available skills as JSON with optional fuzzy search.\n\n")

	b.WriteString("## Commands\n\n")
	cmds := cmd.List()
	if len(cmds) == 0 {
		b.WriteString("(No commands are currently loaded.)\n\n")
	} else {
		b.WriteString("Available commands:\n")
		for _, c := range cmds {
			fmt.Fprintf(&b, "- **%s**: %s\n", c.Name, c.Description)
		}
		b.WriteString("\n")
	}
	b.WriteString("Prompts (`prompts/list`) expose skillpack commands; invoke with arguments to get a rendered user message.\n\n")

	b.WriteString("Use progressive disclosure: call `list_skills`, then `activate_skill` for the relevant one, and use `read_resource` to load bundled files only when the skill instructions reference them.")
	return b.String()
}

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marstid/skillpack/internal/skill"
)

// registerTools installs the activate_skill, list_skills, and read_resource
// tools on the server. The catalog of available skills (tier-1 disclosure,
// ~50-100 tokens per skill) is embedded directly in the activate_skill tool
// description, so the model can pick a valid name without a separate discovery
// call.
func registerTools(srv *mcp.Server, store *skill.Store) {
	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "activate_skill",
			Title:       "Activate an Agent Skill",
			Description: activateSkillDescription(store),
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		activateSkillHandler(store),
	)

	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "list_skills",
			Title:       "List available Agent Skills",
			Description: "List all available Agent Skills (name + description) as JSON. Use this for explicit discovery; the activate_skill tool description also carries the catalog. Pass an optional `query` for case-insensitive fuzzy subsequence ranking over name + description (empty query returns the full catalog unsorted).",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		listSkillsHandler(store),
	)

	mcp.AddTool(srv,
		&mcp.Tool{
			Name:        "read_resource",
			Title:       "Read a bundled skill resource file",
			Description: "Read a bundled file from an activated Agent Skill. Call this after activate_skill when the skill instructions or the <skill_resources> block references a file. Pass the skill name and the relative file path (e.g. name=\"chess\", path=\"references/rules.md\").",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		readResourceHandler(store),
	)
}

// activateSkillInput is the typed input for the activate_skill tool. The
// jsonschema tag supplies the description rendered into the tool's input
// schema.
type activateSkillInput struct {
	Name string `json:"name" jsonschema:"the name of the skill to activate"`
	// HeaderOnly returns just the skill header (frontmatter + resource
	// manifest) without the body — a cheap tier-1 preview.
	HeaderOnly bool `json:"header_only,omitempty" jsonschema:"if true, return only the skill frontmatter (name, description, license, compatibility, metadata, allowed-tools) and the resource manifest; omit the body"`
}

// activateSkillOutput is the structured output of activate_skill.
type activateSkillOutput struct {
	Name       string   `json:"name"`
	HeaderOnly bool     `json:"header_only,omitempty"`
	Resources  []string `json:"resources,omitempty"`
}

// listSkillsInput is the typed input for list_skills.
type listSkillsInput struct {
	Query string `json:"query,omitempty" jsonschema:"optional case-insensitive fuzzy search over skill name and description"`
}

// listSkillsOutput is the structured output of list_skills.
type listSkillsOutput struct {
	Skills      []skill.CatalogEntry `json:"skills"`
	Diagnostics map[string][]string  `json:"diagnostics,omitempty"`
}

func activateSkillDescription(store *skill.Store) string {
	var b strings.Builder
	b.WriteString("Activate an Agent Skill by name to load its instructions into context. ")
	b.WriteString("By default returns the skill body wrapped in <skill_content> tags plus a <skill_resources> listing of bundled files. ")
	b.WriteString("Set header_only=true to get a cheap preview: the <skill_header> block (frontmatter metadata + resource manifest, no body). ")
	b.WriteString("Read bundled resources on demand via the `read_resource` tool — do NOT read them from the local filesystem. ")
	b.WriteString("The <skill_resources> block lists each file with its name and path you can pass to `read_resource`.\n\n")
	b.WriteString("Available skills:\n")
	for _, e := range store.List() {
		fmt.Fprintf(&b, "- %s: %s\n", e.Name, e.Description)
	}
	return b.String()
}

func activateSkillHandler(store *skill.Store) mcp.ToolHandlerFor[activateSkillInput, activateSkillOutput] {
	return func(_ context.Context, _ *mcp.CallToolRequest, in activateSkillInput) (*mcp.CallToolResult, activateSkillOutput, error) {
		sk, err := store.Get(in.Name)
		if err != nil {
			return nil, activateSkillOutput{}, err
		}
		var b strings.Builder
		if in.HeaderOnly {
			writeSkillHeader(&b, sk)
		} else {
			fmt.Fprintf(&b, "<skill_content name=%q>\n", sk.Name)
			b.WriteString(sk.Body)
			b.WriteString("\n\nTo read a bundled file listed below, call `read_resource` with the skill name and the file path (e.g. `name=\"markdown-lint\"`, `path=\"references/rules.md\"`). Do NOT read bundled skill files from the local filesystem.\n")
			writeResourceListing(&b, sk.Name, sk.Resources)
			b.WriteString("</skill_content>")
		}
		out := activateSkillOutput{Name: sk.Name, HeaderOnly: in.HeaderOnly, Resources: sk.Resources}
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: b.String()}},
		}
		return res, out, nil
	}
}

// writeSkillHeader renders the tier-1 preview: frontmatter fields + resource
// manifest, no body. Optional fields are omitted when empty.
func writeSkillHeader(b *strings.Builder, sk *skill.Skill) {
	fmt.Fprintf(b, "<skill_header name=%q>\n", sk.Name)
	fmt.Fprintf(b, "name: %s\n", sk.Name)
	fmt.Fprintf(b, "description: %s\n", sk.Description)
	if strings.TrimSpace(sk.License) != "" {
		fmt.Fprintf(b, "license: %s\n", sk.License)
	}
	if strings.TrimSpace(sk.Compatibility) != "" {
		fmt.Fprintf(b, "compatibility: %s\n", sk.Compatibility)
	}
	if len(sk.Metadata) > 0 {
		b.WriteString("metadata:")
		for k, v := range sk.Metadata {
			fmt.Fprintf(b, " %s=%s", k, v)
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(sk.AllowedTools) != "" {
		fmt.Fprintf(b, "allowed-tools: %s\n", sk.AllowedTools)
	}
	writeResourceListing(b, sk.Name, sk.Resources)
	b.WriteString("</skill_header>")
}

// writeResourceListing emits the <skill_resources> block when resources exist.
// Each file is listed as a full skill:// URI so the agent reads it via the MCP
// resource protocol rather than attempting local filesystem reads.
func writeResourceListing(b *strings.Builder, skillName string, resources []string) {
	if len(resources) == 0 {
		return
	}
	b.WriteString("<skill_resources>\n")
	for _, r := range resources {
		fmt.Fprintf(b, "  <file uri=\"skill://%s/%s\">%s</file>\n", skillName, r, r)
	}
	b.WriteString("</skill_resources>\n")
}

func listSkillsHandler(store *skill.Store) mcp.ToolHandlerFor[listSkillsInput, listSkillsOutput] {
	return func(_ context.Context, _ *mcp.CallToolRequest, in listSkillsInput) (*mcp.CallToolResult, listSkillsOutput, error) {
		out := listSkillsOutput{Skills: store.Search(in.Query)}
		// Surface any diagnostics recorded during load.
		for _, name := range skillNames(store) {
			if sk, err := store.Get(name); err == nil && len(sk.Diagnostics) > 0 {
				if out.Diagnostics == nil {
					out.Diagnostics = make(map[string][]string)
				}
				out.Diagnostics[name] = sk.Diagnostics
			}
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}
		return res, out, nil
	}
}

// skillNames returns the ordered list of skill names by iterating the catalog.
func skillNames(store *skill.Store) []string {
	entries := store.List()
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name
	}
	return out
}

// readResourceInput is the typed input for read_resource.
type readResourceInput struct {
	Name string `json:"name" jsonschema:"the skill name"`
	Path string `json:"path" jsonschema:"the file path relative to the skill directory (e.g. references/rules.md)"`
}

// readResourceOutput is the structured output of read_resource. Content is
// included in the structured output because some harnesses only surface the
// structured output to the agent, not the CallToolResult.Content field.
type readResourceOutput struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	MIMEType string `json:"mime_type"`
	Content  string `json:"content"`
}

func readResourceHandler(store *skill.Store) mcp.ToolHandlerFor[readResourceInput, readResourceOutput] {
	return func(_ context.Context, _ *mcp.CallToolRequest, in readResourceInput) (*mcp.CallToolResult, readResourceOutput, error) {
		data, mime, err := store.ReadResource(in.Name, in.Path)
		if err != nil {
			return nil, readResourceOutput{}, err
		}
		out := readResourceOutput{Name: in.Name, Path: in.Path, MIMEType: mime, Content: string(data)}
		res := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}
		return res, out, nil
	}
}

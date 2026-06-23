package mcp_test

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	skillpack "github.com/marstid/skillpack"
	"github.com/marstid/skillpack/internal/command"
	skillpackmcp "github.com/marstid/skillpack/internal/mcp"
	"github.com/marstid/skillpack/internal/skill"
)

// startServer builds the skillpack MCP server backed by the *embedded* skills/ and
// commands/ trees and connects it to an MCP client over an in-memory
// transport pair. It returns the initialized client session and a cleanup
// function.
func startServer(t *testing.T) (*mcp.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()

	skillFS, err := fs.Sub(skillpack.SkillsFS, "skills")
	if err != nil {
		t.Fatalf("sub skills fs: %v", err)
	}
	commandFS, err := fs.Sub(skillpack.CommandsFS, "commands")
	if err != nil {
		t.Fatalf("sub commands fs: %v", err)
	}
	skStore, err := skill.New(ctx, nil, skillFS)
	if err != nil {
		t.Fatalf("skill.New: %v", err)
	}
	cmdStore, err := command.New(ctx, nil, commandFS)
	if err != nil {
		t.Fatalf("command.New: %v", err)
	}
	if len(skStore.List()) == 0 {
		t.Fatal("embedded skills tree produced no skills")
	}
	srv := skillpackmcp.New(skStore, cmdStore)

	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "skillpack-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	return session, func() {
		_ = session.Close()
		_ = ss.Close()
	}
}

func TestIntegration_ListAndActivateSkill(t *testing.T) {
	ctx := context.Background()
	session, cleanup := startServer(t)
	defer cleanup()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if !hasTool(tools.Tools, "activate_skill") || !hasTool(tools.Tools, "list_skills") || !hasTool(tools.Tools, "read_resource") {
		t.Fatalf("expected activate_skill, list_skills, read_resource; got: %+v", toolNames(tools.Tools))
	}

	// list_skills should include the seeded markdown-lint skill.
	lsres, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_skills",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool list_skills: %v", err)
	}
	if len(lsres.Content) == 0 {
		t.Fatal("list_skills returned no content")
	}
	text := contentText(lsres)
	if !strings.Contains(text, "markdown-lint") {
		t.Errorf("list_skills output missing markdown-lint: %s", text)
	}

	// activate_skill for markdown-lint should return the wrapped body and
	// list the bundled references/rules.md resource.
	actres, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "activate_skill",
		Arguments: map[string]any{"name": "markdown-lint"},
	})
	if err != nil {
		t.Fatalf("CallTool activate_skill: %v", err)
	}
	if actres.IsError {
		t.Fatalf("activate_skill returned tool error: %+v", actres)
	}
	body := contentText(actres)
	for _, want := range []string{
		"<skill_content name=\"markdown-lint\">",
		"references/rules.md",
		"skill://markdown-lint/references/rules.md",
		"call `read_resource`",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("activate_skill output missing %q; got: %s", want, body)
		}
	}
}

func TestIntegration_ActivateSkillHeaderOnly(t *testing.T) {
	ctx := context.Background()
	session, cleanup := startServer(t)
	defer cleanup()

	// header_only=true should return the <skill_header> block (frontmatter +
	// resource manifest) without the body prose.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "activate_skill",
		Arguments: map[string]any{"name": "markdown-lint", "header_only": true},
	})
	if err != nil {
		t.Fatalf("CallTool activate_skill header_only: %v", err)
	}
	if res.IsError {
		t.Fatalf("activate_skill returned tool error: %+v", res)
	}
	body := contentText(res)
	for _, want := range []string{
		`<skill_header name="markdown-lint">`,
		"name: markdown-lint",
		"description:",
		"references/rules.md",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("header output missing %q; got: %s", want, body)
		}
	}
	// Body prose from the markdown-lint skill must NOT be present.
	if strings.Contains(body, "Apply standard Markdown rules") {
		t.Errorf("header_only output should not contain the body; got: %s", body)
	}
}

func TestIntegration_ListSkillsQuery(t *testing.T) {
	ctx := context.Background()
	session, cleanup := startServer(t)
	defer cleanup()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_skills",
		Arguments: map[string]any{"query": "markdown"},
	})
	if err != nil {
		t.Fatalf("CallTool list_skills: %v", err)
	}
	body := contentText(res)
	if !strings.Contains(body, "markdown-lint") {
		t.Errorf("query='markdown' should return markdown-lint; got: %s", body)
	}

	// A query that matches nothing should return an empty skills array.
	res2, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_skills",
		Arguments: map[string]any{"query": "zzzzzzz"},
	})
	if err != nil {
		t.Fatalf("CallTool list_skills no-match: %v", err)
	}
	body2 := contentText(res2)
	if strings.Contains(body2, "markdown-lint") {
		t.Errorf("query='zzzzzzz' should not return markdown-lint; got: %s", body2)
	}
}

func TestIntegration_ResourceRead(t *testing.T) {
	ctx := context.Background()
	session, cleanup := startServer(t)
	defer cleanup()

	// resources/list should advertise the static skill://markdown-lint/SKILL.md.
	res, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	found := false
	for _, r := range res.Resources {
		if r.URI == "skill://markdown-lint/SKILL.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListResources did not advertise skill://markdown-lint/SKILL.md: %+v", res.Resources)
	}

	// Read a bundled resource via the template URI.
	rr, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "skill://markdown-lint/references/rules.md",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(rr.Contents) == 0 {
		t.Fatal("ReadResource returned no contents")
	}
	if !strings.Contains(rr.Contents[0].Text, "Markdown Lint Rules") {
		t.Errorf("resource text = %q", rr.Contents[0].Text)
	}
}

func TestIntegration_PromptRender(t *testing.T) {
	ctx := context.Background()
	session, cleanup := startServer(t)
	defer cleanup()

	prompts, err := session.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if !hasPrompt(prompts.Prompts, "commit") {
		t.Errorf("prompts list missing commit: %+v", prompts.Prompts)
	}

	// Optional arg provided → branch included in rendered message.
	got, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "commit",
		Arguments: map[string]string{"scope": "api"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(got.Messages) == 0 {
		t.Fatal("GetPrompt returned no messages")
	}
	msg := got.Messages[0]
	text := ""
	if tc, ok := msg.Content.(*mcp.TextContent); ok {
		text = tc.Text
	} else {
		t.Fatalf("prompt content is %T, not TextContent", msg.Content)
	}
	if !strings.Contains(text, "in scope 'api'") {
		t.Errorf("rendered prompt missing scope branch: %s", text)
	}

	// Optional arg omitted → branch collapses.
	got2, err := session.GetPrompt(ctx, &mcp.GetPromptParams{Name: "commit"})
	if err != nil {
		t.Fatalf("GetPrompt (no args): %v", err)
	}
	text2 := ""
	if tc, ok := got2.Messages[0].Content.(*mcp.TextContent); ok {
		text2 = tc.Text
	}
	if strings.Contains(text2, "in scope") {
		t.Errorf("{{#if}} should collapse when scope absent: %s", text2)
	}
}

func TestIntegration_ReadResourceTool(t *testing.T) {
	ctx := context.Background()
	session, cleanup := startServer(t)
	defer cleanup()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if !hasTool(tools.Tools, "read_resource") {
		t.Fatalf("read_resource tool not found: %+v", toolNames(tools.Tools))
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "read_resource",
		Arguments: map[string]any{
			"name": "markdown-lint",
			"path": "references/rules.md",
		},
	})
	if err != nil {
		t.Fatalf("CallTool read_resource: %v", err)
	}
	if res.IsError {
		t.Fatalf("read_resource returned tool error: %+v", res)
	}
	body := contentText(res)
	if !strings.Contains(body, "Markdown Lint Rules") {
		t.Errorf("read_resource output missing expected content: %s", body)
	}

	errRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "read_resource",
		Arguments: map[string]any{
			"name": "nonexistent",
			"path": "foo.md",
		},
	})
	if err != nil {
		t.Fatalf("CallTool read_resource (nonexistent): %v", err)
	}
	if !errRes.IsError {
		t.Error("expected IsError for nonexistent skill, got success")
	}
}

func TestIntegration_Instructions(t *testing.T) {
	session, cleanup := startServer(t)
	defer cleanup()

	res := session.InitializeResult()
	if res == nil {
		t.Fatal("InitializeResult is nil")
	}
	instr := res.Instructions
	if instr == "" {
		t.Fatal("server instructions are empty")
	}

	for _, want := range []string{
		"markdown-lint",
		"proactively call `activate_skill`",
		"Available skills:",
		"`read_resource`",
		"Do NOT read bundled",
	} {
		if !strings.Contains(instr, want) {
			t.Errorf("instructions missing %q; got: %s", want, instr)
		}
	}

	for _, want := range []string{
		"commit",
		"Available commands:",
	} {
		if !strings.Contains(instr, want) {
			t.Errorf("instructions missing %q; got: %s", want, instr)
		}
	}
}

// helpers

func hasTool(tools []*mcp.Tool, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

func toolNames(tools []*mcp.Tool) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.Name
	}
	return out
}

func hasPrompt(prompts []*mcp.Prompt, name string) bool {
	for _, p := range prompts {
		if p.Name == name {
			return true
		}
	}
	return false
}

func contentText(r *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range r.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

package command

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func demoFS() fstest.MapFS {
	return fstest.MapFS{
		"commit/COMMAND.md": &fstest.MapFile{
			Data: []byte("---\nname: commit\ndescription: Create a conventional-commit message from staged changes.\narguments:\n  - name: scope\n    description: Optional scope for the commit.\n    required: false\n---\nWrite a commit message{{#if scope}} in scope '{{scope}}'{{/if}}.\nAnalyze staged changes.\n"),
		},
		"no-args/COMMAND.md": &fstest.MapFile{
			Data: []byte("---\nname: no-args\ndescription: A command with no arguments.\n---\nRun the thing.\n"),
		},
		"required-arg/COMMAND.md": &fstest.MapFile{
			Data: []byte("---\nname: required-arg\ndescription: requires an arg.\narguments:\n  - name: target\n    description: the target\n    required: true\n---\nDo something to {{target}}.\n"),
		},
	}
}

func TestNew_ParsesCommands(t *testing.T) {
	store, err := New(context.Background(), nil, demoFS())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	names := []string{}
	for _, c := range store.List() {
		names = append(names, c.Name)
	}
	want := []string{"commit", "no-args", "required-arg"}
	if !equalStrings(names, want) {
		t.Errorf("List names = %v, want %v", names, want)
	}
	c, err := store.Get("commit")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(c.Arguments) != 1 || c.Arguments[0].Name != "scope" {
		t.Errorf("Arguments = %+v", c.Arguments)
	}
	if c.Arguments[0].Required {
		t.Errorf("scope should be optional")
	}
}

func TestRender_OptionalArgPresent(t *testing.T) {
	store, _ := New(context.Background(), nil, demoFS())
	out, err := store.Render("commit", map[string]string{"scope": "api"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "in scope 'api'") {
		t.Errorf("rendered = %q", out)
	}
}

func TestRender_OptionalArgAbsent(t *testing.T) {
	store, _ := New(context.Background(), nil, demoFS())
	out, err := store.Render("commit", nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "in scope") {
		t.Errorf("{{#if}} block should collapse when arg absent; got %q", out)
	}
	if !strings.Contains(out, "Write a commit message.") {
		t.Errorf("rendered = %q", out)
	}
}

func TestRender_RequiredArgMissing(t *testing.T) {
	store, _ := New(context.Background(), nil, demoFS())
	if _, err := store.Render("required-arg", nil); err == nil {
		t.Errorf("expected error for missing required arg")
	}
	if _, err := store.Render("required-arg", map[string]string{"target": "  "}); err == nil {
		t.Errorf("expected error for whitespace-only required arg")
	}
}

func TestRender_UnknownCommand(t *testing.T) {
	store, _ := New(context.Background(), nil, demoFS())
	if _, err := store.Render("nope", nil); err == nil {
		t.Errorf("expected error for unknown command")
	}
}

func TestNew_OsDirFS(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "deploy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "COMMAND.md"), []byte("---\nname: deploy\ndescription: Deploy stuff.\narguments:\n  - name: env\n    description: target env\n    required: true\n---\nDeploy to {{env}}.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := New(context.Background(), nil, os.DirFS(tmp))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c, _ := store.Get("deploy"); c == nil {
		t.Fatalf("expected deploy command in %+v", store.List())
	}
	out, err := store.Render("deploy", map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "Deploy to prod") {
		t.Errorf("rendered = %q", out)
	}
}

func TestNew_ParseFailureLogged(t *testing.T) {
	// Missing closing frontmatter delimiter.
	fsys := fstest.MapFS{
		"broken/COMMAND.md": &fstest.MapFile{Data: []byte("---\nname: broken\ndescription: no closing delim\nbody\n")},
	}
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store, err := New(context.Background(), logger, fsys)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expected broken command to be skipped, got %+v", got)
	}
	if !strings.Contains(logBuf.String(), "command skipped") || !strings.Contains(logBuf.String(), "missing frontmatter delimiters") {
		t.Errorf("expected parse failure to be logged, got: %s", logBuf.String())
	}
}

func TestNew_NameMismatchLogged(t *testing.T) {
	fsys := fstest.MapFS{
		"alpha/COMMAND.md": &fstest.MapFile{Data: []byte("---\nname: beta\ndescription: differing name.\n---\nbody\n")},
	}
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store, err := New(context.Background(), logger, fsys)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expected mismatched-name command to be skipped, got %+v", got)
	}
	if !strings.Contains(logBuf.String(), "does not match parent directory") {
		t.Errorf("expected name-mismatch skip to be logged, got: %s", logBuf.String())
	}
}

func TestNew_InvalidNameLogged(t *testing.T) {
	fsys := fstest.MapFS{
		"UPPER/COMMAND.md": &fstest.MapFile{Data: []byte("---\nname: UPPER\ndescription: invalid name.\n---\nbody\n")},
	}
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store, err := New(context.Background(), logger, fsys)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expected invalid-name command to be skipped, got %+v", got)
	}
	if !strings.Contains(logBuf.String(), "characters outside [a-z0-9-]") {
		t.Errorf("expected invalid-name skip to be logged, got: %s", logBuf.String())
	}
}

func TestNew_MultiFS_MergeUserWins(t *testing.T) {
	embedded := fstest.MapFS{
		"shared/COMMAND.md":        &fstest.MapFile{Data: []byte("---\nname: shared\ndescription: shared\n---\nembedded body\n")},
		"embedded-only/COMMAND.md": &fstest.MapFile{Data: []byte("---\nname: embedded-only\ndescription: embedded-only\n---\nembedded\n")},
	}
	user := fstest.MapFS{
		"shared/COMMAND.md":    &fstest.MapFile{Data: []byte("---\nname: shared\ndescription: shared\n---\nuser body\n")},
		"user-only/COMMAND.md": &fstest.MapFile{Data: []byte("---\nname: user-only\ndescription: user-only\n---\nuser\n")},
	}
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store, err := New(context.Background(), logger, embedded, user)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c, err := store.Get("shared")
	if err != nil {
		t.Fatalf("Get shared: %v", err)
	}
	if !strings.Contains(c.Body, "user body") {
		t.Errorf("expected user body to win, got %q", c.Body)
	}
	if !strings.Contains(logBuf.String(), "duplicate command name") {
		t.Errorf("expected duplicate to be logged, got: %s", logBuf.String())
	}
	if _, err := store.Get("embedded-only"); err != nil {
		t.Errorf("embedded-only lost: %v", err)
	}
	if _, err := store.Get("user-only"); err != nil {
		t.Errorf("user-only missing: %v", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

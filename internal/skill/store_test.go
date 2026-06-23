package skill

import (
	"bytes"
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// A minimal embedded fsys with one skill "demo-skill" containing SKILL.md
// and a bundled references/rule.md.
func demoFS() fstest.MapFS {
	return fstest.MapFS{
		"demo-skill/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: demo-skill\ndescription: A demo skill. Use when testing.\nlicense: MIT\nmetadata:\n  author: skillpack\n  version: \"1.0\"\n---\n# Demo Skill\n\nRead [rules](references/rule.md).\n"),
		},
		"demo-skill/references/rule.md": &fstest.MapFile{
			Data: []byte("# Rule\n\nBe excellent.\n"),
		},
		"not-a-skill/README.md": &fstest.MapFile{
			Data: []byte("ignored\n"),
		},
	}
}

func TestNew_ParsesSkillAndResources(t *testing.T) {
	ctx := context.Background()
	store, err := New(ctx, nil, demoFS())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	list := store.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 skill, got %d: %+v", len(list), list)
	}
	e := list[0]
	if e.Name != "demo-skill" {
		t.Errorf("Name = %q, want demo-skill", e.Name)
	}
	if !strings.Contains(e.Description, "Use when testing") {
		t.Errorf("Description = %q", e.Description)
	}

	sk, err := store.Get("demo-skill")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sk.License != "MIT" {
		t.Errorf("License = %q", sk.License)
	}
	if !strings.Contains(sk.Body, "# Demo Skill") {
		t.Errorf("Body = %q", sk.Body)
	}
	wantResources := []string{"references/rule.md"}
	if !reflect.DeepEqual(sk.Resources, wantResources) {
		t.Errorf("Resources = %v, want %v", sk.Resources, wantResources)
	}
	if sk.Metadata["author"] != "skillpack" || sk.Metadata["version"] != "1.0" {
		t.Errorf("Metadata = %v", sk.Metadata)
	}
}

func TestReadResource(t *testing.T) {
	ctx := context.Background()
	store, _ := New(ctx, nil, demoFS())
	data, mime, err := store.ReadResource("demo-skill", "references/rule.md")
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if !strings.Contains(string(data), "Be excellent") {
		t.Errorf("data = %q", data)
	}
	if mime != "text/markdown" {
		t.Errorf("mime = %q, want text/markdown", mime)
	}
}

func TestReadResource_TraversalRejected(t *testing.T) {
	ctx := context.Background()
	store, _ := New(ctx, nil, demoFS())
	for _, p := range []string{"../escape.md", "/etc/passwd", ".."} {
		if _, _, err := store.ReadResource("demo-skill", p); err == nil {
			t.Errorf("expected error for traversal path %q, got nil", p)
		}
	}
}

func TestNew_OsDirFS(t *testing.T) {
	// Build an on-disk skills tree in a temp dir (external-dir path).
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "SKILL.md"), []byte("---\nname: disk-skill\ndescription: Loaded from disk. Use when testing external dirs.\n---\n# Disk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := New(context.Background(), nil, os.DirFS(tmp))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := store.List(); len(got) != 1 || got[0].Name != "disk-skill" {
		t.Fatalf("List = %+v", got)
	}
}

func TestNew_LenientUnquotedColon(t *testing.T) {
	// A description with an unquoted colon that would break strict YAML.
	raw := "---\nname: tricky-skill\ndescription: Use this skill when: the user asks about PDFs\n---\nbody\n"
	fsys := fstest.MapFS{
		"tricky-skill/SKILL.md": &fstest.MapFile{Data: []byte(raw)},
	}
	store, err := New(context.Background(), nil, fsys)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sk, err := store.Get("tricky-skill")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(sk.Description, "PDFs") {
		t.Errorf("Description = %q after lenient parse", sk.Description)
	}
}

func TestNew_MissingDescriptionSkipped(t *testing.T) {
	fsys := fstest.MapFS{
		"empty/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: empty\n---\nbody\n")},
	}
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store, err := New(context.Background(), logger, fsys)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expected skill with empty description to be skipped, got %+v", got)
	}
	// slog TextHandler escapes inner quotes, so match the quote-free substring.
	if !strings.Contains(logBuf.String(), "skill missing required") || !strings.Contains(logBuf.String(), "description") {
		t.Errorf("expected missing-description skip to be logged, got: %s", logBuf.String())
	}
}

func TestNew_MissingNameLogged(t *testing.T) {
	fsys := fstest.MapFS{
		"anon/SKILL.md": &fstest.MapFile{Data: []byte("---\ndescription: A skill without a name.\n---\nbody\n")},
	}
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store, err := New(context.Background(), logger, fsys)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expected nameless skill to be skipped, got %+v", got)
	}
	if !strings.Contains(logBuf.String(), "skill missing required") || !strings.Contains(logBuf.String(), "name") {
		t.Errorf("expected missing-name skip to be logged, got: %s", logBuf.String())
	}
}

func TestNew_ParseFailureLogged(t *testing.T) {
	// No closing frontmatter delimiter.
	fsys := fstest.MapFS{
		"broken/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: broken\ndescription: no closing delim\nbody\n")},
	}
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store, err := New(context.Background(), logger, fsys)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expected unparseable skill to be skipped, got %+v", got)
	}
	if !strings.Contains(logBuf.String(), "skill skipped") || !strings.Contains(logBuf.String(), "malformed frontmatter delimiters") {
		t.Errorf("expected parse failure to be logged, got: %s", logBuf.String())
	}
}

func TestNew_MultiFS_MergeUserWins(t *testing.T) {
	// Embedded: shared-skill (body "embedded body") + embedded-only.
	// User: shared-skill (body "user body", overriding) + user-only.
	embedded := fstest.MapFS{
		"shared-skill/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: shared-skill\ndescription: shared\n---\nembedded body\n"),
		},
		"embedded-only/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: embedded-only\ndescription: from embedded\n---\nembedded\n"),
		},
	}
	user := fstest.MapFS{
		"shared-skill/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: shared-skill\ndescription: shared\n---\nuser body\n"),
		},
		"user-only/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: user-only\ndescription: from user\n---\nuser\n"),
		},
		// Distinct bundled resources so ReadResource proves it reads the right tree.
		"shared-skill/references/from-user.md": &fstest.MapFile{
			Data: []byte("user resource\n"),
		},
	}
	store, err := New(context.Background(), nil, embedded, user)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sk, err := store.Get("shared-skill")
	if err != nil {
		t.Fatalf("Get shared-skill: %v", err)
	}
	if !strings.Contains(sk.Body, "user body") {
		t.Errorf("expected user body to shadow embedded; got %q", sk.Body)
	}
	// Resource read must come from the user tree (embedded one has no resources).
	data, _, err := store.ReadResource("shared-skill", "references/from-user.md")
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if !strings.Contains(string(data), "user resource") {
		t.Errorf("resource data = %q", data)
	}
	// Both uniques survive.
	if _, err := store.Get("embedded-only"); err != nil {
		t.Errorf("embedded-only lost after merge: %v", err)
	}
	if _, err := store.Get("user-only"); err != nil {
		t.Errorf("user-only missing after merge: %v", err)
	}
}

func TestValidName(t *testing.T) {
	cases := map[string]bool{
		"pdf-processing":        true,
		"data-analysis":         true,
		"code-review":           true,
		"a":                     true,
		"-pdf":                  false,
		"pdf-":                  false,
		"pdf--processing":       false,
		"PDF-Processing":        false,
		"":                      false,
		strings.Repeat("a", 65): false,
	}
	for n, want := range cases {
		if got := validName(n); got != want {
			t.Errorf("validName(%q) = %v, want %v", n, got, want)
		}
	}
}

// Compile-time assertion that fstest.MapFS satisfies fs.FS.
var _ fs.FS = fstest.MapFS{}

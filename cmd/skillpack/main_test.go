package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	skillpack "github.com/marstid/skillpack"
)

func TestResolveSourceList_EmbeddedOnly(t *testing.T) {
	fses, label, merge, err := resolveSourceList(skillpack.SkillsFS, "skills", "", false)
	if err != nil {
		t.Fatalf("resolveSourceList: %v", err)
	}
	if len(fses) != 1 {
		t.Fatalf("expected 1 fs, got %d", len(fses))
	}
	if label != "embedded" {
		t.Errorf("label = %q, want embedded", label)
	}
	if merge {
		t.Errorf("merge should be false when override empty")
	}
	// The embedded sub-FS must contain the demo markdown-lint skill.
	if _, err := fs.Stat(fses[0], prospectSkillMarker); err != nil {
		t.Errorf("embedded FS missing markdown-lint SKILL.md: %v", err)
	}
}

const prospectSkillMarker = "markdown-lint/SKILL.md"

func TestResolveSourceList_OverrideReplacesEmbedded(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "SKILL.md"), []byte("---\nname: disk-only\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fses, label, merge, err := resolveSourceList(skillpack.SkillsFS, "skills", tmp, false)
	if err != nil {
		t.Fatalf("resolveSourceList: %v", err)
	}
	if len(fses) != 1 {
		t.Fatalf("override should replace, expected 1 fs, got %d", len(fses))
	}
	if label != tmp {
		t.Errorf("label = %q, want %q", label, tmp)
	}
	if merge {
		t.Errorf("merge should be false when --merge-skills not set")
	}
	// Replacement FS must NOT expose the embedded markdown-lint skill.
	if _, err := fs.Stat(fses[0], prospectSkillMarker); err == nil {
		t.Errorf("override should fully replace embedded tree; markdown-lint should not be visible")
	}
}

func TestResolveSourceList_MergeUserWinsOrder(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "SKILL.md"), []byte("---\nname: disk-only\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fses, label, merge, err := resolveSourceList(skillpack.SkillsFS, "skills", tmp, true)
	if err != nil {
		t.Fatalf("resolveSourceList: %v", err)
	}
	if len(fses) != 2 {
		t.Fatalf("merge should produce 2 fs (embedded + user), got %d", len(fses))
	}
	if label != tmp {
		t.Errorf("label = %q, want %q", label, tmp)
	}
	if !merge {
		t.Errorf("merge should be true")
	}
	// First fs is embedded (has markdown-lint); second fs is user (has disk-only SKILL.md at root).
	if _, err := fs.Stat(fses[0], prospectSkillMarker); err != nil {
		t.Errorf("merge: first fs should be embedded with markdown-lint; missing: %v", err)
	}
	if _, err := fs.Stat(fses[1], "SKILL.md"); err != nil {
		t.Errorf("merge: second fs should be user dir with root SKILL.md; missing: %v", err)
	}
}

func TestResolveSourceList_NotADirectory(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "notadir.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := resolveSourceList(skillpack.SkillsFS, "skills", file, false); err == nil {
		t.Errorf("expected error for file passed as dir")
	}
}

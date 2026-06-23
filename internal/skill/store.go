package skill

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// SkillName is the dotted identifier of a skill as declared in its frontmatter.
type SkillName = string

// CatalogEntry is the tier-1 disclosure record: just enough to advertise a
// skill to the model without loading its instructions.
type CatalogEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Skill is the parsed representation of a single skill directory.
type Skill struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	Metadata      map[string]string
	AllowedTools  string
	Body          string
	// BaseDir is the skill's path within the source fs.FS (e.g.
	// "skills/markdown-lint"). It is used to resolve relative resource paths.
	BaseDir string
	// Resources lists bundled files (scripts/, references/, assets/)
	// discovered under the skill directory, as paths relative to BaseDir.
	Resources []string
	// Diagnostics records non-fatal parse/validation warnings.
	Diagnostics []string
	// fsys is the originating filesystem the skill was loaded from. Resource
	// reads resolve against it so multi-source stores read the correct tree.
	fsys fs.FS
}

// Store is an in-memory catalog of parsed skills, keyed by skill name. It is
// safe for concurrent reads after New returns.
type Store struct {
	skills map[SkillName]*Skill
	// names preserves a stable iteration order.
	names []SkillName
}

// New walks each given fs.FS in order, parsing every directory that contains a
// SKILL.md file. When the same skill name appears in multiple sources, the
// later source wins (last-wins), so callers should pass the embedded tree
// first and any user overrides last. Validation failures are logged at Warn
// level via log (slog.Default() is used when log is nil); unrecoverable skills
// are skipped so they never appear in List.
func New(ctx context.Context, log *slog.Logger, fsyses ...fs.FS) (*Store, error) {
	if log == nil {
		log = slog.Default()
	}
	s := &Store{skills: make(map[SkillName]*Skill)}
	for _, fsys := range fsyses {
		if err := s.loadOne(ctx, log, fsys); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// loadOne walks a single fs.FS (rooted at the skills tree) and absorbs every
// SKILL.md file it finds.
func (s *Store) loadOne(ctx context.Context, log *slog.Logger, fsys fs.FS) error {
	// Walk the tree. fsys is rooted at the skills dir; each immediate child
	// is a skill dir. We still walk to support arbitrary nesting and to find
	// SKILL.md files reliably.
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if p == "." || filepath.Base(p) != "SKILL.md" {
			return nil
		}
		// Load the SKILL.md file.
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			ctxErr := fmt.Errorf("read SKILL.md %q: %w", p, err)
			log.Warn("skill skipped", "skill_dir", path.Dir(p), "reason", ctxErr.Error())
			return ctxErr
		}
		sk := &Skill{BaseDir: path.Dir(p), fsys: fsys}
		// Compute the skill directory name (sibling of SKILL.md). The spec
		// says the frontmatter name should equal the parent dir name; we
		// parse the frontmatter first, then cross-check.
		dirName := filepath.Base(sk.BaseDir)
		parsed, diags, ok := parseSkill(raw)
		if !ok {
			log.Warn("skill skipped", "skill_dir", sk.BaseDir, "reason", strings.Join(diags, "; "))
			return nil
		}
		sk.Name = parsed.FM.Name
		sk.Description = strings.TrimSpace(parsed.FM.Description)
		sk.License = parsed.FM.License
		sk.Compatibility = strings.TrimSpace(parsed.FM.Compatibility)
		sk.Metadata = parsed.FM.Metadata
		sk.AllowedTools = parsed.FM.AllowedTools
		sk.Body = strings.TrimSpace(parsed.Body)
		sk.Diagnostics = diags

		// Lenient validation per the agentskills.io client guide. Sequential
		// (not switch) so every applicable diagnostic is recorded; the first
		// two are fatal, the rest are non-fatal warnings.
		if sk.Name == "" {
			log.Warn("skill skipped", "skill_dir", sk.BaseDir, "reason", `skill missing required "name"`)
			return nil
		}
		if sk.Description == "" {
			log.Warn("skill skipped", "skill_dir", sk.BaseDir, "skill_name", sk.Name, "reason", `skill missing required "description"`)
			return nil
		}
		if sk.Name != dirName {
			sk.Diagnostics = append(sk.Diagnostics, fmt.Sprintf("name %q does not match parent directory %q", sk.Name, dirName))
		}
		if len(sk.Name) > 64 {
			sk.Diagnostics = append(sk.Diagnostics, "name exceeds 64 characters")
		}
		if !validName(sk.Name) {
			sk.Diagnostics = append(sk.Diagnostics, "name contains characters outside [a-z0-9-] or has invalid hyphen placement")
		}

		// Discover bundled resources (scripts/, references/, assets/ and
		// any other files) relative to the skill dir.
		res, err := discoverResources(fsys, sk.BaseDir)
		if err != nil {
			return fmt.Errorf("discover resources for %q: %w", sk.Name, err)
		}
		sk.Resources = res
		sort.Strings(sk.Resources)

		// Insert; last-wins on name collisions, recording a diagnostic so
		// callers can surface it.
		if _, exists := s.skills[sk.Name]; exists {
			sk.Diagnostics = append(sk.Diagnostics, "duplicate skill name — previous entry shadowed")
		}
		s.skills[sk.Name] = sk

		// Maintain insertion order only for newly-seen names; for shadows we
		// keep the original position.
		if !contains(s.names, sk.Name) {
			s.names = append(s.names, sk.Name)
		}

		// Surface any non-fatal diagnostics for a successfully loaded skill.
		if len(sk.Diagnostics) > 0 {
			log.Warn("skill loaded with diagnostics", "skill_dir", sk.BaseDir, "skill_name", sk.Name, "diagnostics", sk.Diagnostics)
		}
		return nil
	})
	return err
}

// List returns the tier-1 catalog of all loaded skills, in insertion order.
// Skill names whose essential fields were missing were skipped during New and
// do not appear here.
func (s *Store) List() []CatalogEntry {
	out := make([]CatalogEntry, 0, len(s.names))
	for _, n := range s.names {
		sk := s.skills[n]
		out = append(out, CatalogEntry{Name: sk.Name, Description: sk.Description})
	}
	return out
}

// Search returns catalog entries whose name or description fuzzy-match the
// query using case-insensitive subsequence matching with a position-based
// score. Lower scores rank earlier; insertion order breaks ties. An empty
// query returns the full catalog in insertion order (no filtering, no
// reordering). The search is implemented in internal/skill/search.go.
func (s *Store) Search(query string) []CatalogEntry {
	return search(s, query)
}

// Get returns the full skill record (tier-2 instructions + resources), or
// fs.ErrNotExist-equivalent error if no skill with that name is loaded.
func (s *Store) Get(name SkillName) (*Skill, error) {
	sk, ok := s.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return sk, nil
}

// ReadResource returns the bytes and guessed MIME type of a bundled file
// within a skill. relpath is interpreted relative to the skill's BaseDir. It
// must not be absolute or contain ".." segments.
func (s *Store) ReadResource(name SkillName, relpath string) ([]byte, string, error) {
	sk, err := s.Get(name)
	if err != nil {
		return nil, "", err
	}
	cleaned := path.Clean(relpath)
	if filepath.IsAbs(relpath) || strings.HasPrefix(cleaned, "..") || cleaned == ".." {
		return nil, "", fmt.Errorf("refusing to read resource with traversal path %q", relpath)
	}
	if cleaned == "." || cleaned == "" || cleaned == "/" {
		return nil, "", fmt.Errorf("empty or invalid resource path %q", relpath)
	}
	// Resolve against the originating filesystem so multi-source stores read
	// the correct tree for a skill that shadowed an earlier entry.
	full := path.Join(sk.BaseDir, cleaned)
	data, err := fs.ReadFile(sk.fsys, full)
	if err != nil {
		return nil, "", err
	}
	return data, mimeTypeFor(cleaned), nil
}

// validName reports whether n matches the agentskills.io name constraints:
// lowercase alphanumerics and hyphens only, no leading/trailing/double hyphens,
// length 1..64.
func validName(n string) bool {
	if n == "" || len(n) > 64 {
		return false
	}
	if n[0] == '-' || n[len(n)-1] == '-' {
		return false
	}
	if strings.Contains(n, "--") {
		return false
	}
	for _, r := range n {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

func contains(seq []SkillName, v SkillName) bool {
	for _, x := range seq {
		if x == v {
			return true
		}
	}
	return false
}

// discoverResources enumerates files below the skill dir that are not the
// SKILL.md file itself. Returned paths are relative to the skill dir.
func discoverResources(fsys fs.FS, baseDir string) ([]string, error) {
	var res []string
	err := fs.WalkDir(fsys, baseDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if p == path.Join(baseDir, "SKILL.md") {
			return nil
		}
		rel, rerr := filepath.Rel(baseDir, p)
		if rerr != nil {
			rel = p // fallback; shouldn't happen under WalkDir
		}
		res = append(res, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// mimeTypeFor guesses a content type from a filename extension. It covers the
// common bundled-resource types; unknown extensions default to text/plain.
func mimeTypeFor(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md":
		return "text/markdown"
	case ".txt", "":
		return "text/plain"
	case ".sh":
		return "application/x-sh"
	case ".py":
		return "text/x-python"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js", ".mjs":
		return "text/javascript"
	case ".ts":
		return "text/typescript"
	case ".go":
		return "text/x-go"
	case ".xml":
		return "application/xml"
	case ".csv":
		return "text/csv"
	case ".zip":
		return "application/zip"
	default:
		return "text/plain"
	}
}

// Package command loads COMMAND.md files from an fs.FS and renders them as
// parameterized prompt templates. Commands are skillpack's answer to MCP prompts:
// each command maps 1:1 to an MCP prompt whose arguments come from the
// COMMAND.md frontmatter.
package command

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
)

// Arg is one declared template argument of a command.
type Arg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// Command is the parsed representation of a single COMMAND.md file.
type Command struct {
	Name        string
	Description string
	Arguments   []Arg
	Body        string // raw body, before argument rendering
	// BaseDir is the path within the source fs.FS to the command directory
	// (e.g. "commands/commit").
	BaseDir string
}

// Store is an in-memory catalog of parsed commands, keyed by command name.
type Store struct {
	commands map[string]*Command
	names    []string
}

// New walks each given fs.FS in order, parsing every directory that contains a
// COMMAND.md file. Later sources win on duplicate names, so callers should pass
// the embedded tree first and user overrides last. Validation failures are
// logged at Warn level via log (slog.Default() is used when log is nil); the
// offending command is skipped and never appears in List.
func New(ctx context.Context, log *slog.Logger, fsyses ...fs.FS) (*Store, error) {
	if log == nil {
		log = slog.Default()
	}
	s := &Store{commands: make(map[string]*Command)}
	for _, fsys := range fsyses {
		if err := s.loadOne(ctx, log, fsys); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// loadOne walks a single fs.FS (rooted at the commands tree) and absorbs every
// COMMAND.md file it finds.
func (s *Store) loadOne(ctx context.Context, log *slog.Logger, fsys fs.FS) error {
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Base(p) != "COMMAND.md" {
			return nil
		}
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			lsErr := fmt.Errorf("read COMMAND.md %q: %w", p, err)
			log.Warn("command skipped", "command_dir", path.Dir(p), "reason", lsErr.Error())
			return lsErr
		}
		dirName := filepath.Base(path.Dir(p))
		cmd, err := parseCommand(raw)
		if err != nil {
			log.Warn("command skipped", "command_dir", path.Dir(p), "reason", err.Error())
			return nil
		}
		cmd.BaseDir = path.Dir(p)
		// Backfill name from dir if frontmatter omitted it.
		if cmd.Name == "" {
			cmd.Name = dirName
		}
		if cmd.Name != dirName {
			log.Warn("command skipped", "command_dir", path.Dir(p), "command_name", cmd.Name, "reason", fmt.Sprintf("name %q does not match parent directory %q", cmd.Name, dirName))
			return nil
		}
		if !validName(cmd.Name) {
			log.Warn("command skipped", "command_dir", path.Dir(p), "command_name", cmd.Name, "reason", "name contains characters outside [a-z0-9-] or has invalid hyphen placement")
			return nil
		}
		if _, exists := s.commands[cmd.Name]; exists {
			log.Warn("command shadowed by duplicate", "command_dir", path.Dir(p), "command_name", cmd.Name, "reason", "duplicate command name — previous entry shadowed")
		}
		s.commands[cmd.Name] = cmd
		if !containsString(s.names, cmd.Name) {
			s.names = append(s.names, cmd.Name)
		}
		return nil
	})
	return err
}

// List returns all loaded commands, in insertion order.
func (s *Store) List() []*Command {
	out := make([]*Command, 0, len(s.names))
	for _, n := range s.names {
		out = append(out, s.commands[n])
	}
	return out
}

// Get returns the command with the given name, or an error if not found.
func (s *Store) Get(name string) (*Command, error) {
	c, ok := s.commands[name]
	if !ok {
		return nil, fmt.Errorf("command %q not found", name)
	}
	return c, nil
}

// Render substitutes the provided args into the command's body template and
// returns the resulting prompt text. Required args that are missing cause an
// error. Optional args that are unset are simply omitted (the {{#if arg}}...
// {{/if}} blocks collapse).
func (s *Store) Render(name string, args map[string]string) (string, error) {
	c, err := s.Get(name)
	if err != nil {
		return "", err
	}
	for _, a := range c.Arguments {
		if a.Required {
			if v, ok := args[a.Name]; !ok || strings.TrimSpace(v) == "" {
				return "", fmt.Errorf("missing required argument %q for command %q", a.Name, name)
			}
		}
	}
	return renderTemplate(c.Body, args), nil
}

// renderTemplate supports two directives:
//   - {{arg}}         — substituted with the arg value (empty if missing)
//   - {{#if arg}}..{{/if}} — body kept only if arg is present and non-empty
//
// Both forms are matched without nesting support in v1.
func renderTemplate(body string, args map[string]string) string {
	var b strings.Builder
	i := 0
	for i < len(body) {
		// {{#if cond}} ... {{/if}}
		if strings.HasPrefix(body[i:], "{{#if ") {
			close := strings.Index(body[i:], "}}")
			if close < 0 {
				b.WriteString(body[i:])
				break
			}
			field := strings.TrimSpace(body[i+len("{{#if ") : i+close])
			endifIdx := strings.Index(body[i+close+2:], "{{/if}}")
			if endifIdx < 0 {
				b.WriteString(body[i:])
				break
			}
			inner := body[i+close+2 : i+close+2+endifIdx]
			v, present := args[field]
			if present && strings.TrimSpace(v) != "" {
				b.WriteString(renderTemplate(inner, args))
			}
			i += close + 2 + endifIdx + len("{{/if}}")
			continue
		}
		// {{arg}}
		if strings.HasPrefix(body[i:], "{{") {
			close := strings.Index(body[i:], "}}")
			if close < 0 {
				b.WriteString(body[i:])
				break
			}
			field := strings.TrimSpace(body[i+2 : i+close])
			if strings.HasPrefix(field, "#if ") || strings.HasPrefix(field, "/if") {
				b.WriteString(body[i:])
				break
			}
			b.WriteString(args[field])
			i += close + 2
			continue
		}
		b.WriteByte(body[i])
		i++
	}
	return b.String()
}

// containsString reports whether seq contains v.
func containsString(seq []string, v string) bool {
	for _, x := range seq {
		if x == v {
			return true
		}
	}
	return false
}

// validName mirrors the skill name constraints (lowercase alnum + hyphens).
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

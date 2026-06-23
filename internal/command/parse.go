package command

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// commandFrontmatter is the YAML block at the top of a COMMAND.md file.
type commandFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Arguments   []Arg  `yaml:"arguments"`
}

// parseCommand splits a COMMAND.md file into frontmatter and body, parses the
// frontmatter, and returns a populated Command. The body is stored verbatim
// and rendered later by Store.Render.
func parseCommand(raw []byte) (*Command, error) {
	fmBytes, body, ok := splitFrontmatter(raw)
	if !ok {
		return nil, fmt.Errorf("missing frontmatter delimiters")
	}
	var fm commandFrontmatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return nil, fmt.Errorf("unparseable COMMAND.md frontmatter: %w", err)
	}
	// Trim name/description; arguments are validated by name in validName.
	c := &Command{
		Name:        fm.Name,
		Description: fm.Description,
		Arguments:   fm.Arguments,
		Body:        body,
	}
	return c, nil
}

// splitFrontmatter mirrors the skill parser's separator logic: a leading
// `---` line, YAML content, a closing `---` line, then the body. Generously
// duplicated here so the command package has no upward dependency on the
// skill package; the two parsers are intentionally independent.
func splitFrontmatter(raw []byte) (fm []byte, body string, ok bool) {
	trimmed := bytes.TrimPrefix(raw, []byte("\ufeff"))
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return nil, "", false
	}
	rest := trimmed[len("---"):]
	rest = bytes.TrimPrefix(rest, []byte("\r\n"))
	if !bytes.HasPrefix(rest, []byte("\n")) {
		return nil, "", false
	}
	rest = rest[len("\n"):]

	closeIdx := bytes.Index(rest, []byte("\n---"))
	if closeIdx < 0 {
		if !bytes.HasSuffix(rest, []byte("---")) {
			return nil, "", false
		}
		fm = rest[:len(rest)-len("---")]
		body = ""
	} else {
		fm = rest[:closeIdx]
		after := rest[closeIdx+len("\n---"):]
		after = bytes.TrimPrefix(after, []byte("\r\n"))
		after = bytes.TrimPrefix(after, []byte("\n"))
		body = string(after)
	}
	fm = bytes.TrimRight(fm, "\r\n")
	return fm, body, true
}

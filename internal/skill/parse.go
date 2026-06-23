package skill

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontmatter is the set of SKILL.md frontmatter fields we understand. It
// matches the Agent Skills specification. Only Name and Description are
// required.
type frontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
}

// parsedSkill holds the parsed frontmatter plus the markdown body that follows
// the closing `---` delimiter.
type parsedSkill struct {
	FM   frontmatter
	Body string
}

// splitFrontmatter separates a SKILL.md file into its YAML frontmatter block
// and the remaining markdown body. It returns the raw frontmatter bytes (the
// text between the opening and closing `---` lines), the body, and ok=false
// if the file does not start with a `---` delimiter.
func splitFrontmatter(raw []byte) (fm []byte, body string, ok bool) {
	// Accept leading BOM.
	trimmed := bytes.TrimPrefix(raw, []byte("\ufeff"))
	// A frontmatter block starts at the very first byte with `---` on its own
	// line.
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return nil, "", false
	}
	rest := trimmed[len("---"):]
	// The opening line must end the line (only a newline, optional CR).
	rest = bytes.TrimPrefix(rest, []byte("\r\n"))
	if !bytes.HasPrefix(rest, []byte("\n")) {
		return nil, "", false
	}
	rest = rest[len("\n"):]

	// Find the closing delimiter: a line whose content is exactly `---`
	// (optionally `...`, but stick to `---` per spec).
	closeIdx := bytes.Index(rest, []byte("\n---"))
	if closeIdx < 0 {
		// Closing delimiter may be at EOF without a trailing newline.
		if !bytes.HasSuffix(rest, []byte("---")) {
			return nil, "", false
		}
		fm = rest[:len(rest)-len("---")]
		body = ""
	} else {
		fm = rest[:closeIdx]
		// Skip the newline + `---` and any trailing line terminator.
		after := rest[closeIdx+len("\n---"):]
		after = bytes.TrimPrefix(after, []byte("\r\n"))
		after = bytes.TrimPrefix(after, []byte("\n"))
		body = string(after)
	}
	fm = bytes.TrimRight(fm, "\r\n")
	return fm, body, true
}

// parseSkill parses raw SKILL.md bytes into frontmatter + body. It is lenient:
// if strict YAML parsing fails, it retries with a heuristic that quotes bare
// scalar values containing colons (a common cross-author issue called out by
// the agentskills.io client guide). ok=false indicates the file should be
// skipped.
func parseSkill(raw []byte) (parsedSkill, []string, bool) {
	var diags []string
	fmBytes, body, ok := splitFrontmatter(raw)
	if !ok {
		return parsedSkill{}, append(diags, "missing or malformed frontmatter delimiters"), false
	}
	var fm frontmatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		// Retry with the unquoted-colon fallback.
		fixed := quoteUnquotedColons(fmBytes)
		var fm2 frontmatter
		if err2 := yaml.Unmarshal(fixed, &fm2); err2 != nil {
			return parsedSkill{}, append(diags, fmt.Sprintf("unparseable YAML frontmatter: %v", err)), false
		}
		diags = append(diags, "frontmatter required lenient YAML re-parse")
		fm = fm2
	}
	return parsedSkill{FM: fm, Body: body}, diags, true
}

// quoteUnquotedColons is a best-effort repair for lines of the shape
// `key: value:with:colon` where the value is not already quoted. It wraps such
// values in double quotes. It deliberately ignores:
//   - block scalars (`|` or `>`) and lines already starting with a quote
//   - lines that look like list items or nested mappings
//   - values inside the metadata map (which may legitimately contain colons
//     across multiple lines)
//
// This is intentionally conservative; it only fixes the single most common
// parser failure mode described in the spec guide.
func quoteUnquotedColons(b []byte) []byte {
	var out strings.Builder
	for _, line := range bytes.Split(b, []byte("\n")) {
		written := false
		if s := strings.TrimSpace(string(line)); s != "" &&
			!strings.HasPrefix(s, "-") &&
			!strings.HasPrefix(s, "#") &&
			!strings.HasPrefix(s, "\"") &&
			!strings.HasPrefix(s, "'") &&
			!strings.HasPrefix(s, "|") &&
			!strings.HasPrefix(s, ">") {
			if idx := strings.IndexByte(s, ':'); idx > 0 {
				key := s[:idx]
				rest := s[idx+1:]
				// Only single-line scalar values that themselves contain a
				// colon and aren't already quoted.
				val := strings.TrimSpace(rest)
				if val != "" && strings.Contains(val, ":") &&
					!strings.HasPrefix(rest, "\"") &&
					!strings.HasPrefix(rest, "'") {
					// Preserve leading indentation.
					indent := s[:idx-len(key)]
					_ = indent
					// Reconstruct preserving original indentation (key length
					// already excludes indentation; compute from line).
					origIndent := len(line) - len(strings.TrimLeft(string(line), " "))
					fmt.Fprintf(&out, "%s%s: %q\n", strings.Repeat(" ", origIndent), strings.TrimSpace(key), val)
					written = true
				}
			}
		}
		if !written {
			out.Write(line)
			out.WriteByte('\n')
		}
	}
	return []byte(out.String())
}

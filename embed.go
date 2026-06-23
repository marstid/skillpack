// Package skillpack exposes the embedded skills and commands trees.
//
// The directives below compile the on-disk skills/ and commands/ directories
// into the binary at build time. The FS values are rooted at the module root,
// so callers typically take a sub-FS rooted at "skills" or "commands" via
// fs.Sub, or pass an os.DirFS with the same root when overriding at runtime
// via the --skills-dir / --commands-dir flags.
package skillpack

import "embed"

//go:embed all:skills
var SkillsFS embed.FS

//go:embed all:commands
var CommandsFS embed.FS

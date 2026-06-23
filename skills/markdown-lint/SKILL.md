---
name: markdown-lint
description: Lint markdown files for common issues (heading structure, trailing whitespace, broken links). Use when the user asks to check, lint, or validate markdown documentation.
license: Apache-2.0
compatibility: Requires no external tools
metadata:
  author: skillpack
  version: "1.0"
allowed-tools: Read Grep Glob
---
# Markdown Lint

Use this skill when the user wants to lint or validate Markdown files.

## When to use

- The user asks to "lint", "check", or "validate" markdown.
- The user mentions documentation quality or formatting issues.

## Steps

1. Use Glob to find `**/*.md` files in the requested scope.
2. For each file, read it and check the rules in [references/rules.md](references/rules.md).
3. Report findings grouped by file, with line references.

Keep reports concise: one line per finding, severity-prefixed (`error:`, `warning:`, `info:`).
Resources for this skill live in the skill directory; relative paths in the rules file resolve there.
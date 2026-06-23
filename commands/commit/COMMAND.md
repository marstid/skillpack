---
name: commit
description: Create a Conventional Commits message from staged changes.
arguments:
  - name: scope
    description: Optional scope for the commit.
    required: false
---
Write a commit message for the staged changes{{#if scope}} in scope '{{scope}}'{{/if}}.

Steps:
1. Run `git diff --cached` to inspect staged changes.
2. Determine the type (feat, fix, docs, refactor, chore, test, perf, build, ci, style).
3. Summarize the change in the imperative mood for the subject line (max 72 chars).
4. Add a body explaining the *why* and notable *what*, wrapped at 72 columns.
5. Propose a single complete commit message in a fenced code block. Do not run git commit.
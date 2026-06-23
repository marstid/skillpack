# Markdown Lint Rules

The following rules are applied in order. Stop a file's report at the first *error*.

## Errors

- `E1`: A heading level is skipped (e.g. `# Title` followed by `### Sub`).
- `E2`: A code fence is opened but never closed.
- `E3`: A link target is empty: `[]()` or `][`.

## Warnings

- `W1`: Trailing whitespace on a line.
- `W2`: Line longer than 120 characters (excluding code fences).
- `W3`: A list has inconsistent bullet style (mixing `-` and `*`).

## Info

- `I1`: Document has no top-level `#` heading.
- `I2`: More than three consecutive blank lines.
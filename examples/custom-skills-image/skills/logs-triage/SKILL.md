---
name: logs-triage
description: Triage and investigate log-based incidents. Use when the user asks to debug errors, spikes, or anomalies in log data.
license: MIT
compatibility: Requires a logs-search MCP tool (e.g. Datadog logs)
metadata:
  author: example
  version: "1.0"
allowed-tools: Read Grep
---
# Logs Triage

Use this skill when the user wants to investigate errors, spikes, or anomalies
in log data — typically through a logs-search MCP tool.

## When to use

- "Why are we seeing more 5xx errors in the last hour?"
- "Investigate the spike in checkout failures."
- A monitor or alert points at a log-stream anomaly.

## Steps

1. Narrow the **time range** to the incident window (e.g. `*last 30m`).
2. Start from a **service filter**: `service:<name> status:error`.
3. Add facets one at a time — avoid loading broad queries up front.
4. Group by the most selective facet first (`@http.status_code`, `host`, `version`).
5. When you find a representative line, pivot to the **trace** if available.
6. Read [query syntax](references/query-syntax.md) for the supported operators.

## Pitfalls

- Avoid `*` over multi-hour windows — the query will time out or lose recent data.
- Free-text queries (`"payment failed"`) match indexed text only; use facet filters
  for structured fields like status codes or host names.

Resources for this skill live in the skill directory; relative paths resolve there.
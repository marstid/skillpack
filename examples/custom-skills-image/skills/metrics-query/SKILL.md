---
name: metrics-query
description: Aggregate and query metric time series. Use when the user asks to chart, compare, or alert on numeric metrics over time.
license: MIT
compatibility: Requires a metrics-search MCP tool (e.g. Datadog metrics)
metadata:
  author: example
  version: "1.0"
allowed-tools: Read Grep
---
# Metrics Query

Use this skill when the user wants to chart, compare, or set up alerts on
numeric metrics — counters, gauges, or distributions — over a time window.

## When to use

- "Plot p95 latency for the checkout service over the last hour."
- "Compare error rates across the last 3 deploys."
- "Set an alert on request volume dropping below 50% of the 7-day median."

## Steps

1. Identify the **metric name** and its **type** (count, gauge, rate,
   distribution). Distributions use `p50`, `p95`, `p99` aggregates; gauges use
   `avg`, `max`, `min`.
2. Scope with **tags**: `service:checkout`, `env:prod`, `version:<sha>`.
3. Pick the **aggregation window** (`rollup`) to balance resolution vs cost —
   `rollup(60, avg)` for 1-minute buckets, `rollup(300, max)` for 5-minute
   peaks.
4. Compare across tags with `by` clauses: `sum:rate{service:checkout} by
   {host}`.

## Pitfalls

- Don't sum rates across time — sum the underlying count, then divide by the
  window. Summing rates double-counts.
- Distributions lose accuracy when rolled up coarsely — keep `rollup` windows
  under the alert threshold's resolution.

Resources for this skill live in the skill directory; relative paths resolve
there.
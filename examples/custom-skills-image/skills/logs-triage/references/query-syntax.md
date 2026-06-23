# Query syntax reference

Compact reference for the logs-search MCP tool's query language used by the
logs-triage skill.

## Operators

| Operator | Example | Meaning |
|----------|---------|---------|
| `:` | `service:checkout` | Equality facet filter. |
| `*` | `*last 1h` | Wildcard time range (use short windows). |
| `NOT` | `NOT status:200` | Negation. |
| `AND` | `service:checkout AND status:error` | Conjunction (implicit between facet filters). |
| `OR` | `status:500 OR status:503` | Disjunction (needs explicit `OR`). |
| `@` | `@http.status_code:500` | Reserved facet prefix for structured fields. |

## Time range

- `*last 5m`, `*last 1h`, `*last 24h` — relative.
- `*2026-06-23T12:00:00Z/2026-06-23T13:00:00Z` — absolute window.

Keep windows narrow — start at 30 minutes around the incident, broaden only
if the signal is sparse.

## Grouping

Group by the most selective facet first to keep result tables small:
`groupby:@http.status_code`, `groupby:host`, `groupby:version`.
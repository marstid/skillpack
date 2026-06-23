---
name: incident-report
description: Render a structured incident report from a short user description. Use when the user asks to summarize an ongoing or recent incident.
arguments:
  - name: severity
    description: Incident severity (e.g. SEV1, SEV2, SEV3).
    required: true
  - name: service
    description: The affected service or component. Optional.
    required: false
---

Write an incident report for a {{severity}} incident{{#if service}} affecting the {{service}} service{{/if}}.

Sections:
1. **Summary** — one paragraph: what happened, blast radius, and current status.
2. **Timeline** — reverse-chronological, UTC timestamps. Include detection,
   acknowledgment, mitigation, and resolution (if known).
3. **Impact** — users/requests affected, duration, any SLO burn.
4. **Root cause** — best hypothesis with confidence level; mark as "under
   investigation" if unknown.
5. **Action items** — owners and due dates for remediation work.

Keep the report factual and terse. No speculation presented as fact.
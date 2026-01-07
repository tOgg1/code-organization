---
id: code-organization-408.6
status: closed
deps: []
links: []
created: 2025-12-17T08:28:19.835130935+01:00
type: feature
priority: 2
parent: code-organization-408
---
# Sync: persist per-workspace exclude patterns

Add a way to persist sync excludes per workspace (likely in project.json) and have co sync automatically include them. Interactive picker should optionally offer 'save for next time' so users don't need to re-select exclusions every run.

## Acceptance Criteria

When project.json defines sync excludes, co sync uses them by default. Interactive picker can save updated excludes and they are picked up on subsequent sync runs.



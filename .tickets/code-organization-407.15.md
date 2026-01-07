---
id: code-organization-407.15
status: closed
deps: [code-organization-407.4]
links: []
created: 2025-12-14T16:20:39.611632+01:00
type: task
priority: 2
parent: code-organization-407
---
# Testing template listing/source + global precedence

Add/extend unit tests around the new template discovery helpers and any file-listing/mapping logic introduced for the Template Explorer TUI.

Focus on deterministic behavior across multiple template dirs and on _global precedence.

## Acceptance Criteria

- Tests cover multi-dir template precedence (first wins)
- Tests cover source-dir reporting for templates
- Tests cover _global merge precedence (first wins) where applicable



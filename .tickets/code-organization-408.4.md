---
id: code-organization-408.4
status: closed
deps: []
links: []
created: 2025-12-17T08:28:04.574960871+01:00
type: feature
priority: 1
parent: code-organization-408
---
# Sync: interactive exclude picker TUI

Add co sync --interactive (-i): before syncing, launch a Bubble Tea TUI that shows the workspace filesystem as a collapsible tree. Space toggles exclude/include for the selected path (dirs exclude entire subtree). Default excludes start pre-selected. After confirmation, run sync with the selected exclude list (applies to rsync + tar fallback).

## Acceptance Criteria

Interactive mode supports: expand/collapse dirs, toggle exclude with space, clear/reset, confirm-to-sync, and cancel. Excluding a directory prevents syncing anything under it.



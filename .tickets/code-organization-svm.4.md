---
id: code-organization-svm.4
status: closed
deps: [code-organization-407.4, code-organization-407.6]
links: []
created: 2025-12-14T16:23:37.089384+01:00
type: task
priority: 3
parent: code-organization-svm
---
# Compare two templates (manifest + file list)

Add a compare mode:
- Pick two templates
- Show diff for manifest summary (vars/repos/hooks) and file list differences
- Highlight overrides (same output path, different sources)

This is inspection-only and does not modify templates.

## Design

Start with a simple compare: set diffs for variable names, repo names, hook scripts, and effective output paths.

A full textual diff is optional; initial implementation can focus on structured diffs.

## Acceptance Criteria

- User can select two templates to compare
- Compare view summarizes differences clearly (added/removed/changed)
- Navigation allows jumping into file viewer for relevant files



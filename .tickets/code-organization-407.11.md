---
id: code-organization-407.11
status: closed
deps: [code-organization-407.10]
links: []
created: 2025-12-14T16:19:28.504747+01:00
type: task
priority: 1
parent: code-organization-407
---
# Create tab: variable prompting flow

Add variable prompting after owner/project are set:
- Load the selected template (from its source dir)
- Compute builtin vars for the prospective workspace
- Prompt for any required vars missing after considering defaults + builtins
- Support variable types: string, boolean, choice, integer (consistent with existing variable prompt TUI)

This should be integrated into the Template Explorer TUI as a sub-state (not a separate program).

## Design

Ideally refactor internal/tui/variable_prompt.go into a reusable component (model) so it can be embedded.

Ensure template loading uses multi-dir lookup (LoadTemplateMulti) so fallback templates work.

## Acceptance Criteria

- If template has no variables, flow skips prompting
- Required vars without defaults are prompted; defaults are shown and can be accepted
- Choice vars render as a selectable list
- Cancel returns user to Create tab without side effects



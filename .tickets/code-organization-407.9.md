---
id: code-organization-407.9
status: closed
deps: [code-organization-407.5]
links: []
created: 2025-12-14T16:18:40.342874+01:00
type: task
priority: 2
parent: code-organization-407
---
# Open actions: open template/file in editor

Add keybindings/actions to open:
- Selected template root directory
- Selected file (from Files tab)
- The generated workspace after creation (from Create tab)

Use configured editor when set; otherwise fall back to OS open behavior (macOS: `open`, Linux: print path or xdg-open if already used elsewhere).

## Design

Prefer reusing the logic/pattern from cmd/co/cmd/open.go, but without importing cobra.

In Bubble Tea, opening should be a non-blocking tea.Cmd.

## Acceptance Criteria

- One-key open works from Browse and Files tabs
- Respects `config.Editor` when set
- On macOS, opens via `open` when no editor configured
- Failure cases show a non-crashing error message in the UI



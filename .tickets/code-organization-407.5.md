---
id: code-organization-407.5
status: closed
deps: []
links: []
created: 2025-12-14T16:16:35.93773+01:00
type: task
priority: 1
parent: code-organization-407
---
# Scaffold Template Explorer TUI (tabs + layout)

Create the Bubble Tea model for the Template Explorer TUI, including:
- Alt-screen program
- Tab switching (Browse / Files / Create / Validate)
- Keymap + help footer
- Pane layout primitives (left list + right panel) with focus management

This task is about the shell and navigation, not full functionality.

## Design

Implementation notes:
- Follow internal/tui/tui.go patterns (list left, details right, help line).
- Model state: activeTab, activePane, selectedTemplate, message/error.
- Keep per-tab sub-models/types to avoid a giant Update() switch.

## Acceptance Criteria

- `co template` opens a stable TUI shell with tab switching and a help footer
- Window resize behaves correctly
- Focus switching between panes works and is visually obvious
- Code is structured so each tab can be implemented incrementally



---
id: code-organization-407.6
status: closed
deps: [code-organization-407.4, code-organization-407.5]
links: []
created: 2025-12-14T16:16:58.697713+01:00
type: task
priority: 2
parent: code-organization-407
---
# Browse tab: template list + details (incl source dir)

Implement the Browse tab:
- Left pane: filterable list of templates
- Right pane: details panel for selected template (manifest summary)
- Include template location (source dir/path), version/schema, counts, vars/repos/hooks summary.

This is the primary navigation surface.

## Design

Use the output of the multi-dir discovery helper (see discovery task) so the UI does not need to re-stat templates for location.

Keybindings should match existing TUI conventions:
- j/k or arrows to move
- / to filter
- tab/h/l to switch focus

## Acceptance Criteria

- Selecting a template updates details panel immediately
- Details panel includes template location (which templates dir, and full path)
- List supports fuzzy filtering/search
- Handles no-templates case gracefully (shows searched dirs + hint)



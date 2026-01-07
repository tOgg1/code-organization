---
id: code-organization-407.12
status: closed
deps: [code-organization-407.11]
links: []
created: 2025-12-14T16:19:48.7757+01:00
type: task
priority: 1
parent: code-organization-407
---
# Create tab: confirm + run workspace creation

Wire up execution of template-based workspace creation from the Create flow:
- Show a confirmation screen summarizing what will happen (workspace slug/path, template + source, options, vars)
- Run `template.CreateWorkspace` (or dry-run equivalent) with progress feedback
- Display results (files created, repos created/cloned, hooks run, warnings)
- Offer a one-key action to open the created workspace

Must be safe: prevent overwriting existing workspace; handle errors without leaving the TUI in a broken state.

## Design

Run creation behind a tea.Cmd; show spinner/progress message while running.

Reuse the same slug validation rules as `co new` and ensure hooks/no-hooks/dry-run options map to template.CreateOptions.

## Acceptance Criteria

- Creating an existing workspace slug is blocked with a clear error
- Dry-run mode does not write anything and still shows a useful summary
- Non-dry-run mode creates workspace and shows results + warnings
- Errors are surfaced and the user can return to Create tab and retry



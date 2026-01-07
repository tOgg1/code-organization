---
id: code-organization-407.10
status: closed
deps: [code-organization-407.5, code-organization-407.4]
links: []
created: 2025-12-14T16:19:11.203719+01:00
type: task
priority: 1
parent: code-organization-407
---
# Create tab: owner/project inputs + options

Implement the Create tab UI state:
- Owner and project text inputs (lowercase + slug validation)
- Uses the currently selected template by default, with an option to change template from within Create tab
- Options: dry-run toggle, no-hooks toggle
- Clear CTA to proceed to variable prompting / confirmation

This task does not execute creation yet.

## Design

Reuse patterns from internal/tui/new_prompt.go (textinput + focus management), but embed inside the Template Explorer program (no nested tea programs).

Slug rules should match fs.IsValidWorkspaceSlug().

## Acceptance Criteria

- User can enter owner/project and see validation errors inline
- Create tab clearly shows selected template + source dir
- Dry-run and no-hooks options are available
- Switching away from Create tab preserves inputs/state



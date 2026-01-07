---
id: code-organization-407
status: closed
deps: []
links: []
created: 2025-12-14T16:11:54.973624+01:00
type: epic
priority: 1
---
# Epic: Template Explorer TUI for co template

Running `co template` with no subcommands should launch a full-screen TUI for browsing templates across all configured template directories (primary + fallback), inspecting their manifests, browsing template/global files, viewing file contents, and opening files/dirs in the configured editor.

The TUI must also support creating a new workspace from the currently selected template: prompt for owner/project, prompt for required variables, confirm, then run creation (with dry-run/no-hooks options).

This epic focuses on MVP-quality UX + safe behavior; advanced features (rendered previews, diff/compare, etc.) are tracked separately.

## Design

UI shape:
- Tabs: Browse | Files | Create | Validate (hotkeys to switch)
- Browse tab: template list (left) + details (right) including source dir
- Files tab: file tree + content viewer
- Create tab: owner/project inputs, options (dry-run/no-hooks), variable prompting, confirmation, run + result summary
- Validate tab: validate selected/all and show errors with drill-down

Implementation notes:
- Use internal/config.Config.AllTemplatesDirs() for discovery
- Extend internal/template helpers to return per-template source dir (and precedence)
- Keep `co template list/show/validate` behavior unchanged

## Acceptance Criteria

- `co template` (no args) opens the Template Explorer TUI
- TUI lists templates from all template dirs and shows the selected template's source location
- User can browse files and view file contents
- User can open selected template/file in editor
- User can create a workspace from the selected template via a dedicated tab/hotkey



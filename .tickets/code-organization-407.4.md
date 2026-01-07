---
id: code-organization-407.4
status: closed
deps: []
links: []
created: 2025-12-14T16:16:17.72973+01:00
type: task
priority: 1
parent: code-organization-407
---
# Template discovery across dirs with source location

Extend internal/template discovery helpers so the Template Explorer TUI can list templates from *all* template directories (cfg.AllTemplatesDirs()), deduplicate by name (earlier dir wins), and show where each template came from.

This should be a reusable helper for CLI/TUI code (avoid scattering FindTemplateDir calls throughout the UI).

## Design

Options:
- Add `SourceDir`/`SourcePath` fields to `template.TemplateInfo` (backwards-compatible JSON add).
- Or introduce a new type, e.g. `template.TemplateListing { Info TemplateInfo; SourceDir string; TemplateDir string; }` to avoid changing existing JSON.

Prefer keeping TUI code dumb: feed it a slice of view models already containing `SourceDir` and computed paths.

## Acceptance Criteria

- New helper returns template list with per-template source dir/path and precedence
- Duplicate template names are resolved deterministically (earlier dir wins)
- Includes _global detection info needed by the UI
- Unit tests cover multi-dir precedence + source reporting



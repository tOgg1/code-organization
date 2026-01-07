---
id: code-organization-407.13
status: closed
deps: [code-organization-407.5, code-organization-407.4]
links: []
created: 2025-12-14T16:20:06.569507+01:00
type: task
priority: 2
parent: code-organization-407
---
# Validate tab: validate selected/all templates

Add a Validate tab:
- Validate the selected template (using template.ValidateTemplateDir against its source dir)
- Optionally validate all templates in all dirs (deduped by precedence)
- Display results list (✓/✗) and detailed error text for the selected result

This is a UI wrapper around existing validation functions.

## Design

For per-template validate, call ValidateTemplateDir(sourceDir, templateName).

For validate all, iterate over the same listing used by the Browse tab so precedence is consistent.

## Acceptance Criteria

- Validate selected works and shows detailed errors
- Validate all runs and shows per-template results without crashing
- Error output includes which file/hook is missing when applicable



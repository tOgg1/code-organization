---
id: code-organization-407.7
status: closed
deps: [code-organization-407.4, code-organization-407.5]
links: []
created: 2025-12-14T16:18:01.378995+01:00
type: task
priority: 2
parent: code-organization-407
---
# Files tab: source tree browser (template + _global)

Implement a Files tab that can browse the selected template's on-disk files:
- Tree rooted at the template directory (show template.json, files/, hooks/)
- Include a toggle/section for _global files (from all template dirs, honoring precedence)
- Selecting a file drives the content viewer (implemented separately)

This tab is about file navigation, not rendering.

## Design

Represent tree nodes with absolute paths + a stable display path.

For _global: consider a pseudo-root in the tree that lists merged global files with source badges.

## Acceptance Criteria

- File tree shows directories/files and supports expand/collapse
- Clearly indicates whether a file is from template vs _global (and which dir)
- Works when template has no files/ or hooks/
- Large template dirs remain responsive (no full-file reads in tree build)



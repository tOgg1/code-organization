---
id: code-organization-407.8
status: closed
deps: [code-organization-407.7]
links: []
created: 2025-12-14T16:18:19.391044+01:00
type: task
priority: 2
parent: code-organization-407
---
# Files tab: file content viewer (scroll + search)

Add a right-pane file viewer to the Files tab:
- Read and display selected file contents with scrolling
- Line numbers + wrap toggle
- In-file search (next/prev)
- Guardrails for binary / huge files (show message instead of dumping bytes)

Initially show raw file content; rendered template preview is tracked separately.

## Design

Use bubbles viewport for scrolling. Keep file IO behind a tea.Cmd and cache content for quick back/forward.

Define a max file size for inline viewing (e.g. 1–2MB) and show the size in the warning.

## Acceptance Criteria

- Viewing a file never blocks the UI (use streaming/async read if needed)
- Large/binary files show a clear warning + metadata instead of raw bytes
- User can scroll and search within the file
- Works for files under template root and _global roots



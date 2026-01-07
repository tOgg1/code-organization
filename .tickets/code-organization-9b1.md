---
id: code-organization-9b1
status: closed
deps: []
links: []
created: 2025-12-20T10:10:07.614426486+01:00
type: task
priority: 2
---
# Extract import execution logic to internal package

Extract core import logic from cmd/co/cmd/import.go to internal package (e.g., internal/workspace/import.go). Export functions for creating workspace, moving repos, copying extra files. Enables reuse from TUI.



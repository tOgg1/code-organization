---
id: code-organization-6ea
status: closed
deps: []
links: []
created: 2025-12-20T10:10:02.83052035+01:00
type: task
priority: 2
---
# Extract reusable stashFolder to internal package

Move stashFolder() logic from cmd/co/cmd/stash.go to internal package (e.g., internal/archive/stash.go). Export as StashFolder(). Update stash command to use new location. Enables reuse from TUI.



---
id: code-organization-ax2
status: closed
deps: []
links: []
created: 2025-12-20T10:09:51.861919745+01:00
type: feature
priority: 2
---
# Handle terminal resize in import browser

Handle tea.WindowSizeMsg to update width/height. Recalculate visible lines, pane widths, scroll positions. Ensure content remains visible after resize. Follow patterns from template_explorer.go.



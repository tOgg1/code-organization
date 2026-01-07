---
id: code-organization-46w
status: closed
deps: [code-organization-pgc, code-organization-py5]
links: []
created: 2025-12-20T10:08:24.737493026+01:00
type: feature
priority: 1
---
# Create import_browser.go with two-pane layout

Create internal/tui/import_browser.go with left pane showing source folder tree and right pane showing details/actions for selected item. Follow patterns from tui.go (workspace browser) and template_explorer.go. Include pane switching with h/l keys.



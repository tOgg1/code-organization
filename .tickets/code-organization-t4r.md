---
id: code-organization-t4r
status: closed
deps: [code-organization-t4q]
links: []
created: 2025-12-14T16:30:00+01:00
type: task
priority: 2
parent: code-organization-t4p
---
# Add fallback template discovery from XDG config

Support fallback template discovery: check _system/templates first, then ~/.config/co/templates/. This provides backwards compatibility for users with existing templates in the old location.



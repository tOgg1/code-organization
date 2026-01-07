---
id: code-organization-t4q
status: closed
deps: []
links: []
created: 2025-12-14T16:30:00+01:00
type: task
priority: 1
parent: code-organization-t4p
---
# Update TemplatesDir() to use _system/templates

Modify internal/config/config.go TemplatesDir() method to return filepath.Join(c.SystemDir(), 'templates') instead of XDG config path. This is the primary change.



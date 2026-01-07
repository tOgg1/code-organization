---
id: code-organization-diw
status: closed
deps: [code-organization-9tx]
links: []
created: 2025-12-14T12:06:21.135637+01:00
type: task
priority: 2
parent: code-organization-r9d
---
# Add TemplatesDir() to config

Extend internal/config/config.go with TemplatesDir() method returning filepath.Join(c.SystemDir(), 'templates').



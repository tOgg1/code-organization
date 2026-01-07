---
id: code-organization-6r6
status: closed
deps: [code-organization-yn2]
links: []
created: 2025-12-14T12:06:20.323263+01:00
type: task
priority: 1
parent: code-organization-r9d
---
# Implement variable dependency graph and cycle detection

Build dependency graph from variable defaults that reference other variables. Implement topological sort. Detect and report cycles with clear error: 'Circular variable reference: a → b → a'.



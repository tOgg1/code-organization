---
id: code-organization-l8p
status: closed
deps: []
links: []
created: 2025-12-31T16:17:15.348881+01:00
type: task
priority: 2
---
# Enforce partial prerequisites in Apply

Call CheckPrerequisites in partial.Apply, return PrerequisiteFailedError when missing unless --force is set; make --force meaningful



---
id: code-organization-184
status: closed
deps: [code-organization-kc8, code-organization-6ea]
links: []
created: 2025-12-20T10:09:04.101679665+01:00
type: feature
priority: 2
---
# Integrate stashFolder() into import browser TUI

Call stashFolder() from cmd/co/cmd/stash.go during stash execution. Handle both keep and delete modes. Show progress/completion messages. Refresh tree view after stash if source was deleted.



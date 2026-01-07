---
id: code-organization-8pq
status: closed
deps: [code-organization-ryu, code-organization-0u7]
links: []
created: 2025-12-13T18:34:23.368008+01:00
type: feature
priority: 2
---
# co sync command: remote workspace sync

Implement 'co sync <slug> <server> [--force] [--dry-run] [--no-git]'. Remote existence check, rsync transport with excludes, tar fallback. Exit code 10 if exists without --force.



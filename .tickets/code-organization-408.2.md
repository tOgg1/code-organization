---
id: code-organization-408.2
status: closed
deps: []
links: []
created: 2025-12-17T08:27:45.903089509+01:00
type: feature
priority: 1
parent: code-organization-408
---
# Sync: broaden built-in exclude blacklist

Improve the built-in default excludes used by co sync so we stop transferring common build/deps/cache directories (Go/Rust/Node/Python/etc). Ensure the blacklist is directory-first (avoid excluding common file names like 'build' scripts) and is applied consistently for rsync and tar transports.

## Acceptance Criteria

Default sync excludes include at least: node_modules/, vendor/, target/, dist/, build/, out/, bin/, obj/, coverage/, __pycache__/, .venv/, .cache/ (plus existing .DS_Store/*.log/.env rules). README's 'Default Excludes' matches implementation.



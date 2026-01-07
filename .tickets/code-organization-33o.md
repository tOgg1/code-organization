---
id: code-organization-33o
status: closed
deps: [code-organization-cxz, code-organization-2bw, code-organization-0u7, code-organization-ryu]
links: []
created: 2025-12-13T18:34:23.173473+01:00
type: feature
priority: 2
---
# co index command: generate index.jsonl

Implement 'co index' - scans code_root, validates workspaces, computes metrics (commit dates, dirty flags, sizes), writes atomic index.jsonl. Use concurrent worker pool.



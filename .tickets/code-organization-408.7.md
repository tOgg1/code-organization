---
id: code-organization-408.7
status: closed
deps: []
links: []
created: 2025-12-17T08:28:27.170230956+01:00
type: task
priority: 2
parent: code-organization-408
---
# Sync: ensure rsync + tar exclude semantics match

Audit and, if needed, adjust how excludes are passed to rsync (--exclude=…) vs tar (--exclude=…). In particular, confirm that interactive picker-generated relative paths and dir-only patterns behave the same in both transports (including nested paths, trailing slashes, and dotfiles).

## Acceptance Criteria

Documented/verified behavior for at least 10 representative patterns; automated tests exist where practical (unit tests for pattern normalization + at least one integration smoke test gated by tool availability).



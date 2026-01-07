---
id: code-organization-408.1
status: closed
deps: []
links: []
created: 2025-12-17T08:27:33.930616291+01:00
type: task
priority: 1
parent: code-organization-408
---
# Spec: sync exclude sources + precedence

Write a concrete spec for how co sync computes the effective exclude list. Cover: built-in defaults, optional global config overrides, optional per-workspace overrides, CLI flags, and interactive picker output; define precedence + de-duping. Also define pattern semantics (dir-only vs file patterns), and how patterns map consistently to both rsync and tar transports.

## Acceptance Criteria

Spec exists and includes examples for Go/Rust/Node workspaces plus at least 5 edge cases (dotfiles, symlinks, nested repos, path separators, trailing slashes).



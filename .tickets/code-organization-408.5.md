---
id: code-organization-408.5
status: closed
deps: []
links: []
created: 2025-12-17T08:28:13.39621231+01:00
type: task
priority: 1
parent: code-organization-408
---
# Sync picker: lazy file tree + performance guardrails

Implement the filesystem tree backing the interactive sync picker in a way that scales: lazy-load directory children on expand (no full recursive walk), avoid following symlinks, and ensure excluded directories are not traversed. Prefer non-blocking loads via tea.Cmd where needed.

## Acceptance Criteria

Picker initial render does not recursively enumerate the whole workspace; expanding a dir reads only that dir. Symlink loops cannot hang the UI. Large directories don't freeze the UI (either async load or entry caps with a notice).



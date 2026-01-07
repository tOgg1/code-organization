---
id: code-organization-408.3
status: closed
deps: []
links: []
created: 2025-12-17T08:27:56.495275301+01:00
type: feature
priority: 2
parent: code-organization-408
---
# Sync: CLI support for custom excludes

Add explicit CLI controls for sync excludes: repeatable --exclude <pattern> (and optionally --exclude-from <file>) so users can script/work around edge cases without the interactive picker.

## Acceptance Criteria

co sync accepts --exclude multiple times; effective exclude list includes built-ins + CLI patterns (deduped). If --exclude-from is added, it supports one pattern per line and ignores blank lines + '#' comments.



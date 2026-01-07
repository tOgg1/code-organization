---
id: code-organization-svm.3
status: closed
deps: [code-organization-407.4, code-organization-407.7]
links: []
created: 2025-12-14T16:23:20.516979+01:00
type: task
priority: 3
parent: code-organization-svm
---
# Diagnostics: include/exclude debugger + placeholder scan

Add diagnostics tooling in the UI:
- For a given file, show whether it is included/excluded and why (which pattern matched)
- Scan selected template for unresolved placeholders (e.g., {{VAR}} that is not provided by builtins/defaults/user overrides)

Goal: make template debugging faster.

## Design

Pattern debugger likely needs a small extension to PatternMatcher to return match details (not just bool).

Placeholder scan can reuse variableRefPattern in internal/template/variables.go.

## Acceptance Criteria

- User can see why a file is included or excluded (pattern + rule)
- Placeholder scan reports file + line (where possible) for unresolved vars
- Diagnostics are computed without writing to disk



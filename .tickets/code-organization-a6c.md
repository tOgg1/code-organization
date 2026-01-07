---
id: code-organization-a6c
status: closed
deps: [code-organization-3zo, code-organization-26y, code-organization-boj]
links: []
created: 2025-12-31T14:46:02.196721+01:00
type: task
priority: 2
parent: code-organization-bqd
---
# Write tests for Phase 4 features (merge, template integration, prerequisites)

Tests for Phase 4 functionality:

TEST FILE: internal/partial/merge_test.go

MergeGitignore:
- Merges two gitignore files
- Preserves existing lines
- Adds new unique lines
- Removes duplicates
- Handles empty files
- Handles comments correctly
- Works with .dockerignore, .npmignore

MergeJSON:
- Deep merges objects
- Partial values override existing
- Preserves existing-only keys
- Handles nested objects
- Handles arrays (replace)
- Returns error for invalid JSON
- Preserves formatting (indentation)

MergeYAML:
- Same cases as JSON
- Returns error for invalid YAML
- Handles multi-document (uses first)

deepMerge:
- Merges flat maps
- Merges nested maps recursively
- Overlay takes precedence
- Handles nil values

TEST FILE: internal/template/partials_test.go

Template with partials:
- Applies partials after repos
- Resolves partial variables from template vars
- Handles conditional partials (when field)
- Skips partial when condition false
- Fails on missing partial
- Fails on partial apply error
- Order of operations is correct

TEST FILE: internal/partial/prerequisites_test.go

CheckPrerequisites:
- Returns satisfied when all present
- Detects missing commands
- Detects missing files
- Supports file glob patterns
- Empty requires always satisfied
- Returns all missing items (not just first)

commandExists:
- Returns true for existing command
- Returns false for missing command

fileExists:
- Returns true for existing file
- Returns false for missing file
- Supports glob patterns



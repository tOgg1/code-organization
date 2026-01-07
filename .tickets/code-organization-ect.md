---
id: code-organization-ect
status: closed
deps: [code-organization-5i9]
links: []
created: 2025-12-31T14:42:06.961463+01:00
type: task
priority: 1
parent: code-organization-g6j
---
# Write tests for internal/partial/loader.go

Comprehensive tests for the partial loader:

TEST FILE: internal/partial/loader_test.go

TEST CASES FOR ListPartials:
- Empty directory returns empty list
- Single partial discovered correctly
- Multiple partials discovered
- Invalid partial.json skipped with warning
- Hidden directories skipped
- Nested directories not scanned (only top-level)
- Multiple directories with priority (first wins)

TEST CASES FOR FindPartial:
- Finds partial in primary directory
- Finds partial in fallback directory
- Primary takes precedence over fallback (same name)
- Returns error for non-existent partial
- Name matching is exact (no fuzzy)

TEST CASES FOR LoadPartial:
- Loads valid partial.json
- Returns InvalidManifestError for malformed JSON
- Returns error for missing partial.json
- Correctly parses all fields (variables, files, conflicts, hooks, tags, requires)
- Handles optional fields with defaults

TEST CASES FOR ValidatePartial:
- Valid partial passes
- Invalid schema version fails
- Invalid name pattern fails (uppercase, starts with number, special chars)
- Missing description fails
- Invalid variable type fails
- Choice type without choices fails
- Required variable with default fails
- Invalid conflict strategy fails
- Non-existent hook path fails

Use testify/assert for assertions. Create test fixtures as temp directories.



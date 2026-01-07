---
id: code-organization-3zo
status: closed
deps: []
links: []
created: 2025-12-31T14:45:18.545202+01:00
type: task
priority: 2
parent: code-organization-bqd
---
# Create internal/partial/merge.go - Format-specific merge implementations

Implement merge strategies for supported file formats:

SUPPORTED FORMATS:
1. .gitignore (and similar: .dockerignore, .npmignore)
   - Append unique lines from partial
   - Preserve existing lines
   - Remove duplicates
   
2. JSON files
   - Deep merge: partial values override existing
   - Preserve keys only in existing
   - Handle nested objects recursively
   - Arrays: configurable (replace vs append)
   
3. YAML files
   - Same deep merge strategy as JSON
   - Preserve comments where possible (best effort)
   - Handle multi-document YAML (use first doc)

FUNCTIONS:

MergeGitignore(existing, partial []byte) ([]byte, error)
- Parse both as line sets
- Union unique lines
- Preserve order: existing lines first, then new
- Handle comments (lines starting with #)

MergeJSON(existing, partial []byte) ([]byte, error)
- Parse both as map[string]interface{}
- Deep merge with partial overriding
- Marshal back with consistent formatting (indent)
- Preserve key order where possible

MergeYAML(existing, partial []byte) ([]byte, error)
- Parse both as map[string]interface{}
- Same deep merge strategy
- Marshal back to YAML

MergeFile(existingPath, partialPath, destPath string) error
- Detect format from extension
- Call appropriate merge function
- Write result to destPath (might be same as existingPath)

deepMerge(base, overlay map[string]interface{}) map[string]interface{}
- Recursive merge helper
- overlay values take precedence
- Merge nested maps recursively
- Arrays: replace (not merge) by default

ERROR HANDLING:
- Return error if merge not possible (binary files, parse error)
- Fall back to 'prompt' strategy on merge failure



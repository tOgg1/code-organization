---
id: code-organization-0q3
status: closed
deps: [code-organization-6hy, code-organization-t60]
links: []
created: 2025-12-31T14:43:39.074057+01:00
type: task
priority: 1
parent: code-organization-pzq
---
# Write tests for internal/partial/apply.go and files.go

Comprehensive tests for apply logic:

TEST FILES:
- internal/partial/apply_test.go
- internal/partial/files_test.go

TEST CASES FOR files.go:

ScanPartialFiles:
- Finds all files in files/ subdirectory
- Respects include patterns
- Respects exclude patterns
- Handles nested directories
- Detects .tmpl files correctly
- Empty files/ returns empty list

DetectConflicts:
- No conflicts when target is empty
- Detects existing files as conflicts
- Respects preserve patterns (never modified)
- Assigns correct action based on strategy
- Handles nested file conflicts

ProcessFile:
- Copies non-template file correctly
- Substitutes variables in template files
- Strips .tmpl extension
- Creates parent directories
- Preserves file permissions
- Rejects path traversal attempts

TEST CASES FOR apply.go:

Apply (happy path):
- Applies partial to empty directory
- Creates all expected files
- Substitutes variables correctly
- Returns accurate ApplyResult

Apply (conflicts):
- Skip strategy skips existing files
- Overwrite strategy replaces files
- Backup strategy creates .bak files
- Preserve patterns always skipped

Apply (dry run):
- No files written
- Result shows what would happen
- Hooks not executed

Apply (errors):
- Missing partial returns PartialNotFoundError
- Missing target returns TargetNotFoundError
- Missing required variable returns error
- Invalid variable value returns error

FIXTURES:
- Create temp directory structure for each test
- Clean up after each test
- Use t.TempDir() for automatic cleanup



---
id: code-organization-9y0
status: closed
deps: [code-organization-t60]
links: []
created: 2025-12-31T14:42:54.162429+01:00
type: task
priority: 1
parent: code-organization-pzq
---
# Create internal/partial/conflicts.go - Conflict resolution strategies

Implement conflict resolution strategies:

FUNCTIONS:

ResolveConflict(info *FileInfo, strategy string) FileAction
- Map strategy string to action for a conflicting file
- prompt -> ActionPrompt (handled by caller)
- skip -> ActionSkip
- overwrite -> ActionOverwrite
- backup -> ActionBackup
- merge -> ActionMerge (if supported format)

ExecuteBackup(destPath string) (string, error)
- Create backup of existing file before overwrite
- Naming: file.ext -> file.ext.bak
- If .bak exists: file.ext.bak.1, .bak.2, etc.
- Return path to backup file
- Preserve permissions on backup

IsPreserved(relPath string, preservePatterns []string) bool
- Check if a path matches any preserve pattern
- Use glob matching (filepath.Match or template.MatchGlob)
- Preserved files are NEVER overwritten, regardless of strategy

GetDefaultExtensions() []string
- Return default template extensions: [".tmpl"]

FORMAT DETECTION (for merge strategy):
- IsGitignoreFile(path string) bool: Check if .gitignore or similar
- IsJSONFile(path string) bool: Check .json extension
- IsYAMLFile(path string) bool: Check .yaml or .yml extension
- CanMerge(path string) bool: Return true if format supports merging

Note: Actual merge implementations go in merge.go (Phase 4)



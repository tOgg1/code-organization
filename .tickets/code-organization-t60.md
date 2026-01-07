---
id: code-organization-t60
status: closed
deps: []
links: []
created: 2025-12-31T14:42:40.259404+01:00
type: task
priority: 1
parent: code-organization-pzq
---
# Create internal/partial/files.go - File processing and conflict detection

File operations for partial application:

TYPES:
- FileAction enum: ActionCreate, ActionSkip, ActionOverwrite, ActionBackup, ActionMerge
- FileInfo struct: RelPath, AbsSourcePath, AbsDestPath, ExistsInTarget, TargetModTime, Action
- FilePlan struct: Files []FileInfo, Creates/Skips/Overwrites/Backups/Merges counts

FUNCTIONS:

ScanPartialFiles(partialPath string, filesConfig PartialFiles) ([]string, error)
- Walk the files/ subdirectory of the partial
- Apply include/exclude patterns using template.NewPatternMatcher
- Return list of relative paths (from files/ dir)
- Detect .tmpl files for later processing

DetectConflicts(files []string, partialPath, targetPath string, conflicts ConflictConfig) (*FilePlan, error)
- For each file, check if it exists in target
- Check against preserve patterns (never overwrite)
- Assign initial action based on conflict strategy
- Return complete plan of what will happen

CopyFile(src, dest string, mode os.FileMode) error
- Simple file copy (no templating)
- Create parent directories as needed
- Preserve permissions

ProcessFile(srcPath, destPath string, isTemplate bool, vars map[string]string, extensions []string) error
- If isTemplate: read, substitute variables via template.ProcessTemplateContent, write
- Else: simple copy
- Strip .tmpl extension from dest if applicable
- Create parent directories

SECURITY:
- Validate all dest paths are within target directory (no ../ escape)
- Return PathTraversalError if violated

Reuse from template package:
- template.NewPatternMatcher for include/exclude
- template.IsTemplateFile for .tmpl detection
- template.StripTemplateExtension for removing .tmpl
- template.ProcessTemplateContent for variable substitution



---
id: code-organization-svm.2
status: closed
deps: [code-organization-407.4]
links: []
created: 2025-12-14T16:23:02.035802+01:00
type: task
priority: 3
parent: code-organization-svm
---
# Output mapping view (global + template overrides)

Add an Output view that shows the merged set of files that would land in the workspace:
- Combine _global files and template files into a single output file list
- Indicate origin (global dir X / template) and whether the template overrides a global file
- For templated files, show output path after stripping template extension

This is a pure inspection tool (no writes).

## Design

Implement a reusable mapping helper in internal/template that returns: outputPath -> {sourcePath, originType, originDir, isOverride}.

Reuse the same precedence rules as ProcessGlobalFilesMulti/ProcessAllFilesMulti.

## Acceptance Criteria

- Output view lists effective workspace-relative paths deterministically
- Each output entry shows origin + override status
- Selecting an output file can jump to the corresponding source file(s)



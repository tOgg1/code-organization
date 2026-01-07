---
id: code-organization-svm.1
status: closed
deps: [code-organization-407.8]
links: []
created: 2025-12-14T16:22:05.464417+01:00
type: task
priority: 3
parent: code-organization-svm
---
# Rendered preview for template files

In the Files tab, add a toggle to view templated files as:
- Raw source
- Rendered output (ProcessConditionals + SubstituteVariables)

Rendered mode should use a controllable variable context (builtins + user overrides) and clearly indicate which context is active.

## Design

Prefer reusing internal/template.ProcessTemplateContent.

Variable context sources:
- Builtins derived from current Create tab owner/project (if present)
- Optional ad-hoc overrides UI (simple key/value input) for preview-only

Do not write files; this is preview-only.

## Acceptance Criteria

- User can toggle raw/rendered for .tmpl (and configured template extension) files
- Rendered output matches internal/template.ProcessTemplateContent behavior
- Viewer indicates when unresolved vars remain (e.g., leaves {{VAR}} visible)
- Changing variable context updates rendered preview



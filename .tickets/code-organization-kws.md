---
id: code-organization-kws
status: closed
deps: [code-organization-605]
links: []
created: 2025-12-31T14:41:16.230828+01:00
type: task
priority: 1
parent: code-organization-g6j
---
# Create internal/partial/errors.go - Error types

Define error types for the partial package:

ERROR TYPES:
- PartialNotFoundError: Name field, returns 'partial not found: <name>'
- InvalidManifestError: Path, Err fields, implements Unwrap()
- TargetNotFoundError: Path field for when target directory doesn't exist
- PrerequisiteFailedError: Missing Commands[], Missing Files[], used when requirements aren't met
- HookFailedError: HookType, Script, ExitCode, Output fields
- ConflictAbortedError: When user cancels during conflict prompts
- PathTraversalError: Path, TargetPath fields (security: files must stay in target)
- MultiError: Errors []error with Add() and ErrorOrNil() methods

IMPLEMENT:
- Error() string method for all types
- Unwrap() error method where applicable (InvalidManifestError, HookFailedError)

Follow patterns from internal/template/errors.go exactly.



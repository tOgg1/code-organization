---
id: code-organization-605
status: closed
deps: []
links: []
created: 2025-12-31T14:41:04.338592+01:00
type: task
priority: 1
parent: code-organization-g6j
---
# Create internal/partial/partial.go - Core types and constants

Define the foundational data structures for partials:

STRUCTS TO CREATE:
- Partial: Main struct with Schema, Name, Description, Version, Variables, Files, Conflicts, Hooks, Tags, Requires
- PartialVar: Variable definition with Name, Description, Type, Required, Default, Validation, Choices (reuse VarType from template)
- PartialFiles: Include, Exclude patterns, TemplateExtensions
- ConflictConfig: Strategy (prompt|skip|overwrite|backup|merge), Preserve patterns
- PartialHooks: PreApply, PostApply (reuse HookSpec from template)
- Requirements: Commands, Files arrays for prerequisites
- ApplyOptions: PartialName, TargetPath, Variables map, ConflictStrategy, DryRun, NoHooks, Force, Yes flags
- ApplyResult: PartialName, TargetPath, FilesCreated/Skipped/Overwritten/Merged/BackedUp lists, HooksRun, Warnings

CONSTANTS:
- ConflictStrategy constants: StrategyPrompt, StrategySkip, StrategyOverwrite, StrategyBackup, StrategyMerge
- Default values: DefaultConflictStrategy = StrategyPrompt

VALIDATION:
- Partial name regex: ^[a-z0-9][a-z0-9-]*$ (no reserved names like _global)
- Schema version validation (currently 1)

Mirror the structure in internal/template/template.go but adapted for partial-specific concerns.



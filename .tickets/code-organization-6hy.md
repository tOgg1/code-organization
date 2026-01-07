---
id: code-organization-6hy
status: closed
deps: [code-organization-t60, code-organization-9y0]
links: []
created: 2025-12-31T14:43:10.166614+01:00
type: task
priority: 1
parent: code-organization-pzq
---
# Create internal/partial/apply.go - Main apply logic

Main orchestration for applying partials:

CORE FUNCTION:
Apply(opts ApplyOptions) (*ApplyResult, error)
- Main entry point for partial application

FLOW:
1. Load partial from name (via loader.FindPartial + LoadPartial)
2. Validate target directory exists (or create with --force?)
3. Resolve variables (user-provided + defaults + builtins)
4. Validate all required variables are provided
5. Scan partial files (via ScanPartialFiles)
6. Detect conflicts (via DetectConflicts)
7. If DryRun: return result without writing
8. Execute pre_apply hook (if not NoHooks)
9. Process each file according to its action:
   - Create: ProcessFile
   - Skip: log and continue
   - Overwrite: ProcessFile (overwrites existing)
   - Backup: ExecuteBackup then ProcessFile
   - Merge: defer to merge.go (Phase 4, skip for now)
10. Execute post_apply hook (if not NoHooks)
11. Build and return ApplyResult

HELPER FUNCTIONS:
- validateTarget(path string) error: Check directory exists and is writable
- buildFileEnv(files *ApplyResult) map[string]string: Build CO_FILES_* env vars for hooks

DRY RUN OUTPUT:
- Return ApplyResult with all fields populated
- FilesCreated etc. contain what WOULD happen
- Caller (CLI) formats output

ERROR HANDLING:
- Partial failures: some files written, some not
- Log which files succeeded
- Return error with context about partial state
- Suggest re-running with --conflict skip

This does NOT include:
- Interactive prompting (Phase 3)
- Merge strategy execution (Phase 4)
- Hook execution (Phase 3 - stub for now)



---
id: code-organization-z5e
status: closed
deps: [code-organization-6hy]
links: []
created: 2025-12-31T14:43:25.994504+01:00
type: task
priority: 1
parent: code-organization-pzq
---
# Add 'co partial apply' CLI command

Add the apply subcommand to partial CLI:

COMMAND:
co partial apply <name> [path] [flags]

ARGUMENTS:
- name: Required - partial name to apply
- path: Optional - target directory (default: current directory)

FLAGS:
- -v, --var <key=value>: Set variable (repeatable via StringArrayVar)
- --conflict <strategy>: Override conflict strategy (prompt|skip|overwrite|backup)
- --dry-run: Preview changes without applying
- --no-hooks: Skip lifecycle hooks
- --force: Apply even if prerequisites fail
- -y, --yes: Accept all prompts automatically

IMPLEMENTATION:
1. Load config
2. Parse flags into ApplyOptions struct
3. Resolve target path (default to cwd, handle relative paths)
4. Call partial.Apply(opts)
5. Format and display result

OUTPUT FORMATTING:
- Human readable: Show each file action with status indicator
  ✓ Created AGENTS.md
  ✓ Created .claude/settings.json
  - Skipped .gitignore (exists)
  ! Backed up pyproject.toml → pyproject.toml.bak
- JSON mode: Output ApplyResult struct

DRY RUN OUTPUT:
- Clear header: 'DRY RUN - No changes will be made'
- Show resolved variables
- Show planned actions with CREATE/SKIP/OVERWRITE/BACKUP labels
- Show hooks that WOULD run
- Summary counts at end

ERROR HANDLING:
- PartialNotFoundError: List available partials
- TargetNotFoundError: Suggest creating directory or check path
- PrerequisiteFailedError: Show what's missing, suggest --force
- Other errors: Show error and exit code 1

Follow patterns from template creation in new.go



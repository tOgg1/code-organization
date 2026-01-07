---
id: code-organization-787
status: closed
deps: [code-organization-8mo]
links: []
created: 2025-12-31T14:44:22.472069+01:00
type: task
priority: 2
parent: code-organization-i39
---
# Create internal/partial/hooks.go - Hook execution

Implement lifecycle hooks for partials:

HOOK TYPES:
- HookTypePreApply: Runs before any files are written
- HookTypePostApply: Runs after all files are written

ENVIRONMENT VARIABLES FOR HOOKS:
Standard:
- CO_PARTIAL_NAME: Name of the partial being applied
- CO_PARTIAL_PATH: Absolute path to partial directory
- CO_TARGET_PATH: Absolute path to target directory
- CO_TARGET_DIRNAME: Name of target directory
- CO_DRY_RUN: 'true' or 'false'
- CO_VERBOSE: 'true' or 'false' (future)

Git info (if target is git repo):
- CO_IS_GIT_REPO: 'true' or 'false'
- CO_GIT_REMOTE_URL: Remote URL
- CO_GIT_BRANCH: Current branch

User variables (prefixed):
- CO_VAR_<name>=<value> for each variable

File results (post_apply only):
- CO_FILES_CREATED: Newline-separated list
- CO_FILES_SKIPPED: Newline-separated list
- CO_FILES_OVERWRITTEN: Newline-separated list
- CO_FILES_MERGED: Newline-separated list
- CO_FILES_BACKED_UP: Newline-separated list

TYPES:
type PartialHookEnv struct {
    PartialName   string
    PartialPath   string
    TargetPath    string
    TargetDirname string
    DryRun        bool
    IsGitRepo     bool
    GitRemoteURL  string
    GitBranch     string
    Variables     map[string]string
    Result        *ApplyResult  // Only for post_apply
}

FUNCTIONS:

BuildPartialHookEnv(env PartialHookEnv) []string
- Build environment variable slice from struct
- Add CO_VAR_ prefixed user variables
- Add file lists for post_apply

RunPartialHook(hookType string, spec template.HookSpec, partialPath string, env PartialHookEnv, output io.Writer) (*template.HookResult, error)
- Delegate to template.RunHook with partial-specific env
- Handle both string and object hook specs
- Support workdir relative to partial directory
- Timeout enforcement (from template package)

HOOK EXECUTION:
- pre_apply: Validation, prerequisite checks
  - If fails (exit != 0): abort apply, no files written
- post_apply: Post-setup commands
  - If fails: log warning, apply is still considered successful
  - Files are already written



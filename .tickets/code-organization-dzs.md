---
id: code-organization-dzs
status: closed
deps: [code-organization-8mo, code-organization-787, code-organization-bxs]
links: []
created: 2025-12-31T14:44:53.374453+01:00
type: task
priority: 2
parent: code-organization-i39
---
# Write tests for Phase 3 features (variables, hooks, prompts)

Tests for Phase 3 functionality:

TEST FILE: internal/partial/variables_test.go

GetPartialBuiltins:
- Returns DIRNAME correctly
- Returns DIRPATH as absolute path
- Returns PARENT_DIRNAME correctly
- Returns IS_GIT_REPO='true' for git repos
- Returns IS_GIT_REPO='false' for non-git dirs
- Returns GIT_BRANCH for git repos
- Returns GIT_REMOTE_URL for git repos
- Handles missing git remote gracefully

ResolvePartialVariables:
- Merges builtins with provided values
- Applies defaults when not provided
- Substitutes builtins in default values
- Validates required variables present
- Validates variable types and patterns
- Returns error for missing required vars

TEST FILE: internal/partial/hooks_test.go

BuildPartialHookEnv:
- Includes all standard env vars
- Includes CO_VAR_ prefixed user vars
- Includes file lists for post_apply
- Correctly formats newline-separated lists

RunPartialHook:
- Executes script with correct env
- Returns exit code and output
- Handles timeout
- Handles missing script
- Supports string hook spec
- Supports object hook spec with workdir

TEST FILE: internal/partial/prompts_test.go (if separate)

promptForConflict:
- Returns correct action for each key
- Handles applyToAll flags
- Generates diff correctly
- Handles non-text files (no diff available)

Use mocking for stdin in interactive tests.



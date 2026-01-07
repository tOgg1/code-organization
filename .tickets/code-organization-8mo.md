---
id: code-organization-8mo
status: closed
deps: []
links: []
created: 2025-12-31T14:44:05.587591+01:00
type: task
priority: 2
parent: code-organization-i39
---
# Create internal/partial/variables.go - Partial-specific built-in variables

Implement partial-specific variable resolution:

BUILT-IN VARIABLES:
Standard (from template package):
- {{GIT_USER_NAME}}: git config user.name
- {{GIT_USER_EMAIL}}: git config user.email  
- {{YEAR}}: Current year (2025)
- {{DATE}}: Current date (YYYY-MM-DD)
- {{TIMESTAMP}}: Unix timestamp

Partial-specific (NEW):
- {{DIRNAME}}: Name of target directory (e.g., 'backend')
- {{DIRPATH}}: Absolute path to target directory
- {{PARENT_DIRNAME}}: Name of parent directory (e.g., 'repos')
- {{IS_GIT_REPO}}: 'true' if target is a git repository
- {{GIT_REMOTE_URL}}: Git remote origin URL (if git repo, else empty)
- {{GIT_BRANCH}}: Current git branch (if git repo, else empty)

FUNCTIONS:

GetPartialBuiltins(targetPath string) (map[string]string, error)
- Build map of all built-in variables
- Use internal/git package for git info:
  - git.IsRepo(targetPath)
  - git.GetInfo(targetPath) for branch, remote, etc.
- Handle non-git directories gracefully (empty git vars)

ResolvePartialVariables(p *Partial, provided, builtins map[string]string) (map[string]string, error)
- Merge builtins with user-provided variables
- Apply defaults for non-provided variables
- Substitute built-ins in defaults (e.g., default: '{{DIRNAME}}')
- Validate all required variables have values
- Validate variable values against type and validation regex
- Return complete variable map

DELEGATION:
- Use template.ValidateVarValue for validation
- Use template.ProcessTemplateContent for substitution in defaults
- Adapt template.ResolveVariables pattern



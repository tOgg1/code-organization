---
id: code-organization-c1t
status: closed
deps: [code-organization-aqu]
links: []
created: 2025-12-31T14:47:22.097636+01:00
type: task
priority: 3
parent: code-organization-2j3
---
# Update README.md with partials documentation

Add comprehensive partials documentation to README.md:

SECTIONS TO ADD:

## Partials

Introduction paragraph explaining what partials are and how they differ from templates.

### Partial Locations

- Primary: ~/Code/_system/partials/
- Fallback: ~/.config/co/partials/

### Using Partials

#### List available partials
co partial list
co partial list --tag agent

#### Show partial details
co partial show agent-setup
co partial show agent-setup --files

#### Apply a partial
co partial apply agent-setup
co partial apply agent-setup ./repos/backend
co partial apply agent-setup -v primary_stack=go
co partial apply agent-setup --dry-run
co partial apply agent-setup --conflict skip -y

### Conflict Resolution

Table of strategies: prompt, skip, overwrite, backup, merge
Explain preserve patterns

### Creating Partials

1. Directory structure
2. partial.json manifest
3. Variables
4. Files and templates
5. Hooks
6. Tags

Example partial.json with annotations

### Built-in Variables

Table of partial-specific variables:
- DIRNAME, DIRPATH, PARENT_DIRNAME
- IS_GIT_REPO, GIT_REMOTE_URL, GIT_BRANCH

### Lifecycle Hooks

- pre_apply: validation
- post_apply: setup commands

Environment variables available to hooks

### Template Integration

How templates can reference partials
The 'partials' array in template.json
Application order



---
id: code-organization-03r
status: closed
deps: []
links: []
created: 2025-12-31T14:46:43.976699+01:00
type: task
priority: 3
parent: code-organization-2j3
---
# Create built-in partial: agent-setup

Create the agent-setup partial for AI agent configuration:

LOCATION: ~/Code/_system/partials/agent-setup/

partial.json:
{
  "schema": 1,
  "name": "agent-setup",
  "description": "AI agent configuration for multi-agent development workflows",
  "version": "1.0.0",
  "variables": [
    {
      "name": "project_name",
      "description": "Project name for documentation",
      "type": "string",
      "default": "{{DIRNAME}}"
    },
    {
      "name": "primary_stack",
      "description": "Primary technology stack",
      "type": "choice",
      "choices": ["bun", "node", "python", "go", "rust", "other"],
      "required": true
    },
    {
      "name": "use_beads",
      "description": "Include Beads task tracking",
      "type": "boolean",
      "default": true
    }
  ],
  "files": {
    "include": ["**/*"],
    "exclude": ["partial.json", "hooks/**"]
  },
  "conflicts": {
    "strategy": "prompt",
    "preserve": [".beads/beads.db", ".beads/*.log"]
  },
  "hooks": {
    "post_apply": "hooks/post-apply.sh"
  },
  "tags": ["agent", "ai", "workflow"]
}

FILES TO CREATE:
files/AGENTS.md.tmpl
- Multi-agent coordination guide
- Project-specific sections based on stack
- Links to runbooks

files/.claude/settings.json.tmpl
- Claude Code settings

files/agent_docs/README.md.tmpl
- Agent documentation index

files/agent_docs/gotchas.md
- Common gotchas and warnings

files/agent_docs/runbooks/dev.md.tmpl
- Development runbook

hooks/post-apply.sh
- Initialize beads if use_beads=true
- Add .beads/ to .gitignore if git repo
- Print success message

This is the primary use case partial from the spec.



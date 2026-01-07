---
id: code-organization-26y
status: closed
deps: [code-organization-3zo]
links: []
created: 2025-12-31T14:45:34.86529+01:00
type: task
priority: 2
parent: code-organization-bqd
---
# Add template.json support for partials integration

Allow templates to reference and apply partials during workspace creation:

TEMPLATE MANIFEST EXTENSION:
Add 'partials' array to template.json schema:

{
  "partials": [
    {
      "name": "agent-setup",
      "target": ".",                    // relative to workspace
      "variables": {                      // override partial vars
        "project_name": "{{PROJECT}}",
        "primary_stack": "{{frontend_stack}}"
      },
      "when": "{{use_agents}} == 'true'"  // optional condition
    },
    {
      "name": "eslint",
      "target": "repos/frontend",
      "when": "{{frontend_stack}} == 'node'"
    }
  ]
}

IMPLEMENTATION:

1. Extend internal/template/template.go:
   - Add Partials []PartialRef to Template struct
   - PartialRef: Name, Target, Variables map, When string

2. Extend internal/template/create.go:
   - After post_clone hook, apply partials in order
   - Resolve 'when' conditions using template variables
   - Resolve partial variables from template variables
   - Handle partial failures (continue vs abort)

3. Application order (update spec section 8.2):
   1. Workspace structure created
   2. Global files applied
   3. Template files applied
   4. post_create hook runs
   5. Repos cloned/initialized
   6. post_clone hook runs
   7. Partials applied (in order)
   8. post_complete hook runs

CONDITIONAL PARTIALS:
- 'when' field is optional
- If present, evaluate as simple condition
- Support == and != operators
- Variables substituted before evaluation
- If condition false, skip partial silently

ERROR HANDLING:
- Missing partial: fail template creation
- Partial apply failure: fail template creation
- Log which partials were applied successfully



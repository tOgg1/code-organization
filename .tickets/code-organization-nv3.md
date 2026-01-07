---
id: code-organization-nv3
status: closed
deps: [code-organization-5i9, code-organization-80n]
links: []
created: 2025-12-31T14:41:54.333404+01:00
type: task
priority: 1
parent: code-organization-g6j
---
# Create cmd/co/cmd/partial.go - CLI commands (list, show)

Implement the partial CLI subcommands:

MAIN COMMAND:
var partialCmd = &cobra.Command{
  Use:     'partial',
  Aliases: []string{'p'},
  Short:   'Manage reusable file sets (partials)',
  Long:    Description of partials concept
}

SUBCOMMAND: list
- co partial list [--json] [--tag <tag>]
- List all available partials from all directories
- Output columns: NAME, DESCRIPTION, FILES, VARS
- Use tabwriter for human output (like template list)
- JSON output: array of PartialInfo structs
- Filter by tag if --tag provided

SUBCOMMAND: show
- co partial show <name> [--json] [--files]
- Display detailed partial information:
  - Name, Description, Version
  - Variables with types, defaults, descriptions
  - Files that would be created (when --files)
  - Hooks defined
  - Tags
  - Requirements
- Error if partial not found (suggest listing)

SUBCOMMAND: validate (optional for Phase 1)
- co partial validate [name]
- Validate one or all partial manifests
- Return validation errors in human-readable format

REGISTRATION:
func init() {
  rootCmd.AddCommand(partialCmd)
  partialCmd.AddCommand(partialListCmd)
  partialCmd.AddCommand(partialShowCmd)
}

Follow patterns from cmd/co/cmd/template.go exactly for consistency.



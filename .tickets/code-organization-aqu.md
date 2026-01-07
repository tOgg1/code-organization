---
id: code-organization-aqu
status: closed
deps: [code-organization-bxs]
links: []
created: 2025-12-31T14:46:27.992369+01:00
type: task
priority: 3
parent: code-organization-2j3
---
# Create TUI for partial selection and application

Bubble Tea TUI for interactive partial management:

LOCATION: internal/tui/partial_explorer.go

TABS (similar to Template Explorer):
1. Browse - List partials with details pane
2. Apply - Apply partial to directory with variable prompts
3. Validate - Validate partial manifests

BROWSE TAB:
- Left pane: List of partials (name, description)
- Right pane: Details of selected partial
  - Name, Description, Version
  - Variables (with types and defaults)
  - Files preview
  - Hooks defined
  - Tags
- Keybindings:
  - j/k: Navigate list
  - l/h: Switch panes
  - /: Search partials
  - o: Open partial directory
  - Tab: Switch to Apply tab

APPLY TAB:
- Target directory input (default: current directory)
- Browse button to select directory
- Variable prompts for selected partial
- Conflict strategy dropdown
- Dry-run checkbox
- No-hooks checkbox
- Apply button

VARIABLE PROMPTING:
- String: text input
- Boolean: checkbox/toggle
- Choice: dropdown/select list
- Show description as help text
- Show default value
- Validate on blur

APPLY RESULT SCREEN:
- Show files created/skipped/overwritten
- Show hooks executed
- Option to open target directory
- Return to Browse

VALIDATE TAB:
- List partials with validation status
- Show validation errors for each
- Validate all button

KEYBINDINGS (Global):
- Tab/Shift+Tab: Navigate tabs
- 1-3: Jump to tab
- q/Ctrl+C: Quit

INTEGRATION:
- Add 'co partial' (no args) to launch TUI
- Similar to 'co template' launching Template Explorer



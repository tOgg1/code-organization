---
id: code-organization-407.3
status: closed
deps: [code-organization-407.5]
links: []
created: 2025-12-14T16:15:22.151831+01:00
type: task
priority: 1
parent: code-organization-407
---
# Default co template to TUI

Change the Cobra `template` command so running `co template` with no additional args launches the Template Explorer TUI.

Keep existing subcommands (`list`, `show`, `validate`) unchanged and ensure `--help` behaves normally.

## Design

Implementation sketch:
- Add `RunE` to templateCmd for the no-args case.
- Ensure help flag short-circuits as usual.
- Optional: add explicit `co template tui` alias (nice-to-have).

## Acceptance Criteria

- `co template` launches the Template Explorer TUI
- `co template list|show|validate` behave exactly as before
- `co template --help` prints help (does not enter TUI)



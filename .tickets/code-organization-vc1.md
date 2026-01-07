---
id: code-organization-vc1
status: closed
deps: [code-organization-46w]
links: []
created: 2025-12-20T10:09:36.350119863+01:00
type: feature
priority: 1
---
# Create RunImportBrowser() entry point function

Create public RunImportBrowser(rootPath string, cfg *config.Config) error function. Initialize model, run tea.Program with alt screen, return error if any. This is the main entry point for the TUI.



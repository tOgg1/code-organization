---
id: code-organization-169
status: closed
deps: [code-organization-46w]
links: []
created: 2025-12-20T10:08:37.658692357+01:00
type: feature
priority: 1
---
# Create ImportBrowserState enum and state machine

Define states: StateBrowse, StateImportConfig, StateExtraFiles, StateImportPreview, StateImportExecute, StateStashConfirm, StateStashExecute, StateComplete. Handle transitions between states in Update() method.



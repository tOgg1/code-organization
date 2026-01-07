---
id: code-organization-43w
status: closed
deps: [code-organization-ues]
links: []
created: 2025-12-20T10:08:46.287823918+01:00
type: feature
priority: 2
---
# Integrate extra files picker into import flow

After config, if non-git items exist, transition to StateExtraFiles. Reuse FindNonGitItems() and embed extra_files_picker patterns or call RunExtraFilesPicker() inline. Handle destination subfolder prompt.



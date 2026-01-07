---
id: code-organization-m7u
status: closed
deps: [code-organization-atq, code-organization-ttk, code-organization-diw, code-organization-oyc]
links: []
created: 2025-12-14T12:06:21.675293+01:00
type: task
priority: 1
parent: code-organization-r9d
---
# Implement template-based workspace creation

Create orchestration function that: runs pre_create hook, creates workspace dir, processes global files, processes template files, runs post_create hook, clones/inits repos, runs post_clone hook, saves project.json, runs post_complete hook.



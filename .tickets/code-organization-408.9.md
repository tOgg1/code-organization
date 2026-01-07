---
id: code-organization-408.9
status: closed
deps: []
links: []
created: 2025-12-17T08:28:43.885262378+01:00
type: task
priority: 3
parent: code-organization-408
---
# Sync: reconcile --include-env docs vs implementation

README documents a --include-env flag but co sync currently only supports --force/--dry-run/--no-git. Decide whether to implement --include-env (to override default .env excludes) or update docs accordingly; ensure behavior also integrates with interactive picker.

## Acceptance Criteria

Either: (a) co sync supports --include-env and it overrides default .env/.env.* excludes, or (b) README no longer advertises the flag. Interactive mode remains consistent.



---
id: code-organization-bxx
status: closed
deps: []
links: []
created: 2025-12-20T10:08:12.464019969+01:00
type: feature
priority: 1
---
# Create sourceNode data structure for import browser tree

Define sourceNode struct in internal/tui/import_browser.go with fields: Name, Path, RelPath, IsDir, IsExpanded, IsSelected, IsGitRepo, GitInfo (*git.RepoInfo), HasGitChild, Depth, Children. This is the core data structure for the import browser tree.



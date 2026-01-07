---
id: code-organization-pgc
status: closed
deps: [code-organization-bxx]
links: []
created: 2025-12-20T10:08:14.058060863+01:00
type: feature
priority: 1
---
# Implement source tree building with git repo detection

Implement buildSourceTree() that walks directory, creates sourceNode hierarchy. Call git.FindGitRoots() to detect repos. Mark IsGitRepo=true for detected roots. Set GitInfo by calling git.GetInfo(). Set HasGitChild for parent directories.



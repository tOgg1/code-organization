---
id: code-organization-7ow
status: closed
deps: [code-organization-bxx]
links: []
created: 2025-12-20T10:08:15.320064466+01:00
type: feature
priority: 2
---
# Add lazy loading for source tree directories

Implement loadChildren() method on sourceNode. Only load when directory is expanded. Follow patterns from exclude_picker.go loadChildren(). Cap entries at maxDirEntries (500). Handle symlinks. Show placeholder for truncated lists.



---
id: code-organization-5i9
status: closed
deps: [code-organization-605, code-organization-kws]
links: []
created: 2025-12-31T14:41:29.629712+01:00
type: task
priority: 1
parent: code-organization-g6j
---
# Create internal/partial/loader.go - Discovery and loading

Implement partial discovery, loading, and validation:

DISCOVERY FUNCTIONS:
- ListPartials(dirs []string) ([]PartialInfo, error): Scan directories for partials, return info with Name, Description, FilesCount, VarsCount, Tags, SourceDir
- FindPartial(name string, dirs []string) (string, error): Find partial path by name across directories (first match wins)
- PartialInfo struct: Name, Description, Path, SourceDir, FilesCount, VarsCount, Tags

LOADING FUNCTIONS:
- LoadPartial(partialPath string) (*Partial, error): Load and parse partial.json from directory
- parseManifest(data []byte, path string) (*Partial, error): JSON unmarshaling with validation

VALIDATION FUNCTIONS:
- ValidatePartial(p *Partial) error: Comprehensive validation
  - Schema version must be 1
  - Name must match ^[a-z0-9][a-z0-9-]*$
  - Description required
  - Variables have valid types, required have no defaults, choices non-empty for choice type
  - ConflictStrategy is valid enum value
  - Hook paths exist (relative to partial dir)

DIRECTORY SCANNING:
- Scan for directories containing partial.json
- Skip hidden directories, _global (not used for partials)
- Handle both primary and fallback directories
- Earlier directories take precedence (like templates)

Follow patterns from internal/template/loader.go closely.



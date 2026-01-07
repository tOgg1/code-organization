---
id: code-organization-80n
status: closed
deps: []
links: []
created: 2025-12-31T14:41:40.977714+01:00
type: task
priority: 1
parent: code-organization-g6j
---
# Extend internal/config/config.go - Add partials directories

Add methods to Config struct for partials directory resolution:

NEW METHODS:
- PartialsDir() string: Return primary partials directory
  filepath.Join(c.SystemDir(), 'partials')
  
- FallbackPartialsDir() string: Return XDG config partials directory
  Same pattern as FallbackTemplatesDir():
  - Check XDG_CONFIG_HOME, else ~/.config
  - Return filepath.Join(xdgConfig, 'co', 'partials')
  
- AllPartialsDirs() []string: Return all partial directories to search in priority order
  []string{c.PartialsDir(), c.FallbackPartialsDir()}

TESTS:
- Add tests to config_test.go for the new methods
- Test with XDG_CONFIG_HOME set and unset
- Verify priority order

This follows exactly the same pattern as TemplatesDir(), FallbackTemplatesDir(), AllTemplatesDirs().



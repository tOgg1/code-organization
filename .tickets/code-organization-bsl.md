---
id: code-organization-bsl
status: closed
deps: [code-organization-kxf]
links: []
created: 2025-12-13T22:56:12.290307+01:00
type: task
priority: 2
parent: code-organization-k0l
---
# Extend config.go with embedding settings

Add EmbeddingsConfig struct to internal/config/config.go with Backend, OllamaURL, OllamaModel, OpenAIModel, OpenAIAPIKeyEnv fields. Add IndexingConfig with ChunkMaxLines, ExcludePatterns, IncludeLanguages.



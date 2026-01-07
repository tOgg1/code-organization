---
id: code-organization-k9n
status: closed
deps: [code-organization-c86, code-organization-zr3, code-organization-9xn]
links: []
created: 2025-12-13T22:55:44.340875+01:00
type: task
priority: 1
parent: code-organization-k0l
---
# Build indexing pipeline

Create internal/search/indexer.go with Indexer struct that orchestrates: file scanning, parallel chunk extraction, batch embedding via Ollama, and database storage. Support incremental indexing via content hashes.



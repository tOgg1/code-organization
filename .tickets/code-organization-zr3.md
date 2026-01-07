---
id: code-organization-zr3
status: closed
deps: []
links: []
created: 2025-12-13T22:55:29.911444+01:00
type: task
priority: 1
parent: code-organization-k0l
---
# Implement Ollama embedder interface and client

Create internal/embedder/ with Embedder interface (Embed, EmbedBatch, Dimension, ModelName) and OllamaEmbedder implementation using Ollama's /api/embed endpoint. Support nomic-embed-text model (768 dims).



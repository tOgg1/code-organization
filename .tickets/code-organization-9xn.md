---
id: code-organization-9xn
status: closed
deps: [code-organization-5dr]
links: []
created: 2025-12-13T22:55:43.463914+01:00
type: task
priority: 1
parent: code-organization-k0l
---
# Implement tree-sitter AST-aware code chunker

Create internal/chunker/treesitter.go using go-tree-sitter bindings. Support Go, Python, JS/TS, Rust, Ruby, Java, C/C++, C#, Bash. Extract functions, classes, methods as semantic chunks. Include fallback line-based chunking.



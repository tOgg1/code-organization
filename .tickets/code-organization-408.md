---
id: code-organization-408
status: closed
deps: []
links: []
created: 2025-12-17T08:26:47.551059092+01:00
type: epic
priority: 1
---
# Sync: smarter excludes + interactive exclude picker

Today co sync can transfer gigabytes of build artifacts/binaries because it effectively syncs the whole workspace. Add (1) a stronger built-in blacklist of common build/deps/cache dirs (Go/Rust/Node/Python/etc) and (2) an interactive TUI tree view that lets the user press space to exclude specific paths before syncing (applies to rsync + tar fallback).



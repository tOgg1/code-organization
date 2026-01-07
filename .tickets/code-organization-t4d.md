---
id: code-organization-t4d
status: closed
deps: []
links: []
created: 2025-12-31T14:47:08.128571+01:00
type: task
priority: 3
parent: code-organization-2j3
---
# Create built-in partials: eslint and github-actions

Create additional built-in partials:

ESLINT PARTIAL:
Location: ~/Code/_system/partials/eslint/

partial.json:
- name: eslint
- description: ESLint configuration for JavaScript/TypeScript
- variables: typescript (boolean), prettier (boolean)
- requires: commands: [node, npm]
- tags: [javascript, typescript, linting]

Files:
- files/.eslintrc.json.tmpl (config with TS support if typescript=true)
- files/.eslintignore (node_modules, dist, etc.)

GITHUB-ACTIONS PARTIAL:
Location: ~/Code/_system/partials/github-actions/

partial.json:
- name: github-actions
- description: GitHub Actions CI/CD workflows
- variables:
  - language (choice: node, python, go, rust)
  - run_tests (boolean, default: true)
  - build_artifact (boolean, default: false)
- tags: [ci, github, automation]

Files:
- files/.github/workflows/ci.yml.tmpl
  - Runs tests based on language
  - Conditional steps based on variables
- files/.github/workflows/release.yml.tmpl
  - Release automation
  - Only if build_artifact=true

These partials demonstrate different use cases:
- eslint: Language-specific tooling
- github-actions: CI/CD automation with conditionals



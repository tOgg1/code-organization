---
id: code-organization-3sx
status: closed
deps: []
links: []
created: 2025-12-31T14:46:56.971257+01:00
type: task
priority: 3
parent: code-organization-2j3
---
# Create built-in partial: python-tooling

Create the python-tooling partial:

LOCATION: ~/Code/_system/partials/python-tooling/

partial.json:
{
  "schema": 1,
  "name": "python-tooling",
  "description": "Modern Python project tooling with ruff and pyproject.toml",
  "version": "1.0.0",
  "variables": [
    {
      "name": "python_version",
      "description": "Python version",
      "type": "string",
      "default": "3.12"
    },
    {
      "name": "package_name",
      "description": "Package name",
      "type": "string",
      "default": "{{DIRNAME}}"
    },
    {
      "name": "use_mypy",
      "description": "Include mypy type checking",
      "type": "boolean",
      "default": true
    }
  ],
  "conflicts": {
    "strategy": "prompt"
  },
  "requires": {
    "commands": ["python3"]
  },
  "tags": ["python", "tooling", "linting"]
}

FILES TO CREATE:
files/pyproject.toml.tmpl
- Modern Python packaging
- ruff configuration
- mypy configuration (if use_mypy)
- pytest configuration

files/.python-version
- Contains {{python_version}}

files/ruff.toml
- Ruff linter configuration
- Line length, rules, etc.

files/.gitignore.tmpl (or merge into existing)
- __pycache__/
- *.pyc
- .venv/
- .mypy_cache/
- .pytest_cache/
- dist/
- *.egg-info/



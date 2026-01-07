---
id: code-organization-boj
status: closed
deps: []
links: []
created: 2025-12-31T14:45:49.221587+01:00
type: task
priority: 2
parent: code-organization-bqd
---
# Implement prerequisites checking (requires field)

Add prerequisite validation before applying partials:

REQUIRES FIELD IN partial.json:
{
  "requires": {
    "commands": ["git", "node", "npm"],
    "files": ["package.json", "tsconfig.json"]
  }
}

IMPLEMENTATION:

CheckPrerequisites(p *Partial, targetPath string) (*PrerequisiteResult, error)
- Check all required commands exist in PATH
- Check all required files exist in target directory

PrerequisiteResult struct:
- Satisfied: bool
- MissingCommands: []string
- MissingFiles: []string

commandExists(name string) bool
- Use exec.LookPath to check if command is in PATH
- Handle platform differences if needed

fileExists(targetPath, relPath string) bool
- Check if file exists relative to target
- Support glob patterns (e.g., "*.go" means any .go file)

INTEGRATION:

In Apply():
1. After loading partial, before any other work
2. Call CheckPrerequisites
3. If not satisfied and --force not set:
   - Return PrerequisiteFailedError with details
4. If not satisfied and --force set:
   - Log warning but continue
5. If satisfied: continue normally

CLI OUTPUT:
When prerequisites fail:
  Error: Prerequisites not met for partial 'python-tooling':
    Missing commands: python3, pip
    Missing files: setup.py
  
  Use --force to apply anyway.

TESTING:
- Test with missing commands (mock exec.LookPath)
- Test with missing files
- Test with all satisfied
- Test --force bypasses check



---
id: code-organization-bxs
status: closed
deps: [code-organization-6hy]
links: []
created: 2025-12-31T14:44:40.249364+01:00
type: task
priority: 2
parent: code-organization-i39
---
# Implement interactive conflict prompts

Add interactive conflict resolution during apply:

CONFLICT PROMPT UI:
When strategy is 'prompt' and conflicts exist, prompt for each:

Conflicts:
  ? .gitignore (exists)
    [s]kip  [o]verwrite  [b]ackup  [d]iff  [a]ll-skip  [A]ll-overwrite: 

OPTIONS:
- s: Skip this file (keep existing)
- o: Overwrite this file
- b: Backup and overwrite
- d: Show diff between existing and new
- m: Merge (if supported format, Phase 4)
- a: Skip ALL remaining conflicts
- A: Overwrite ALL remaining conflicts
- B: Backup ALL remaining conflicts
- q: Quit (abort apply)

DIFF DISPLAY:
When 'd' is pressed:
  --- existing .gitignore
  +++ partial .gitignore
  @@ -1,3 +1,5 @@
   node_modules/
  +.beads/
  +.co-*
   .env

After diff, re-prompt for action.

IMPLEMENTATION:
- Add to apply.go: promptForConflict(file FileInfo, vars map[string]string) (FileAction, bool, error)
  - Returns: action, applyToAll, error
  - applyToAll indicates 'a', 'A', or 'B' was selected
- Use bufio.Reader for stdin or Bubble Tea simple prompt
- Support --yes flag to skip prompts (use default strategy)

DIFF GENERATION:
- For text files: generate unified diff
- Use go-diff library or simple line comparison
- Truncate if diff is very long (>50 lines)

INTEGRATION:
- In Apply(), when conflict strategy is 'prompt':
  1. If --yes flag: use 'skip' as default action
  2. Else: prompt for each conflict
  3. Track 'applyToAll' state for remaining files
- Update FilePlan based on user choices



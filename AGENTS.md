RULE 1 – ABSOLUTE (DO NOT EVER VIOLATE THIS)

You may NOT delete any file or directory unless I explicitly give the exact command **in this session**.

- This includes files you just created (tests, tmp files, scripts, etc.).
- You do not get to decide that something is “safe” to remove.
- If you think something should be removed, stop and ask. You must receive clear written approval **before** any deletion command is even proposed.

Treat “never delete files without permission” as a hard invariant.

---

### IRREVERSIBLE GIT & FILESYSTEM ACTIONS

Absolutely forbidden unless I give the **exact command and explicit approval** in the same message:

- `git reset --hard`
- `git clean -fd`
- `rm -rf`
- Any command that can delete or overwrite code/data

Rules:

1. If you are not 100% sure what a command will delete, do not propose or run it. Ask first.
2. Prefer safe tools: `git status`, `git diff`, `git stash`, copying to backups, etc.
3. After approval, restate the command verbatim, list what it will affect, and wait for confirmation.
4. When a destructive command is run, record in your response:
   - The exact user text authorizing it
   - The command run
   - When you ran it

If that audit trail is missing, then you must act as if the operation never happened.


---

### Code Editing Discipline

- Do **not** run scripts that bulk-modify code (codemods, invented one-off scripts, giant `sed`/regex refactors).
- Large mechanical changes: break into smaller, explicit edits and review diffs.
- Subtle/complex changes: edit by hand, file-by-file, with careful reasoning.

- The bar for adding files is very high.

---

### Third-Party Libraries

When unsure of an API, look up current docs (late-2025) rather than guessing.

---


---

## MCP Agent Mail — Multi-Agent Coordination

Agent Mail is already available as an MCP server; do not treat it as a CLI you must shell out to.

What it gives:

- Identities, inbox/outbox, searchable threads.
- Advisory file reservations (leases) to avoid agents clobbering each other.
- Persistent artifacts in git (human-auditable).

Core patterns:

1. **Same repo**
   - Register identity:
     - `ensure_project` then `register_agent` with the repo’s absolute path as `project_key`.
   - Reserve files before editing:
     - `file_reservation_paths(project_key, agent_name, ["src/**"], ttl_seconds=3600, exclusive=true)`.
   - Communicate:
     - `send_message(..., thread_id="FEAT-123")`.
     - `fetch_inbox`, then `acknowledge_message`.
   - Fast reads:
     - `resource://inbox/{Agent}?project=<abs-path>&limit=20`.
     - `resource://thread/{id}?project=<abs-path>&include_bodies=true`.
   - Optional:
     - Set `AGENT_NAME` so the pre-commit guard can block conflicting commits.
     - `WORKTREES_ENABLED=1` and `AGENT_MAIL_GUARD_MODE=warn` during trials.
     - Check hooks with `mcp-agent-mail guard status .` and identity with `mcp-agent-mail mail status .`.

2. **Multiple repos in one product**
   - Option A: Same `project_key` for all; use specific reservations (`frontend/**`, `backend/**`).
   - Option B: Different projects linked via:
     - `macro_contact_handshake` or `request_contact` / `respond_contact`.
     - Use a shared `thread_id` (e.g., ticket key) for cross-repo threads.

Macros vs granular:

- Prefer macros when speed is more important than fine-grained control:
  - `macro_start_session`, `macro_prepare_thread`, `macro_file_reservation_cycle`, `macro_contact_handshake`.
- Use granular tools when you need explicit behavior.

Product bus:

- Create/ensure product: `mcp-agent-mail products ensure MyProduct --name "My Product"`.
- Link repo: `mcp-agent-mail products link MyProduct .`.
- Inspect: `mcp-agent-mail products status MyProduct`.
- Product inbox: `mcp-agent-mail products inbox MyProduct YourAgent --limit 50 --urgent-only --include-bodies`.

Server-side tools (for orchestrators) include:

- `ensure_product(product_key|name)`
- `products_link(product_key, project_key)`
- `resource://product/{key}`
- `search_messages_product(product_key, query, limit=20)`

Common pitfalls:

- “from_agent not registered” → call `register_agent` with correct `project_key`.
- `FILE_RESERVATION_CONFLICT` → adjust patterns, wait for expiry, or use non-exclusive reservation.
- Auth issues with JWT+JWKS → bearer token with `kid` matching server JWKS; static bearer only when JWT disabled.

---

## Issue Tracking with tk

This project uses `tk` for issue tracking.
Tickets live in `.tickets/` and should be committed with related code changes.
Run `tk help` when you need more commands.

## This project

This project is the OSS tool "code organization". Read README.md for more information.


---


This project uses `tk` for issue tracking. Run `tk help` when you need it.

### Workflow Pattern

3. **Work**: Implement the task

### Key Concepts

- **Priority**: P0=critical, P1=high, P2=medium, P3=low, P4=backlog (use numbers, not words)
- **Types**: task, bug, feature, epic, question, docs

### Session Protocol

**Before ending any session, run this checklist:**

```bash
git status              # Check what changed
git add <files>         # Stage code changes
git commit -m "..."     # Commit code
git push                # Push to remote
```

### Best Practices

- Update status as you work (in_progress → closed)
- Use descriptive titles and set appropriate priority/type

# respec Operational Runbook

## Health checks

1. `respec doctor`
2. `respec help`
3. `respec history -a | head`

## Common incidents

### `respec: wrapped sessions require a git workspace`

- Cause: command run outside a git repository.
- Action: run inside a repository root or initialize one (`git init`).

### Missing history entries

1. Check hidden state:
   - `respec history -a`
   - `respec ls --hidden`
2. Verify range/filter arguments:
   - `respec history -from 1 -to 100`
3. Confirm `.respec/state.db` exists.

### Overlay not opening

1. Run `respec doctor`.
2. If not in tmux and not macOS Terminal, overlay is currently unsupported.
3. For debug traces, run with `SD_PANEL_DEBUG=1`.

## Recovery actions

### Rebuild generated spec

1. `respec spec`
2. Verify:
   - `.respec/state.db` updated (generated spec snapshot persisted)

### State repair (local)

1. Back up `.respec/` directory.
2. Validate SQLite state in `.respec/state.db` (schema + tables readable).
3. Re-run `respec init` to restore embedded assets.

## Rollback guidance

- For binary regressions, roll back to previous released tag.
- Preserve `.respec/` backup before downgrade/upgrade operations.

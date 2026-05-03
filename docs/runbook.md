# sd Operational Runbook

## Health checks

1. `sd doctor`
2. `sd help`
3. `sd history -a | head`

## Common incidents

### `sd: wrapped sessions require a git workspace`

- Cause: command run outside a git repository.
- Action: run inside a repository root or initialize one (`git init`).

### Missing history entries

1. Check hidden state:
   - `sd history -a`
   - `sd ls --hidden`
2. Verify range/filter arguments:
   - `sd history -from 1 -to 100`
3. Confirm `.sd/state.db` exists.

### Overlay not opening

1. Run `sd doctor`.
2. If not in tmux and not macOS Terminal, overlay is currently unsupported.
3. For debug traces, run with `SD_PANEL_DEBUG=1`.

## Recovery actions

### Rebuild generated spec

1. `sd spec`
2. Verify:
   - `.sd/state.db` updated (generated spec snapshot persisted)

### State repair (local)

1. Back up `.sd/` directory.
2. Validate SQLite state in `.sd/state.db` (schema + tables readable).
3. Re-run `sd init` to restore embedded assets.

## Rollback guidance

- For binary regressions, roll back to previous released tag.
- Preserve `.sd/` backup before downgrade/upgrade operations.

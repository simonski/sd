# USER GUIDE

This guide explains each `sd` command and typical workflows.

## Mental model

- `sd <agent> ...` wraps an agent binary and records interaction/session artifacts in `.sd/`.
- Session numbering is stable integer-based (`0, 1, 2, ...`).
- You can list, inspect, hide, unhide, remove, and summarize sessions.

## Core setup

### `sd init`

Initializes/updates `.sd/` in the current git repo, including config and bootstrap assets.

```bash
sd init
```

### `sd version`

Prints the current CLI version.

```bash
sd version
```

## Wrapped agent runs

### `sd <agent-binary> [args...]`

Runs an agent command while capturing session metadata and conversation artifacts.

Examples:

```bash
sd copilot
sd codex
sd claude
sd sh -c "echo hi"
```

## Session listing and inspection

### `sd ls`

Lists visible sessions with stable session number, state, interaction count, and changed files.

```bash
sd ls
```

### `sd ls N`

Shows an abbreviated interaction view for session `N` (timestamp + input preview).

```bash
sd ls 0
```

### `sd ls` filters/options

- `-a`, `--all` — include hidden sessions (hidden rows appear dimmed)
- `--hidden` — only hidden sessions
- `--active` — only in-progress sessions
- `--agent <name>` — filter by command/agent name
- `--since <RFC3339|YYYY-MM-DD>` — time filter
- `--timeline`, `-t` — raw event timeline mode
- `--verbose` / `--compact` — output density

Examples:

```bash
sd ls -a
sd ls --hidden
sd ls --agent copilot --since 2026-04-30
sd ls --timeline --active
```

### `sd cat N`

Shows full details for session `N` (metadata + conversation/log view).

```bash
sd cat 0
```

### `sd get N`

Prints cleaned **user input** for session `N`.

```bash
sd get 0
```

## Session lifecycle commands

### `sd hide N`

Soft-delete session `N` (excluded from default `ls`/`cat` views).

```bash
sd hide 0
```

### `sd unhide N`

Restore a hidden session (index from `sd ls --hidden`).

```bash
sd ls --hidden
sd unhide 0
```

### `sd rm N`

Hard-delete session `N` and related artifacts.

```bash
sd rm 0
```

### `sd prune`

Removes hidden sessions and orphaned artifacts.

```bash
sd prune
```

## Input history commands

### `sd inputs` and `sd history`

`history` is an alias of `inputs`. Prints user-side conversation sequence across sessions, grouped by day and ordered by time.

```bash
sd inputs
sd history
sd history -a
```

`-a/--all` includes hidden sessions.

## Spec generation

### `sd spec`

Builds a generated spec view from baseline spec pointers + captured interactions and writes `.sd/spec.generated.md`, while also printing it to stdout.

```bash
sd spec
```

## Common workflow

```bash
sd init
sd copilot
sd ls
sd ls 0
sd get 0
sd spec
```

# USER GUIDE

This guide reflects current `sd` behavior and is organized for fast task completion.

## 1. Day-one setup

```bash
sd init
alias copilot="sd copilot"
```

`sd init` creates `.sd/` state and can install the embedded `sd` skill.

## 2. Core workflow

1. Start your wrapped agent session.
2. Work against your baseline spec (`SPEC.md` / `PRD.md`).
3. Reconcile current behavior into generated spec output.

```bash
copilot
# work...
sd spec
sd history
```

## 3. Command reference

### Cross-session history (interaction IDs)

```bash
sd history
sd ls
sd history -o
sd history 120-125
sd history 120 125
sd history -from 10 -to 30
sd hide 12
sd unhide 12
```

### Session operations

```bash
sd ls
sd ls 1
sd cat 1
sd hide 1
sd unhide 1
sd rm 1
```

### Diagnostics

```bash
sd doctor
```

## 4. Troubleshooting

### `sd: wrapped sessions require a git workspace`

Run `sd` inside a git repository.

### Overlay won’t open

Run `sd doctor`. Overlay is supported in tmux and native macOS Terminal.  
Set `SD_PANEL_DEBUG=1` for debug diagnostics.

### History looks incomplete

Check hidden entries and filters:

```bash
sd history -a
sd ls --hidden
```

### Color output issues

Disable ANSI colors:

```bash
NO_COLOR=1 sd history
```

## 5. Notes

- `sd history` is an alias for `sd inputs`.
- `sd cat` and `sd get` provide per-session detail views.

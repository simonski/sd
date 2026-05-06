# USER GUIDE

This guide reflects current `respec` behavior and is organized for fast task completion.

## 1. Day-one setup

```bash
respec init
alias copilot="respec copilot"
```

`respec init` creates `.respec/` state and can install the embedded `respec` skill.

## 2. Core workflow

1. Start your wrapped agent session.
2. Work against your baseline spec (`SPEC.md` / `PRD.md`).
3. Reconcile current behavior into generated spec output.

```bash
copilot
# work...
respec spec
respec history
```

## 3. Command reference

### Cross-session history (interaction IDs)

```bash
respec history
respec ls
respec history -o
respec history 120-125
respec history 120 125
respec history -from 10 -to 30
respec hide 12
respec unhide 12
```

### Session operations

```bash
respec ls
respec ls 1
respec cat 1
respec hide 1
respec unhide 1
respec rm 1
```

### Diagnostics

```bash
respec doctor
```

## 4. Troubleshooting

### `respec: wrapped sessions require a git workspace`

Run `respec` inside a git repository.

### Overlay won’t open

Run `respec doctor`. Overlay is supported in tmux and native macOS Terminal.  
Set `SD_PANEL_DEBUG=1` for debug diagnostics.

### History looks incomplete

Check hidden entries and filters:

```bash
respec history -a
respec ls --hidden
```

### Color output issues

Disable ANSI colors:

```bash
NO_COLOR=1 respec history
```

## 5. Notes

- `respec history` is an alias for `respec inputs`.
- `respec cat` and `respec get` provide per-session detail views.

# sd

`sd` wraps agent CLIs (Copilot, Codex, Claude, shell commands), records interactions in `.sd/state.db`, and provides history/session inspection plus generated spec output.

## Install

### Homebrew

```bash
brew install simonski/tap/sd
```

### From source

```bash
git clone https://github.com/simonski/sd.git
cd sd
make build
./bin/sd help
```

## Build and test

```bash
make build
make test
make cover
make vet
```

`make build` increments the patch version in `VERSION` before building.

## Quick usage

```bash
sd init
sd copilot

# session and interaction history
sd ls
sd ls 0
sd cat 0
sd history
sd history -o  # include agent responses (`< ...`) with one-space-indented timestamps
sd history -from 10 -to 30
sd hide 12
sd unhide 12
sd rm 12

# diagnostics and spec
sd doctor
sd spec
```

`sd init` prompts before creating the embedded agent skill at `.sd/skills/sd/SKILL.md`.

`-from` and `-to` define an inclusive range (both endpoints included).
Range shorthand is also supported for history:
- `sd history 120-125`
- `sd history 120 125`

## Command model

- Session rows are selected by numeric index (`sd ls N`, `sd cat N`).
- `sd history` is an alias for `sd inputs`.
- `sd hide`/`sd unhide`/`sd rm` operate on session index values from `sd ls`.

## Compatibility policy

- Existing top-level commands and aliases are compatibility-sensitive.
- Removed commands return migration guidance at runtime and in docs.
- Range forms are inclusive and stable:
  - `N-M`
  - `N M`
  - `-from ... -to ...`

## Spec output

`sd spec` writes generated spec content to **stdout** and persists the latest generated content in `.sd/state.db`.

## Golden workflow: baseline -> build -> reconcile

```bash
sd init
sd copilot
# ... work with your baseline spec (SPEC.md / PRD.md)
sd spec
sd history
```

Expected persisted state after `sd spec`:
- `.sd/state.db` (generated spec snapshot + interaction/session state)

## Overlay shortcuts while wrapping an agent

During `sd <agent> ...`, press `Esc`, `` ` ``, or `~` twice quickly to open/focus the spec overlay. Press twice again (or single `Esc`) to dismiss.

- In **tmux**, it uses a popup overlay.
- In **macOS Terminal** (outside tmux), it uses a native terminal overlay.

Set `SD_PANEL_DEBUG=1` to print overlay debug diagnostics.

## Accessibility and no-color usage

- Hidden entries include explicit `[hidden]` text markers.
- Set `NO_COLOR=1` to disable ANSI color formatting.

## Troubleshooting

- `sd: wrapped sessions require a git workspace`  
  Run inside a git repository.
- Overlay not opening  
  Run `sd doctor`; use tmux or macOS Terminal.
- Missing entries in `sd history`  
  Check `sd history -a` and `sd ls --hidden`.

## Command aliases

| Command | Alias / Relationship |
|---|---|
| `sd history` | alias for `sd inputs` |
| `sd ls` | session listing (supports `sd ls N`) |

## Documentation

See:
- [USER_GUIDE.md](./USER_GUIDE.md)
- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [SECURITY.md](./SECURITY.md)
- [docs/runbook.md](./docs/runbook.md)
- [docs/data-retention.md](./docs/data-retention.md)
- [docs/threat-model.md](./docs/threat-model.md)
- [docs/architecture.md](./docs/architecture.md)
- [docs/roadmap.md](./docs/roadmap.md)
- [docs/cli-contract.json](./docs/cli-contract.json)

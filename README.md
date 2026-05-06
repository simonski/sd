# respec

`respec` wraps agent CLIs (Copilot, Codex, Claude, shell commands), records interactions in `.respec/state.db`, and provides history/session inspection plus generated spec output.

## Install

### Homebrew

```bash
brew install simonski/tap/respec
```

### From source

```bash
git clone https://github.com/simonski/respec.git
cd respec
make build
./bin/respec help
```

## Build and test

```bash
make build
make test
```

`make build` increments the patch version in `VERSION` before building.
`make test` runs the same checks as GitHub CI (unit tests, coverage, vet, benchmark thresholds, vulnerability scan, and GoReleaser config validation).

## Quick usage

```bash
respec init
respec copilot

# session and interaction history
respec ls
respec ls 0
respec cat 0
respec history
respec history -o  # include agent responses (`< ...`) with one-space-indented timestamps
respec history -from 10 -to 30
respec hide 12
respec unhide 12
respec rm 12

# diagnostics and spec
respec doctor
respec spec
```

`respec init` prompts before creating the embedded agent skill at `.respec/skills/respec/SKILL.md`.

`-from` and `-to` define an inclusive range (both endpoints included).
Range shorthand is also supported for history:
- `respec history 120-125`
- `respec history 120 125`

## Command model

- Session rows are selected by numeric index (`respec ls N`, `respec cat N`).
- `respec history` is an alias for `respec inputs`.
- `respec hide`/`respec unhide`/`respec rm` operate on session index values from `respec ls`.

## Compatibility policy

- Existing top-level commands and aliases are compatibility-sensitive.
- Removed commands return migration guidance at runtime and in docs.
- Range forms are inclusive and stable:
  - `N-M`
  - `N M`
  - `-from ... -to ...`

## Spec output

`respec spec` writes generated spec content to **stdout** and persists the latest generated content in `.respec/state.db`.

## Golden workflow: baseline -> build -> reconcile

```bash
respec init
respec copilot
# ... work with your baseline spec (SPEC.md / PRD.md)
respec spec
respec history
```

Expected persisted state after `respec spec`:
- `.respec/state.db` (generated spec snapshot + interaction/session state)

## Overlay shortcuts while wrapping an agent

During `respec <agent> ...`, press `Esc`, `` ` ``, or `~` twice quickly to open/focus the spec overlay. Press twice again (or single `Esc`) to dismiss.

- In **tmux**, it uses a popup overlay.
- In **macOS Terminal** (outside tmux), it uses a native terminal overlay.

Set `SD_PANEL_DEBUG=1` to print overlay debug diagnostics.

## Accessibility and no-color usage

- Hidden entries include explicit `[hidden]` text markers.
- Set `NO_COLOR=1` to disable ANSI color formatting.

## Troubleshooting

- `respec: wrapped sessions require a git workspace`  
  Run inside a git repository.
- Overlay not opening  
  Run `respec doctor`; use tmux or macOS Terminal.
- Missing entries in `respec history`  
  Check `respec history -a` and `respec ls --hidden`.

## Command aliases

| Command | Alias / Relationship |
|---|---|
| `respec history` | alias for `respec inputs` |
| `respec ls` | session listing (supports `respec ls N`) |

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

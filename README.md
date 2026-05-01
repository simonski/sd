# sd

`sd` is a spec-driven wrapper for agentic CLIs (Copilot, Codex, Claude, shell commands). It runs the agent command, captures conversation/state in `.sd/`, and provides tools to inspect sessions and regenerate spec views.

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
```

## Publish

`make publish` will:
1. Ensure a semantic tag exists (auto-creates/pushes one if needed).
2. Use local `gh auth` credentials when token env vars are not set.
3. Run GoReleaser to publish GitHub release artifacts and Homebrew formula updates.

```bash
make publish
```

Optional explicit tag:

```bash
make publish PUBLISH_TAG=v0.2.0
```

## Quick usage

```bash
sd init
sd copilot
sd ls
sd ls 0
sd cat 0
sd history
sd doctor
sd spec
```

During `sd <agent> ...`, press `Esc`, `` ` ``, or `~` twice quickly to open/focus the spec overlay. While it is open, press any of those keys twice quickly (or `Esc` once) to dismiss and return focus to the agent.

- In **tmux**, this uses a left popup overlay (~2/3 width).
- In **macOS Terminal** (non-tmux), this uses a native in-terminal overlay renderer.

## Documentation

See [USER_GUIDE.md](./USER_GUIDE.md) for full command-by-command usage and examples.

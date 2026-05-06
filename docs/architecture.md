# Architecture (Current + Target)

## Current runtime model

- Single CLI binary (`cmd/respec`)
- Local state directory `.respec/`
- Local SQLite state in `.respec/state.db` (interactions, sessions, visibility, conversations, generated spec)

## Bounded areas

1. Command parsing and dispatch
2. Session wrapping and lifecycle capture
3. Persistence/state storage
4. History/session rendering
5. Overlay and terminal capability handling

## Refactor direction

`cmd/respec/main.go` is being decomposed into bounded files/packages while preserving behavior locked by tests.

Initial decomposition targets:

- `commands` (dispatch and command handlers)
- `state` (atomic writes, locking, persistence schemas)
- `overlay` (tmux/native overlay + keyboard routing)
- `render` (history/session formatting)
- `doctor` (terminal capability detection and diagnostics)

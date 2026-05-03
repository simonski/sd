# sd — Reconciled Specification (Baseline + Recorded Interactions)

> Status: Active  
> Reconciled from baseline `SPEC.md` and persisted `sd` interaction history (`sd spec`, `sd history -a`)  
> Scope: Current implemented behavior in this repository

## 1. Product intent

`sd` wraps interactive coding agents/CLIs, captures conversation and repo-change context, and provides commands to inspect that history plus generate a spec view from stored state.

The tool is designed for local, repo-scoped use and prioritizes recoverable history over ephemeral terminal output.

## 2. Canonical state model

1. State directory is `.sd/` at repo root.
2. Canonical persistence backend is SQLite at `.sd/state.db`.
3. Legacy file-based state is migrated into DB-backed structures during initialization/state ensure.
4. Session conversation logs are persisted as JSON message sequences under `.sd/sessions/*.conversation.json` (with DB migration support for older stdin/stdout log formats).

## 3. Implemented command surface

The current executable dispatch supports these top-level built-ins:

- `help`
- `version` (prints semantic version only, e.g. `1.2.3`)
- `init`
- `spec`
- `ls`
- `cat`
- `hide`
- `unhide`
- `rm`
- `prune`
- `inputs`
- `history` (alias of `inputs`)
- `get`
- `doctor`

Any other first token is treated as an agent binary to wrap (e.g. `sd copilot`, `sd claude`, `sd codex`).

## 4. Core behaviors

### 4.1 Initialization

`sd init` ensures `.sd/` exists, initializes/opens DB state, migrates legacy state where present, and extracts embedded bootstrap assets (including skill assets).

### 4.2 Wrapped agent sessions

When running `sd <agent> ...`:

- A new session key is created (`<UTC timestamp>-<agent>`).
- Session number mapping is maintained via session index state.
- Start/update/final events are appended to interaction history.
- User input is persisted on submit (Enter), not per keystroke.
- Assistant output is persisted line-by-line from terminal output.
- Conversation logs are written as role-tagged message sequences.
- Incremental update events can include changed file snapshots with debounce/min-interval behavior.

### 4.3 Session and history inspection

- `sd ls`: list session summaries (supports filters such as `--hidden`, `--all`, `--active`, `--since`, `--agent`, `--timeline`, `--verbose`).
- `sd ls N`: abbreviated view for one session.
- `sd cat N`: full detail for one session.
- `sd hide N` / `sd unhide N`: soft visibility controls.
- `sd rm N`: hard-delete one session’s persisted artifacts and corresponding events.
- `sd prune`: hard-delete hidden sessions and orphan logs.
- `sd history` / `sd inputs`: cross-session chronological dialog-style view.
- `sd history -a`: include hidden sessions.
- `sd history -o`: include assistant output lines prefixed with `<`.
- `sd get N`: prints cleaned user-only input for a session.

### 4.4 Spec generation

`sd spec`:

1. Reads baseline spec pointers from state config (auto-discovered `SPEC.md` files).
2. Renders baseline spec content + curated session summary + ordered session inputs + timeline sections.
3. Writes generated spec view to stdout.
4. Persists generated output into `.sd/state.db`.

## 5. Overlay and terminal diagnostics

- Overlay toggling is implemented for wrapped sessions (tmux popup and native macOS terminal overlay paths).
- `sd doctor` reports terminal and overlay capability diagnostics.
- `SD_PANEL_DEBUG=1` enables overlay debug output.

## 6. Reconciled decisions captured from interaction history

These behavior shifts are reflected in implementation/state:

1. Move toward DB-backed `.sd/state.db` as primary source of truth.
2. Capture user input at submit boundaries; avoid character-by-character persistence.
3. Persist assistant output incrementally at line boundaries.
4. Keep history readable in timeline form (date grouping, wrapped text, continuation formatting).
5. Keep `sd spec` as stdout-first generation plus persisted generated snapshot.
6. Keep soft-delete/hard-delete distinction for history artifacts.

## 7. Explicit conflict resolution (intent vs current implementation)

Interaction history and docs mention newer command model elements (`session` subcommands, `checkpoint`, `count`, `show`, range syntax for `history/hide/show`, and removal of some legacy commands).

Current command dispatch in `cmd/sd/main.go` does **not** implement those tokens as built-ins; they currently fall through to agent-wrapper lookup and fail if no matching binary exists. Therefore, this reconciled spec treats them as **not implemented in the current executable**, regardless of documentation text that advertises them.

## 8. Testable acceptance criteria for this reconciled state

1. `sd version` returns semantic-only output.
2. `sd init` creates/updates `.sd/state.db` in a git repo.
3. `sd history` prints chronological input history; `-a` and `-o` alter inclusion/rendering as described.
4. `sd spec` prints generated spec view and persists generated output in DB state.
5. `sd ls`, `sd ls N`, `sd cat N`, `sd hide N`, `sd unhide N`, `sd rm N`, and `sd prune` operate on numeric session indexes.
6. `sd checkpoint ...` and `sd session ...` are currently non-functional in dispatch and are not accepted as implemented behavior.

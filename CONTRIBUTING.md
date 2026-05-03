# Contributing

## Development workflow

1. Fork/branch from `main`.
2. Implement changes with tests.
3. Run:
   - `make test`
   - `go test -cover ./...`
4. Open PR with a clear summary and risk notes.

## Coding expectations

- Keep behavior explicit; avoid silent failures.
- Prefer small, focused changes.
- Preserve CLI compatibility unless deprecation/replacement is documented.

## Command compatibility policy

- Existing top-level commands and aliases are compatibility-sensitive.
- Removed commands must provide migration guidance in docs and runtime errors.
- Range semantics (`N-M`, `N M`, `-from/-to`) are inclusive and must remain stable.

## Release expectations

- CI must pass.
- Security/operational docs must remain up to date when behavior changes.


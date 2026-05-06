# Threat Model Checklist

## Assets

- Local interaction history and conversation logs in `.respec/`
- Release artifacts (binary, checksums, formula updates)
- Command contract and behavior semantics

## Trust boundaries

- User terminal environment variables
- Wrapped agent process execution
- Filesystem writes under repository `.respec/`

## Primary threat scenarios

1. Environment abuse via shell execution paths (overlay commands).
2. Local state corruption via interrupted writes or concurrent writes.
3. Sensitive conversation data exposure on shared machines.
4. Supply-chain risks in dependency or release tooling.

## Controls

- Input/identifier validation for session and interaction selectors.
- State write locking and atomic file writes (temp + rename + fsync).
- Security policy with disclosure channel (`SECURITY.md`).
- CI checks for tests, coverage, vet, and vulnerability scan.

## Residual risk notes

- Overlay shell invocation still depends on host shell behavior.
- Local data protection depends on host user permissions.


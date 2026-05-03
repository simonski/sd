---
name: sd
description: Rebuild a project specification from the baseline spec and captured sd interaction history so the resulting spec reflects implemented behavior and decisions.
version: 1.0.0
author: Simon
---

# sd-spec-reconciliation

## When to use
Use this skill when the user invokes `/sd spec` or asks to reconcile a baseline specification with recorded `sd` interactions.

## Objective
Take the original specification and all captured `sd` interactions, then rebuild a new specification (for example `SPEC-2.md`) that accurately represents what was actually changed and implemented.

## Required workflow
1. Identify baseline spec source.
   - Prefer explicit user instruction.
   - If ambiguous, ask whether `SPEC.md` or `PRD.md` is canonical.
2. Collect complete interaction context with `sd`.
   - Run `sd spec` first (this applies a new checkpoint and resets `applies-from`).
   - Confirm checkpoint/state context with `sd checkpoint current` and `sd history -a` (`.sd/state.db` persists state).
   - Gather raw interaction history with:
     - `sd history -a`
     - `sd session ls -a`
     - `sd session history S#### -a` when deeper session review is needed.
3. Reconcile original spec and interactions.
   - Preserve intent from baseline unless contradicted by implemented behavior.
   - Prefer implemented behavior and recorded decisions over stale baseline text.
   - Resolve conflicts explicitly in the rewritten spec.
4. Produce the rebuilt specification.
   - Default target: `SPEC-2.md` unless the user requests a different path.
   - Ensure it accurately reflects the current project behavior and scope.

## Output expectations
- The rewritten spec must be internally consistent and testable.
- It must reflect what the project actually does now, not only initial plans.
- Record key behavior changes introduced since the baseline spec.

# Software Factories: Spec-Driven Agentic Engineering

> A methodology for disciplined, living software development that embraces both top-down spec-driven delivery and bottom-up exploratory (vibe) engineering — unified by a living specification, pluggable SDLC profiles, and an adversarial agent ensemble.

---

## Genesis: Original Idea (verbatim)

> Software Factories, SDLC, Specs and Vibing.
>
> I want to work on a methodology that embraces agentic engineering, with spec-driven development, where we start from a spec, refine it with an agent until we are happy, perform a breakdown to be able to track implementation progress and compliance, then release a version.
>
> That would be the spec-driven part.
>
> Then I want a healthy dose of reality in it insofar as once we have a piece of software, I want to be able to keep chipping away at the marble to release the david, as it were - that is, permit vibing/discovery, with minimal friction to an engineer - at the same time capturing these additive/amendments to a piece of software by keeping a specification up to date.

---

## Refinement Conversation (30 April 2026)

### Turn 1 — Copilot: Reflecting the core loop and probing

The core loop described:

```
Spec → [Agent refinement] → Breakdown → Implement → Release
                                                        ↓
                                              Vibe/discover
                                                        ↓
                                              Spec stays in sync
```

Two distinct modes, both legitimate: **spec-driven** (disciplined, top-down) and **vibe-driven** (exploratory, bottom-up) — with the key insight being that both should feed the same living spec artifact.

Hard problems identified:

1. **Spec format** — What does a "spec" look like? Natural language + structured constraints? Machine-parseable enough to auto-generate a breakdown?
2. **The compliance gap** — How do you know implementation matches the spec? Agent auditing code? Tests generated from spec?
3. **The vibe→spec feedback loop** — When someone vibes, they don't want to stop and document. Does the agent observe what changed and propose spec amendments? At commit time? At release boundary?
4. **What triggers a version?** — Deliberate act, or when spec coverage crosses a threshold?
5. **"Software factory" framing** — Tool, methodology, or both?

---

### Turn 2 — Human: Constraints and decisions

1. **Spec format**: Must be human AND machine readable — Markdown with simple diagrams (Excalidraw, Lucid, Mermaid).
2. **Testing**: Red/green testing with adversarial role-based agents baked into the whole thing.
3. **SDLC is pluggable**: Starting with a profile that describes contemporary best practice — naming styles, coding styles, branching/git usage, test coverage, red/green testing. All the way to idiomatic specifics.
4. **Engineer interface (the wrapper)**: Minimal friction — freeform conversation within a codebase. Perhaps capturing the conversation with a wrapper which then invokes the various agentic engineering. Not fully resolved.
5. **Versioning**: Like git — *any* change is a successive, discoverable version. Not "release" in the product artifact sense.
6. **Scope**: Start with a methodology, with a reference implementation tool to follow.

---

### Turn 3 — Copilot: Synthesis diagram and open questions

**Synthesised architecture:**

```
┌─────────────────────────────────────────────────┐
│  SPEC (living markdown + mermaid/excalidraw)     │
│  ← human readable, machine parseable            │
│  ← versioned like git (every state addressable) │
└────────────┬────────────────────────────────────┘
             │
     ┌───────▼────────┐
     │  SDLC Profile  │  ← pluggable: naming, style,
     │  (config/rules)│    branching, coverage floor,
     └───────┬────────┘    test philosophy
             │
     ┌───────▼────────────────────────────────────┐
     │  Agent Ensemble                             │
     │  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
     │  │ Builder  │  │ Adversary│  │Compliance│  │
     │  │  agent   │  │  agent   │  │  agent   │  │
     │  └──────────┘  └──────────┘  └──────────┘  │
     │         red/green loop, spec-gated          │
     └───────┬────────────────────────────────────┘
             │
     ┌───────▼────────────────────────────────────┐
     │  Engineer Interface (the "wrapper")         │
     │  freeform conversation → captured intent    │
     │  → routes to agents + spec amendment        │
     └────────────────────────────────────────────┘
```

**Decisions from discussion:**

- **A. Wrapper capture boundary**: Both conversation-time and commit-time capture, to keep friction low while retaining intent context.
- **B. Adversarial agent scope**: Red tests against implementation and attacks against spec ambiguity/contradiction and SDLC profile assumptions.
- **C. SDLC profile format**: Hybrid model — declarative, human-readable config with agent instruction layering.

---

## Methodology: First Structured Draft

### Principles

1. **The Spec is the source of truth** — not the code, not the tests, not the tickets. Everything else is derived from or reconciled with the spec.
2. **Both modes are legitimate** — Spec-first and vibe-first are not in tension; they are two entry points into the same loop.
3. **Minimal engineer friction** — The methodology must not require engineers to context-switch into bureaucracy. Capture happens at natural boundaries (conversation, commit), not as extra steps.
4. **Adversarial by default** — Every implementation is challenged. Specs are challenged. SDLC compliance is enforced. The burden of proof is on correctness.
5. **Pluggable discipline** — SDLC profiles define the rules of the game. Teams choose their profile; the machinery enforces it.
6. **Every change is history** — No destructive state. Every version of the spec, every implementation state, is addressable.

---

### Lifecycle Stages

#### Stage 1: Spec Inception
- Engineer (or agent) drafts an initial spec in Markdown.
- Spec contains: intent, constraints, functional requirements, non-functional requirements, open questions.
- Diagrams (Mermaid inline, Excalidraw/Lucid linked) used to express architecture, flows, data models.
- A **Spec Refinement Agent** challenges the draft: surfaces ambiguities, missing edge cases, contradictions.
- Iteration continues until the engineer marks the spec `READY`.

#### Stage 2: Breakdown
- A **Breakdown Agent** decomposes the spec into a tracked work graph: epics → stories → tasks.
- Each item is linked back to the spec section it satisfies.
- This graph is the compliance scaffold — implementation is measured against it.
- Breakdown is stored as structured data within or alongside the spec (Markdown checklist, YAML, or both).

#### Stage 3: Implementation (Spec-Driven mode)
- Engineer implements against the breakdown.
- A **Builder Agent** assists with code generation, guided by the SDLC profile.
- A **Compliance Agent** continuously checks: does the code satisfy the linked spec item? Does it conform to SDLC profile rules (naming, style, branching, coverage)?
- An **Adversary Agent** generates red tests: attempts to break the implementation before it is considered done.
- Green = all adversary tests pass + compliance checks pass + spec item marked satisfied.

#### Stage 4: Vibe / Discovery mode
- Engineer has a freeform conversation or makes freeform changes within the codebase.
- The **Wrapper** captures intent from the conversation transcript and/or diffs.
- At a natural boundary (conversation end, commit, or explicit trigger), a **Spec Amendment Agent** proposes updates to the spec to reflect what changed.
- Engineer reviews and accepts/rejects amendments. Accepted amendments become a new spec version.
- The breakdown is re-evaluated for any new items or invalidated items.

#### Stage 5: Versioning
- Every accepted spec state is a version (git-like: content-addressed, fully traversable history).
- A "release" in the product sense is a tag on a spec+implementation version pair — a deliberate act, not an automatic threshold.
- The spec changelog is human-readable; the version graph is machine-traversable.

---

### Artifact Definitions

| Artifact | Format | Description |
|---|---|---|
| **Spec** | Markdown + Mermaid/Excalidraw | Living document. Source of truth. Versioned. |
| **Breakdown** | Markdown checklist or YAML | Hierarchical work graph derived from spec. Compliance scaffold. |
| **SDLC Profile** | YAML/TOML (or prompt instructions) | Rules for coding style, naming, branching, test coverage floors, test philosophy. |
| **Conversation Log** | Markdown transcript | Captured engineer intent. Input to spec amendment. |
| **`.respec/` Workspace** | Repo-local directory | Stores spec pointers, captured interaction state, and reconciliation artifacts for the wrapper. |
| **Test Suite** | Language-native | Generated and/or written. Red tests from Adversary Agent; green tests from Builder Agent. |
| **Version Graph** | Git history + spec metadata | Every spec+implementation state. Fully addressable. |

---

### Agent Roles

#### Spec Refinement Agent
- **Input**: Draft spec
- **Output**: Questions, contradiction flags, missing edge case callouts
- **Goal**: Elevate spec quality before breakdown

#### Breakdown Agent
- **Input**: Approved spec
- **Output**: Hierarchical work graph (epics/stories/tasks), each linked to spec
- **Goal**: Make implementation trackable and compliance measurable

#### Builder Agent
- **Input**: Task from breakdown, SDLC profile, codebase context
- **Output**: Code, inline with SDLC constraints
- **Goal**: Implement against spec with style/quality adherence

#### Adversary Agent
- **Input**: Implementation, spec item, SDLC profile
- **Output**: Red tests, edge case challenges, spec ambiguity flags
- **Scope**: Attacks implementation, spec quality, and SDLC profile compliance assumptions
- **Goal**: Break before it ships

#### Compliance Agent
- **Input**: Code diff, SDLC profile, breakdown item
- **Output**: Pass/fail per rule, with remediation suggestions
- **Goal**: Continuous enforcement of profile rules

#### Spec Amendment Agent
- **Input**: Conversation transcript and/or code diff
- **Output**: Proposed spec amendments (additions, modifications, deprecations)
- **Goal**: Keep spec in sync with exploratory/vibe work, with minimal engineer effort

---

### SDLC Profile Contract

A profile is a versioned, named configuration that defines:

```yaml
profile:
  name: "contemporary-best-practice"
  version: "1.0.0"

coding:
  style: "idiomatic"          # language-specific style guides
  naming: "descriptive"       # e.g. no abbreviations, domain language
  max_function_length: 40     # lines
  no_magic_numbers: true

testing:
  philosophy: "red-green-refactor"
  coverage_floor: 80          # percent
  adversarial: true           # Adversary Agent enabled
  test_location: "colocated"  # tests next to source

branching:
  strategy: "trunk-based"
  branch_naming: "{type}/{ticket-id}-{short-description}"
  require_pr: true
  min_reviewers: 1

spec:
  require_breakdown: true
  amendment_on: ["commit", "conversation-end"]
  version_on: ["amendment-accepted"]
```

Profiles are composable and overridable. A team starts from a base profile and layers overrides.

---

## Reference Implementation v0 (CLI: `respec`)

- `respec` has two command classes: hardcoded product commands and pass-through agent runtime commands.
- Hardcoded product commands include `respec init`, `respec help`, and other explicit lifecycle/config commands.
- `respec init` creates or updates `.respec/` in the current repo, including pointers to source spec files and local state metadata.
- `respec <agent-binary> [args...]` launches a target agentic programming binary as a transparent terminal wrapper.
- Initial agent-runtime targets include `copilot`, `codex`, and `claude` (for example: `respec copilot`, `respec codex`, `respec claude`).
- The wrapper delegates all keystrokes and terminal output end-to-end (no workflow interruption).
- The wrapper implicitly captures the interactive stream (stdin/stdout + session metadata) as conversation intent in `.respec/`.
- `respec spec` reconstructs/proposes the current spec by combining baseline spec files, captured conversation intent, and notable change sequences (features, bugs, and amendments inferred from diffs/history).
- Commit-time hooks consume captured intent plus git diff to drive spec amendment proposals.

---

## Open Questions (active)

- [x] **Wrapper boundary**: Use both conversation-time and commit-time capture to minimize engineer friction while preserving intent
- [x] **Adversary scope**: Also includes spec + SDLC profile attacks (not implementation-only)
- [x] **SDLC profile format**: Hybrid (declarative + agent instruction layer), human-readable first
- [~] **Diagram tooling**: Deferred for now; revisit after CLI wrapper v0 shape is stable
- [x] **Reference implementation**: Start with a Go-based CLI wrapper distributed via Homebrew, then expand to VS Code extension and MCP server

---

## Next Steps

- [ ] Review and stress-test the Agent Roles — are there gaps or overlaps?
- [ ] Define the Spec format schema (what sections are required, what is optional?)
- [ ] Define full Go CLI v0 command surface (hardcoded `respec` commands + pass-through agent commands)
- [ ] Define `.respec/` layout (spec pointers, interaction logs, reconciliation outputs, metadata)
- [ ] Define git hook lifecycle for commit-time reconciliation
- [ ] Define `respec spec` assembly rules (merge/priority model across baseline spec, conversations, and code-change signals)
- [ ] Draft a worked example: pick a trivial piece of software and walk the full lifecycle on paper
- [ ] Sequence the reference implementation roadmap (Go CLI + Homebrew first, then extension and MCP)

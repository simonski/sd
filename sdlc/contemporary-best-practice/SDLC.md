# SDLC Profile: Contemporary Best Practice

> **Profile ID**: `contemporary-best-practice`  
> **Version**: 1.0.0  
> **Status**: ACTIVE  
> **Scope**: General-purpose software engineering. Language-agnostic unless noted. Intended as a base profile — teams layer overrides on top.

---

## How to Read This Document

Rules are grouped by **role**. Each rule is tagged:

- `[MUST]` — non-negotiable; compliance agent will flag violations
- `[SHOULD]` — strong default; deviations require a recorded reason
- `[MAY]` — recommended practice; no enforcement

---

## 1. Engineer (All)

### Code Quality

- `[MUST]` Write code for the next reader, not the machine. Optimise for clarity first.
- `[MUST]` No magic numbers or magic strings — name every constant at the point of declaration.
- `[MUST]` No commented-out code in committed files. Delete it; git remembers.
- `[MUST]` Functions do one thing. If you need "and" to describe what a function does, split it.
- `[SHOULD]` Keep functions under 40 lines. Keep files under 400 lines. Exceptions require a comment explaining why.
- `[SHOULD]` Prefer pure functions. Isolate side effects at the edges of the system.
- `[SHOULD]` No abbreviations in names unless the abbreviation is the industry-standard term (e.g. `url`, `http`, `id`).
- `[SHOULD]` Use the language of the domain in names, not the language of the implementation (e.g. `createOrder`, not `makeNewOrderObject`).
- `[MAY]` Prefer composition over inheritance.
- `[MAY]` Avoid `else` after a `return` — flatten the happy path.

### Error Handling

- `[MUST]` Never swallow exceptions silently. Log or re-throw.
- `[MUST]` Validate all inputs at system boundaries (API surface, CLI args, file reads). Do not validate internally between trusted functions.
- `[MUST]` Error messages must be human-readable and, where possible, suggest a fix.
- `[SHOULD]` Distinguish user errors from system errors in both exit codes and messages.
- `[SHOULD]` Do not use exceptions for control flow.

### Dependencies

- `[MUST]` Pin dependency versions in lock files. Commit lock files to version control.
- `[MUST]` No dependency added without knowing what it does. No `npm i` without reading the README.
- `[SHOULD]` Prefer standard library over a dependency for anything under ~50 lines of equivalent code.
- `[SHOULD]` Audit dependencies for known vulnerabilities before adding (e.g. `npm audit`, `pip-audit`, `osv-scanner`).
- `[SHOULD]` Remove unused dependencies promptly.
- `[MAY]` Prefer well-maintained, widely-adopted packages over clever niche ones.

### Security (general)

- `[MUST]` Never hardcode secrets, credentials, tokens, or keys. Use environment variables or a secrets manager.
- `[MUST]` Never log sensitive data (passwords, tokens, PII).
- `[MUST]` Sanitise and validate all external input. Treat all data from outside the process boundary as untrusted.
- `[MUST]` Use parameterised queries. Never interpolate user input into SQL or shell commands.
- `[SHOULD]` Follow the principle of least privilege in both code and infrastructure.
- `[SHOULD]` Prefer well-audited cryptographic libraries. Never implement your own crypto.

---

## 2. Testing

### Philosophy

- `[MUST]` Follow red-green-refactor. Write a failing test before writing implementation code.
- `[MUST]` Tests are first-class code. They live in the same repository, follow the same style rules, and are reviewed with the same rigour.
- `[MUST]` Tests must be deterministic. Flaky tests must be fixed or deleted immediately — a flaky test is worse than no test.
- `[SHOULD]` Tests must be fast. Unit tests in milliseconds; integration tests in seconds; full suite in minutes.
- `[SHOULD]` Tests should test behaviour, not implementation. Test what a function does, not how it does it.

### Coverage

- `[MUST]` Minimum 80% line coverage across the codebase. Coverage must be measured in CI.
- `[SHOULD]` 100% coverage on the core domain/business logic layer.
- `[SHOULD]` Do not write tests purely to hit a coverage number. Untested edge cases are more valuable than padded coverage on happy paths.

### Test Types

- `[MUST]` **Unit tests**: for all pure functions, domain logic, data transforms.
- `[MUST]` **Integration tests**: for every external boundary (database, file system, network, CLI interface).
- `[SHOULD]` **Contract tests**: for any service-to-service interface (API, message queue).
- `[SHOULD]` **Performance/smoke test**: at least one benchmark test per performance-critical path.
- `[MAY]` **Property-based tests**: for functions with large or complex input spaces.

### Adversarial Testing

- `[MUST]` For every feature, explicitly consider and test: empty input, maximum input, invalid type, concurrent access, interrupted operation.
- `[SHOULD]` Write at least one test that proves the system fails gracefully (not silently or catastrophically).
- `[MAY]` Use mutation testing to validate test suite strength.

### Test Organisation

- `[SHOULD]` Collocate tests with source files (e.g. `todo.ts` and `todo.test.ts` in the same directory).
- `[SHOULD]` Test file names mirror source file names.
- `[SHOULD]` Use descriptive test names: `"returns empty list when no todos exist"` not `"test1"`.
- `[MAY]` Group tests using `describe`/`context` blocks that mirror the structure of the module under test.

---

## 3. Version Control (Git)

### Commits

- `[MUST]` Every commit must pass the full test suite. Never commit broken code to a shared branch.
- `[MUST]` Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):
  ```
  <type>(<scope>): <short description>

  [optional body]

  [optional footer: BREAKING CHANGE, closes #id]
  ```
  Types: `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `ci`
- `[MUST]` Commit messages are written in the imperative mood: `"add todo list pagination"` not `"added"` or `"adds"`.
- `[SHOULD]` Commits are atomic: one logical change per commit. Do not bundle unrelated changes.
- `[SHOULD]` Commit messages explain *why*, not *what* — the diff shows what; the message shows why.

### Branching

- `[MUST]` Branching strategy: **trunk-based development**. `main` is always deployable/releasable.
- `[MUST]` Feature branches are short-lived: merged within 1–2 days. Long-running branches are a smell.
- `[MUST]` Branch naming: `{type}/{ticket-id}-{short-description}` e.g. `feat/T-3.1-add-command`
  - Types: `feat`, `fix`, `chore`, `docs`, `hotfix`
- `[MUST]` Never commit directly to `main` (except solo projects with no review requirement).
- `[SHOULD]` Rebase feature branches on `main` before merging. Prefer merge commits for the merge itself (preserves history).
- `[SHOULD]` Delete branches after merge.
- `[MAY]` Use `--no-ff` merge to preserve branch context in history.

### Pull Requests / Code Review

- `[MUST]` Every merge to `main` goes through a pull request.
- `[MUST]` PRs must include: a description of what changed and why, a link to the spec item(s) satisfied, and test evidence (CI green).
- `[MUST]` Minimum 1 reviewer on all PRs. Minimum 2 on changes to core domain logic, auth, or data schema.
- `[SHOULD]` PRs are small: aim for < 400 lines changed. Large PRs must be broken down.
- `[SHOULD]` Reviewers check: correctness, spec compliance, SDLC profile adherence, test quality — not just style (style is for linters).
- `[SHOULD]` Authors respond to all review comments before merging. Do not leave unresolved threads.
- `[MAY]` Use draft PRs for early feedback on approach before implementation is complete.

---

## 4. Tech Lead / Architect

### Design Decisions

- `[MUST]` Significant architectural decisions are recorded as Architecture Decision Records (ADRs) in `/docs/adr/`. Each ADR is immutable once accepted — new decisions supersede old ones with a new record.
- `[MUST]` ADR format: **Context → Decision → Consequences** (3 sections minimum).
- `[SHOULD]` Design for the current requirements, not imagined future requirements. Document future considerations in the spec, not in the code.
- `[SHOULD]` Prefer boring technology. Use the well-understood tool before the novel one.
- `[SHOULD]` Every system boundary (service, module, API) has a defined interface contract before implementation begins.

### Modularity

- `[MUST]` High cohesion within modules; low coupling between them. A module change should not require changes in unrelated modules.
- `[MUST]` Dependency direction is explicit and unidirectional. No circular dependencies between modules.
- `[SHOULD]` Distinguish layers: presentation / interface, application / use-case, domain, infrastructure. Dependencies point inward (domain has no dependencies on infrastructure).
- `[SHOULD]` Public API surface of a module is deliberately small. Default to unexported/private.

### Performance

- `[SHOULD]` Measure before optimising. No premature optimisation.
- `[SHOULD]` Define performance budgets per feature in the spec (NFRs). Enforce them in CI.
- `[SHOULD]` Identify the critical path before assuming where bottlenecks are.

### Documentation

- `[MUST]` Every public API, CLI command, and configuration option is documented.
- `[MUST]` A `README.md` exists at the repo root with: what the project is, how to install it, how to run it, how to run tests.
- `[SHOULD]` Architecture diagrams (Mermaid or equivalent) are kept in the repo alongside the code they describe.
- `[SHOULD]` Documentation lives as close to the code as possible. Docs in a separate wiki rot faster.
- `[MAY]` Use inline doc comments (`JSDoc`, `docstring`) for public functions in library code. Not required for application code.

---

## 5. QA / Test Engineer

### Spec Compliance

- `[MUST]` Every functional requirement in the spec has at least one test that proves it is satisfied.
- `[MUST]` Every acceptance criterion is expressed as an executable test, not a manual checklist.
- `[SHOULD]` Maintain a traceability matrix: FR/NFR → test(s). Automate this where possible.

### Adversarial Review

- `[MUST]` Before a feature is marked complete, perform an adversarial review: attempt to break it with unexpected input, race conditions, resource exhaustion, and boundary values.
- `[SHOULD]` Document adversarial cases attempted, even if they pass — this becomes the regression baseline.
- `[SHOULD]` Report spec ambiguities found during testing back to the spec as proposed amendments.

### Test Environments

- `[MUST]` Tests run in isolation — no shared mutable state between tests.
- `[MUST]` Test environment is reproducible from a single command.
- `[SHOULD]` Use test fixtures and factories rather than manual test data setup.
- `[SHOULD]` Integration tests use real implementations (real DB, real filesystem) unless cost is prohibitive. Mocks are for unit tests at system boundaries only.

---

## 6. DevOps / Platform Engineer

### CI/CD

- `[MUST]` Every push to any branch triggers a CI pipeline: lint → type-check → unit tests → integration tests → coverage check.
- `[MUST]` `main` branch is protected: CI must pass before any merge.
- `[MUST]` Builds are reproducible: given the same commit, the output is identical.
- `[SHOULD]` CI pipeline completes in under 10 minutes for developer feedback loop. Longer pipelines run asynchronously.
- `[SHOULD]` Deployment to any environment is triggered from CI, not from a developer's local machine.
- `[SHOULD]` Secrets are injected at runtime by CI/CD, never baked into build artifacts.

### Infrastructure

- `[MUST]` Infrastructure is defined as code (IaC). No manual cloud console changes.
- `[MUST]` Every environment (dev, staging, prod) is defined in version control and reproducible.
- `[SHOULD]` Environments are ephemeral by default. Spinning up a new environment is automated and documented.
- `[SHOULD]` Observability is built in from the start: logs, metrics, and traces are structured and shipped to a central store.

### Releases

- `[MUST]` Releases are tagged in git with a version number following [Semantic Versioning](https://semver.org/) (`MAJOR.MINOR.PATCH`).
- `[MUST]` A changelog is generated from commit history for every release (`CHANGELOG.md`). Format follows [Keep a Changelog](https://keepachangelog.com/).
- `[SHOULD]` Release artifacts are immutable. A released version is never modified in place.
- `[SHOULD]` Rollback procedure is documented and tested before a release goes to production.

---

## 7. Spec / Product Owner

### Spec Quality

- `[MUST]` Every feature has a spec item before implementation begins. No "just build it" work.
- `[MUST]` Functional requirements are testable. A requirement that cannot be expressed as a test is not a requirement — it is a wish.
- `[MUST]` Open questions in a spec are resolved before the spec is marked `READY`. A spec with unresolved blockers cannot proceed to breakdown.
- `[SHOULD]` Requirements are stated as outcomes, not implementations: `"user can filter todos by priority"`, not `"add a --priority flag"`.
- `[SHOULD]` Non-functional requirements have measurable acceptance criteria: `"< 100ms"`, not `"fast"`.

### Amendments

- `[MUST]` Any change to scope, requirements, or constraints during implementation is recorded as a spec amendment with a rationale.
- `[MUST]` Amendments that invalidate completed breakdown items trigger a re-assessment of affected tasks.
- `[SHOULD]` Amendments are small and incremental. A sweeping rewrite of the spec mid-implementation is a smell — consider a new version.

### Versioning

- `[MUST]` Spec version follows `MAJOR.MINOR.PATCH`:
  - `MAJOR` bump: breaking change to scope or core requirements
  - `MINOR` bump: new requirement or significant amendment
  - `PATCH` bump: clarification, correction, or non-functional change
- `[SHOULD]` Spec version is kept in sync with implementation version at release boundaries.

---

## Appendix: Quick Reference — The Non-Negotiables

| # | Rule |
|---|---|
| 1 | No secrets in code |
| 2 | No committed broken tests |
| 3 | No silent exception swallowing |
| 4 | No magic numbers or strings |
| 5 | No commented-out code |
| 6 | Every FR has a test |
| 7 | Coverage ≥ 80% enforced in CI |
| 8 | Every merge via PR, CI green |
| 9 | Conventional commits on every commit |
| 10 | No implementation without a spec item |

---

## Appendix: Tooling Suggestions (non-normative)

> These are suggestions, not mandates. Swap for equivalents that fit your stack.

| Concern | Suggested tooling |
|---|---|
| Linting | ESLint (JS/TS), Ruff (Python), golangci-lint (Go) |
| Formatting | Prettier (JS/TS), Black (Python), gofmt (Go) |
| Type checking | TypeScript compiler, mypy/pyright (Python) |
| Test runner | Vitest / Jest (JS/TS), pytest (Python) |
| Coverage | c8 / Istanbul (JS/TS), coverage.py (Python) |
| Vulnerability scanning | `npm audit`, `pip-audit`, `osv-scanner` |
| Conventional commits | commitlint + husky (JS/TS projects) |
| Changelog generation | `conventional-changelog`, `git-cliff` |
| ADR format | [adr-tools](https://github.com/npryce/adr-tools) or plain Markdown |
| IaC | Terraform, Pulumi, or CDK |
| CI | GitHub Actions, GitLab CI, Buildkite |

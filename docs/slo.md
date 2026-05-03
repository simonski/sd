# Service-Level Objectives (CLI Reliability Targets)

## Command reliability

- `sd help`, `sd history`, `sd count`, `sd doctor`, `sd spec`: target **>= 99.9% successful execution** in supported local environments.

## Performance targets (local machine, medium repository)

- `sd history` on 10k interactions: target **< 1.5s**
- `sd spec` generation on 10k interactions: target **< 4s**

## Error budget policy

- Repeated persistence-write warnings are treated as reliability incidents.
- Any regression violating above targets should block release until triaged.


# Service-Level Objectives (CLI Reliability Targets)

## Command reliability

- `respec help`, `respec history`, `respec count`, `respec doctor`, `respec spec`: target **>= 99.9% successful execution** in supported local environments.

## Performance targets (local machine, medium repository)

- `respec history` on 10k interactions: target **< 1.5s**
- `respec spec` generation on 10k interactions: target **< 4s**

## Error budget policy

- Repeated persistence-write warnings are treated as reliability incidents.
- Any regression violating above targets should block release until triaged.


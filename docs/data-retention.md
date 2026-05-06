# Data Retention and Deletion

## Data captured by `respec`

- `.respec/state.db` (interaction events, conversation messages, visibility/index metadata, generated spec snapshots)

## Default retention

- Data is retained locally until the user deletes it.

## Deletion controls

- Remove a session:
  - `respec session rm S####`
- Hide/show controls:
  - `respec session hide/show S####`
  - `respec hide/show <interaction-id>`

## Operational recommendation

- Treat `.respec/` as sensitive local data.
- Include `.respec/` in local backup policy only when needed.
- On shared systems, use strict filesystem permissions.

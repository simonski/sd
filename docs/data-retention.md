# Data Retention and Deletion

## Data captured by `sd`

- `.sd/state.db` (interaction events, conversation messages, visibility/index metadata, generated spec snapshots)

## Default retention

- Data is retained locally until the user deletes it.

## Deletion controls

- Remove a session:
  - `sd session rm S####`
- Hide/show controls:
  - `sd session hide/show S####`
  - `sd hide/show <interaction-id>`

## Operational recommendation

- Treat `.sd/` as sensitive local data.
- Include `.sd/` in local backup policy only when needed.
- On shared systems, use strict filesystem permissions.

# shared/contracts

## Ownership

The `shared/contracts/` package owns versioned contracts shared across the daemon and the client.

- IPC request and response payloads
- Health/status DTOs surfaced in the UI
- Stable event identifiers and semantic meanings
- Future schema/versioning rules for cross-process compatibility

## Boundaries

- Contracts here MUST stay transport-friendly and side-effect free.
- Runtime behavior, filesystem access, and Ubuntu-specific logic do not belong here.

When daemon and client evolve independently, this directory is the compatibility seam.

## Current versions

- `rpc/v1/` — current daemon RPC DTOs for health, sites, PHP runtimes, and service actions.

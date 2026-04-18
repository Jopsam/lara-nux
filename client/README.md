# client

## Ownership

The `client/` package owns the unprivileged desktop experience for Lara Nuxt.

- Wails shell integration and native window/tray lifecycle
- Nuxt UI, view models, and operator workflows
- Socket client transport to the privileged daemon
- Presentation of health, remediation, and bootstrap state

## Boundaries

- `wails/` is reserved for the native shell bridge.
- `ui/` is reserved for the Nuxt application.

The client MUST NOT mutate privileged machine state directly. All system changes cross the Unix socket boundary into the daemon.

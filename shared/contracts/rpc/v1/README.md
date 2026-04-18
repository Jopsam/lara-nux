# RPC Contracts v1

This directory is the versioned compatibility seam for the current daemon RPC surface.

## Scope

- `GET /rpc/health`
- `GET /rpc/sites.list`
- `GET /rpc/sites.get?siteId=...`
- `POST /rpc/sites.register`
- `POST /rpc/sites.update`
- `GET /rpc/php.list`
- `GET /rpc/php.default`
- `POST /rpc/php.default`
- `GET /rpc/php.inventory`
- `POST /rpc/php.register`
- `POST /rpc/php.switch`
- `POST /rpc/services.action`

## Artifacts

- `contracts.ts` — frontend-facing TypeScript DTOs for Wails/Nuxt consumers.
- `contracts.schema.json` — JSON Schema document for validation, tooling, and future code generation.

## Rules

- Additive changes MAY extend `v1` when backward compatible.
- Breaking changes MUST create `v2/` rather than mutating existing DTO semantics.
- Daemon handlers SHOULD map internal structs into these contracts instead of exposing handler-private shapes directly over time.

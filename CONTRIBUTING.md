# Contributing

Thanks for caring enough to contribute. Let's keep the repo maintainable from day one.

## Before you start

- Search existing issues first
- Open an issue before non-trivial code changes
- Keep scope tight; this project is intentionally Ubuntu-first for v1
- Do not open drive-by PRs that redefine product scope without prior discussion
- Read the PR template before you start; PRs are checked for a closing issue reference and a `type:*` label

## What belongs in this repo

Good fits:

- Ubuntu-first local Laravel environment work
- daemon/client boundary improvements
- RPC contract hardening
- packaging, install, rollback, and docs improvements

Out of scope for now:

- non-Ubuntu support
- feature creep unrelated to Laravel local-dev workflows
- broad platform abstraction before Ubuntu v1 is solid

## Issue process

1. Use the right issue template
2. Be specific about the problem, environment, and expected behavior
3. Maintainers triage new issues with labels such as `status:*`, `priority:*`, `area:*`, and `effort:*`
4. Wait for maintainer feedback before starting large implementation work

## Pull request process

1. Create a branch with a clear name, for example: `fix/php-version-check`
2. Keep PRs focused and small
3. Use a conventional-commit-style PR title, for example: `fix: validate php version before switch`
4. Add a closing issue reference in the PR body, for example: `Closes #123`
5. Ensure the PR has at least one `type:*` label before merge; if you cannot label PRs yourself, call out the intended type in the PR body and a maintainer will apply it during triage
6. Update docs when behavior changes
7. Add or update tests when backend behavior changes
8. Add a `CHANGELOG.md` entry for user-facing changes

### Label taxonomy

The repo uses a small, practical label namespace:

- `type:*` — kind of change (`type:bug`, `type:feature`, `type:docs`, `type:chore`, `type:security`)
- `status:*` — workflow state (`status:needs-triage`, `status:approved`, `status:in-progress`, `status:blocked`, `status:stale`)
- `priority:*` — urgency (`priority:critical`, `priority:high`, `priority:low`)
- `area:*` — subsystem (`area:daemon`, `area:client`, `area:shared`, `area:packaging`, `area:docs`, `area:ci`)
- `effort:*` — rough size (`effort:small`, `effort:medium`, `effort:large`)

Only `type:*` is required by automation on pull requests. The other namespaces are maintainer triage tools.

## Quality bar

- Backend logic changes should include or update tests when feasible
- Architecture boundaries matter more than clever shortcuts
- Docs must stay aligned with behavior
- Unrelated cleanup should not be mixed into feature work

## Review expectations

- New issues should get initial triage as time allows
- PRs should explain the why, not only the what
- Vague issues or PRs may be redirected, narrowed, or closed

## Triage cadence

- New issues: best-effort initial triage within a few days
- Pull requests: best-effort first review within about a week
- Backlog sweep: maintainers periodically close or stale inactive work unless it is approved, in progress, or critical

## Local validation

- For daemon changes, run Go tests from the `daemon/` package before requesting review
- For docs-only or repo-infra-only changes, keep validation scoped to the files you touched

## Security

Do not open public issues for suspected vulnerabilities. Follow [SECURITY.md](./SECURITY.md) instead.

## Code of collaboration

Be direct, be respectful, bring evidence, and optimize for maintainability over speed.

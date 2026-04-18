# Ubuntu support, operations, and troubleshooting

This page is the operator-facing reference for **Lara Nux on Ubuntu**.

## Support matrix

| Ubuntu release | Status | Notes |
| --- | --- | --- |
| 22.04 LTS (Jammy) | Supported beta | Included in packaging metadata and Ubuntu workflow scaffold coverage |
| 24.04 LTS (Noble) | Supported beta | Included in packaging metadata and Ubuntu workflow scaffold coverage |
| Other Ubuntu releases | Unsupported | Report only if you can reproduce on Jammy or Noble too |
| Non-Ubuntu distros | Unsupported for v1 | Debian/Fedora/etc. support is a later design concern |

## What Lara Nux manages

When installed from the Ubuntu package, Lara Nux MAY manage:

- the `lara-nuxd.service` systemd unit
- the `root:lara-nux` Unix socket at `/run/lara-nux/lara-nux.sock`
- a managed `systemd-resolved` stub for `.test` routing
- managed Caddy site configs for Lara Nux sites
- managed PHP-FPM pool / override fragments for Lara Nux runtimes
- package metadata used for `.deb` / apt-repo publication

It MUST NOT remove your Laravel project files during uninstall or rollback.

## Privileges and ownership

- The desktop client is expected to stay unprivileged.
- Privileged operations go through the daemon socket boundary.
- Socket access relies on OS permissions first: `root:lara-nux` with mode `0660`.
- The current packaged daemon unit still runs as `root` because full internal privilege separation is not finished yet.
- The package also creates a `lara-nuxd` system user/group relationship for future hardening and directory ownership consistency.

If your desktop user was added to the `lara-nux` group during install, you usually need to **log out and back in** before the socket becomes accessible from a new session.

## Install / upgrade / rollback behavior

### Install

The package maintainer scripts are expected to:

1. create the `lara-nux` group if missing
2. create the `lara-nuxd` system user if missing
3. create `/etc/lara-nux`, `/var/lib/lara-nux`, and `/run/lara-nux`
4. enable/start `lara-nuxd.service`
5. repair socket ownership back to `root:lara-nux` if the socket already exists

### Upgrade

- Upgrades SHOULD be idempotent.
- The package stops the daemon before upgrade and attempts to start it again afterward.
- Abort paths try to restore daemon availability instead of leaving the machine half-configured.

### Rollback / uninstall / purge

- Rollback and uninstall remove only **Lara Nux-managed** resolver, Caddy, PHP-FPM, socket, and package-state assets.
- User code under your Laravel project directories is preserved.
- Purge may remove Lara Nux packaging state plus the dedicated daemon account/group when they are no longer in use.

## Troubleshooting

### Unsupported Ubuntu release

If `lsb_release -cs` is not `jammy` or `noble`, stop there first. Lara Nux v1 does **not** claim support beyond the published LTS matrix.

### Cannot talk to the daemon socket

Check:

```bash
id
ls -l /run/lara-nux/lara-nux.sock
systemctl status lara-nuxd --no-pager
```

If your user is missing from the `lara-nux` group, that is an access problem, not an application bug.

### Ports 80 / 443 are already in use

Check:

```bash
sudo ss -ltnp
systemctl status caddy --no-pager
```

Lara Nux is expected to report the conflict rather than silently stealing the port.

### `.test` domains do not resolve

Check:

```bash
resolvectl status
systemctl status systemd-resolved --no-pager
```

Also inspect whether you already have custom resolver ownership under `/etc/systemd/resolved.conf.d/`. Lara Nux is supposed to refuse takeover when resolver ownership is ambiguous.

### Site activates but PHP runtime is wrong or broken

Check:

```bash
systemctl status 'php*-fpm' --no-pager
journalctl -u lara-nuxd -b --no-pager
```

Attach the exact runtime version you selected and the exact runtime version you observed.

### Package / maintainer script problems

If install, upgrade, or removal fails, include:

```bash
dpkg -l | grep lara-nux || true
journalctl -u lara-nuxd -b --no-pager
```

Say whether you installed from:

- a local `.deb`
- a future apt repository
- a source checkout / development environment

## Reproducible Ubuntu bug reports

Before opening a public bug, gather this evidence:

```bash
lsb_release -a
uname -a
id
systemctl status lara-nuxd --no-pager
journalctl -u lara-nuxd -b --no-pager
ls -l /run/lara-nux/lara-nux.sock
resolvectl status
systemctl status caddy --no-pager
```

Your issue SHOULD include:

- the exact Ubuntu release (`jammy` or `noble`)
- how Lara Nux was installed
- the exact Laravel project path shape involved (you can redact usernames)
- exact reproduction steps from a clean starting point
- expected behavior vs actual behavior
- the first failing command, service, or log line

Do **not** paste secrets, access tokens, or private project code. But DO include the real error text. A bug report without reproducible environment details is usually not actionable.

## CI / release caveat

Current repo automation validates packaging shape and signing readiness, but it is still intentionally honest about placeholders:

- package CI builds a validation `.deb` from staged placeholder assets
- release signing checks require maintainers to replace `SignWith: CHANGE_ME`
- missing GPG secrets are treated as configuration failures, not silently ignored behavior

# packaging/ubuntu

Ubuntu packaging assets for **Lara Nux** live here.

- `systemd/` for packaged system units and socket-permission alignment
- `scripts/` for install, upgrade, rollback, and uninstall hooks
- `debian/` for Debian metadata, maintainer-script wiring, and managed file manifests
- `repo/` for apt repository metadata used after `.deb` signing

## Packaging invariants

- The daemon socket contract stays `root:lara-nux` with mode `0660`.
- Install and upgrade hooks must be idempotent.
- Uninstall removes only Lara Nux managed host assets and **never** user Laravel projects.
- Debian packaging expects prebuilt artifacts to be staged under `packaging/ubuntu/stage/` before invoking `dpkg-buildpackage`.

This directory exists early so packaging ownership stays explicit instead of bleeding into daemon runtime code.

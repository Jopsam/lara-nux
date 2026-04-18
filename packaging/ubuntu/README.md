# packaging/ubuntu

Ubuntu packaging assets live here.

- `systemd/` for units and socket/service permissions
- `scripts/` for install, upgrade, rollback, and uninstall hooks
- `debian/` for `.deb` metadata and manifests

This directory exists early so packaging ownership stays explicit instead of bleeding into daemon runtime code.

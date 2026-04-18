---
name: Bug report
about: Report a reproducible Lara Nux problem on supported Ubuntu
title: "[Bug]: "
labels: ["type:bug", "status:needs-triage"]
---

## Support check

- [ ] I reproduced this on Ubuntu 22.04 LTS (Jammy) or 24.04 LTS (Noble)
- [ ] I read `docs/ubuntu.md`
- [ ] This is not a private security report (if it is, use `SECURITY.md` instead)

## Description

What is broken?

## Steps to reproduce

1.
2.
3.

## Reproduction frequency

- [ ] Every time
- [ ] Sometimes
- [ ] Only happened once

## Expected behavior

What should happen?

## Actual behavior

What happens instead?

## Environment

- Ubuntu version / codename:
- Install source: local `.deb` / apt repo / source checkout / other
- Lara Nux version or commit:
- Desktop session / shell:
- Relevant runtime/service details:

## Diagnostics

Paste the relevant output or say `not available`.

- `id`:
- `ls -l /run/lara-nux/lara-nux.sock`:
- `systemctl status lara-nuxd --no-pager`:
- `journalctl -u lara-nuxd -b --no-pager`:
- `resolvectl status`:
- `systemctl status caddy --no-pager`:
- Active / expected PHP runtime:

## Logs or screenshots

Add only the relevant evidence. Redact secrets, but keep the real error text.

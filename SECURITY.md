# Security Policy

## Supported scope

Lara Nux is currently an early-stage project focused on an Ubuntu-first local Laravel environment.

Security reports are especially relevant for:

- privilege boundaries between client and daemon
- local IPC / Unix socket exposure
- service management and host mutation behavior
- DNS, web-server, and certificate handling
- packaging, install, upgrade, rollback, and uninstall flows

## Reporting a vulnerability

Please do **not** open a public issue for a suspected security vulnerability.

Instead, report it privately through GitHub Security Advisories for this repository if available, or contact the maintainer directly through GitHub with enough detail to reproduce and assess the issue.

Include:

- affected area
- impact
- reproduction steps or proof of concept
- environment details
- any suggested mitigation if you have one

## Response expectations

This is a young OSS project, so response times are best-effort. Still, valid reports will be triaged seriously and handled with priority.

## Disclosure

Please allow time for triage and remediation before public disclosure.

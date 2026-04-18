## Exploration: lara-nux (Local Laravel Development Tool for Ubuntu/Linux)

### Current State
This is a brand-new OSS project. No product code exists yet. The problem space is well-defined: Ubuntu/Linux developers lack a zero-config, blazing-fast local development environment tailored for Laravel (analogous to Laravel Herd on macOS or Laragon on Windows). Current solutions rely heavily on Docker (Sail) which adds overhead, or messy manual `apt-get` installations which make switching PHP versions and managing local `.test` domains cumbersome.

### Affected Areas
- **New Codebase** — Entire project architecture needs to be defined from scratch.
- **System Integration (Linux/Ubuntu)** — The tool will need to interact deeply with `systemd` for service management, `systemd-resolved` or `dnsmasq` for DNS routing, and `/etc/hosts`.
- **Privilege Management** — Binding to ports 80/443 and installing system services requires `root` privileges, while the GUI and user-level apps should run unprivileged.

### Approaches

1. **Approach A: Go Daemon + Wails (Nuxt/Vue Frontend)**
   - *Description*: A background daemon written in Go (handles system-level tasks, binary management, DNS) communicating via IPC with an unprivileged Wails desktop app built using Nuxt/Vue.
   - *Pros*: Go is phenomenal for system-level CLI tools, single static binary, extremely fast, low memory footprint. Nuxt frontend leverages the Laravel community's familiarity with Vue.
   - *Cons*: Contributors need to know Go for backend work.
   - *Effort*: Medium

2. **Approach B: Rust Daemon + Tauri (Nuxt/Vue Frontend)**
   - *Description*: Similar architecture to A, but the daemon and desktop bindings are written in Rust.
   - *Pros*: Memory safe, minimal footprint, native performance, Tauri is highly popular and well-supported in the JS ecosystem.
   - *Cons*: Rust has a very steep learning curve for the typical PHP/JS developer, potentially limiting OSS contributions to the core system engine.
   - *Effort*: High

3. **Approach C: NativePHP (Electron/GTK)**
   - *Description*: Build the tool entirely in PHP using NativePHP, packaging the Laravel framework itself as a desktop app.
   - *Pros*: Maximum familiarity for the target audience (Laravel developers). Highly attractive for OSS contributions.
   - *Cons*: NativePHP on Linux is still relatively immature. Distributing standalone PHP apps with system-level bindings can be fragile.
   - *Effort*: High (due to platform immaturity and workarounds needed for system tasks)

### Recommendation
**Approach A (Go Daemon + Wails/Nuxt Frontend)** or **Approach B (Rust + Tauri + Nuxt)** is recommended. Given the user value of "OSS-grade quality and maintainable long-term", separating the privileged system daemon from the unprivileged user-facing GUI is non-negotiable for security and stability on Linux. 
We strongly recommend the **Daemon + GUI Client architecture**:
- **Daemon**: Runs as `root` (started via `pkexec` or `systemd` system service). Manages PHP binaries, Nginx/Caddy, and DNS.
- **GUI/Tray**: Runs as the current user. Communicates with the daemon via Unix sockets or local gRPC. Built with Nuxt.
- Go is generally preferred over Rust for system tooling in the web-dev space due to a flatter learning curve, but Tauri (Rust) is a fantastic modern alternative. 

### Core Jobs-to-be-Done (JTBD) for v1
1. **PHP Version Management**: Download, install, and instantly switch between pre-compiled PHP binaries (e.g., 8.1, 8.2, 8.3) without apt-get conflicts.
2. **Local Web Server**: Bundle a lightweight web server (Caddy is highly recommended over Nginx for automatic local SSL generation).
3. **Local DNS**: Intercept `*.test` requests and route them to `127.0.0.1` (via `systemd-resolved` configuration).
4. **Service Management**: Start/Stop/Restart PHP-FPM, Caddy, MySQL/MariaDB, and Redis.

### Risks
- **Privilege Escalation**: Forcing the user to enter their `sudo` password constantly will degrade UX. The tool needs a clean `pkexec` prompt during setup to install a `systemd` service or `sudoers` rule.
- **Port Conflicts**: Users frequently have Apache or Nginx already running on ports 80/443. The tool must detect this and fail gracefully or offer to disable the conflicting services.
- **Desktop Environment Fragmentation**: Linux has many DEs (GNOME, KDE Plasma, XFCE). Ensuring the AppIndicator/System Tray icon works consistently across all of them can be challenging.
- **Binary Distribution**: Distributing pre-compiled PHP binaries for Ubuntu means managing dependencies (OpenSSL, cURL, libxml) that might differ between Ubuntu 20.04, 22.04, and 24.04. Statically linking PHP or using a tool like `static-php-cli` is highly recommended.

### Ready for Proposal
**Yes**. The orchestrator should tell the user that the problem space is well understood, the platform constraints (Ubuntu/Linux) require a separated Daemon/GUI architecture for security, and we are ready to draft the formal Proposal focusing on the v1 scope and technical stack selection (e.g., Go/Wails vs Rust/Tauri).
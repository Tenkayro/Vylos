# Vylos Family Control — Developer & Architecture Guide

## 1. Architectural Overview

Vylos Family Control is structured as a modular single-repository Go + Wails 3 + Svelte + TypeScript desktop application, Windows Service (`VylosService`), and WiX MSI installer (`dist/Vylos-Setup.msi`):

```text
FamilyControl/
├── main.go                     # Entrypoint (Desktop UI, System Tray, CLI flags, VylosService)
├── app.go                      # Application controller & RPC bindings for Svelte frontend
├── package.json                # Root npm scripts (start, dev, build, build:msi, release, test)
├── wails.json                  # Wails 3 desktop configuration (Vylos Family Control)
├── scripts/
│   ├── start.mjs               # Builds & launches Vylos.exe
│   ├── build.mjs               # Builds frontend, Vylos.exe, and dist/Vylos-Setup.msi
│   ├── build-msi.mjs           # WiX Toolset (candle.exe + light.exe) MSI packager
│   ├── release.mjs             # @clack/prompts interactive release + latest.json/latest.yml + GitHub Release
│   └── test.mjs                # Runs frontend and Go test suites
├── backend/
│   ├── family/                 # Operating mode, setup wizard, Argon2id auth, recovery keys, child profiles
│   ├── devices/                # Cryptographic device registry, one-time pairing codes, heartbeat state
│   ├── monitoring/             # Transparent session & foreground app duration tracking
│   ├── screen_time/            # Daily limits, weekly windows, bedtime, break reminders, lock/pause
│   ├── applications/           # Registry software scanner, app rules, critical-process safety guard
│   ├── websites/               # Cloudflare Family DNS (1.1.1.3) & atomic Windows hosts blocklist
│   ├── protection/             # Integrity checks, firewall/DNS verification, security audit log
│   ├── networking/             # TLS 1.3 LAN server/client, Ed25519 request signing, UDP discovery
│   ├── security/               # Argon2id, Windows DPAPI + XChaCha20-Poly1305, Ed25519, ReplayGuard
│   ├── updater/                # Automatic GitHub Release updater (latest.json, SHA-256 verification, msiexec)
│   ├── database/               # Pure-Go SQLite (modernc.org/sqlite), WAL migrations, encrypted backups
│   └── windows/                # Win32 API integration (user32, kernel32, shell32, advapi32, registry)
├── service/
│   ├── agent.go                # Background enforcement, monitoring & LAN sync loop
│   ├── service_windows.go      # Native golang.org/x/sys/windows/svc (VylosService) & SDDL hardening
│   └── service_other.go        # Non-Windows fallback stubs
├── frontend/                   # Svelte 5 + TypeScript + Vite desktop interface
├── installer/                  # WiX MSI definition (Vylos.wxs) & PowerShell install/uninstall scripts
└── tests/                      # Automated Go unit & integration tests
```

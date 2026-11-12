# FamilyControl — Developer & Architecture Guide

## 1. Architectural Overview

FamilyControl is structured as a modular single-repository Go + Wails 3 + Svelte + TypeScript desktop application and Windows Service:

```text
FamilyControl/
├── main.go                     # Entrypoint (Desktop UI, System Tray, CLI flags, Windows Service)
├── app.go                      # Application controller & RPC bindings for Svelte frontend
├── go.mod / go.sum             # Go module manifest
├── wails.json                  # Wails 3 desktop configuration
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
│   ├── database/               # Pure-Go SQLite (modernc.org/sqlite), WAL migrations, encrypted backups
│   └── windows/                # Win32 API integration (user32, kernel32, shell32, advapi32, registry)
├── service/
│   ├── agent.go                # Background enforcement, monitoring & LAN sync loop
│   ├── service_windows.go      # Native golang.org/x/sys/windows/svc implementation & SDDL hardening
│   └── service_other.go        # Non-Windows fallback stubs
├── frontend/                   # Svelte 5 + TypeScript + Vite desktop interface
│   └── src/
│       ├── components/         # SetupWizard, StatCard, StatusBadge
│       ├── layouts/            # SidebarLayout
│       ├── pages/              # Overview, Devices, Activity, ScreenTime, Applications, Websites, Protection, Settings, ChildMode
│       ├── services/           # Typed RPC client (api.ts)
│       ├── stores/             # Reactive stores & domain validators (appStore.ts)
│       └── styles/             # Minimalist dark theme CSS (#0C1017 / #101620 / #141C27 / #273344)
├── assets/                     # SVG icon and Windows DPI-aware application manifest
├── installer/                  # WiX MSI definition (FamilyControl.wxs) & PowerShell install/uninstall scripts
└── tests/                      # Automated Go unit & integration tests
```

---

## 2. Security & Cryptographic Design

1. **Password Storage (`backend/security/argon2.go`):**
   - Uses `Argon2id` (`t=3`, `m=64MB`, `p=4`, 32-byte key, 16-byte random salt) encoded in standard PHC format.
   - Verified with `crypto/subtle.ConstantTimeCompare`.
2. **Local Secret Storage (`backend/security/crypto.go` & `keystore_windows.go`):**
   - Generates a 256-bit master key sealed via Windows Data Protection API (`windows.CryptProtectData` / `CryptUnprotectData` with `CRYPTPROTECT_UI_FORBIDDEN`).
   - Encrypts sensitive SQLite columns (Ed25519 private keys, TLS private keys, device tokens, and configuration backups) using `XChaCha20-Poly1305`.
3. **LAN Communication & Mutual Authentication (`backend/networking/lan_protocol.go`):**
   - Strictly enforces **TLS 1.3** (`tls.VersionTLS13`).
   - During pairing, the Child PC pins the Parent PC's SHA-256 X.509 certificate fingerprint and signs the pairing request using its Ed25519 private key.
   - Every subsequent `/api/v1/sync` request requires:
     - `X-FC-Device-Id`
     - `Authorization: Bearer <256-bit random device token>`
     - `X-FC-Timestamp` (checked within ±120 seconds)
     - `X-FC-Nonce` (checked against `ReplayGuard` nonce cache)
     - `X-FC-Signature` (Ed25519 signature over `METHOD:PATH:DEVICE_ID:TIMESTAMP:NONCE:SHA256(BODY)`)
   - Token-bucket rate limiting (`golang.org/x/time/rate`) protects all endpoints and login actions.

---

## 3. Build & Test Instructions

### Prerequisites
- **Go 1.24+** (`CGO_ENABLED=0` supported via pure-Go `modernc.org/sqlite`)
- **Node.js 20+ & npm**

### 1. Build and Test Frontend
```powershell
cd frontend
npm install
npm test
npm run build
cd ..
```

### 2. Run Backend & Integration Tests
```powershell
go test -v ./tests/...
```

### 3. Build Unified Windows Executable
```powershell
go build -ldflags="-s -w" -o FamilyControl.exe .
```
To build as a pure GUI subsystem binary without a console window on Windows:
```powershell
go build -ldflags="-s -w -H=windowsgui" -o FamilyControl.exe .
```
*(Note: When built with `-H=windowsgui`, CLI output on stdout is suppressed by Windows; keep the standard build if you wish to see CLI output for `--recover` or `--install-service` in a terminal).*

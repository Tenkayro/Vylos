# Vylos Family Control

**Complete Open-Source Local Parental Control Application for Windows 10 & Windows 11**

**Vylos Family Control** (`Vylos`) is a modern, lightweight, privacy-respecting parental control desktop application and background Windows service that operates **100% within your local home network (LAN)**.

- **Zero Cloud Infrastructure:** No external servers, no mandatory accounts, no subscriptions, and no third-party telemetry.
- **Single Unified MSI Installer (`dist/Vylos-Setup.msi`):** One installer and one executable (`Vylos.exe`) is used on both Parent and Child computers. In the Initial Setup Wizard, choose **Parent Mode** (coordination dashboard) or **Child Mode** (parent-authorized background agent).
- **Parent-Authorized Child Setup:** When setting up Child Mode on a child's computer, a parent must authorize the installation with a local Argon2id Parent Protection Password and pair it with the Parent PC.
- **Built-in Automatic Updater:** Checks `https://github.com/Tenkayro/Vylos/releases/latest/download/latest.json` (`latest.json` & `latest.yml`), verifies SHA-256 integrity, and installs signed MSI updates automatically.
- **Offline-Resilient Enforcement:** Child computers store policies locally in SQLite and continue enforcing screen time, bedtime, application rules, and DNS/website rules even when the Parent computer is powered off.

---

## 1. Quick Start & Root `npm run` Commands

Run all commands directly in the main project folder (`C:\Daten\Dev\Go\FamilyControl`):

```powershell
# Build & start Vylos Family Control immediately for testing
npm run start

# Start in development mode on port 41800
npm run dev

# Build frontend, Vylos.exe, and unified MSI installer (dist/Vylos-Setup.msi)
npm run build

# Build only the Windows MSI package (dist/Vylos-Setup.msi)
npm run build:msi

# Run all Frontend + Go Backend & TLS 1.3 LAN Integration Tests
npm test

# Interactive @clack/prompts Release Menu -> Bump Version, Build MSI + latest.json/latest.yml, Upload to GitHub Releases
npm run release
```

---

## 2. Installation via Unified MSI (`dist/Vylos-Setup.msi`)

1. Run `dist\Vylos-Setup.msi` on the Parent PC and on each Child PC.
2. The MSI installs `Vylos.exe` into `C:\Program Files\Vylos`, creates Start Menu and Desktop shortcuts for **Vylos Family Control**, and registers `VylosService` (`Vylos Family Control Service`).
3. Alternatively, run `.\installer\install.ps1` as Administrator to also configure LocalSubnet-scoped Windows Firewall rules (`TCP 41843` and `UDP 41880`).

---

## 3. Configuring Parent Mode & Child Mode

### Parent Computer
1. Launch **Vylos Family Control** on the parent computer.
2. In the **Unified Setup Wizard**, select **Parent Mode (Coordinator)**.
3. Enter your **Family Name**, your first child's name, a **Parent Contact Note**, and a **Parent Master Password** (hashed locally with **Argon2id**).
4. **Save your Emergency Recovery Key** (`FC-XXXX-XXXX-XXXX-XXXX`).

### Child Computer (Parent Setup Required)
1. On the Parent PC, open **Devices** in the sidebar and click **Generate One-Time Pairing Code**.
2. Install `Vylos-Setup.msi` on the child's computer and open **Vylos Family Control**.
3. Select **Child Mode (Agent)** in the Setup Wizard.
4. **A parent must complete this step:** Set the local **Parent Protection Password** (so the child cannot change rules or uninstall Vylos) and enter the **One-Time Pairing Code** (`XXXX-XXXX`) along with the discovered Parent PC endpoint.

---

## 4. Releasing & Auto-Updating (`npm run release`)

Running `npm run release` opens an interactive `@clack/prompts` CLI menu:
1. Select version bump (`Patch`, `Minor`, `Major`, or `Custom`).
2. Automatically updates version numbers in `package.json`, `frontend/package.json`, `wails.json`, and `installer/Vylos.wxs`.
3. Builds both Windows architectures (`dist/Vylos-windows-amd64.exe`, `dist/Vylos-windows-arm64.exe`) and packages the unified Windows installer `dist/Vylos-Setup.msi`.
4. Generates `dist/latest.json` and `dist/latest.yml` with SHA-256 / SHA-512 checksums for the integrated Vylos Auto-Updater.
5. Creates a Git tag and publishes the release assets to `https://github.com/Tenkayro/Vylos/releases`.

---

## License

Released under the [MIT License](LICENSE).

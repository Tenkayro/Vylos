# FamilyControl

**Complete Open-Source Local Parental Control Application for Windows 10 & Windows 11**

FamilyControl is a modern, lightweight, privacy-respecting parental control desktop application and background Windows service that operates **100% within your local home network (LAN)**.

- **Zero Cloud Infrastructure:** No external servers, no mandatory accounts, no subscriptions, and no third-party telemetry.
- **Unified Single-Binary Architecture:** One application supports both **Parent Mode** (coordination dashboard) and **Child Mode** (lightweight tray agent + Windows background service).
- **Offline-Resilient Enforcement:** Child computers store policies locally in SQLite and continue enforcing screen time, bedtime, application rules, and DNS/website rules even when the Parent computer is powered off.
- **Transparent Protection:** Shows clear protection status and remaining screen time to children. Never uses keylogging, covert screen capture, microphone recording, or stealth rootkit techniques.

---

## 1. Installation

### Recommended Windows Account Setup
For effective operating system security boundaries on Windows 10/11:
1. Configure a **Standard Windows User** account for each child.
2. Maintain a separate password-protected **Windows Administrator** account for parents.

### Installing via PowerShell / MSI
Run PowerShell as **Administrator** in the project directory:
```powershell
.\installer\install.ps1
```
This installs `FamilyControl.exe` into `C:\Program Files\FamilyControl`, configures Windows Firewall rules restricted strictly to your `LocalSubnet` (TCP `41843` for TLS 1.3 sync, UDP `41880` for LAN discovery), and registers `FamilyControlService` as an auto-start Windows Service with hardened SDDL permissions.

Alternatively, compile the MSI package from `installer\FamilyControl.wxs` using the WiX Toolset (`candle.exe` / `light.exe` or `wix build`).

---

## 2. Configuring Parent Mode

1. Launch **FamilyControl** on the parent computer.
2. In the **Initial Setup Wizard**, select **Parent Mode (Coordinator)**.
3. Enter your **Family Name**, your first child's name, a **Parent Contact Note** (displayed on the child's computer), and a **Parent Master Password** (hashed locally with **Argon2id**).
4. **Save your Emergency Recovery Key** (`FC-XXXX-XXXX-XXXX-XXXX`) shown on screen. Because FamilyControl has no cloud server, this recovery key is the only way to reset a forgotten Parent password.

---

## 3. Configuring Child Mode & Connecting Devices

1. On the Parent PC, open **Devices** in the sidebar, select the child profile, and click **Generate One-Time Pairing Code** (valid for 15 minutes).
2. Install and launch **FamilyControl** on the child's computer and select **Child Mode (Agent)** in the Setup Wizard.
3. Choose the discovered Parent PC from the **Discovered Parent Endpoints on LAN** list (via UDP Broadcast discovery on port `41880`), or enter the Parent PC's `IP:Port` (e.g. `192.168.1.10:41843`) manually.
4. Enter the **One-Time Pairing Code** (`XXXX-XXXX`).
5. Both computers complete a **TLS 1.3 handshake with SHA-256 certificate fingerprint pinning**, verify each other's **Ed25519** cryptographic signatures, and begin event-driven synchronization.

---

## 4. Using Parental Controls

- **Overview:** Compact summary of connected children, online/offline device status, today's screen usage, active applications, blocked domain events, and security notifications.
- **Devices:** Add children, pair/rename child computers, inspect OS and last seen time, jump to device rules, or revoke device credentials.
- **Activity:** View computer login/logout events, foreground application usage duration, daily usage summaries, and 7-day usage trends.
- **Screen Time:** Set weekday and weekend daily limits, 7-day allowed time windows, overnight bedtime schedules, break reminders, bonus minutes, manual pause, or immediate workstation lock.
- **Applications:** Scan installed software from the Windows Registry, review newly installed applications, block specific executables, or set allowed hours and daily time limits per application. System-critical Windows processes (`csrss.exe`, `svchost.exe`, `lsass.exe`, `explorer.exe`, etc.) are permanently protected from termination.
- **Websites:** Configure **Cloudflare 1.1.1.1 for Families DNS** (`1.1.1.3` / `1.0.0.3` IPv4 and `2606:4700:4700::1113` / `2606:4700:4700::1003` IPv6) for malware and adult-content filtering, plus custom domain allowlists and blocklists enforced via an atomic managed section in the Windows `hosts` file.
- **Protection:** Inspect DNS protection state, active resolvers, Windows Firewall status, background service status, TLS 1.3 certificate fingerprint, and the chronological security audit log.
- **Settings:** Manage household settings, change the Argon2id Parent password, switch operating modes, configure data retention pruning, export/import XChaCha20-Poly1305 encrypted backups, and authorize uninstallation.

---

## 5. Emergency Password Recovery

If you forget your Parent Master Password, click **Use Recovery Key** on the lock screen or run:
```powershell
.\FamilyControl.exe --recover "FC-XXXX-XXXX-XXXX-XXXX" --new-password "YourNewStrongPassword"
```
A new one-time recovery key will be generated and printed.

---

## 6. Uninstalling FamilyControl

Uninstalling or stopping the background service requires **Windows Administrator** privileges **and** the **Parent Master Password**:
```powershell
.\installer\uninstall.ps1 -ParentPassword "YourParentPassword"
```
Or directly via the binary:
```powershell
.\FamilyControl.exe --uninstall-service --password "YourParentPassword"
```

---

## 7. Known Limitations & Transparency

1. **DNS & Website Filtering Scope:** DNS filtering (Cloudflare `1.1.1.3` and hosts-file rules) blocks domain resolution across browsers and applications. Because modern web traffic uses encrypted HTTPS/TLS and browsers may enable DNS-over-HTTPS (DoH), DNS monitoring does not inspect full HTTPS URLs, page contents, or search queries.
2. **Windows Administrator Authority:** FamilyControl hardens `FamilyControlService` using Windows SDDL permissions so standard user accounts cannot stop or remove the service. However, any user with local Windows Administrator credentials can ultimately control the operating system. Always assign children a standard Windows user account.

---

## License

Released under the [MIT License](LICENSE).

<div align="center">

# Vylos Family Control

**Modern, 100% Local Open-Source Parental Control for Windows 10 & Windows 11**  
*Zero Cloud · No Subscriptions · No Online Accounts · No Covert Surveillance*

[![Website](https://img.shields.io/badge/Website-vylos.tenkayro.de-DCE9FF?style=for-the-badge)](https://vylos.tenkayro.de)
[![Download Parent Edition](https://img.shields.io/badge/Download-Parent_Edition_(.msi)-59C99A?style=for-the-badge)](https://github.com/Tenkayro/Vylos/releases/latest/download/Vylos-Parent.msi)
[![Download Child Edition](https://img.shields.io/badge/Download-Child_Edition_(.msi)-E5B96B?style=for-the-badge)](https://github.com/Tenkayro/Vylos/releases/latest/download/Vylos-Child.msi)
[![License: MIT](https://img.shields.io/badge/License-MIT-273344?style=for-the-badge)](LICENSE)

</div>

---

## What is Vylos Family Control?

**Vylos Family Control** gives parents a clean, reliable way to manage screen time, bedtime schedules, games, and website filtering on their children’s Windows computers — **entirely inside your own home network (Wi-Fi / LAN)**.

Unlike commercial parental control suites, Vylos never uploads your family’s activity to external servers, never requires a monthly subscription or online account, and never spies on your children with keyloggers or secret screenshots.

---

## Download Vylos for Windows 10 & 11

Vylos provides **two dedicated Windows installers (`.msi`)** so each computer only runs the role it needs:

| Edition | Who installs it? | Direct Download | What it does |
|---|---|---|---|
| **Vylos Parent Edition** | Install on the **Parent PC** | [**Download `Vylos-Parent.msi`**](https://github.com/Tenkayro/Vylos/releases/latest/download/Vylos-Parent.msi) | Full Parent Dashboard to manage children, pair devices, set screen-time schedules, block apps, and configure Family DNS. *(Automatically blocked from installing on protected Child PCs).* |
| **Vylos Child Edition** | Install on each **Child PC** *(set up by a parent)* | [**Download `Vylos-Child.msi`**](https://github.com/Tenkayro/Vylos/releases/latest/download/Vylos-Child.msi) | Lightweight background agent & Windows Service (`VylosService`) that enforces family rules even when offline and shows remaining screen time in the System Tray. |

> **Prefer downloading via our website?** Visit **[vylos.tenkayro.de](https://vylos.tenkayro.de)** to compare editions and preview the dashboard interactively.

---

## How to Set Up Vylos in 3 Minutes

### Step 1 — Set up the Parent Computer (`Vylos-Parent.msi`)
1. Download and install **[`Vylos-Parent.msi`](https://github.com/Tenkayro/Vylos/releases/latest/download/Vylos-Parent.msi)** on your computer.
2. Open **Vylos Family Control**, enter your **Family Name**, your child’s name, and create your **Parent Master Password**.
3. Write down your **Emergency Recovery Key** (`FC-XXXX-XXXX-XXXX-XXXX`) shown on screen — because Vylos has no cloud server, this key is your offline backup if you ever forget your password.
4. Open the **Devices** tab and click **Generate One-Time Pairing Code**.

### Step 2 — Set up the Child Computer (`Vylos-Child.msi`)
1. Download and run **[`Vylos-Child.msi`](https://github.com/Tenkayro/Vylos/releases/latest/download/Vylos-Child.msi)** on your child’s Windows PC.
2. **Complete the Parent Setup step:** Set a local **Parent Protection Password** so your child cannot alter rules or uninstall Vylos.
3. Select your automatically discovered **Parent PC** on the local network (or enter its IP address) and type in the **8-character One-Time Pairing Code** (`XXXX-XXXX`).

### Step 3 — Manage Everything from Your Dashboard
- Both computers connect directly over your home network using **TLS 1.3 with certificate pinning**.
- Every rule you configure on the Parent PC is synced to the Child PC and stored locally — **rules continue to work even when the Parent PC is turned off**.

---

## Key Features for Parents

### 1. Screen Time & Bedtime Schedules
- **Daily Usage Limits:** Set separate daily screen-time allowances for school days (Monday–Friday) and weekends (Saturday–Sunday).
- **Allowed Time Windows & Bedtime:** Define exact hours when the computer can be used each day and lock the workstation automatically at bedtime.
- **Break Reminders:** Encourage healthy habits with automatic rest break notifications.
- **Instant Bonus Time & Remote Lock:** Grant `+15m` or `+30m` of extra time with one click, pause usage, or lock the screen immediately from your Parent Dashboard.

### 2. Application & Game Control
- **Automatic Software Discovery:** View all installed applications and games on your child’s PC and spot newly installed software immediately.
- **Block or Schedule Apps:** Block specific games/programs completely or restrict them to specific afternoon hours and daily time limits.
- **Safe by Design:** Critical Windows system processes (`explorer.exe`, `svchost.exe`, `csrss.exe`, etc.) are permanently protected so your child’s operating system always stays stable.

### 3. Family DNS & Website Filtering
- **One-Click Cloudflare Family DNS (`1.1.1.3`):** Filter malware, phishing, and adult content across the entire computer using Cloudflare *1.1.1.1 for Families* (IPv4 & IPv6).
- **Custom Allowlist & Blocklist:** Block specific distracting domains (e.g., `tiktok.com`) or explicitly allow educational sites.

### 4. Transparent Protection — Never Spyware
- Children can see their **remaining screen time**, **bedtime schedule**, **active family rules**, and a **Parent Contact Note** at any time via the Vylos System Tray icon.
- **Strict Ethical Boundary:** Vylos **never** records keystrokes, **never** takes secret screenshots, and **never** accesses webcams or microphones.

### 5. Multi-Layered Tamper Protection
- **Edition Lock:** `Vylos-Child.msi` is compiled exclusively for Child Mode.
- **Parent-Installer Block:** Once `Vylos-Child.msi` is active on a child’s computer, Windows Installer, database guards, and process rules automatically block the child from installing or launching `Vylos-Parent.msi` to bypass restrictions.
- **Hardened Windows Service:** `VylosService` runs automatically in the background and requires the **Parent Password** before it can be uninstalled.
- **Automatic Updates:** Built-in updater checks GitHub Releases, verifies SHA-256 integrity, and keeps both Parent and Child installations up to date.

---

## Frequently Asked Questions (FAQ)

**Does my Parent PC need to stay turned on all day?**  
No. The Child PC stores all screen-time schedules, app rules, and DNS settings locally in an encrypted database. Even if the Parent PC is powered off, `VylosService` enforces every rule on the Child PC and syncs activity automatically the next time both computers are online.

**What Windows user account should my child use?**  
For the strongest operating system security on Windows 10/11, configure a **Standard Windows User** account for your child and keep your **Windows Administrator** account password-protected for parents.

**What if I forget my Parent Master Password?**  
Click **Use Recovery Key** on the Vylos unlock screen and enter the Emergency Recovery Key (`FC-XXXX-XXXX-XXXX-XXXX`) generated during initial setup to set a new password.

---

## License & Source Code

Vylos Family Control is free, open-source software released under the [MIT License](LICENSE).  
*(Looking for technical architecture and build instructions? See [DEVELOPER.md](DEVELOPER.md)).*

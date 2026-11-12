(function () {
  'use strict';

  const REPO = 'Tenkayro/Vylos';
  const FALLBACK_VERSION = '1.1.0';

  const editions = {
    parent: {
      id: 'parent',
      title: 'Vylos Parent Edition (Parent PC)',
      badge: 'Coordinator · Dashboard',
      filename: 'Vylos-Parent.msi',
      otherFilename: 'Vylos-Child.msi',
      otherLabel: 'Or download Child Edition (Vylos-Child.msi)',
      url: `https://github.com/${REPO}/releases/latest/download/Vylos-Parent.msi`,
      otherUrl: `https://github.com/${REPO}/releases/latest/download/Vylos-Child.msi`,
      checklist: [
        'Install on the parent computer (Windows 10 & 11)',
        'Manage child profiles, screen time, bedtime schedules & application rules',
        'Generates 8-character one-time pairing codes for Child PCs over LAN',
        'Blocked from installing or running on protected Child PCs',
      ],
    },
    child: {
      id: 'child',
      title: 'Vylos Child Edition (Child PC)',
      badge: 'Background Agent · Enforcer',
      filename: 'Vylos-Child.msi',
      otherFilename: 'Vylos-Parent.msi',
      otherLabel: 'Or download Parent Edition (Vylos-Parent.msi)',
      url: `https://github.com/${REPO}/releases/latest/download/Vylos-Child.msi`,
      otherUrl: `https://github.com/${REPO}/releases/latest/download/Vylos-Parent.msi`,
      checklist: [
        'Install on your child’s computer (must be configured by a parent)',
        'Enforces screen time, breaks, app locks & DNS filtering even when offline',
        'Hard-locked to Child Mode — uninstall requires Parent Password',
        'Transparent remaining-time indicator in System Tray (no covert spying)',
      ],
    },
  };

  let selectedEdition = 'parent';
  let currentVersion = FALLBACK_VERSION;

  function renderEditionCard() {
    const ed = editions[selectedEdition];
    const tabParent = document.getElementById('tab-parent');
    const tabChild = document.getElementById('tab-child');
    if (tabParent && tabChild) {
      tabParent.classList.toggle('active', selectedEdition === 'parent');
      tabChild.classList.toggle('active', selectedEdition === 'child');
    }

    const titleEl = document.getElementById('edition-title');
    const listEl = document.getElementById('edition-list');
    const dlBtn = document.getElementById('primary-download-btn');
    const dlBtnText = document.getElementById('primary-download-text');
    const otherLink = document.getElementById('secondary-download-link');
    const versionBadge = document.getElementById('release-version-badge');

    if (titleEl) titleEl.textContent = ed.title;
    if (listEl) {
      listEl.innerHTML = ed.checklist.map((item) => `<li>${item}</li>`).join('');
    }
    if (dlBtn) dlBtn.setAttribute('href', ed.url);
    if (dlBtnText) {
      dlBtnText.textContent = `Download ${ed.filename} (v${currentVersion})`;
    }
    if (otherLink) {
      otherLink.textContent = ed.otherLabel;
      otherLink.setAttribute('href', ed.otherUrl);
    }
    if (versionBadge) {
      versionBadge.textContent = `Release v${currentVersion} · Windows 10 & 11 (64-bit MSI)`;
    }
  }

  const previewTabs = {
    overview: `
      <div class="preview-grid-4">
        <div class="preview-stat">
          <div class="preview-stat-label">Connected Child PCs</div>
          <div class="preview-stat-val">2 Online</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">Today's Usage</div>
          <div class="preview-stat-val">1h 25m</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">Family DNS Filter</div>
          <div class="preview-stat-val" style="color:var(--success);">1.1.1.3 Active</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">LAN Security</div>
          <div class="preview-stat-val" style="color:var(--success);">TLS 1.3 Pin</div>
        </div>
      </div>
      <div class="preview-card">
        <div style="display:flex;justify-content:space-between;align-items:center;">
          <strong style="font-size:13.5px;">Paired Child Computers on Local Network (LAN)</strong>
          <span class="badge badge-success"><span class="badge-dot"></span>Event-Driven Sync</span>
        </div>
        <table class="preview-table">
          <thead>
            <tr><th>Device / Child</th><th>Status</th><th>Used / Remaining</th><th>Active App</th><th>Quick Action</th></tr>
          </thead>
          <tbody>
            <tr>
              <td><strong>Leo-Gaming-PC</strong> <span style="color:var(--text-secondary);">(Leo)</span></td>
              <td><span class="badge badge-success">Online</span></td>
              <td>1h 10m / <strong>50m left</strong></td>
              <td>Minecraft Launcher</td>
              <td><span class="badge">+15m Bonus / Lock PC</span></td>
            </tr>
            <tr>
              <td><strong>Maya-Laptop</strong> <span style="color:var(--text-secondary);">(Maya)</span></td>
              <td><span class="badge badge-success">Online</span></td>
              <td>15m / <strong>1h 45m left</strong></td>
              <td>Firefox (Wikipedia)</td>
              <td><span class="badge">+15m Bonus / Lock PC</span></td>
            </tr>
          </tbody>
        </table>
      </div>
    `,
    screentime: `
      <div class="preview-grid-4">
        <div class="preview-stat">
          <div class="preview-stat-label">Weekday Limit</div>
          <div class="preview-stat-val">2h 00m</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">Weekend Limit</div>
          <div class="preview-stat-val">3h 30m</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">Bedtime Window</div>
          <div class="preview-stat-val">20:30 – 07:00</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">Break Reminder</div>
          <div class="preview-stat-val">Every 45m</div>
        </div>
      </div>
      <div class="preview-card">
        <strong style="font-size:13.5px;">Offline-Resilient Local Enforcement</strong>
        <p style="font-size:12.5px;color:var(--text-secondary);margin-top:6px;">
          All daily limits, allowed weekday hours, and bedtime schedules are stored locally in an encrypted SQLite database on the child computer. Even if the Parent PC is powered off, <code>VylosService</code> enforces screen-time rules and locks the workstation at bedtime.
        </p>
      </div>
    `,
    apps: `
      <div class="preview-card">
        <div style="display:flex;justify-content:space-between;align-items:center;">
          <strong style="font-size:13.5px;">Application Rules &amp; Tamper Protection</strong>
          <span class="badge badge-warning">Critical OS Processes Guarded</span>
        </div>
        <table class="preview-table">
          <thead>
            <tr><th>Application</th><th>Executable</th><th>Policy</th><th>Schedule / Limit</th></tr>
          </thead>
          <tbody>
            <tr>
              <td><strong>Roblox</strong></td>
              <td class="mono">robloxplayerbeta.exe</td>
              <td><span class="badge badge-warning">Scheduled</span></td>
              <td class="mono">Max 60m · 15:00–18:30</td>
            </tr>
            <tr>
              <td><strong>Vylos Parent Setup</strong></td>
              <td class="mono">vylos-parent.exe</td>
              <td><span class="badge" style="color:var(--error);">Blocked on Child PC</span></td>
              <td>Automatic tamper protection</td>
            </tr>
            <tr>
              <td><strong>VS Code</strong></td>
              <td class="mono">code.exe</td>
              <td><span class="badge badge-success">Allowed</span></td>
              <td>No separate app limit</td>
            </tr>
          </tbody>
        </table>
      </div>
    `,
    dns: `
      <div class="preview-grid-4">
        <div class="preview-stat">
          <div class="preview-stat-label">Primary DNS (IPv4)</div>
          <div class="preview-stat-val mono">1.1.1.3</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">Secondary DNS (IPv4)</div>
          <div class="preview-stat-val mono">1.0.0.3</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">Filter Profile</div>
          <div class="preview-stat-val" style="color:var(--success);">Malware + Adult</div>
        </div>
        <div class="preview-stat">
          <div class="preview-stat-label">Custom Blocklist</div>
          <div class="preview-stat-val">Atomic Hosts</div>
        </div>
      </div>
      <div class="preview-card">
        <strong style="font-size:13.5px;">Cloudflare 1.1.1.1 for Families + Custom Domain Filtering</strong>
        <p style="font-size:12.5px;color:var(--text-secondary);margin-top:6px;">
          Blocks adult content and malware/phishing domains at the resolver level (IPv4 &amp; IPv6) and enforces custom domain blocklists (e.g. <code>tiktok.com</code>) without invasive browser proxies.
        </p>
      </div>
    `,
  };

  function renderPreview(tabKey) {
    const container = document.getElementById('preview-stage');
    if (container && previewTabs[tabKey]) {
      container.innerHTML = previewTabs[tabKey];
    }
    document.querySelectorAll('[data-preview-tab]').forEach((btn) => {
      btn.classList.toggle('active', btn.getAttribute('data-preview-tab') === tabKey);
    });
  }

  async function fetchLatestReleaseVersion() {
    try {
      const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`, {
        headers: { Accept: 'application/vnd.github+json' },
      });
      if (!res.ok) return;
      const data = await res.json();
      if (data && data.tag_name) {
        currentVersion = String(data.tag_name).replace(/^v/i, '');
        renderEditionCard();
      }
    } catch {
      // Fallback to embedded version
    }
  }

  document.addEventListener('DOMContentLoaded', () => {
    const tabParent = document.getElementById('tab-parent');
    const tabChild = document.getElementById('tab-child');
    if (tabParent) {
      tabParent.addEventListener('click', () => {
        selectedEdition = 'parent';
        renderEditionCard();
      });
    }
    if (tabChild) {
      tabChild.addEventListener('click', () => {
        selectedEdition = 'child';
        renderEditionCard();
      });
    }

    document.querySelectorAll('[data-preview-tab]').forEach((btn) => {
      btn.addEventListener('click', () => {
        renderPreview(btn.getAttribute('data-preview-tab'));
      });
    });

    renderEditionCard();
    renderPreview('overview');
    fetchLatestReleaseVersion();
  });
})();

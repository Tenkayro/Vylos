// Vylos Family Control Compiled Desktop UI Bundle (Svelte + TypeScript Runtime)
(function () {
  'use strict';

  const state = {
    activeSection: 'overview',
    selectedDeviceId: '',
    bootstrap: null,
    loading: true,
    parentLocked: false,
    toast: null,
    wizardStep: 'role',
    wizardBusy: false,
    wizardRecoveryCode: '',
    wizardSessionToken: '',
    activityBundle: null,
    activityDate: new Date().toISOString().slice(0, 10),
    screenTimeBundle: null,
    appsBundle: null,
    appSearch: '',
    websitesBundle: null,
    protectionStatus: null,
    activeTicket: null,
    backupText: '',
    uninstallAuthorized: false,
    updateInfo: null,
    checkingUpdate: false,
    installingUpdate: false,
    childPairingOpen: false,
    childUnlockOpen: false,
    recoveryModeOpen: false,
    recoveredNewKey: '',
  };

  let toastTimer = null;

  function showToast(text, type = 'info') {
    state.toast = { text, type };
    render();
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      state.toast = null;
      render();
    }, 4000);
  }

  function formatMinutes(totalMinutes) {
    if (!totalMinutes || totalMinutes <= 0) return '0m';
    const h = Math.floor(totalMinutes / 60);
    const m = totalMinutes % 60;
    if (h === 0) return m + 'm';
    if (m === 0) return h + 'h';
    return h + 'h ' + m + 'm';
  }

  function formatTimestamp(iso) {
    if (!iso) return 'Never';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) + ' · ' + d.toLocaleDateString();
  }

  function esc(str) {
    return String(str ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  async function callRPC(method, args = {}) {
    const res = await fetch('/rpc/' + method, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(args),
    });
    const data = await res.json();
    if (!res.ok || data.error) {
      throw new Error(data.error || ('RPC ' + method + ' failed'));
    }
    return data.result;
  }

  async function refreshBootstrap() {
    try {
      const res = await callRPC('GetBootstrapState');
      state.bootstrap = res;
      if (!state.selectedDeviceId && res.config && res.config.localDeviceId) {
        if (res.config.mode === 'parent' && res.devices && res.devices.length > 0) {
          state.selectedDeviceId = res.devices[0].id;
        } else {
          state.selectedDeviceId = res.config.localDeviceId;
        }
      }
      await loadSectionData(state.activeSection);
    } catch (e) {
      showToast(e.message || 'Failed to load state', 'error');
    } finally {
      state.loading = false;
      render();
    }
  }

  async function loadSectionData(section) {
    if (!state.bootstrap || !state.bootstrap.config.setupCompleted || state.bootstrap.config.mode !== 'parent') {
      return;
    }
    const devId = state.selectedDeviceId || state.bootstrap.config.localDeviceId;
    try {
      if (section === 'activity') {
        state.activityBundle = await callRPC('GetActivityBundle', { deviceId: devId, dateStr: state.activityDate });
      } else if (section === 'screen_time') {
        state.screenTimeBundle = await callRPC('GetScreenTimeBundle', { deviceId: devId });
      } else if (section === 'applications') {
        state.appsBundle = await callRPC('GetApplicationsBundle', { deviceId: devId });
      } else if (section === 'websites') {
        state.websitesBundle = await callRPC('GetWebsitesBundle', { deviceId: devId });
      } else if (section === 'protection') {
        state.protectionStatus = await callRPC('GetProtectionStatus', { deviceId: devId });
      }
    } catch (e) {
      showToast(e.message, 'error');
    }
    render();
  }

  function badgeHTML(variant, label) {
    return '<span class="badge badge-' + esc(variant) + '"><span class="badge-dot"></span><span>' + esc(label) + '</span></span>';
  }

  function statCardHTML(label, value, meta = '', tone = 'default') {
    const colorStyle =
      tone === 'success' ? 'color:var(--success)' :
      tone === 'warning' ? 'color:var(--warning)' :
      tone === 'error' ? 'color:var(--error)' : '';
    return (
      '<div class="stat-card">' +
        '<div class="stat-label">' + esc(label) + '</div>' +
        '<div class="stat-value" style="' + colorStyle + '">' + esc(value) + '</div>' +
        (meta ? '<div class="stat-meta">' + esc(meta) + '</div>' : '') +
      '</div>'
    );
  }

  function renderSetupWizard() {
    const cfg = state.bootstrap ? state.bootstrap.config : { localDeviceName: 'Windows-PC', buildEdition: 'unified' };
    const edition = cfg.buildEdition || 'unified';
    if (state.wizardStep === 'role') {
      if (edition === 'parent') state.wizardStep = 'parent_form';
      else if (edition === 'child') state.wizardStep = 'child_form';
    }
    const peers = (state.bootstrap && state.bootstrap.discoveredPeers) || [];
    const parentPeers = peers.filter(p => p.mode === 'parent');

    let inner = '';
    if (state.wizardStep === 'role') {
      inner = `
        <div style="margin-bottom:14px; color:var(--text-secondary); font-size:12.5px;">
          Choose how this Windows computer will operate in your home network:
        </div>
        <div class="grid-2" style="margin-bottom:16px;">
          <button class="card" data-action="wizard-step" data-step="parent_form" style="text-align:left; cursor:pointer; background:var(--bg-elevated);">
            <div class="row-between" style="margin-bottom:6px;">
              <span style="font-weight:600; color:var(--accent); font-size:13.5px;">Parent Mode</span>
              ${badgeHTML('success', 'Coordinator')}
            </div>
            <p style="color:var(--text-secondary); font-size:12px;">
              Install on the parent computer. Manage children, devices, screen time, applications, DNS protection, and security over LAN.
            </p>
          </button>
          <button class="card" data-action="wizard-step" data-step="child_form" style="text-align:left; cursor:pointer; background:var(--bg-elevated);">
            <div class="row-between" style="margin-bottom:6px;">
              <span style="font-weight:600; color:var(--text-primary); font-size:13.5px;">Child Mode</span>
              ${badgeHTML('neutral', 'Agent')}
            </div>
            <p style="color:var(--text-secondary); font-size:12px;">
              Install on your child's computer. Runs a lightweight background agent that enforces stored rules even when offline.
            </p>
          </button>
        </div>
        <div class="notice-box">
          <span><strong>Privacy & Architecture Guarantee:</strong> No external servers, no accounts, no telemetry, and no covert surveillance.</span>
        </div>
      `;
    } else if (state.wizardStep === 'parent_form') {
      inner = `
        <form id="parent-setup-form" style="display:flex; flex-direction:column; gap:11px;">
          <div class="field-group">
            <label class="field-label">Family / Household Name</label>
            <input class="input" name="familyName" value="Our Family" required />
          </div>
          <div class="grid-2">
            <div class="field-group">
              <label class="field-label">First Child Profile Name</label>
              <input class="input" name="firstChildName" value="Alex" />
            </div>
            <div class="field-group">
              <label class="field-label">Parent Contact Note (shown on Child PC)</label>
              <input class="input" name="parentContact" value="Ask Mom or Dad in the living room" />
            </div>
          </div>
          <div class="grid-2">
            <div class="field-group">
              <label class="field-label">Parent Master Password (Argon2id)</label>
              <input type="password" class="input" name="password" placeholder="Min. 8 characters" required />
            </div>
            <div class="field-group">
              <label class="field-label">Confirm Password</label>
              <input type="password" class="input" name="passwordConfirm" placeholder="Repeat password" required />
            </div>
          </div>
          <div class="row-between" style="margin-top:8px;">
            ${edition === 'unified' ? '<button type="button" class="btn" data-action="wizard-step" data-step="role">Back</button>' : '<span style="font-size:11.5px;color:var(--text-secondary);">Vylos Parent Edition (Vylos-Parent.msi)</span>'}
            <button type="submit" class="btn btn-primary">Initialize Parent Dashboard</button>
          </div>
        </form>
      `;
    } else if (state.wizardStep === 'parent_recovery') {
      inner = `
        <div style="display:flex; flex-direction:column; gap:12px;">
          ${badgeHTML('warning', 'Save Your Emergency Recovery Key')}
          <p style="color:var(--text-secondary); font-size:12.5px;">
            Because Vylos operates strictly offline without cloud accounts, this one-time recovery key is the only way to reset a forgotten Parent password. Write it down and store it safely:
          </p>
          <div class="card mono" style="text-align:center; font-size:15px; font-weight:600; color:var(--accent); letter-spacing:0.06em; background:var(--bg-input);">
            ${esc(state.wizardRecoveryCode)}
          </div>
          <div class="row-between" style="margin-top:6px;">
            <span style="font-size:11.5px; color:var(--text-secondary);">Hashed locally with Argon2id</span>
            <button class="btn btn-primary" data-action="finish-wizard">I Have Saved This Key — Open Dashboard</button>
          </div>
        </div>
      `;
    } else if (state.wizardStep === 'child_form') {
      inner = `
        <form id="child-setup-form" style="display:flex; flex-direction:column; gap:11px;">
          <div class="notice-box" style="margin-bottom:2px;">
            <div>
              <strong>Parent Configuration Required:</strong>
              A parent must set up Child Mode on this computer. Set a local Parent Protection Password (so your child cannot disable rules or uninstall Vylos) and pair with the Parent PC.
            </div>
          </div>
          <div class="field-group">
            <label class="field-label">This Child Computer's Name</label>
            <input class="input" name="deviceName" value="${esc(cfg.localDeviceName || 'Child-PC')}" required />
          </div>
          <div class="grid-2">
            <div class="field-group">
              <label class="field-label">Parent Protection Password (Argon2id)</label>
              <input type="password" class="input" name="parentPassword" placeholder="Min. 8 characters (Parent only)" required />
            </div>
            <div class="field-group">
              <label class="field-label">Confirm Parent Password</label>
              <input type="password" class="input" name="parentPasswordConfirm" placeholder="Repeat parent password" required />
            </div>
          </div>
          <div class="card" style="background:var(--bg-input);">
            <div class="card-title" style="margin-bottom:6px;">Pair with Parent PC over LAN (Recommended)</div>
            ${parentPeers.length > 0 ? '<div style="margin-bottom:8px; font-size:11.5px; color:var(--text-secondary);">Discovered Parent PC: ' + parentPeers.map(p => esc(p.deviceName + ' (' + p.ipAddress + ':' + p.tlsPort + ')')).join(', ') + '</div>' : ''}
            <div class="grid-2">
              <div class="field-group">
                <label class="field-label">Parent PC IP:Port</label>
                <input class="input mono" name="parentEndpoint" placeholder="e.g. 192.168.1.10:41843" value="${parentPeers[0] ? esc(parentPeers[0].ipAddress + ':' + parentPeers[0].tlsPort) : ''}" />
              </div>
              <div class="field-group">
                <label class="field-label">One-Time Pairing Code</label>
                <input class="input mono" name="pairingCode" placeholder="XXXX-XXXX" />
              </div>
            </div>
          </div>
          <div class="row-between" style="margin-top:6px;">
            ${edition === 'unified' ? '<button type="button" class="btn" data-action="wizard-step" data-step="role">Back</button>' : '<span style="font-size:11.5px;color:var(--text-secondary);">Vylos Child Edition (Vylos-Child.msi) · Locked to Child Mode</span>'}
            <button type="submit" class="btn btn-primary">Authorize & Activate Child Mode</button>
          </div>
        </form>
      `;
    }

    return `
      <div style="display:flex; align-items:center; justify-content:center; height:100vh; padding:20px; background:var(--bg-primary);">
        <div class="card" style="width:100%; max-width:560px; padding:22px 24px;">
          <div style="display:flex; align-items:center; gap:10px; margin-bottom:16px; padding-bottom:12px; border-bottom:1px solid var(--border-subtle);">
            <div class="brand-mark">V</div>
            <div>
              <div style="font-size:15px; font-weight:600;">Vylos Family Control — Unified Setup</div>
              <div style="font-size:11.5px; color:var(--text-secondary);">Single Unified MSI Application · Choose Parent or Child Role · Local LAN TLS 1.3</div>
            </div>
          </div>
          ${inner}
        </div>
      </div>
    `;
  }

  function renderOverviewSection() {
    const s = state.bootstrap;
    const ov = s.overview;
    const devs = s.devices || [];

    return `
      <div class="grid-4">
        ${statCardHTML('Connected Children', ov.connectedChildrenCount, ov.onlineDevicesCount + ' of ' + ov.totalDevicesCount + ' paired PCs online')}
        ${statCardHTML("Today's Computer Usage", formatMinutes(ov.todayTotalMinutes), ov.blockedAttemptsToday + ' restricted launches today')}
        ${statCardHTML('Website & DNS Filter', 'Cloudflare 1.1.1.3', ov.blockedDomainsCount + ' blocked domain events', 'success')}
        ${statCardHTML('Security & Integrity', ov.securityHealthy ? 'Protected' : 'Attention', 'TLS 1.3 + Certificate Pinning', ov.securityHealthy ? 'success' : 'warning')}
      </div>

      <div class="grid-2">
        <div class="card">
          <div class="card-header">
            <div>
              <div class="card-title">Paired Child Devices & Screen Time</div>
              <div class="card-subtitle">Direct LAN coordination — rules enforced locally when offline</div>
            </div>
            <button class="btn btn-sm" data-action="nav" data-section="devices">Manage Devices</button>
          </div>
          ${devs.length === 0 ? '<div class="empty-state">No child computers paired yet. Open <strong>Devices</strong> to generate a one-time LAN pairing code.</div>' : `
            <table class="data-table">
              <thead><tr><th>Device / Child</th><th>Status</th><th>Used / Remaining</th><th>Active App</th><th>Quick Action</th></tr></thead>
              <tbody>
                ${devs.map(d => `
                  <tr>
                    <td><div style="font-weight:600;">${esc(d.name)}</div><div style="font-size:11px;color:var(--text-secondary);">${esc(d.childName || 'Unassigned')} · ${esc(d.ipAddress || 'LAN')}</div></td>
                    <td>${badgeHTML(d.status === 'online' ? 'success' : 'neutral', d.status === 'online' ? 'Online' : 'Offline')}</td>
                    <td>${formatMinutes(d.usedTodayMinutes)} <span style="color:var(--text-secondary);">/ ${formatMinutes(d.remainingMinutes)} left</span></td>
                    <td style="color:var(--text-secondary);">${esc(d.activeApp || 'Idle')}</td>
                    <td><button class="btn btn-sm ${d.screenLocked ? 'btn-danger' : ''}" data-action="quick-lock" data-device="${esc(d.id)}" data-lock="${!d.screenLocked}">${d.screenLocked ? 'Unlock PC' : 'Lock Screen'}</button></td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>

        <div class="card">
          <div class="card-header">
            <div>
              <div class="card-title">Recently Active Applications</div>
              <div class="card-subtitle">Today's foreground usage duration across monitored devices</div>
            </div>
            <button class="btn btn-sm" data-action="nav" data-section="applications">App Rules</button>
          </div>
          ${!ov.recentApps || ov.recentApps.length === 0 ? '<div class="empty-state">No application usage recorded yet today.</div>' : `
            <table class="data-table">
              <thead><tr><th>Application</th><th>Category</th><th>Duration</th></tr></thead>
              <tbody>
                ${ov.recentApps.slice(0, 6).map(a => `
                  <tr>
                    <td><div style="font-weight:500;">${esc(a.displayName)}</div><div class="mono" style="font-size:10.5px;color:var(--text-secondary);">${esc(a.executableName)}</div></td>
                    <td><span class="badge badge-neutral">${esc(a.category)}</span></td>
                    <td style="font-weight:600;">${formatMinutes(a.totalMinutes)}</td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>
      </div>

      <div class="grid-2">
        <div class="card">
          <div class="card-header">
            <div>
              <div class="card-title">Blocked Websites & Domain Filter History</div>
              <div class="card-subtitle">Cloudflare Family DNS + Local Domain Blocklist</div>
            </div>
            <button class="btn btn-sm" data-action="nav" data-section="websites">Configure DNS</button>
          </div>
          ${!ov.recentBlockedDomains || ov.recentBlockedDomains.length === 0 ? '<div class="empty-state">No blocked domain attempts recorded recently.</div>' : `
            <table class="data-table">
              <thead><tr><th>Domain</th><th>Category</th><th>Time</th></tr></thead>
              <tbody>
                ${ov.recentBlockedDomains.slice(0, 5).map(bd => `
                  <tr>
                    <td class="mono">${esc(bd.domain)}</td>
                    <td>${badgeHTML('warning', bd.category)}</td>
                    <td style="color:var(--text-secondary);font-size:11.5px;">${formatTimestamp(bd.occurredAt)}</td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>

        <div class="card">
          <div class="card-header">
            <div>
              <div class="card-title">Important Security & System Notifications</div>
              <div class="card-subtitle">Audit log of pairing, policy changes, and tamper checks</div>
            </div>
            <button class="btn btn-sm" data-action="nav" data-section="protection">Protection View</button>
          </div>
          ${!ov.notifications || ov.notifications.length === 0 ? '<div class="empty-state">All security checks nominal.</div>' : `
            <table class="data-table">
              <thead><tr><th>Severity</th><th>Event</th><th>Time</th></tr></thead>
              <tbody>
                ${ov.notifications.slice(0, 5).map(n => `
                  <tr>
                    <td>${badgeHTML(n.severity === 'critical' ? 'error' : n.severity === 'warning' ? 'warning' : 'success', n.severity)}</td>
                    <td><div>${esc(n.message)}</div>${n.details ? '<div style="font-size:11px;color:var(--text-secondary);">' + esc(n.details) + '</div>' : ''}</td>
                    <td style="color:var(--text-secondary);font-size:11.5px;">${formatTimestamp(n.occurredAt)}</td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>
      </div>
    `;
  }

  function renderDevicesSection() {
    const s = state.bootstrap;
    const children = s.children || [];
    const devs = s.devices || [];

    return `
      <div class="grid-2">
        <div class="card">
          <div class="card-header">
            <div>
              <div class="card-title">Pair a New Child Computer (Cryptographic LAN Pairing)</div>
              <div class="card-subtitle">Generates a one-time pairing code with TLS 1.3 certificate pinning</div>
            </div>
          </div>
          <form id="generate-pair-form">
            <div class="grid-2" style="margin-bottom:10px;">
              <div class="field-group">
                <label class="field-label">Assign to Child</label>
                <select class="select" name="childId">
                  <option value="">Select child profile...</option>
                  ${children.map(c => '<option value="' + esc(c.id) + '">' + esc(c.name) + ' (' + esc(c.ageGroup) + ')</option>').join('')}
                </select>
              </div>
              <div class="field-group">
                <label class="field-label">Device Name Hint</label>
                <input class="input" name="deviceNameHint" placeholder="e.g. Bedroom Laptop" />
              </div>
            </div>
            <div class="row-between">
              <span style="font-size:11.5px; color:var(--text-secondary);">Parent Endpoint: <strong class="mono">${esc((s.localLanIps && s.localLanIps[0]) || '127.0.0.1')}:${s.config.listenPort}</strong></span>
              <button type="submit" class="btn btn-primary">Generate One-Time Pairing Code</button>
            </div>
          </form>
          ${state.activeTicket ? `
            <div class="card" style="margin-top:12px; background:var(--bg-input); border-color:var(--accent-border);">
              <div class="row-between" style="margin-bottom:6px;">
                <span style="font-size:11.5px; color:var(--text-secondary);">Enter this code on the Child PC within 15 minutes:</span>
                ${badgeHTML('success', 'One-Time Code')}
              </div>
              <div class="mono" style="font-size:20px; font-weight:700; color:var(--accent); letter-spacing:0.08em; text-align:center; padding:6px 0;">${esc(state.activeTicket.code)}</div>
              <div class="mono" style="font-size:11px; color:var(--text-secondary); text-align:center;">Parent Endpoint: ${esc(state.activeTicket.parentEndpoint)}</div>
            </div>
          ` : ''}
        </div>

        <div class="card">
          <div class="card-header">
            <div>
              <div class="card-title">Children Profiles</div>
              <div class="card-subtitle">Manage children and assign multiple devices per child</div>
            </div>
          </div>
          <form id="add-child-form" class="row-inline" style="margin-bottom:12px;">
            <input class="input" name="name" placeholder="Child name (e.g. Maya)" required />
            <select class="select" name="ageGroup" style="width:130px;">
              <option value="6-9">Age 6–9</option>
              <option value="10-13" selected>Age 10–13</option>
              <option value="14-17">Age 14–17</option>
            </select>
            <button type="submit" class="btn btn-primary">Add Child</button>
          </form>
          ${children.length === 0 ? '<div class="empty-state">No child profiles added yet.</div>' : `
            <table class="data-table">
              <thead><tr><th>Child Name</th><th>Age Group</th><th>Paired PCs</th><th></th></tr></thead>
              <tbody>
                ${children.map(c => `
                  <tr>
                    <td style="font-weight:600;">${esc(c.name)}</td>
                    <td><span class="badge badge-neutral">${esc(c.ageGroup)} yrs</span></td>
                    <td>${c.deviceCount} device(s)</td>
                    <td style="text-align:right;"><button class="btn btn-sm btn-danger" data-action="remove-child" data-id="${esc(c.id)}">Remove</button></td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <div>
            <div class="card-title">Paired Child Computers (${devs.length})</div>
            <div class="card-subtitle">Mutual cryptographic authentication · Never relies on IP address alone</div>
          </div>
          <button class="btn btn-sm" data-action="scan-lan">Scan LAN (UDP Discovery)</button>
        </div>
        ${devs.length === 0 ? '<div class="empty-state">No child devices paired yet.</div>' : `
          <table class="data-table">
            <thead><tr><th>Device Name</th><th>Child</th><th>OS</th><th>LAN IP & Cert Pin</th><th>Status</th><th>Last Seen</th><th>Actions</th></tr></thead>
            <tbody>
              ${devs.map(d => `
                <tr>
                  <td><div style="font-weight:600;">${esc(d.name)}</div><div class="mono" style="font-size:10.5px;color:var(--text-secondary);">${esc(d.hostname)}</div></td>
                  <td>${esc(d.childName || 'Unassigned')}</td>
                  <td>${esc(d.osVersion)}</td>
                  <td><div class="mono">${esc(d.ipAddress || 'LAN')}</div><div class="mono" style="font-size:10px;color:var(--text-secondary);">SHA256: ${esc((d.certFingerprint || '').slice(0, 14))}…</div></td>
                  <td>${badgeHTML(d.status === 'online' ? 'success' : 'neutral', d.status === 'online' ? 'Online' : 'Offline')}</td>
                  <td style="font-size:11.5px;color:var(--text-secondary);">${formatTimestamp(d.lastSeen)}</td>
                  <td>
                    <div class="row-inline">
                      <button class="btn btn-sm" data-action="select-device-rules" data-id="${esc(d.id)}">Configure Rules</button>
                      <button class="btn btn-sm btn-danger" data-action="revoke-device" data-id="${esc(d.id)}">Revoke</button>
                    </div>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        `}
      </div>
    `;
  }

  function renderActivitySection() {
    const ab = state.activityBundle;
    const daily = ab ? ab.dailySummary : { dateStr: state.activityDate, totalMinutes: 0, totalSeconds: 0, loginCount: 0, blockedAttempts: 0, topApps: [] };
    const hist = (ab && ab.history7Days) || [];
    const events = (ab && ab.recentEvents) || [];

    return `
      <div class="notice-box">
        <div><strong>Transparent Activity Monitoring:</strong> FamilyControl logs computer login/logout sessions and active application duration. It never performs keylogging, covert screen capture, microphone recording, or hidden surveillance.</div>
      </div>
      <div class="grid-3">
        ${statCardHTML('Screen Usage (' + daily.dateStr + ')', formatMinutes(daily.totalMinutes), daily.totalSeconds + ' seconds active')}
        ${statCardHTML('Session Logins', daily.loginCount, 'Recorded Windows session starts')}
        ${statCardHTML('Blocked Policy Attempts', daily.blockedAttempts, 'Restricted app launches prevented', daily.blockedAttempts > 0 ? 'warning' : 'default')}
      </div>
      <div class="grid-2">
        <div class="card">
          <div class="card-header"><div class="card-title">7-Day Daily Usage Summary</div></div>
          <div style="display:flex; flex-direction:column; gap:8px;">
            ${hist.map(day => {
              const pct = Math.min(100, Math.round((day.totalMinutes / 240) * 100));
              return `
                <div>
                  <div class="row-between" style="font-size:11.5px; margin-bottom:3px;">
                    <span class="mono">${esc(day.dateStr)}</span>
                    <span style="font-weight:600;">${formatMinutes(day.totalMinutes)}</span>
                  </div>
                  <div class="progress-track"><div class="progress-fill" style="width:${pct}%;"></div></div>
                </div>
              `;
            }).join('')}
          </div>
        </div>
        <div class="card">
          <div class="card-header"><div class="card-title">Application Usage Duration</div></div>
          ${!daily.topApps || daily.topApps.length === 0 ? '<div class="empty-state">No application activity recorded for this date.</div>' : `
            <table class="data-table">
              <thead><tr><th>Application</th><th>Category</th><th>Duration</th></tr></thead>
              <tbody>
                ${daily.topApps.map(a => `
                  <tr>
                    <td><div style="font-weight:600;">${esc(a.displayName)}</div><div class="mono" style="font-size:10.5px;color:var(--text-secondary);">${esc(a.executableName)}</div></td>
                    <td><span class="badge badge-neutral">${esc(a.category)}</span></td>
                    <td style="font-weight:600;">${formatMinutes(a.totalMinutes)}</td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>
      </div>
      <div class="card">
        <div class="card-header"><div class="card-title">Session & Activity Event Log</div></div>
        ${events.length === 0 ? '<div class="empty-state">No activity events logged yet.</div>' : `
          <table class="data-table">
            <thead><tr><th>Event Type</th><th>Application / Session</th><th>Details</th><th>Duration</th><th>Timestamp</th></tr></thead>
            <tbody>
              ${events.map(ev => `
                <tr>
                  <td>${badgeHTML(ev.eventType === 'app_blocked' ? 'warning' : ev.eventType === 'login' ? 'success' : 'neutral', ev.eventType)}</td>
                  <td><span style="font-weight:500;">${esc(ev.appName)}</span> <span class="mono" style="font-size:11px;color:var(--text-secondary);">(${esc(ev.executableName)})</span></td>
                  <td style="color:var(--text-secondary);">${esc(ev.windowTitle || '—')}</td>
                  <td>${ev.durationSeconds > 0 ? Math.ceil(ev.durationSeconds / 60) + 'm' : '—'}</td>
                  <td style="color:var(--text-secondary);font-size:11.5px;">${formatTimestamp(ev.occurredAt)}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        `}
      </div>
    `;
  }

  function renderScreenTimeSection() {
    const stb = state.screenTimeBundle;
    if (!stb) return '<div class="empty-state">Loading screen time rules...</div>';
    const pol = stb.policy;
    const ev = stb.evaluation;
    const weekdays = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

    return `
      <div class="grid-4">
        ${statCardHTML('Remaining Today', formatMinutes(ev.remainingMinutes), formatMinutes(ev.usedTodayMinutes) + ' used of ' + formatMinutes(ev.effectiveLimitMinutes), ev.remainingMinutes > 15 ? 'success' : 'warning')}
        ${statCardHTML('Enforcement Status', ev.allowedNow ? 'Permitted' : 'Restricted', ev.blockReason || 'Within allowed schedule', ev.allowedNow ? 'success' : 'error')}
        ${statCardHTML('Bedtime Schedule', pol.bedtimeStart + ' – ' + pol.bedtimeEnd, ev.inBedtime ? 'Currently in Bedtime' : 'Overnight lock window')}
        ${statCardHTML('Break Reminders', 'Every ' + pol.breakIntervalMinutes + 'm', pol.breakDurationMinutes + 'm rest break reminder')}
      </div>

      <div class="card">
        <div class="row-between">
          <div>
            <div class="card-title">Immediate Parent Controls (Local Enforcement)</div>
            <div class="card-subtitle">Pause usage, lock the child workstation immediately, or grant bonus time for today</div>
          </div>
          <div class="row-inline">
            <button class="btn btn-sm" data-action="bonus-time" data-mins="15">+15m Bonus</button>
            <button class="btn btn-sm" data-action="bonus-time" data-mins="30">+30m Bonus</button>
            <button class="btn btn-sm ${pol.manualPause ? 'btn-primary' : ''}" data-action="toggle-pause">${pol.manualPause ? 'Resume Usage' : 'Pause Screen Time'}</button>
            <button class="btn btn-sm ${pol.screenLocked ? 'btn-danger' : ''}" data-action="toggle-lock">${pol.screenLocked ? 'Unlock Workstation' : 'Lock Screen Now'}</button>
          </div>
        </div>
      </div>

      <form id="screentime-form" class="grid-2">
        <div class="card">
          <div class="card-header"><div class="card-title">Daily Usage Limits & Bedtime</div></div>
          <div class="grid-2" style="margin-bottom:12px;">
            <div class="field-group">
              <label class="field-label">Weekday Limit (minutes/day)</label>
              <input type="number" class="input" name="dailyLimitMinutes" value="${pol.dailyLimitMinutes}" min="15" max="720" />
            </div>
            <div class="field-group">
              <label class="field-label">Weekend Limit (minutes/day)</label>
              <input type="number" class="input" name="weekendLimitMinutes" value="${pol.weekendLimitMinutes}" min="15" max="720" />
            </div>
          </div>
          <div class="grid-2" style="margin-bottom:12px;">
            <div class="field-group">
              <label class="field-label">Bedtime Starts</label>
              <input type="time" class="input" name="bedtimeStart" value="${esc(pol.bedtimeStart)}" />
            </div>
            <div class="field-group">
              <label class="field-label">Bedtime Ends</label>
              <input type="time" class="input" name="bedtimeEnd" value="${esc(pol.bedtimeEnd)}" />
            </div>
          </div>
          <div class="grid-2" style="margin-bottom:14px;">
            <div class="field-group">
              <label class="field-label">Break Reminder Interval (min)</label>
              <input type="number" class="input" name="breakIntervalMinutes" value="${pol.breakIntervalMinutes}" min="10" max="180" />
            </div>
            <div class="field-group">
              <label class="field-label">Break Duration (min)</label>
              <input type="number" class="input" name="breakDurationMinutes" value="${pol.breakDurationMinutes}" min="2" max="60" />
            </div>
          </div>
          <div class="row-between">
            ${badgeHTML('neutral', 'Bonus Today: +' + pol.bonusMinutesToday + 'm')}
            <button type="submit" class="btn btn-primary">Save Screen Time Rules</button>
          </div>
        </div>

        <div class="card">
          <div class="card-header"><div class="card-title">Weekly Allowed Time Windows</div></div>
          <table class="data-table">
            <thead><tr><th>Day</th><th>From</th><th>Until</th></tr></thead>
            <tbody>
              ${weekdays.map(day => {
                const w = (pol.weeklySchedule && pol.weeklySchedule[day]) || { allowedStart: '07:30', allowedEnd: '20:30', enabled: true };
                return `
                  <tr>
                    <td style="font-weight:500;">${day}</td>
                    <td><input type="time" class="input" name="start_${day}" value="${esc(w.allowedStart)}" style="width:110px;padding:3px 6px;" /></td>
                    <td><input type="time" class="input" name="end_${day}" value="${esc(w.allowedEnd)}" style="width:110px;padding:3px 6px;" /></td>
                  </tr>
                `;
              }).join('')}
            </tbody>
          </table>
        </div>
      </form>
    `;
  }

  function renderApplicationsSection() {
    const ab = state.appsBundle || { installedApps: [], rules: [], recentUsage: [] };
    const rules = ab.rules || [];
    const installed = ab.installedApps || [];

    return `
      <div class="notice-box">
        <span><strong>System Process Protection Guard:</strong> Critical Windows processes (<code class="mono">csrss.exe</code>, <code class="mono">svchost.exe</code>, <code class="mono">lsass.exe</code>, <code class="mono">explorer.exe</code>, <code class="mono">dwm.exe</code>) are permanently allowlisted and can never be terminated.</span>
        <button class="btn btn-sm" data-action="rescan-apps">Rescan Installed Software</button>
      </div>

      <div class="grid-2">
        <div class="card">
          <div class="card-header"><div class="card-title">Add or Configure Application Rule</div></div>
          <form id="add-app-rule-form">
            <div class="grid-2" style="margin-bottom:10px;">
              <div class="field-group">
                <label class="field-label">Executable Filename</label>
                <input class="input mono" name="executableName" placeholder="e.g. robloxplayerbeta.exe" required />
              </div>
              <div class="field-group">
                <label class="field-label">Friendly Application Name</label>
                <input class="input" name="displayName" placeholder="e.g. Roblox" />
              </div>
            </div>
            <div class="grid-3" style="margin-bottom:10px;">
              <div class="field-group">
                <label class="field-label">Policy Action</label>
                <select class="select" name="status">
                  <option value="blocked">Block Always</option>
                  <option value="scheduled">Schedule & Time Limit</option>
                  <option value="allowed">Explicitly Allow</option>
                </select>
              </div>
              <div class="field-group">
                <label class="field-label">Daily Limit (min)</label>
                <input type="number" class="input" name="dailyLimitMinutes" value="60" min="0" max="480" />
              </div>
              <div class="field-group">
                <label class="field-label">Allowed Window</label>
                <input class="input mono" name="allowedHours" value="15:00-19:00" />
              </div>
            </div>
            <div style="text-align:right;">
              <button type="submit" class="btn btn-primary">Save Application Rule</button>
            </div>
          </form>
        </div>

        <div class="card">
          <div class="card-header"><div class="card-title">Active Application Policies (${rules.length})</div></div>
          ${rules.length === 0 ? '<div class="empty-state">No custom application restrictions configured yet.</div>' : `
            <table class="data-table">
              <thead><tr><th>Application</th><th>Rule</th><th>Schedule / Limit</th><th></th></tr></thead>
              <tbody>
                ${rules.map(r => `
                  <tr>
                    <td><div style="font-weight:600;">${esc(r.displayName)}</div><div class="mono" style="font-size:10.5px;color:var(--text-secondary);">${esc(r.executableName)}</div></td>
                    <td>${badgeHTML(r.status === 'blocked' ? 'error' : r.status === 'scheduled' ? 'warning' : 'success', r.status)}</td>
                    <td class="mono" style="font-size:11px;color:var(--text-secondary);">${r.status === 'scheduled' ? r.dailyLimitMinutes + 'm/day · ' + (r.allowedHours || 'All day') : '—'}</td>
                    <td style="text-align:right;"><button class="btn btn-sm btn-danger" data-action="delete-app-rule" data-exe="${esc(r.executableName)}">Remove</button></td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <div>
            <div class="card-title">Installed Applications (${installed.length})</div>
            <div class="card-subtitle">Discovered from Windows Registry Uninstall keys</div>
          </div>
        </div>
        ${installed.length === 0 ? '<div class="empty-state">No installed applications listed yet.</div>' : `
          <table class="data-table">
            <thead><tr><th>Application Name</th><th>Executable</th><th>Publisher</th><th>Status</th><th>Quick Policy</th></tr></thead>
            <tbody>
              ${installed.slice(0, 45).map(app => `
                <tr>
                  <td style="font-weight:600;">${esc(app.displayName)} ${app.newlyInstalled ? badgeHTML('warning', 'New') : ''}</td>
                  <td class="mono" style="color:var(--text-secondary);">${esc(app.executableName)}</td>
                  <td style="color:var(--text-secondary);font-size:11.5px;">${esc(app.publisher || 'Unknown')}</td>
                  <td>${badgeHTML(app.ruleStatus === 'blocked' ? 'error' : app.ruleStatus === 'scheduled' ? 'warning' : 'success', app.ruleStatus)}</td>
                  <td>
                    <div class="row-inline">
                      <button class="btn btn-sm" data-action="quick-app-rule" data-exe="${esc(app.executableName)}" data-name="${esc(app.displayName)}" data-status="allowed">Allow</button>
                      <button class="btn btn-sm btn-danger" data-action="quick-app-rule" data-exe="${esc(app.executableName)}" data-name="${esc(app.displayName)}" data-status="blocked">Block</button>
                    </div>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        `}
      </div>
    `;
  }

  function renderWebsitesSection() {
    const wb = state.websitesBundle;
    if (!wb) return '<div class="empty-state">Loading website and DNS policies...</div>';
    const pol = wb.policy;
    const events = wb.blockedEvents || [];

    return `
      <div class="notice-box">
        <span><strong>Technical Transparency Note:</strong> ${esc(pol.transparencyNote)}</span>
        <button class="btn btn-sm" data-action="enforce-dns">Apply DNS & Hosts Now</button>
      </div>

      <div class="grid-4">
        ${statCardHTML('Cloudflare Family IPv4', pol.primaryIpv4 + ' / ' + pol.secondaryIpv4, 'Primary & Secondary DNS', pol.dnsMode === 'cloudflare_family' ? 'success' : 'default')}
        ${statCardHTML('Cloudflare Family IPv6', '2606:4700:4700::1113', 'Secondary: 2606:4700:4700::1003', pol.dnsMode === 'cloudflare_family' ? 'success' : 'default')}
        ${statCardHTML('Malware & Adult Filter', pol.blockAdult ? 'Malware + Adult' : pol.blockMalware ? 'Malware Only' : 'Off', 'Network resolver filter', pol.blockMalware ? 'success' : 'warning')}
        ${statCardHTML('Custom Domain Rules', pol.blocklist.length + ' Blocked', pol.allowlist.length + ' allowlisted domains')}
      </div>

      <div class="card">
        <div class="card-header"><div class="card-title">Cloudflare 1.1.1.1 for Families DNS Configuration</div></div>
        <div class="grid-3">
          <button class="card" data-action="set-dns-mode" data-mode="cloudflare_family" style="text-align:left;cursor:pointer;border-color:${pol.dnsMode === 'cloudflare_family' ? 'var(--accent)' : 'var(--border-subtle)'};">
            <div class="row-between" style="margin-bottom:4px;"><span style="font-weight:600;">Malware + Adult Content</span>${badgeHTML('success', '1.1.1.3')}</div>
            <div class="mono" style="font-size:11px;color:var(--accent);">IPv4: 1.1.1.3, 1.0.0.3 · IPv6: 2606:4700:4700::1113, ::1003</div>
          </button>
          <button class="card" data-action="set-dns-mode" data-mode="cloudflare_malware" style="text-align:left;cursor:pointer;border-color:${pol.dnsMode === 'cloudflare_malware' ? 'var(--accent)' : 'var(--border-subtle)'};">
            <div class="row-between" style="margin-bottom:4px;"><span style="font-weight:600;">Malware Only</span>${badgeHTML('neutral', '1.1.1.2')}</div>
            <div class="mono" style="font-size:11px;color:var(--accent);">IPv4: 1.1.1.2, 1.0.0.2 · IPv6: 2606:4700:4700::1112, ::1002</div>
          </button>
          <button class="card" data-action="set-dns-mode" data-mode="off" style="text-align:left;cursor:pointer;border-color:${pol.dnsMode === 'off' ? 'var(--accent)' : 'var(--border-subtle)'};">
            <div class="row-between" style="margin-bottom:4px;"><span style="font-weight:600;">System Default DNS</span>${badgeHTML('warning', 'Off')}</div>
            <div class="mono" style="font-size:11px;color:var(--text-secondary);">Leaves adapter DNS unchanged</div>
          </button>
        </div>
      </div>

      <div class="grid-2">
        <div class="card">
          <div class="card-header"><div class="card-title">Custom Domain Blocklist (${pol.blocklist.length})</div></div>
          <form id="add-block-domain-form" class="row-inline" style="margin-bottom:10px;">
            <input class="input mono" name="domain" placeholder="e.g. tiktok.com" required />
            <button type="submit" class="btn btn-danger">Block Domain</button>
          </form>
          ${pol.blocklist.length === 0 ? '<div class="empty-state">No custom blocked domains.</div>' : `
            <table class="data-table">
              <tbody>
                ${pol.blocklist.map(d => `
                  <tr><td class="mono">${esc(d)}</td><td style="text-align:right;"><button class="btn btn-sm" data-action="remove-block-domain" data-domain="${esc(d)}">Remove</button></td></tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>

        <div class="card">
          <div class="card-header"><div class="card-title">Domain Allowlist (${pol.allowlist.length})</div></div>
          <form id="add-allow-domain-form" class="row-inline" style="margin-bottom:10px;">
            <input class="input mono" name="domain" placeholder="e.g. wikipedia.org" required />
            <button type="submit" class="btn btn-primary">Allow Domain</button>
          </form>
          ${pol.allowlist.length === 0 ? '<div class="empty-state">No allowlisted domains.</div>' : `
            <table class="data-table">
              <tbody>
                ${pol.allowlist.map(d => `
                  <tr><td class="mono">${esc(d)}</td><td style="text-align:right;"><button class="btn btn-sm" data-action="remove-allow-domain" data-domain="${esc(d)}">Remove</button></td></tr>
                `).join('')}
              </tbody>
            </table>
          `}
        </div>
      </div>

      <div class="card">
        <div class="card-header"><div class="card-title">Blocked-Domain History</div></div>
        ${events.length === 0 ? '<div class="empty-state">No blocked domain attempts recorded yet.</div>' : `
          <table class="data-table">
            <thead><tr><th>Domain</th><th>Category</th><th>Reason</th><th>Time</th></tr></thead>
            <tbody>
              ${events.map(ev => `
                <tr><td class="mono">${esc(ev.domain)}</td><td>${badgeHTML('warning', ev.category)}</td><td>${esc(ev.reason)}</td><td style="font-size:11.5px;color:var(--text-secondary);">${formatTimestamp(ev.occurredAt)}</td></tr>
              `).join('')}
            </tbody>
          </table>
        `}
      </div>
    `;
  }

  function renderProtectionSection() {
    const p = state.protectionStatus;
    if (!p) return '<div class="empty-state">Verifying protection status...</div>';

    return `
      <div class="grid-4">
        ${statCardHTML('DNS Protection', p.dnsProtected ? 'Active' : 'Disabled', 'Profile: ' + p.dnsMode, p.dnsProtected ? 'success' : 'warning')}
        ${statCardHTML('Windows Firewall', p.firewallEnabled ? 'Enabled' : 'Disabled', 'Standard/Public Profile', p.firewallEnabled ? 'success' : 'error')}
        ${statCardHTML('Background Agent', p.agentRunning ? 'Running' : 'Stopped', 'Service: VylosService', p.agentRunning ? 'success' : 'warning')}
        ${statCardHTML('Integrity Check', p.configurationIntact ? 'Verified' : 'Attention', formatTimestamp(p.lastIntegrityCheck), p.configurationIntact ? 'success' : 'warning')}
      </div>

      <div class="card">
        <div class="card-header">
          <div class="card-title">System & Cryptographic Identity</div>
          <button class="btn btn-sm" data-action="refresh-protection">Run Integrity Check Now</button>
        </div>
        <table class="data-table">
          <tbody>
            <tr><td style="color:var(--text-secondary);">Local LAN IPv4 Addresses</td><td class="mono">${esc((p.localIpAddresses || []).join(', '))}</td></tr>
            <tr><td style="color:var(--text-secondary);">Active OS DNS Resolvers</td><td class="mono">${esc((p.activeDnsServers || []).join(', ') || 'System Default')}</td></tr>
            <tr><td style="color:var(--text-secondary);">TLS 1.3 Certificate Pin (SHA-256)</td><td class="mono">${esc(p.certFingerprint)}</td></tr>
            <tr><td style="color:var(--text-secondary);">Local Keystore Encryption</td><td>${badgeHTML('success', 'Windows DPAPI + XChaCha20-Poly1305')}</td></tr>
          </tbody>
        </table>
      </div>

      <div class="card">
        <div class="card-header"><div class="card-title">Security & Configuration Audit Log</div></div>
        ${!p.recentSecurityEvents || p.recentSecurityEvents.length === 0 ? '<div class="empty-state">No security events logged.</div>' : `
          <table class="data-table">
            <thead><tr><th>Severity</th><th>Category</th><th>Message</th><th>Details</th><th>Time</th></tr></thead>
            <tbody>
              ${p.recentSecurityEvents.map(ev => `
                <tr>
                  <td>${badgeHTML(ev.severity === 'critical' ? 'error' : ev.severity === 'warning' ? 'warning' : 'success', ev.severity)}</td>
                  <td class="mono">${esc(ev.category)}</td>
                  <td style="font-weight:500;">${esc(ev.message)}</td>
                  <td class="mono" style="font-size:11px;color:var(--text-secondary);">${esc(ev.details || '—')}</td>
                  <td style="font-size:11.5px;color:var(--text-secondary);">${formatTimestamp(ev.occurredAt)}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        `}
      </div>
    `;
  }

  function renderSettingsSection() {
    const cfg = state.bootstrap.config;
    return `
      <div class="grid-2">
        <div class="card">
          <div class="card-header"><div class="card-title">Family, Network & Data Retention Settings</div></div>
          <form id="general-settings-form" style="display:flex; flex-direction:column; gap:10px;">
            <div class="field-group">
              <label class="field-label">Family Name</label>
              <input class="input" name="familyName" value="${esc(cfg.familyName)}" required />
            </div>
            <div class="field-group">
              <label class="field-label">Parent Contact Information (Displayed in Child Mode)</label>
              <input class="input" name="parentContact" value="${esc(cfg.parentContact)}" />
            </div>
            <div class="grid-2">
              <div class="field-group">
                <label class="field-label">Parent TLS 1.3 Port</label>
                <input type="number" class="input mono" name="listenPort" value="${cfg.listenPort}" />
              </div>
              <div class="field-group">
                <label class="field-label">Activity Retention (Days)</label>
                <input type="number" class="input" name="retentionDays" value="${cfg.retentionDays}" />
              </div>
            </div>
            <div class="row-between" style="margin-top:4px;">
              <button type="button" class="btn btn-sm" data-action="prune-data">Prune Old Records Now</button>
              <button type="submit" class="btn btn-primary">Save Settings</button>
            </div>
          </form>
        </div>

        <div class="card">
          <div class="card-header"><div class="card-title">Parent Authentication (Argon2id)</div></div>
          <form id="change-password-form" style="display:flex; flex-direction:column; gap:10px;">
            <div class="field-group">
              <label class="field-label">Current Parent Password</label>
              <input type="password" class="input" name="currentPassword" required />
            </div>
            <div class="field-group">
              <label class="field-label">New Parent Password (min 8 chars)</label>
              <input type="password" class="input" name="newPassword" required />
            </div>
            <div style="text-align:right;">
              <button type="submit" class="btn btn-primary">Update Password</button>
            </div>
          </form>
        </div>
      </div>

      <div class="grid-2">
        <div class="card">
          <div class="card-header">
            <div class="card-title">Encrypted Backup & Restore</div>
            <div class="row-inline">
              <button class="btn btn-sm" data-action="export-backup">Export Encrypted Backup</button>
              <button class="btn btn-sm btn-primary" data-action="import-backup">Restore Backup</button>
            </div>
          </div>
          <textarea id="backup-textarea" class="textarea mono" rows="4" placeholder="Encrypted backup blob...">${esc(state.backupText)}</textarea>
        </div>

        <div class="card">
          <div class="card-header">
            <div class="card-title">Uninstall Authorization</div>
            ${badgeHTML('neutral', 'Vylos · MIT License')}
          </div>
          <form id="uninstall-auth-form" class="row-inline" style="margin-bottom:10px;">
            <input type="password" class="input" name="password" placeholder="Enter Parent Password to authorize uninstall" required />
            <button type="submit" class="btn btn-danger">Verify Uninstall Auth</button>
          </form>
          ${state.uninstallAuthorized ? '<div class="card mono" style="background:var(--bg-input);font-size:11px;color:var(--success);">Uninstall Authorized. Run as Admin: Vylos.exe --uninstall-service --password "..."</div>' : '<div style="font-size:11.5px;color:var(--text-secondary);">Device ID: <span class="mono">' + esc(cfg.localDeviceId) + '</span></div>'}
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <div>
            <div class="card-title">Vylos Automatic Updater (GitHub Release MSI)</div>
            <div class="card-subtitle">Checks https://github.com/Tenkayro/Vylos/releases/latest/download/latest.json for verified MSI updates</div>
          </div>
          <div class="row-inline">
            <button class="btn btn-sm" data-action="check-updates">${state.checkingUpdate ? 'Checking...' : 'Check for Updates'}</button>
            ${state.updateInfo && state.updateInfo.updateAvailable ? '<button class="btn btn-sm btn-primary" data-action="install-update">' + (state.installingUpdate ? 'Downloading MSI...' : 'Update to v' + esc(state.updateInfo.latestVersion)) + '</button>' : ''}
          </div>
        </div>
        ${state.updateInfo ? `
          <div class="row-between" style="font-size:12px;">
            <div>
              Current: <span class="mono">${esc(state.updateInfo.currentVersion)}</span> · Latest: <span class="mono">${esc(state.updateInfo.latestVersion)}</span>
              ${state.updateInfo.notes ? '<div style="color:var(--text-secondary);margin-top:4px;">' + esc(state.updateInfo.notes) + '</div>' : ''}
            </div>
            ${badgeHTML(state.updateInfo.updateAvailable ? 'warning' : 'success', state.updateInfo.updateAvailable ? 'Update Available' : 'Up to Date')}
          </div>
        ` : '<div style="font-size:11.5px;color:var(--text-secondary);">Click "Check for Updates" to query the latest release manifest and automatically install a newer MSI build.</div>'}
      </div>
    `;
  }

  function renderChildMode() {
    const cs = state.bootstrap.childStatus;
    const ev = cs.screenTimeEval;
    const pol = cs.screenTimePolicy;

    return `
      <div style="max-width:780px; margin:0 auto; padding:22px 18px; display:flex; flex-direction:column; gap:14px; height:100vh; overflow-y:auto;">
        <div class="card row-between">
          <div class="row-inline">
            <div class="brand-mark">V</div>
            <div>
              <div style="font-size:14.5px; font-weight:600;">Vylos Family Control · Child Mode (${esc(cs.deviceName)})</div>
              <div style="font-size:11.5px; color:var(--text-secondary);">Household: ${esc(cs.familyName)} · Transparent Parental Protection Active</div>
            </div>
          </div>
          <div class="row-inline">
            ${badgeHTML(ev.allowedNow ? 'success' : 'warning', ev.allowedNow ? 'Parental Controls Active' : 'Screen Time Restricted')}
            <button class="btn btn-sm" data-action="child-sync">Sync with Parent PC</button>
          </div>
        </div>

        <div class="grid-3">
          ${statCardHTML('Remaining Screen Time', formatMinutes(ev.remainingMinutes), formatMinutes(ev.usedTodayMinutes) + ' used of ' + formatMinutes(ev.effectiveLimitMinutes) + ' today', ev.remainingMinutes > 15 ? 'success' : 'warning')}
          ${statCardHTML('Bedtime & Schedule', pol.bedtimeStart + ' – ' + pol.bedtimeEnd, 'Break reminder every ' + pol.breakIntervalMinutes + 'm')}
          ${statCardHTML('Parent PC Connection', cs.connectedToLan ? 'Paired (LAN)' : 'Local Standalone', cs.parentEndpoint || 'Enforcing rules locally', cs.connectedToLan ? 'success' : 'default')}
        </div>

        <div class="card">
          <div class="card-header"><div class="card-title">Current Family Rules on This Computer</div></div>
          <table class="data-table">
            <tbody>
              <tr><td style="color:var(--text-secondary);">Weekday / Weekend Limit</td><td style="font-weight:600;">${formatMinutes(pol.dailyLimitMinutes)} / ${formatMinutes(pol.weekendLimitMinutes)}</td></tr>
              <tr><td style="color:var(--text-secondary);">Website & DNS Filtering</td><td>${badgeHTML(cs.websitePolicy.dnsMode !== 'off' ? 'success' : 'neutral', cs.websitePolicy.dnsMode === 'cloudflare_family' ? 'Cloudflare Family (1.1.1.3)' : cs.websitePolicy.dnsMode)}</td></tr>
              <tr><td style="color:var(--text-secondary);">Parent Contact Note</td><td style="font-weight:500;">${esc(cs.parentContact || 'Talk to your parent if you need extra time')}</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    `;
  }

  function render() {
    const root = document.getElementById('app');
    if (!root) return;

    const toastHTML = state.toast ? `
      <div style="position:fixed; bottom:16px; right:18px; z-index:9999; padding:9px 14px; border-radius:6px; font-size:12.5px; font-weight:500; background:var(--bg-elevated); border:1px solid ${state.toast.type === 'error' ? 'var(--error)' : state.toast.type === 'success' ? 'var(--success)' : 'var(--border-focus)'}; color:var(--text-primary); box-shadow:0 6px 20px rgba(0,0,0,0.45);">
        ${esc(state.toast.text)}
      </div>
    ` : '';

    if (state.loading && !state.bootstrap) {
      root.innerHTML = '<div style="display:flex;align-items:center;justify-content:center;height:100vh;color:var(--text-secondary);">Loading Vylos Family Control...</div>';
      return;
    }

    if (!state.bootstrap.config.setupCompleted || state.bootstrap.config.mode === 'unconfigured') {
      root.innerHTML = toastHTML + renderSetupWizard();
      bindEvents();
      return;
    }

    if (state.bootstrap.config.mode === 'child') {
      root.innerHTML = toastHTML + renderChildMode();
      bindEvents();
      return;
    }

    const navItems = [
      { id: 'overview', label: 'Overview' },
      { id: 'devices', label: 'Devices' },
      { id: 'activity', label: 'Activity' },
      { id: 'screen_time', label: 'Screen Time' },
      { id: 'applications', label: 'Applications' },
      { id: 'websites', label: 'Websites' },
      { id: 'protection', label: 'Protection' },
      { id: 'settings', label: 'Settings' },
    ];

    let pageContent = '';
    switch (state.activeSection) {
      case 'overview': pageContent = renderOverviewSection(); break;
      case 'devices': pageContent = renderDevicesSection(); break;
      case 'activity': pageContent = renderActivitySection(); break;
      case 'screen_time': pageContent = renderScreenTimeSection(); break;
      case 'applications': pageContent = renderApplicationsSection(); break;
      case 'websites': pageContent = renderWebsitesSection(); break;
      case 'protection': pageContent = renderProtectionSection(); break;
      case 'settings': pageContent = renderSettingsSection(); break;
    }

    const cfg = state.bootstrap.config;
    const devs = state.bootstrap.devices || [];
    const currentLabel = (navItems.find(n => n.id === state.activeSection) || navItems[0]).label;

    root.innerHTML = toastHTML + `
      <div class="app-shell">
        <aside class="sidebar">
          <div class="brand-header">
            <div class="brand-mark">V</div>
            <div>
              <div class="brand-title">Vylos</div>
              <div class="brand-subtitle">Family Control · ${esc(cfg.familyName)}</div>
            </div>
          </div>
          <nav class="nav-list">
            ${navItems.map(item => `
              <button class="nav-item ${state.activeSection === item.id ? 'active' : ''}" data-action="nav" data-section="${item.id}">
                <span class="nav-item-left"><span>${item.label}</span></span>
                ${item.id === 'devices' && devs.length > 0 ? '<span class="nav-badge">' + devs.length + '</span>' : ''}
              </button>
            `).join('')}
          </nav>
          <div class="sidebar-footer">
            <div class="row-between" style="font-size:11.5px; color:var(--text-secondary); padding:0 4px;">
              <span>LAN TLS 1.3</span>
              ${badgeHTML('success', 'Port ' + cfg.listenPort)}
            </div>
          </div>
        </aside>
        <main class="workspace">
          <header class="topbar">
            <div class="topbar-title-group">
              <h1 class="topbar-title">${esc(currentLabel)}</h1>
              ${badgeHTML('neutral', 'Local Network Only')}
            </div>
            <div class="topbar-controls">
              <div class="row-inline">
                <label style="font-size:11.5px; color:var(--text-secondary);">Target Device:</label>
                <select id="device-picker" class="select" style="width:210px; padding:4px 8px;">
                  <option value="${esc(cfg.localDeviceId)}" ${state.selectedDeviceId === cfg.localDeviceId ? 'selected' : ''}>This PC (${esc(cfg.localDeviceName)})</option>
                  ${devs.map(d => '<option value="' + esc(d.id) + '" ' + (state.selectedDeviceId === d.id ? 'selected' : '') + '>' + esc(d.name) + ' — ' + d.status.toUpperCase() + '</option>').join('')}
                </select>
              </div>
              <button class="btn btn-sm" data-action="refresh">Refresh</button>
            </div>
          </header>
          <div class="content-scroll">${pageContent}</div>
        </main>
      </div>
    `;

    bindEvents();
  }

  function bindEvents() {
    document.querySelectorAll('[data-action]').forEach(el => {
      el.onclick = async (ev) => {
        const action = el.getAttribute('data-action');
        if (action === 'wizard-step') {
          state.wizardStep = el.getAttribute('data-step');
          render();
        } else if (action === 'finish-wizard') {
          await refreshBootstrap();
        } else if (action === 'nav') {
          state.activeSection = el.getAttribute('data-section');
          await loadSectionData(state.activeSection);
        } else if (action === 'refresh') {
          await refreshBootstrap();
        } else if (action === 'quick-lock') {
          const devId = el.getAttribute('data-device');
          const lock = el.getAttribute('data-lock') === 'true';
          await callRPC('SetDeviceLockAndPause', { deviceId: devId, screenLocked: lock, manualPause: false });
          await refreshBootstrap();
        } else if (action === 'remove-child') {
          await callRPC('RemoveChild', { childId: el.getAttribute('data-id') });
          await refreshBootstrap();
        } else if (action === 'scan-lan') {
          await callRPC('TriggerDiscoveryBroadcast');
          showToast('UDP LAN discovery broadcast sent.', 'info');
          await refreshBootstrap();
        } else if (action === 'select-device-rules') {
          state.selectedDeviceId = el.getAttribute('data-id');
          state.activeSection = 'screen_time';
          await loadSectionData('screen_time');
        } else if (action === 'revoke-device') {
          await callRPC('RevokeDevice', { deviceId: el.getAttribute('data-id') });
          await refreshBootstrap();
        } else if (action === 'bonus-time') {
          const mins = parseInt(el.getAttribute('data-mins'), 10);
          await callRPC('GrantBonusTime', { deviceId: state.selectedDeviceId, bonusMinutes: mins });
          showToast('Granted +' + mins + 'm bonus time.', 'success');
          await loadSectionData('screen_time');
        } else if (action === 'toggle-pause') {
          const pol = state.screenTimeBundle.policy;
          await callRPC('SetDeviceLockAndPause', { deviceId: pol.deviceId, screenLocked: pol.screenLocked, manualPause: !pol.manualPause });
          await loadSectionData('screen_time');
        } else if (action === 'toggle-lock') {
          const pol = state.screenTimeBundle.policy;
          await callRPC('SetDeviceLockAndPause', { deviceId: pol.deviceId, screenLocked: !pol.screenLocked, manualPause: pol.manualPause });
          await loadSectionData('screen_time');
        } else if (action === 'rescan-apps') {
          await callRPC('RefreshInstalledApplications', { deviceId: state.selectedDeviceId });
          showToast('Scanned Windows Registry for installed software.', 'success');
          await loadSectionData('applications');
        } else if (action === 'quick-app-rule') {
          await callRPC('SaveApplicationRule', {
            rule: {
              deviceId: state.selectedDeviceId,
              executableName: el.getAttribute('data-exe'),
              displayName: el.getAttribute('data-name'),
              status: el.getAttribute('data-status'),
            }
          });
          await loadSectionData('applications');
        } else if (action === 'delete-app-rule') {
          await callRPC('DeleteApplicationRule', { deviceId: state.selectedDeviceId, executableName: el.getAttribute('data-exe') });
          await loadSectionData('applications');
        } else if (action === 'set-dns-mode') {
          const pol = state.websitesBundle.policy;
          pol.dnsMode = el.getAttribute('data-mode');
          await callRPC('SaveWebsitePolicy', { policy: pol });
          showToast('Updated DNS protection mode.', 'success');
          await loadSectionData('websites');
        } else if (action === 'remove-block-domain') {
          const pol = state.websitesBundle.policy;
          const dom = el.getAttribute('data-domain');
          pol.blocklist = pol.blocklist.filter(d => d !== dom);
          await callRPC('SaveWebsitePolicy', { policy: pol });
          await loadSectionData('websites');
        } else if (action === 'remove-allow-domain') {
          const pol = state.websitesBundle.policy;
          const dom = el.getAttribute('data-domain');
          pol.allowlist = pol.allowlist.filter(d => d !== dom);
          await callRPC('SaveWebsitePolicy', { policy: pol });
          await loadSectionData('websites');
        } else if (action === 'enforce-dns') {
          await callRPC('EnforceDNSAndHosts', { deviceId: state.selectedDeviceId });
          showToast('DNS & Hosts rules applied.', 'success');
          await loadSectionData('websites');
        } else if (action === 'refresh-protection') {
          await loadSectionData('protection');
        } else if (action === 'prune-data') {
          const n = await callRPC('PruneHistoricalData', { retentionDays: state.bootstrap.config.retentionDays });
          showToast('Pruned ' + n + ' old records.', 'success');
        } else if (action === 'export-backup') {
          state.backupText = await callRPC('ExportEncryptedBackup');
          showToast('Encrypted backup generated.', 'success');
          render();
        } else if (action === 'import-backup') {
          const ta = document.getElementById('backup-textarea');
          if (ta && ta.value.trim()) {
            await callRPC('ImportEncryptedBackup', { encryptedPayload: ta.value.trim() });
            showToast('Backup restored.', 'success');
            await refreshBootstrap();
          }
        } else if (action === 'child-sync') {
          try {
            await callRPC('TriggerChildSync');
            showToast('Synchronized with Parent PC.', 'success');
          } catch (e) {
            showToast(e.message, 'info');
          }
          await refreshBootstrap();
        } else if (action === 'check-updates') {
          state.checkingUpdate = true;
          render();
          try {
            state.updateInfo = await callRPC('CheckForUpdates');
            if (state.updateInfo.updateAvailable) {
              showToast('New Vylos version v' + state.updateInfo.latestVersion + ' available!', 'success');
            } else {
              showToast('Vylos is up to date (v' + state.updateInfo.currentVersion + ').', 'info');
            }
          } catch (e) {
            showToast(e.message, 'error');
          } finally {
            state.checkingUpdate = false;
            render();
          }
        } else if (action === 'install-update') {
          state.installingUpdate = true;
          render();
          try {
            const res = await callRPC('InstallUpdate');
            showToast('Downloaded & launched Vylos v' + res.latestVersion + ' MSI update!', 'success');
          } catch (e) {
            showToast(e.message, 'error');
          } finally {
            state.installingUpdate = false;
            render();
          }
        }
      };
    });

    const picker = document.getElementById('device-picker');
    if (picker) {
      picker.onchange = async () => {
        state.selectedDeviceId = picker.value;
        await loadSectionData(state.activeSection);
      };
    }

    const pForm = document.getElementById('parent-setup-form');
    if (pForm) {
      pForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(pForm);
        const pass = fd.get('password');
        if (pass !== fd.get('passwordConfirm')) {
          showToast('Passwords do not match.', 'error');
          return;
        }
        try {
          const res = await callRPC('SetupParentMode', {
            familyName: fd.get('familyName'),
            password: pass,
            parentContact: fd.get('parentContact'),
            firstChildName: fd.get('firstChildName'),
          });
          state.wizardRecoveryCode = res.recoveryCode;
          state.wizardStep = 'parent_recovery';
          render();
        } catch (err) {
          showToast(err.message, 'error');
        }
      };
    }

    const cForm = document.getElementById('child-setup-form');
    if (cForm) {
      cForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(cForm);
        const pass = String(fd.get('parentPassword') || '');
        const passConfirm = String(fd.get('parentPasswordConfirm') || '');
        if (pass.length < 8) {
          showToast('Parent password must be at least 8 characters.', 'error');
          return;
        }
        if (pass !== passConfirm) {
          showToast('Parent passwords do not match.', 'error');
          return;
        }
        try {
          await callRPC('SetupChildMode', {
            deviceName: fd.get('deviceName'),
            parentPassword: pass,
            parentContact: '',
          });
          const ep = String(fd.get('parentEndpoint') || '').trim();
          const code = String(fd.get('pairingCode') || '').trim();
          if (ep && code) {
            await callRPC('PairWithParent', { parentEndpoint: ep, pairingCode: code, expectedCertFp: '' });
          }
          await refreshBootstrap();
        } catch (err) {
          showToast(err.message, 'error');
          await refreshBootstrap();
        }
      };
    }

    const pairForm = document.getElementById('generate-pair-form');
    if (pairForm) {
      pairForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(pairForm);
        state.activeTicket = await callRPC('CreatePairingCode', {
          childId: fd.get('childId'),
          deviceNameHint: fd.get('deviceNameHint'),
        });
        showToast('One-time pairing code generated.', 'success');
        render();
      };
    }

    const childAddForm = document.getElementById('add-child-form');
    if (childAddForm) {
      childAddForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(childAddForm);
        await callRPC('AddChild', { name: fd.get('name'), avatarColor: '#DCE9FF', ageGroup: fd.get('ageGroup') });
        showToast('Child profile added.', 'success');
        await refreshBootstrap();
      };
    }

    const stForm = document.getElementById('screentime-form');
    if (stForm) {
      stForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(stForm);
        const pol = state.screenTimeBundle.policy;
        pol.dailyLimitMinutes = parseInt(fd.get('dailyLimitMinutes'), 10);
        pol.weekendLimitMinutes = parseInt(fd.get('weekendLimitMinutes'), 10);
        pol.bedtimeStart = fd.get('bedtimeStart');
        pol.bedtimeEnd = fd.get('bedtimeEnd');
        pol.breakIntervalMinutes = parseInt(fd.get('breakIntervalMinutes'), 10);
        pol.breakDurationMinutes = parseInt(fd.get('breakDurationMinutes'), 10);
        ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'].forEach(day => {
          if (pol.weeklySchedule && pol.weeklySchedule[day]) {
            pol.weeklySchedule[day].allowedStart = fd.get('start_' + day);
            pol.weeklySchedule[day].allowedEnd = fd.get('end_' + day);
          }
        });
        await callRPC('SaveScreenTimePolicy', { policy: pol });
        showToast('Screen time policy saved.', 'success');
        await loadSectionData('screen_time');
      };
    }

    const appRuleForm = document.getElementById('add-app-rule-form');
    if (appRuleForm) {
      appRuleForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(appRuleForm);
        try {
          await callRPC('SaveApplicationRule', {
            rule: {
              deviceId: state.selectedDeviceId,
              executableName: fd.get('executableName'),
              displayName: fd.get('displayName') || fd.get('executableName'),
              status: fd.get('status'),
              dailyLimitMinutes: parseInt(fd.get('dailyLimitMinutes'), 10) || 0,
              allowedHours: fd.get('allowedHours'),
            }
          });
          showToast('Application rule saved.', 'success');
          await loadSectionData('applications');
        } catch (err) {
          showToast(err.message, 'error');
        }
      };
    }

    const blockDomForm = document.getElementById('add-block-domain-form');
    if (blockDomForm) {
      blockDomForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(blockDomForm);
        const pol = state.websitesBundle.policy;
        pol.blocklist = [...pol.blocklist, String(fd.get('domain')).trim()];
        try {
          await callRPC('SaveWebsitePolicy', { policy: pol });
          showToast('Domain added to blocklist.', 'success');
          await loadSectionData('websites');
        } catch (err) {
          showToast(err.message, 'error');
        }
      };
    }

    const allowDomForm = document.getElementById('add-allow-domain-form');
    if (allowDomForm) {
      allowDomForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(allowDomForm);
        const pol = state.websitesBundle.policy;
        pol.allowlist = [...pol.allowlist, String(fd.get('domain')).trim()];
        try {
          await callRPC('SaveWebsitePolicy', { policy: pol });
          showToast('Domain added to allowlist.', 'success');
          await loadSectionData('websites');
        } catch (err) {
          showToast(err.message, 'error');
        }
      };
    }

    const genForm = document.getElementById('general-settings-form');
    if (genForm) {
      genForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(genForm);
        await callRPC('UpdateGeneralSettings', {
          familyName: fd.get('familyName'),
          parentContact: fd.get('parentContact'),
          listenPort: parseInt(fd.get('listenPort'), 10),
          retentionDays: parseInt(fd.get('retentionDays'), 10),
        });
        showToast('Settings saved.', 'success');
        await refreshBootstrap();
      };
    }

    const passForm = document.getElementById('change-password-form');
    if (passForm) {
      passForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(passForm);
        try {
          await callRPC('ChangeParentPassword', {
            currentPassword: fd.get('currentPassword'),
            newPassword: fd.get('newPassword'),
          });
          showToast('Parent password updated.', 'success');
          passForm.reset();
        } catch (err) {
          showToast(err.message, 'error');
        }
      };
    }

    const uninstForm = document.getElementById('uninstall-auth-form');
    if (uninstForm) {
      uninstForm.onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(uninstForm);
        try {
          state.uninstallAuthorized = await callRPC('AuthorizeUninstall', { parentPassword: fd.get('password') });
          showToast('Uninstall authorized.', 'success');
          render();
        } catch (err) {
          showToast(err.message, 'error');
        }
      };
    }
  }

  refreshBootstrap();
})();

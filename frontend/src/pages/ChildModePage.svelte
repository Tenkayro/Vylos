<script lang="ts">
  import { api, type ChildModeStatus, type DiscoveryBeacon } from '../services/api';
  import { formatMinutes, formatTimestamp, showToast } from '../stores/appStore';
  import StatCard from '../components/StatCard.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';

  export let childStatus: ChildModeStatus;
  export let discoveredPeers: DiscoveryBeacon[] = [];
  export let onUpdated: () => void;

  let showPairingPanel = false;
  let showParentUnlock = false;
  let parentEndpoint = childStatus.parentEndpoint || '';
  let pairingCode = '';
  let unlockPassword = '';
  let syncing = false;

  async function handleSyncNow() {
    syncing = true;
    try {
      await api.triggerChildSync();
      showToast('Synchronized rules and activity with Parent PC.', 'success');
      onUpdated();
    } catch (e: any) {
      showToast(e.message, 'info');
      onUpdated();
    } finally {
      syncing = false;
    }
  }

  async function handlePair() {
    if (!parentEndpoint.trim() || !pairingCode.trim()) {
      showToast('Enter both Parent PC IP:Port and the One-Time Pairing Code.', 'error');
      return;
    }
    try {
      await api.pairWithParent(parentEndpoint.trim(), pairingCode.trim());
      pairingCode = '';
      showPairingPanel = false;
      showToast('Paired with Parent PC over TLS 1.3!', 'success');
      onUpdated();
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleSwitchToParent() {
    if (!unlockPassword) {
      showToast('Enter the Parent password.', 'error');
      return;
    }
    try {
      await api.switchOperatingMode(unlockPassword, 'parent');
      unlockPassword = '';
      showParentUnlock = false;
      showToast('Unlocked Parent Dashboard mode.', 'success');
      onUpdated();
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }
</script>

<div style="max-width:780px; margin:0 auto; padding:22px 18px; display:flex; flex-direction:column; gap:14px; height:100vh; overflow-y:auto;">
  <div class="card" style="display:flex; align-items:center; justify-content:space-between;">
    <div class="row-inline">
      <div class="brand-mark">V</div>
      <div>
        <div style="font-size:14.5px; font-weight:600;">
          Vylos Family Control · Child Mode ({childStatus.deviceName})
        </div>
        <div style="font-size:11.5px; color:var(--text-secondary);">
          Household: {childStatus.familyName} · Transparent Parental Protection Active
        </div>
      </div>
    </div>

    <div class="row-inline">
      <StatusBadge
        variant={childStatus.screenTimeEval.allowedNow ? 'success' : 'warning'}
        label={childStatus.screenTimeEval.allowedNow ? 'Parental Controls Active' : 'Screen Time Restricted'}
      />
      <button class="btn btn-sm" disabled={syncing} on:click={handleSyncNow}>
        {syncing ? 'Syncing...' : 'Sync with Parent PC'}
      </button>
    </div>
  </div>

  <div class="grid-3">
    <StatCard
      label="Remaining Screen Time"
      value={formatMinutes(childStatus.screenTimeEval.remainingMinutes)}
      meta="{formatMinutes(childStatus.screenTimeEval.usedTodayMinutes)} used of {formatMinutes(childStatus.screenTimeEval.effectiveLimitMinutes)} today"
      tone={childStatus.screenTimeEval.remainingMinutes > 15 ? 'success' : 'warning'}
    />
    <StatCard
      label="Bedtime & Schedule"
      value="{childStatus.screenTimePolicy.bedtimeStart} – {childStatus.screenTimePolicy.bedtimeEnd}"
      meta="Break reminder every {childStatus.screenTimePolicy.breakIntervalMinutes}m"
    />
    <StatCard
      label="Parent PC Connection"
      value={childStatus.connectedToLan ? 'Paired (LAN)' : 'Local Standalone'}
      meta={childStatus.parentEndpoint || 'Enforcing rules locally'}
      tone={childStatus.connectedToLan ? 'success' : 'default'}
    />
  </div>

  <div class="grid-2">
    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">Current Family Rules on This Computer</div>
          <div class="card-subtitle">Rules are enforced locally and cannot be modified without Parent authorization</div>
        </div>
      </div>

      <table class="data-table">
        <tbody>
          <tr>
            <td style="color:var(--text-secondary);">Weekday / Weekend Limit</td>
            <td style="font-weight:600;">
              {formatMinutes(childStatus.screenTimePolicy.dailyLimitMinutes)} / {formatMinutes(childStatus.screenTimePolicy.weekendLimitMinutes)}
            </td>
          </tr>
          <tr>
            <td style="color:var(--text-secondary);">DNS & Website Filtering</td>
            <td>
              <StatusBadge
                variant={childStatus.websitePolicy.dnsMode !== 'off' ? 'success' : 'neutral'}
                label={childStatus.websitePolicy.dnsMode === 'cloudflare_family'
                  ? 'Cloudflare Family (1.1.1.3)'
                  : childStatus.websitePolicy.dnsMode}
              />
            </td>
          </tr>
          <tr>
            <td style="color:var(--text-secondary);">Custom Blocked Websites</td>
            <td>{childStatus.websitePolicy.blocklist.length} domain(s)</td>
          </tr>
          <tr>
            <td style="color:var(--text-secondary);">Parent Contact Note</td>
            <td style="font-weight:500;">{childStatus.parentContact || 'Talk to your parent if you need extra time'}</td>
          </tr>
          <tr>
            <td style="color:var(--text-secondary);">Last LAN Synchronization</td>
            <td style="font-size:11.5px; color:var(--text-secondary);">{formatTimestamp(childStatus.lastSyncAt)}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">Restricted & Scheduled Applications</div>
          <div class="card-subtitle">Active application rules on this device</div>
        </div>
      </div>

      {#if childStatus.appRules.length === 0}
        <div class="empty-state">All standard applications are currently permitted within screen-time hours.</div>
      {:else}
        <table class="data-table">
          <thead>
            <tr>
              <th>Application</th>
              <th>Status</th>
              <th>Schedule</th>
            </tr>
          </thead>
          <tbody>
            {#each childStatus.appRules as r}
              <tr>
                <td style="font-weight:500;">{r.displayName}</td>
                <td>
                  <StatusBadge
                    variant={r.status === 'blocked' ? 'error' : r.status === 'scheduled' ? 'warning' : 'success'}
                    label={r.status}
                  />
                </td>
                <td class="mono" style="font-size:11px; color:var(--text-secondary);">
                  {r.status === 'scheduled' ? `${r.dailyLimitMinutes}m · ${r.allowedHours}` : '—'}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  </div>

  <div class="card">
    <div class="row-between">
      <div style="font-size:12px; color:var(--text-secondary);">
        Running in System Tray · Device ID: <span class="mono">{childStatus.deviceId}</span>
      </div>
      <div class="row-inline">
        <button class="btn btn-sm" on:click={() => (showPairingPanel = !showPairingPanel)}>
          {childStatus.connectedToLan ? 'Update Parent Pairing' : 'Pair with Parent PC'}
        </button>
        <button class="btn btn-sm" on:click={() => (showParentUnlock = !showParentUnlock)}>
          Parent Administration
        </button>
      </div>
    </div>

    {#if showPairingPanel}
      <div style="margin-top:12px; padding-top:12px; border-top:1px solid var(--border-subtle);">
        {#if discoveredPeers.filter((p) => p.mode === 'parent').length > 0}
          <div style="margin-bottom:8px; font-size:11.5px; color:var(--text-secondary);">
            Discovered Parent PCs on LAN:
            {#each discoveredPeers.filter((p) => p.mode === 'parent') as peer}
              <button
                class="btn btn-sm"
                style="margin-left:6px;"
                on:click={() => (parentEndpoint = `${peer.ipAddress}:${peer.tlsPort}`)}
              >
                {peer.deviceName} ({peer.ipAddress}:{peer.tlsPort})
              </button>
            {/each}
          </div>
        {/if}
        <div class="row-inline">
          <input class="input mono" bind:value={parentEndpoint} placeholder="Parent IP:Port (e.g. 192.168.1.10:41843)" />
          <input class="input mono" style="width:180px;" bind:value={pairingCode} placeholder="Code (XXXX-XXXX)" />
          <button class="btn btn-primary" on:click={handlePair}>Connect & Pair</button>
        </div>
      </div>
    {/if}

    {#if showParentUnlock}
      <div style="margin-top:12px; padding-top:12px; border-top:1px solid var(--border-subtle);" class="row-inline">
        <input
          type="password"
          class="input"
          bind:value={unlockPassword}
          placeholder="Enter Parent Master Password to open Parent Mode"
        />
        <button class="btn btn-primary" on:click={handleSwitchToParent}>Unlock Parent Dashboard</button>
      </div>
    {/if}
  </div>
</div>

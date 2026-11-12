<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type InstalledApp, type AppRule, type AppUsageEntry } from '../services/api';
  import { selectedDeviceId, formatMinutes, showToast } from '../stores/appStore';
  import StatusBadge from '../components/StatusBadge.svelte';

  let installedApps: InstalledApp[] = [];
  let rules: AppRule[] = [];
  let recentUsage: AppUsageEntry[] = [];
  let searchQuery = '';
  let scanning = false;

  // Custom rule form
  let customExe = '';
  let customDisplayName = '';
  let customStatus: 'allowed' | 'blocked' | 'scheduled' = 'blocked';
  let customLimitMin = 60;
  let customHours = '15:00-19:00';

  async function loadApps(devId: string) {
    try {
      const res = await api.getApplicationsBundle(devId);
      installedApps = res.installedApps || [];
      rules = res.rules || [];
      recentUsage = res.recentUsage || [];
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  $: if ($selectedDeviceId !== undefined) {
    loadApps($selectedDeviceId);
  }

  onMount(() => {
    loadApps($selectedDeviceId);
  });

  async function handleRescan() {
    scanning = true;
    try {
      installedApps = await api.refreshInstalledApplications($selectedDeviceId);
      showToast('Scanned Windows Registry for installed applications.', 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    } finally {
      scanning = false;
    }
  }

  async function updateAppPolicy(app: InstalledApp, status: 'allowed' | 'blocked' | 'scheduled') {
    try {
      await api.saveApplicationRule({
        deviceId: $selectedDeviceId,
        executableName: app.executableName,
        displayName: app.displayName,
        publisher: app.publisher,
        installPath: app.installPath,
        status,
        dailyLimitMinutes: status === 'scheduled' ? 60 : 0,
        allowedHours: status === 'scheduled' ? '15:00-19:00' : '',
      });
      await loadApps($selectedDeviceId);
      showToast(`${app.displayName} set to ${status}.`, 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleAddCustomRule() {
    if (!customExe.trim()) {
      showToast('Enter an executable name (e.g. steam.exe).', 'error');
      return;
    }
    try {
      await api.saveApplicationRule({
        deviceId: $selectedDeviceId,
        executableName: customExe.trim(),
        displayName: customDisplayName.trim() || customExe.trim(),
        status: customStatus,
        dailyLimitMinutes: customStatus === 'scheduled' ? customLimitMin : 0,
        allowedHours: customStatus === 'scheduled' ? customHours : '',
      });
      customExe = '';
      customDisplayName = '';
      await loadApps($selectedDeviceId);
      showToast('Application control rule saved.', 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleRemoveRule(exeName: string) {
    try {
      await api.deleteApplicationRule($selectedDeviceId, exeName);
      await loadApps($selectedDeviceId);
      showToast('Rule removed.', 'info');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleAcknowledgeNew(exeName: string) {
    try {
      await api.acknowledgeNewlyInstalledApp($selectedDeviceId, exeName);
      await loadApps($selectedDeviceId);
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  $: filteredInstalled = installedApps.filter(
    (a) =>
      !searchQuery.trim() ||
      a.displayName.toLowerCase().includes(searchQuery.toLowerCase()) ||
      a.executableName.toLowerCase().includes(searchQuery.toLowerCase())
  );
</script>

<div class="notice-box">
  <span>
    <strong>System Process Protection Guard:</strong> Critical Windows processes (<code class="mono">csrss.exe</code>, <code class="mono">svchost.exe</code>, <code class="mono">lsass.exe</code>, <code class="mono">explorer.exe</code>, <code class="mono">dwm.exe</code>) are permanently allowlisted and can never be terminated.
  </span>
  <button class="btn btn-sm" disabled={scanning} on:click={handleRescan}>
    {scanning ? 'Scanning Registry...' : 'Rescan Installed Software'}
  </button>
</div>

<div class="grid-2">
  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Add or Configure Application Rule</div>
        <div class="card-subtitle">Block specific executables or restrict them to scheduled hours / daily time limits</div>
      </div>
    </div>

    <div class="grid-2" style="margin-bottom:10px;">
      <div class="field-group">
        <label class="field-label" for="rule-exe">Executable Filename</label>
        <input id="rule-exe" class="input mono" bind:value={customExe} placeholder="e.g. robloxplayerbeta.exe" />
      </div>
      <div class="field-group">
        <label class="field-label" for="rule-name">Friendly Application Name</label>
        <input id="rule-name" class="input" bind:value={customDisplayName} placeholder="e.g. Roblox" />
      </div>
    </div>

    <div class="grid-3" style="margin-bottom:10px;">
      <div class="field-group">
        <label class="field-label" for="rule-status">Policy Action</label>
        <select id="rule-status" class="select" bind:value={customStatus}>
          <option value="blocked">Block Always</option>
          <option value="scheduled">Schedule & Time Limit</option>
          <option value="allowed">Explicitly Allow</option>
        </select>
      </div>
      <div class="field-group">
        <label class="field-label" for="rule-limit">Daily Limit (min)</label>
        <input
          id="rule-limit"
          type="number"
          min="0"
          max="480"
          class="input"
          disabled={customStatus !== 'scheduled'}
          bind:value={customLimitMin}
        />
      </div>
      <div class="field-group">
        <label class="field-label" for="rule-hours">Allowed Window</label>
        <input
          id="rule-hours"
          class="input mono"
          disabled={customStatus !== 'scheduled'}
          bind:value={customHours}
          placeholder="15:00-19:00"
        />
      </div>
    </div>

    <div style="text-align:right;">
      <button class="btn btn-primary" on:click={handleAddCustomRule}>Save Application Rule</button>
    </div>
  </div>

  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Active Application Policies ({rules.length})</div>
        <div class="card-subtitle">Configured blocks, allow rules, and application schedules</div>
      </div>
    </div>

    {#if rules.length === 0}
      <div class="empty-state">No custom application restrictions configured yet.</div>
    {:else}
      <table class="data-table">
        <thead>
          <tr>
            <th>Application</th>
            <th>Rule</th>
            <th>Schedule / Limit</th>
            <th>Used Today</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each rules as rule}
            <tr>
              <td>
                <div style="font-weight:600;">{rule.displayName}</div>
                <div class="mono" style="font-size:10.5px; color:var(--text-secondary);">{rule.executableName}</div>
              </td>
              <td>
                <StatusBadge
                  variant={rule.status === 'blocked'
                    ? 'error'
                    : rule.status === 'scheduled'
                      ? 'warning'
                      : 'success'}
                  label={rule.status}
                />
              </td>
              <td class="mono" style="font-size:11px; color:var(--text-secondary);">
                {rule.status === 'scheduled'
                  ? `${rule.dailyLimitMinutes}m/day · ${rule.allowedHours || 'All day'}`
                  : '—'}
              </td>
              <td>{formatMinutes(rule.usedTodayMinutes)}</td>
              <td style="text-align:right;">
                <button class="btn btn-sm btn-danger" on:click={() => handleRemoveRule(rule.executableName)}>Remove</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

<div class="card">
  <div class="card-header">
    <div>
      <div class="card-title">Installed Applications ({installedApps.length})</div>
      <div class="card-subtitle">Discovered from Windows Registry Uninstall keys · Review newly installed software</div>
    </div>
    <input
      class="input"
      style="width:240px;"
      bind:value={searchQuery}
      placeholder="Filter installed software..."
    />
  </div>

  {#if filteredInstalled.length === 0}
    <div class="empty-state">No matching installed applications found.</div>
  {:else}
    <table class="data-table">
      <thead>
        <tr>
          <th>Application Name</th>
          <th>Executable</th>
          <th>Publisher & Version</th>
          <th>Status</th>
          <th>Quick Policy</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredInstalled.slice(0, 50) as app}
          <tr>
            <td>
              <div class="row-inline">
                <span style="font-weight:600;">{app.displayName}</span>
                {#if app.newlyInstalled}
                  <StatusBadge variant="warning" label="Newly Installed" />
                {/if}
              </div>
            </td>
            <td class="mono" style="color:var(--text-secondary);">{app.executableName}</td>
            <td style="color:var(--text-secondary); font-size:11.5px;">
              {app.publisher || 'Unknown'} {app.version ? `· v${app.version}` : ''}
            </td>
            <td>
              <StatusBadge
                variant={app.ruleStatus === 'blocked'
                  ? 'error'
                  : app.ruleStatus === 'scheduled'
                    ? 'warning'
                    : 'success'}
                label={app.ruleStatus}
              />
            </td>
            <td>
              <div class="row-inline">
                <button class="btn btn-sm" on:click={() => updateAppPolicy(app, 'allowed')}>Allow</button>
                <button class="btn btn-sm" on:click={() => updateAppPolicy(app, 'scheduled')}>Schedule</button>
                <button class="btn btn-sm btn-danger" on:click={() => updateAppPolicy(app, 'blocked')}>Block</button>
                {#if app.newlyInstalled}
                  <button class="btn btn-sm" on:click={() => handleAcknowledgeNew(app.executableName)}>Approve New</button>
                {/if}
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

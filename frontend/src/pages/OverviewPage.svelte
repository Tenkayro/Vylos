<script lang="ts">
  import { activeSection, selectedDeviceId, formatMinutes, formatTimestamp } from '../stores/appStore';
  import type { BootstrapState } from '../services/api';
  import StatCard from '../components/StatCard.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';

  export let state: BootstrapState;
  export let onQuickLockToggle: (deviceId: string, lock: boolean) => void;
</script>

<div class="grid-4">
  <StatCard
    label="Connected Children"
    value={state.overview.connectedChildrenCount}
    meta="{state.overview.onlineDevicesCount} of {state.overview.totalDevicesCount} paired PCs online"
  />
  <StatCard
    label="Today's Computer Usage"
    value={formatMinutes(state.overview.todayTotalMinutes)}
    meta="{state.overview.blockedAttemptsToday} restricted launches today"
  />
  <StatCard
    label="Website & DNS Filter"
    value="Cloudflare 1.1.1.3"
    meta="{state.overview.blockedDomainsCount} blocked domain events"
    tone="success"
  />
  <StatCard
    label="Security & Integrity"
    value={state.overview.securityHealthy ? 'Protected' : 'Check Needed'}
    meta="TLS 1.3 + Certificate Pinning"
    tone={state.overview.securityHealthy ? 'success' : 'warning'}
  />
</div>

<div class="grid-2">
  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Paired Child Devices & Screen Time</div>
        <div class="card-subtitle">Direct LAN coordination — rules enforced locally when offline</div>
      </div>
      <button class="btn btn-sm" on:click={() => activeSection.set('devices')}>Manage Devices</button>
    </div>

    {#if state.devices.length === 0}
      <div class="empty-state">
        No child computers paired yet. Open <strong>Devices</strong> to generate a one-time LAN pairing code.
      </div>
    {:else}
      <table class="data-table">
        <thead>
          <tr>
            <th>Device / Child</th>
            <th>Status</th>
            <th>Used / Remaining</th>
            <th>Active App</th>
            <th>Quick Action</th>
          </tr>
        </thead>
        <tbody>
          {#each state.devices as dev}
            <tr>
              <td>
                <div style="font-weight:600;">{dev.name}</div>
                <div style="font-size:11px; color:var(--text-secondary);">
                  {dev.childName || 'Unassigned'} · {dev.ipAddress || 'LAN'}
                </div>
              </td>
              <td>
                <StatusBadge
                  variant={dev.status === 'online' ? 'success' : 'neutral'}
                  label={dev.status === 'online' ? 'Online' : 'Offline'}
                />
              </td>
              <td>
                <span>{formatMinutes(dev.usedTodayMinutes)}</span>
                <span style="color:var(--text-secondary);"> / {formatMinutes(dev.remainingMinutes)} left</span>
              </td>
              <td style="color:var(--text-secondary);">{dev.activeApp || 'Idle'}</td>
              <td>
                <button
                  class="btn btn-sm {dev.screenLocked ? 'btn-danger' : ''}"
                  on:click={() => onQuickLockToggle(dev.id, !dev.screenLocked)}
                >
                  {dev.screenLocked ? 'Unlock PC' : 'Lock Screen'}
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Recently Active Applications</div>
        <div class="card-subtitle">Today's foreground usage duration across monitored devices</div>
      </div>
      <button class="btn btn-sm" on:click={() => activeSection.set('applications')}>App Rules</button>
    </div>

    {#if state.overview.recentApps.length === 0}
      <div class="empty-state">No application usage recorded yet today.</div>
    {:else}
      <table class="data-table">
        <thead>
          <tr>
            <th>Application</th>
            <th>Category</th>
            <th>Duration</th>
          </tr>
        </thead>
        <tbody>
          {#each state.overview.recentApps.slice(0, 6) as app}
            <tr>
              <td>
                <div style="font-weight:500;">{app.displayName}</div>
                <div class="mono" style="font-size:10.5px; color:var(--text-secondary);">{app.executableName}</div>
              </td>
              <td><span class="badge badge-neutral">{app.category}</span></td>
              <td style="font-weight:600;">{formatMinutes(app.totalMinutes)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

<div class="grid-2">
  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Blocked Websites & Domain Filter History</div>
        <div class="card-subtitle">Cloudflare Family DNS + Local Domain Blocklist</div>
      </div>
      <button class="btn btn-sm" on:click={() => activeSection.set('websites')}>Configure DNS</button>
    </div>

    {#if state.overview.recentBlockedDomains.length === 0}
      <div class="empty-state">No blocked domain attempts recorded recently.</div>
    {:else}
      <table class="data-table">
        <thead>
          <tr>
            <th>Domain</th>
            <th>Category</th>
            <th>Time</th>
          </tr>
        </thead>
        <tbody>
          {#each state.overview.recentBlockedDomains.slice(0, 5) as bd}
            <tr>
              <td class="mono">{bd.domain}</td>
              <td><StatusBadge variant="warning" label={bd.category} /></td>
              <td style="color:var(--text-secondary); font-size:11.5px;">{formatTimestamp(bd.occurredAt)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Important Security & System Notifications</div>
        <div class="card-subtitle">Audit log of pairing, policy changes, and tamper checks</div>
      </div>
      <button class="btn btn-sm" on:click={() => activeSection.set('protection')}>Protection View</button>
    </div>

    {#if state.overview.notifications.length === 0}
      <div class="empty-state">All security checks nominal. No warnings recorded.</div>
    {:else}
      <table class="data-table">
        <thead>
          <tr>
            <th>Severity</th>
            <th>Event</th>
            <th>Time</th>
          </tr>
        </thead>
        <tbody>
          {#each state.overview.notifications.slice(0, 5) as note}
            <tr>
              <td>
                <StatusBadge
                  variant={note.severity === 'critical'
                    ? 'error'
                    : note.severity === 'warning'
                      ? 'warning'
                      : 'success'}
                  label={note.severity}
                />
              </td>
              <td>
                <div>{note.message}</div>
                {#if note.details}
                  <div style="font-size:11px; color:var(--text-secondary);">{note.details}</div>
                {/if}
              </td>
              <td style="color:var(--text-secondary); font-size:11.5px;">{formatTimestamp(note.occurredAt)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

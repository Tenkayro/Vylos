<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type DailySummary, type ActivityEvent } from '../services/api';
  import { selectedDeviceId, formatMinutes, formatTimestamp, showToast } from '../stores/appStore';
  import StatCard from '../components/StatCard.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';

  let daily: DailySummary | null = null;
  let history7Days: DailySummary[] = [];
  let recentEvents: ActivityEvent[] = [];
  let selectedDate = new Date().toISOString().slice(0, 10);
  let loading = false;

  async function loadActivity(devId: string, dateStr: string) {
    loading = true;
    try {
      const res = await api.getActivityBundle(devId, dateStr);
      daily = res.dailySummary;
      history7Days = res.history7Days || [];
      recentEvents = res.recentEvents || [];
    } catch (e: any) {
      showToast(e.message, 'error');
    } finally {
      loading = false;
    }
  }

  $: if ($selectedDeviceId !== undefined) {
    loadActivity($selectedDeviceId, selectedDate);
  }

  onMount(() => {
    loadActivity($selectedDeviceId, selectedDate);
  });
</script>

<div class="notice-box">
  <div>
    <strong>Transparent Activity Monitoring:</strong> FamilyControl logs computer login/logout sessions and active application duration. It never performs keylogging, covert screen capture, microphone recording, or hidden surveillance.
  </div>
  <div class="row-inline">
    <label for="act-date" style="font-size:11.5px;">Date:</label>
    <input
      id="act-date"
      type="date"
      class="input"
      style="width:145px; padding:4px 8px;"
      bind:value={selectedDate}
      on:change={() => loadActivity($selectedDeviceId, selectedDate)}
    />
  </div>
</div>

{#if daily}
  <div class="grid-3">
    <StatCard label="Screen Usage ({daily.dateStr})" value={formatMinutes(daily.totalMinutes)} meta="{daily.totalSeconds} seconds active" />
    <StatCard label="Session Logins" value={daily.loginCount} meta="Recorded Windows session starts" />
    <StatCard
      label="Blocked Policy Attempts"
      value={daily.blockedAttempts}
      meta="Restricted app launches prevented"
      tone={daily.blockedAttempts > 0 ? 'warning' : 'default'}
    />
  </div>
{/if}

<div class="grid-2">
  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">7-Day Daily Usage Summary</div>
        <div class="card-subtitle">Historical computer usage duration</div>
      </div>
    </div>

    <div style="display:flex; flex-direction:column; gap:8px;">
      {#each history7Days as day}
        {@const pct = Math.min(100, Math.round((day.totalMinutes / 240) * 100))}
        <div>
          <div class="row-between" style="font-size:11.5px; margin-bottom:3px;">
            <span class="mono">{day.dateStr}</span>
            <span style="font-weight:600;">{formatMinutes(day.totalMinutes)}</span>
          </div>
          <div class="progress-track">
            <div class="progress-fill" style="width:{pct}%;"></div>
          </div>
        </div>
      {/each}
    </div>
  </div>

  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Application Usage Duration ({selectedDate})</div>
        <div class="card-subtitle">Active foreground application time</div>
      </div>
    </div>

    {#if !daily || daily.topApps.length === 0}
      <div class="empty-state">No application activity recorded for {selectedDate}.</div>
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
          {#each daily.topApps as app}
            <tr>
              <td>
                <div style="font-weight:600;">{app.displayName}</div>
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

<div class="card">
  <div class="card-header">
    <div>
      <div class="card-title">Session & Activity Event Log</div>
      <div class="card-subtitle">Login/logout events, foreground focus summaries, and rule enforcement events</div>
    </div>
    {#if loading}
      <span style="font-size:11px; color:var(--text-secondary);">Refreshing...</span>
    {/if}
  </div>

  {#if recentEvents.length === 0}
    <div class="empty-state">No activity events logged yet.</div>
  {:else}
    <table class="data-table">
      <thead>
        <tr>
          <th>Event Type</th>
          <th>Application / Session</th>
          <th>Details</th>
          <th>Duration</th>
          <th>Timestamp</th>
        </tr>
      </thead>
      <tbody>
        {#each recentEvents as ev}
          <tr>
            <td>
              <StatusBadge
                variant={ev.eventType === 'app_blocked'
                  ? 'warning'
                  : ev.eventType === 'login'
                    ? 'success'
                    : 'neutral'}
                label={ev.eventType}
              />
            </td>
            <td>
              <span style="font-weight:500;">{ev.appName}</span>
              <span class="mono" style="font-size:11px; color:var(--text-secondary);"> ({ev.executableName})</span>
            </td>
            <td style="color:var(--text-secondary); font-size:12px;">{ev.windowTitle || '—'}</td>
            <td>{ev.durationSeconds > 0 ? `${Math.ceil(ev.durationSeconds / 60)}m` : '—'}</td>
            <td style="color:var(--text-secondary); font-size:11.5px;">{formatTimestamp(ev.occurredAt)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

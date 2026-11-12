<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type ScreenTimePolicy, type ScreenTimeEvaluation } from '../services/api';
  import { selectedDeviceId, formatMinutes, showToast } from '../stores/appStore';
  import StatCard from '../components/StatCard.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';

  let policy: ScreenTimePolicy | null = null;
  let evalState: ScreenTimeEvaluation | null = null;
  let saving = false;

  const weekdays = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

  async function loadScreenTime(devId: string) {
    try {
      const res = await api.getScreenTimeBundle(devId);
      policy = res.policy;
      evalState = res.evaluation;
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  $: if ($selectedDeviceId !== undefined) {
    loadScreenTime($selectedDeviceId);
  }

  onMount(() => {
    loadScreenTime($selectedDeviceId);
  });

  async function saveChanges() {
    if (!policy) return;
    saving = true;
    try {
      policy = await api.saveScreenTimePolicy(policy);
      await loadScreenTime($selectedDeviceId);
      showToast('Screen time rules saved and queued for local enforcement.', 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    } finally {
      saving = false;
    }
  }

  async function toggleLock(lock: boolean, pause: boolean) {
    if (!policy) return;
    try {
      policy = await api.setDeviceLockAndPause(policy.deviceId, lock, pause);
      await loadScreenTime($selectedDeviceId);
      showToast(lock ? 'Device screen locked.' : pause ? 'Screen time paused.' : 'Device unlocked.', 'info');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function addBonus(mins: number) {
    if (!policy) return;
    try {
      policy = await api.grantBonusTime(policy.deviceId, mins);
      await loadScreenTime($selectedDeviceId);
      showToast(`Granted ${mins > 0 ? '+' : ''}${mins}m bonus time for today.`, 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }
</script>

{#if evalState && policy}
  <div class="grid-4">
    <StatCard
      label="Remaining Today"
      value={formatMinutes(evalState.remainingMinutes)}
      meta="{formatMinutes(evalState.usedTodayMinutes)} used of {formatMinutes(evalState.effectiveLimitMinutes)}"
      tone={evalState.remainingMinutes > 15 ? 'success' : 'warning'}
    />
    <StatCard
      label="Enforcement Status"
      value={evalState.allowedNow ? 'Permitted' : 'Restricted'}
      meta={evalState.blockReason || 'Within allowed schedule'}
      tone={evalState.allowedNow ? 'success' : 'error'}
    />
    <StatCard
      label="Bedtime Schedule"
      value="{policy.bedtimeStart} – {policy.bedtimeEnd}"
      meta={evalState.inBedtime ? 'Currently in Bedtime' : 'Overnight lock window'}
    />
    <StatCard
      label="Break Reminders"
      value="Every {policy.breakIntervalMinutes}m"
      meta="{policy.breakDurationMinutes}m rest break reminder"
    />
  </div>

  <div class="card">
    <div class="row-between">
      <div>
        <div class="card-title">Immediate Parent Controls (Local Enforcement)</div>
        <div class="card-subtitle">Pause usage, lock the child workstation immediately, or grant bonus time for today</div>
      </div>
      <div class="row-inline">
        <button class="btn btn-sm" on:click={() => addBonus(15)}>+15m Bonus</button>
        <button class="btn btn-sm" on:click={() => addBonus(30)}>+30m Bonus</button>
        <button
          class="btn btn-sm {policy.manualPause ? 'btn-primary' : ''}"
          on:click={() => toggleLock(policy!.screenLocked, !policy!.manualPause)}
        >
          {policy.manualPause ? 'Resume Usage' : 'Pause Screen Time'}
        </button>
        <button
          class="btn btn-sm {policy.screenLocked ? 'btn-danger' : ''}"
          on:click={() => toggleLock(!policy!.screenLocked, policy!.manualPause)}
        >
          {policy.screenLocked ? 'Unlock Workstation' : 'Lock Screen Now'}
        </button>
      </div>
    </div>
  </div>

  <div class="grid-2">
    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">Daily Usage Limits & Bedtime</div>
          <div class="card-subtitle">Enforced locally by the background service even when offline</div>
        </div>
      </div>

      <div class="grid-2" style="margin-bottom:12px;">
        <div class="field-group">
          <label class="field-label" for="st-daily">Weekday Limit (minutes/day)</label>
          <input id="st-daily" type="number" min="15" max="720" step="15" class="input" bind:value={policy.dailyLimitMinutes} />
          <span style="font-size:11px; color:var(--text-secondary);">{formatMinutes(policy.dailyLimitMinutes)} per Mon–Fri</span>
        </div>

        <div class="field-group">
          <label class="field-label" for="st-weekend">Weekend Limit (minutes/day)</label>
          <input id="st-weekend" type="number" min="15" max="720" step="15" class="input" bind:value={policy.weekendLimitMinutes} />
          <span style="font-size:11px; color:var(--text-secondary);">{formatMinutes(policy.weekendLimitMinutes)} per Sat–Sun</span>
        </div>
      </div>

      <div class="grid-2" style="margin-bottom:12px;">
        <div class="field-group">
          <label class="field-label" for="st-bed-start">Bedtime Starts</label>
          <input id="st-bed-start" type="time" class="input" bind:value={policy.bedtimeStart} />
        </div>
        <div class="field-group">
          <label class="field-label" for="st-bed-end">Bedtime Ends</label>
          <input id="st-bed-end" type="time" class="input" bind:value={policy.bedtimeEnd} />
        </div>
      </div>

      <div class="grid-2" style="margin-bottom:14px;">
        <div class="field-group">
          <label class="field-label" for="st-brk-int">Break Reminder Interval (min)</label>
          <input id="st-brk-int" type="number" min="15" max="180" step="5" class="input" bind:value={policy.breakIntervalMinutes} />
        </div>
        <div class="field-group">
          <label class="field-label" for="st-brk-dur">Break Duration (min)</label>
          <input id="st-brk-dur" type="number" min="2" max="60" class="input" bind:value={policy.breakDurationMinutes} />
        </div>
      </div>

      <div class="row-between">
        <StatusBadge variant="neutral" label="Bonus Today: +{policy.bonusMinutesToday}m" />
        <button class="btn btn-primary" disabled={saving} on:click={saveChanges}>
          {saving ? 'Saving...' : 'Save Screen Time Rules'}
        </button>
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">Weekly Allowed Time Windows</div>
          <div class="card-subtitle">Permitted computer hours for each day of the week</div>
        </div>
      </div>

      <table class="data-table">
        <thead>
          <tr>
            <th>Day</th>
            <th>Active</th>
            <th>From</th>
            <th>Until</th>
          </tr>
        </thead>
        <tbody>
          {#each weekdays as day}
            {#if policy.weeklySchedule[day]}
              <tr>
                <td style="font-weight:500;">{day}</td>
                <td>
                  <input type="checkbox" bind:checked={policy.weeklySchedule[day].enabled} />
                </td>
                <td>
                  <input
                    type="time"
                    class="input"
                    style="padding:3px 6px; width:105px;"
                    bind:value={policy.weeklySchedule[day].allowedStart}
                  />
                </td>
                <td>
                  <input
                    type="time"
                    class="input"
                    style="padding:3px 6px; width:105px;"
                    bind:value={policy.weeklySchedule[day].allowedEnd}
                  />
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    </div>
  </div>
{/if}

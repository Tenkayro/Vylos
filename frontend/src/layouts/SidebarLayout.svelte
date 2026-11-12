<script lang="ts">
  import { activeSection, selectedDeviceId, parentLocked, type NavSection } from '../stores/appStore';
  import type { Device, FamilyConfig } from '../services/api';
  import StatusBadge from '../components/StatusBadge.svelte';

  export let config: FamilyConfig;
  export let devices: Device[] = [];
  export let newlyInstalledCount: number = 0;
  export let onRefresh: () => void;

  const navItems: { id: NavSection; label: string }[] = [
    { id: 'overview', label: 'Overview' },
    { id: 'devices', label: 'Devices' },
    { id: 'activity', label: 'Activity' },
    { id: 'screen_time', label: 'Screen Time' },
    { id: 'applications', label: 'Applications' },
    { id: 'websites', label: 'Websites' },
    { id: 'protection', label: 'Protection' },
    { id: 'settings', label: 'Settings' },
  ];

  $: currentSectionLabel = navItems.find((i) => i.id === $activeSection)?.label || 'Overview';
</script>

<div class="app-shell">
  <aside class="sidebar">
    <div class="brand-header">
      <div class="brand-mark">V</div>
      <div>
        <div class="brand-title">Vylos</div>
        <div class="brand-subtitle">{config.familyName} · Family Control</div>
      </div>
    </div>

    <nav class="nav-list" aria-label="Main Navigation">
      {#each navItems as item}
        <button
          class="nav-item {$activeSection === item.id ? 'active' : ''}"
          on:click={() => activeSection.set(item.id)}
        >
          <span class="nav-item-left">
            <span>{item.label}</span>
          </span>
          {#if item.id === 'devices' && devices.length > 0}
            <span class="nav-badge">{devices.length}</span>
          {:else if item.id === 'applications' && newlyInstalledCount > 0}
            <span class="nav-badge" style="color:var(--warning); border-color:var(--warning);">{newlyInstalledCount} new</span>
          {/if}
        </button>
      {/each}
    </nav>

    <div class="sidebar-footer">
      <div class="row-between" style="font-size:11.5px; color:var(--text-secondary); padding:0 4px;">
        <span>LAN TLS 1.3</span>
        <StatusBadge variant="success" label="Port {config.listenPort}" />
      </div>
      <button class="btn btn-sm" style="width:100%;" on:click={() => parentLocked.set(true)}>
        Lock Dashboard
      </button>
    </div>
  </aside>

  <main class="workspace">
    <header class="topbar">
      <div class="topbar-title-group">
        <h1 class="topbar-title">{currentSectionLabel}</h1>
        <StatusBadge variant="neutral" label="Local Network Only" />
      </div>

      <div class="topbar-controls">
        <div class="row-inline">
          <label for="device-picker" style="font-size:11.5px; color:var(--text-secondary);">Target Device:</label>
          <select id="device-picker" class="select" style="width:210px; padding:4px 8px;" bind:value={$selectedDeviceId}>
            <option value={config.localDeviceId}>This PC ({config.localDeviceName})</option>
            {#each devices as d}
              <option value={d.id}>
                {d.name} {d.childName ? `(${d.childName})` : ''} — {d.status.toUpperCase()}
              </option>
            {/each}
          </select>
        </div>

        <button class="btn btn-sm" on:click={onRefresh}>Refresh</button>
      </div>
    </header>

    <div class="content-scroll">
      <slot />
    </div>
  </main>
</div>

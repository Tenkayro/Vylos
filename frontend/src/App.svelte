<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, type BootstrapState } from './services/api';
  import {
    activeSection,
    selectedDeviceId,
    bootstrapStore,
    sessionToken,
    parentLocked,
    toastMessage,
    showToast,
  } from './stores/appStore';

  import SetupWizard from './components/SetupWizard.svelte';
  import SidebarLayout from './layouts/SidebarLayout.svelte';
  import OverviewPage from './pages/OverviewPage.svelte';
  import DevicesPage from './pages/DevicesPage.svelte';
  import ActivityPage from './pages/ActivityPage.svelte';
  import ScreenTimePage from './pages/ScreenTimePage.svelte';
  import ApplicationsPage from './pages/ApplicationsPage.svelte';
  import WebsitesPage from './pages/WebsitesPage.svelte';
  import ProtectionPage from './pages/ProtectionPage.svelte';
  import SettingsPage from './pages/SettingsPage.svelte';
  import ChildModePage from './pages/ChildModePage.svelte';

  let state: BootstrapState | null = null;
  let loading = true;
  let loginPassword = '';
  let showRecoveryMode = false;
  let recoveryCodeInput = '';
  let recoveryNewPass = '';
  let newRecoveryGenerated = '';
  let pollInterval: ReturnType<typeof setInterval> | null = null;

  async function refreshState() {
    try {
      const res = await api.getBootstrapState();
      state = res;
      bootstrapStore.set(res);

      if (!$selectedDeviceId && res.config.localDeviceId) {
        if (res.config.mode === 'parent' && res.devices.length > 0) {
          selectedDeviceId.set(res.devices[0].id);
        } else {
          selectedDeviceId.set(res.config.localDeviceId);
        }
      }
    } catch (e: any) {
      showToast(e.message || 'Failed to load application state', 'error');
    } finally {
      loading = false;
    }
  }

  onMount(async () => {
    await refreshState();
    pollInterval = setInterval(refreshState, 15000);
  });

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
  });

  async function handleUnlockDashboard() {
    try {
      const sess = await api.loginParent(loginPassword);
      sessionToken.set(sess.token);
      parentLocked.set(false);
      loginPassword = '';
      showToast('Parent dashboard unlocked.', 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleEmergencyRecovery() {
    try {
      newRecoveryGenerated = await api.recoverParentPassword(recoveryCodeInput, recoveryNewPass);
      recoveryCodeInput = '';
      recoveryNewPass = '';
      showToast('Password reset via emergency recovery key!', 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleQuickLockToggle(deviceId: string, lock: boolean) {
    try {
      await api.setDeviceLockAndPause(deviceId, lock, false);
      await refreshState();
      showToast(lock ? 'Locked device workstation.' : 'Unlocked device workstation.', 'info');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }
</script>

{#if $toastMessage}
  <div
    style="position:fixed; bottom:16px; right:18px; z-index:9999; padding:9px 14px; border-radius:6px; font-size:12.5px; font-weight:500; background:var(--bg-elevated); border:1px solid {$toastMessage.type === 'error'
      ? 'var(--error)'
      : $toastMessage.type === 'success'
        ? 'var(--success)'
        : 'var(--border-focus)'}; color:var(--text-primary); box-shadow: 0 6px 20px rgba(0,0,0,0.45);"
  >
    {$toastMessage.text}
  </div>
{/if}

{#if loading && !state}
  <div style="display:flex; align-items:center; justify-content:center; height:100vh; color:var(--text-secondary);">
    Loading FamilyControl...
  </div>
{:else if state && (!state.config.setupCompleted || state.config.mode === 'unconfigured')}
  <SetupWizard
    discoveredPeers={state.discoveredPeers}
    localDeviceName={state.config.localDeviceName}
    onCompleted={(tok) => {
      if (tok) sessionToken.set(tok);
      refreshState();
    }}
  />
{:else if state && state.config.mode === 'child'}
  <ChildModePage
    childStatus={state.childStatus}
    discoveredPeers={state.discoveredPeers}
    onUpdated={refreshState}
  />
{:else if state && $parentLocked}
  <div style="display:flex; align-items:center; justify-content:center; height:100vh; padding:20px;">
    <div class="card" style="width:100%; max-width:420px; padding:20px 22px;">
      <div class="row-inline" style="margin-bottom:14px;">
        <div class="brand-mark">FC</div>
        <div>
          <div style="font-weight:600; font-size:14px;">Parent Dashboard Locked</div>
          <div style="font-size:11.5px; color:var(--text-secondary);">{state.config.familyName} · Argon2id Protected</div>
        </div>
      </div>

      {#if !showRecoveryMode}
        <div class="field-group" style="margin-bottom:12px;">
          <label class="field-label" for="unlock-pass">Parent Master Password</label>
          <input
            id="unlock-pass"
            type="password"
            class="input"
            bind:value={loginPassword}
            placeholder="Enter password"
            on:keydown={(e) => e.key === 'Enter' && handleUnlockDashboard()}
          />
        </div>
        <div class="row-between">
          <button class="btn btn-sm" on:click={() => (showRecoveryMode = true)}>Use Recovery Key</button>
          <button class="btn btn-primary" on:click={handleUnlockDashboard}>Unlock Dashboard</button>
        </div>
      {:else}
        <div style="display:flex; flex-direction:column; gap:10px;">
          <div class="field-group">
            <label class="field-label" for="rec-key">Emergency Recovery Key</label>
            <input id="rec-key" class="input mono" bind:value={recoveryCodeInput} placeholder="FC-XXXX-XXXX-XXXX-XXXX" />
          </div>
          <div class="field-group">
            <label class="field-label" for="rec-newpass">New Parent Password</label>
            <input id="rec-newpass" type="password" class="input" bind:value={recoveryNewPass} placeholder="Min. 8 characters" />
          </div>
          {#if newRecoveryGenerated}
            <div class="card mono" style="background:var(--bg-input); color:var(--accent); font-size:12px;">
              New Recovery Key (save this!):<br />
              <strong>{newRecoveryGenerated}</strong>
            </div>
          {/if}
          <div class="row-between">
            <button
              class="btn btn-sm"
              on:click={() => {
                showRecoveryMode = false;
                newRecoveryGenerated = '';
              }}
            >
              Back to Login
            </button>
            <button class="btn btn-primary" on:click={handleEmergencyRecovery}>Reset Password</button>
          </div>
        </div>
      {/if}
    </div>
  </div>
{:else if state}
  <SidebarLayout
    config={state.config}
    devices={state.devices}
    newlyInstalledCount={state.overview.newlyInstalledCount}
    onRefresh={refreshState}
  >
    {#if $activeSection === 'overview'}
      <OverviewPage {state} onQuickLockToggle={handleQuickLockToggle} />
    {:else if $activeSection === 'devices'}
      <DevicesPage {state} onStateChanged={refreshState} />
    {:else if $activeSection === 'activity'}
      <ActivityPage />
    {:else if $activeSection === 'screen_time'}
      <ScreenTimePage />
    {:else if $activeSection === 'applications'}
      <ApplicationsPage />
    {:else if $activeSection === 'websites'}
      <WebsitesPage />
    {:else if $activeSection === 'protection'}
      <ProtectionPage />
    {:else if $activeSection === 'settings'}
      <SettingsPage config={state.config} onConfigUpdated={refreshState} />
    {/if}
  </SidebarLayout>
{/if}

<script lang="ts">
  import { api, type FamilyConfig } from '../services/api';
  import { showToast } from '../stores/appStore';
  import StatusBadge from '../components/StatusBadge.svelte';

  export let config: FamilyConfig;
  export let onConfigUpdated: () => void;

  let familyName = config.familyName;
  let parentContact = config.parentContact;
  let retentionDays = config.retentionDays || 30;
  let listenPort = config.listenPort || 41843;

  // Password change
  let currentPassword = '';
  let newPassword = '';

  // Mode switch
  let modeSwitchPassword = '';
  let targetMode: 'parent' | 'child' = config.mode === 'parent' ? 'child' : 'parent';

  // Backup / restore
  let backupText = '';

  // Uninstall check
  let uninstallPassword = '';
  let uninstallAuthorized = false;

  async function saveGeneral() {
    try {
      await api.updateGeneralSettings(familyName, parentContact, retentionDays, listenPort);
      showToast('Family & network settings saved.', 'success');
      onConfigUpdated();
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handlePruneNow() {
    try {
      const removed = await api.pruneHistoricalData(retentionDays);
      showToast(`Pruned ${removed} expired historical record(s).`, 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleChangePassword() {
    if (newPassword.length < 8) {
      showToast('New password must be at least 8 characters.', 'error');
      return;
    }
    try {
      await api.changeParentPassword(currentPassword, newPassword);
      currentPassword = '';
      newPassword = '';
      showToast('Parent password updated (Argon2id).', 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleSwitchMode() {
    if (!modeSwitchPassword) {
      showToast('Enter parent password to switch operating mode.', 'error');
      return;
    }
    try {
      await api.switchOperatingMode(modeSwitchPassword, targetMode);
      modeSwitchPassword = '';
      showToast(`Switched computer to ${targetMode.toUpperCase()} mode.`, 'success');
      onConfigUpdated();
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleExportBackup() {
    try {
      backupText = await api.exportEncryptedBackup();
      showToast('Encrypted configuration backup generated.', 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleImportBackup() {
    if (!backupText.trim()) {
      showToast('Paste an encrypted backup string first.', 'error');
      return;
    }
    try {
      await api.importEncryptedBackup(backupText.trim());
      showToast('Configuration restored from backup.', 'success');
      onConfigUpdated();
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleAuthorizeUninstall() {
    try {
      uninstallAuthorized = await api.authorizeUninstall(uninstallPassword);
      if (uninstallAuthorized) {
        showToast('Uninstall authorized. You may now run the uninstaller or --uninstall-service.', 'success');
      }
    } catch (e: any) {
      uninstallAuthorized = false;
      showToast(e.message, 'error');
    }
  }
</script>

<div class="grid-2">
  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Family, Network & Data Retention Settings</div>
        <div class="card-subtitle">Configure LAN coordination port and automatic local SQLite pruning</div>
      </div>
    </div>

    <div style="display:flex; flex-direction:column; gap:10px;">
      <div class="field-group">
        <label class="field-label" for="set-fam">Family Name</label>
        <input id="set-fam" class="input" bind:value={familyName} />
      </div>

      <div class="field-group">
        <label class="field-label" for="set-contact">Parent Contact Information (Displayed in Child Mode)</label>
        <input id="set-contact" class="input" bind:value={parentContact} placeholder="e.g. Call Mom at 555-0199 or ask in person" />
      </div>

      <div class="grid-2">
        <div class="field-group">
          <label class="field-label" for="set-port">Parent TLS 1.3 Listen Port</label>
          <input id="set-port" type="number" class="input mono" bind:value={listenPort} />
        </div>
        <div class="field-group">
          <label class="field-label" for="set-ret">Activity Retention (Days)</label>
          <input id="set-ret" type="number" min="1" max="365" class="input" bind:value={retentionDays} />
        </div>
      </div>

      <div class="row-between" style="margin-top:4px;">
        <button class="btn btn-sm" on:click={handlePruneNow}>Prune Old Records Now</button>
        <button class="btn btn-primary" on:click={saveGeneral}>Save Settings</button>
      </div>
    </div>
  </div>

  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Parent Authentication (Argon2id) & Operating Mode</div>
        <div class="card-subtitle">Change parent master password or switch this PC between Parent and Child mode</div>
      </div>
    </div>

    <div style="display:flex; flex-direction:column; gap:10px; margin-bottom:14px; padding-bottom:12px; border-bottom:1px solid var(--border-subtle);">
      <div class="grid-2">
        <div class="field-group">
          <label class="field-label" for="chg-cur">Current Parent Password</label>
          <input id="chg-cur" type="password" class="input" bind:value={currentPassword} />
        </div>
        <div class="field-group">
          <label class="field-label" for="chg-new">New Parent Password (min 8 chars)</label>
          <input id="chg-new" type="password" class="input" bind:value={newPassword} />
        </div>
      </div>
      <div style="text-align:right;">
        <button class="btn btn-primary" on:click={handleChangePassword}>Update Password</button>
      </div>
    </div>

    <div class="row-between">
      <div class="row-inline">
        <select class="select" style="width:145px;" bind:value={targetMode}>
          <option value="parent">Parent Mode</option>
          <option value="child">Child Mode</option>
        </select>
        <input
          type="password"
          class="input"
          style="width:165px;"
          bind:value={modeSwitchPassword}
          placeholder="Parent password"
        />
      </div>
      <button class="btn" on:click={handleSwitchMode}>Switch Mode</button>
    </div>
  </div>
</div>

<div class="grid-2">
  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Encrypted Backup & Restore</div>
        <div class="card-subtitle">Export or restore family profiles, screen-time schedules, and website policies</div>
      </div>
      <div class="row-inline">
        <button class="btn btn-sm" on:click={handleExportBackup}>Export Encrypted Backup</button>
        <button class="btn btn-sm btn-primary" on:click={handleImportBackup}>Restore Backup</button>
      </div>
    </div>

    <textarea
      class="textarea mono"
      rows="4"
      bind:value={backupText}
      placeholder="Click 'Export Encrypted Backup' to generate an XChaCha20-Poly1305 encrypted backup blob, or paste a backup here to restore..."
    ></textarea>
  </div>

  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Uninstall Authorization & Application Info</div>
        <div class="card-subtitle">Parent authorization is required before removing the background service</div>
      </div>
      <StatusBadge variant="neutral" label="v1.0.0 · MIT License" />
    </div>

    <div class="row-inline" style="margin-bottom:10px;">
      <input
        type="password"
        class="input"
        bind:value={uninstallPassword}
        placeholder="Enter Parent Password to authorize uninstall"
      />
      <button class="btn btn-danger" on:click={handleAuthorizeUninstall}>Verify Uninstall Auth</button>
    </div>

    {#if uninstallAuthorized}
      <div class="card mono" style="background:var(--bg-input); font-size:11px; color:var(--success);">
        Uninstall Authorized. Run as Administrator:<br />
        FamilyControl.exe --uninstall-service --password "&lt;YOUR_PASSWORD&gt;"
      </div>
    {:else}
      <div style="font-size:11.5px; color:var(--text-secondary);">
        Device ID: <span class="mono">{config.localDeviceId}</span> · UDP Discovery Port: <span class="mono">{config.discoveryPort}</span>
      </div>
    {/if}
  </div>
</div>

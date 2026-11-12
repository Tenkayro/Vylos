<script lang="ts">
  import { api, type BootstrapState, type PairingTicket } from '../services/api';
  import { activeSection, selectedDeviceId, showToast, formatTimestamp } from '../stores/appStore';
  import StatusBadge from '../components/StatusBadge.svelte';

  export let state: BootstrapState;
  export let onStateChanged: () => void;

  let newChildName = '';
  let newChildAge = '10-13';
  let pairingChildId = '';
  let pairingDeviceHint = '';
  let activeTicket: PairingTicket | null = null;
  let editingDeviceId = '';
  let editingDeviceName = '';
  let editingDeviceChildId = '';
  let scanningLan = false;

  async function handleAddChild() {
    if (!newChildName.trim()) {
      showToast('Enter a child name.', 'error');
      return;
    }
    try {
      await api.addChild(newChildName.trim(), '#DCE9FF', newChildAge);
      newChildName = '';
      showToast('Child profile added.', 'success');
      onStateChanged();
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleRemoveChild(id: string) {
    try {
      await api.removeChild(id);
      showToast('Child profile removed.', 'info');
      onStateChanged();
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleGenerateCode() {
    try {
      activeTicket = await api.createPairingCode(pairingChildId, pairingDeviceHint);
      showToast('One-time pairing code generated (valid for 15 minutes).', 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleScanLAN() {
    scanningLan = true;
    try {
      await api.triggerDiscoveryBroadcast();
      onStateChanged();
      showToast('UDP LAN discovery broadcast sent.', 'info');
    } catch (e: any) {
      showToast(e.message, 'error');
    } finally {
      scanningLan = false;
    }
  }

  function startEditDevice(d: any) {
    editingDeviceId = d.id;
    editingDeviceName = d.name;
    editingDeviceChildId = d.childId;
  }

  async function saveEditDevice() {
    try {
      await api.renameDevice(editingDeviceId, editingDeviceName, editingDeviceChildId);
      editingDeviceId = '';
      showToast('Device updated.', 'success');
      onStateChanged();
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  async function handleRevokeDevice(deviceId: string) {
    try {
      await api.revokeDevice(deviceId);
      showToast('Device credentials revoked and removed.', 'info');
      onStateChanged();
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }
</script>

<div class="grid-2">
  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Pair a New Child Computer (Cryptographic LAN Pairing)</div>
        <div class="card-subtitle">Generates a one-time pairing code with TLS 1.3 certificate pinning</div>
      </div>
    </div>

    <div class="grid-2" style="margin-bottom:10px;">
      <div class="field-group">
        <label class="field-label" for="pair-child">Assign to Child</label>
        <select id="pair-child" class="select" bind:value={pairingChildId}>
          <option value="">Select child profile...</option>
          {#each state.children as c}
            <option value={c.id}>{c.name} ({c.ageGroup})</option>
          {/each}
        </select>
      </div>
      <div class="field-group">
        <label class="field-label" for="pair-hint">Device Name Hint</label>
        <input id="pair-hint" class="input" bind:value={pairingDeviceHint} placeholder="e.g. Bedroom Laptop" />
      </div>
    </div>

    <div class="row-between">
      <span style="font-size:11.5px; color:var(--text-secondary);">
        Parent Endpoint: <strong class="mono">{state.localLanIps[0] || '127.0.0.1'}:{state.config.listenPort}</strong>
      </span>
      <button class="btn btn-primary" on:click={handleGenerateCode}>Generate One-Time Pairing Code</button>
    </div>

    {#if activeTicket}
      <div class="card" style="margin-top:12px; background:var(--bg-input); border-color:var(--accent-border);">
        <div class="row-between" style="margin-bottom:6px;">
          <span style="font-size:11.5px; color:var(--text-secondary);">Enter this code on the Child PC within 15 minutes:</span>
          <StatusBadge variant="success" label="One-Time Code" />
        </div>
        <div class="mono" style="font-size:20px; font-weight:700; color:var(--accent); letter-spacing:0.08em; text-align:center; padding:6px 0;">
          {activeTicket.code}
        </div>
        <div class="mono" style="font-size:11px; color:var(--text-secondary); text-align:center;">
          Parent Endpoint: {activeTicket.parentEndpoint} · TLS SHA256: {activeTicket.parentCertFingerprint.slice(0, 20)}...
        </div>
      </div>
    {/if}
  </div>

  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Children Profiles</div>
        <div class="card-subtitle">Manage children and assign multiple devices per child</div>
      </div>
    </div>

    <div class="row-inline" style="margin-bottom:12px;">
      <input class="input" bind:value={newChildName} placeholder="Child name (e.g. Maya)" />
      <select class="select" style="width:130px;" bind:value={newChildAge}>
        <option value="6-9">Age 6–9</option>
        <option value="10-13">Age 10–13</option>
        <option value="14-17">Age 14–17</option>
      </select>
      <button class="btn btn-primary" on:click={handleAddChild}>Add Child</button>
    </div>

    {#if state.children.length === 0}
      <div class="empty-state">No child profiles added yet.</div>
    {:else}
      <table class="data-table">
        <thead>
          <tr>
            <th>Child Name</th>
            <th>Age Group</th>
            <th>Paired PCs</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each state.children as child}
            <tr>
              <td style="font-weight:600;">{child.name}</td>
              <td><span class="badge badge-neutral">{child.ageGroup} yrs</span></td>
              <td>{child.deviceCount} device(s)</td>
              <td style="text-align:right;">
                <button class="btn btn-sm btn-danger" on:click={() => handleRemoveChild(child.id)}>Remove</button>
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
      <div class="card-title">Paired Child Computers ({state.devices.length})</div>
      <div class="card-subtitle">Mutual cryptographic authentication · Never relies on IP address alone</div>
    </div>
    <button class="btn btn-sm" disabled={scanningLan} on:click={handleScanLAN}>
      {scanningLan ? 'Scanning LAN...' : 'Scan LAN (UDP Discovery)'}
    </button>
  </div>

  {#if state.devices.length === 0}
    <div class="empty-state">
      No child devices paired yet. Generate a pairing code above and enter it in Child Mode on the child's PC.
    </div>
  {:else}
    <table class="data-table">
      <thead>
        <tr>
          <th>Device Name</th>
          <th>Child</th>
          <th>Operating System</th>
          <th>LAN IP & Cert Pin</th>
          <th>Connection Status</th>
          <th>Last Connection</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each state.devices as dev}
          <tr>
            <td>
              {#if editingDeviceId === dev.id}
                <input class="input" bind:value={editingDeviceName} style="width:150px;" />
              {:else}
                <div style="font-weight:600;">{dev.name}</div>
                <div class="mono" style="font-size:10.5px; color:var(--text-secondary);">{dev.hostname}</div>
              {/if}
            </td>
            <td>
              {#if editingDeviceId === dev.id}
                <select class="select" bind:value={editingDeviceChildId} style="width:130px;">
                  <option value="">Unassigned</option>
                  {#each state.children as c}
                    <option value={c.id}>{c.name}</option>
                  {/each}
                </select>
              {:else}
                {dev.childName || 'Unassigned'}
              {/if}
            </td>
            <td>{dev.osVersion}</td>
            <td>
              <div class="mono">{dev.ipAddress || 'LAN'}</div>
              <div class="mono" style="font-size:10px; color:var(--text-secondary);">
                SHA256: {dev.certFingerprint ? dev.certFingerprint.slice(0, 14) + '…' : 'Pinned'}
              </div>
            </td>
            <td>
              <StatusBadge
                variant={dev.status === 'online' ? 'success' : 'neutral'}
                label={dev.status === 'online' ? 'Online' : 'Offline (Enforcing Locally)'}
              />
            </td>
            <td style="font-size:11.5px; color:var(--text-secondary);">{formatTimestamp(dev.lastSeen)}</td>
            <td>
              <div class="row-inline">
                {#if editingDeviceId === dev.id}
                  <button class="btn btn-sm btn-primary" on:click={saveEditDevice}>Save</button>
                  <button class="btn btn-sm" on:click={() => (editingDeviceId = '')}>Cancel</button>
                {:else}
                  <button class="btn btn-sm" on:click={() => startEditDevice(dev)}>Rename</button>
                  <button
                    class="btn btn-sm"
                    on:click={() => {
                      selectedDeviceId.set(dev.id);
                      activeSection.set('screen_time');
                    }}
                  >
                    Rules
                  </button>
                  <button class="btn btn-sm btn-danger" on:click={() => handleRevokeDevice(dev.id)}>Revoke</button>
                {/if}
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

{#if state.discoveredPeers.length > 0}
  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Discovered FamilyControl Agents on Local Network (UDP Broadcast)</div>
        <div class="card-subtitle">Discovery packets contain zero passwords or tokens</div>
      </div>
    </div>
    <table class="data-table">
      <thead>
        <tr>
          <th>Device Name</th>
          <th>Role</th>
          <th>LAN Endpoint</th>
          <th>OS</th>
          <th>TLS Fingerprint</th>
        </tr>
      </thead>
      <tbody>
        {#each state.discoveredPeers as peer}
          <tr>
            <td style="font-weight:600;">{peer.deviceName}</td>
            <td><StatusBadge variant="neutral" label={peer.mode} /></td>
            <td class="mono">{peer.ipAddress}:{peer.tlsPort}</td>
            <td>{peer.osVersion}</td>
            <td class="mono" style="color:var(--text-secondary);">{peer.certFingerprint.slice(0, 20)}...</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

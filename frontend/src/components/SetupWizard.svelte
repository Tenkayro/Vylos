<script lang="ts">
  import { api, type DiscoveryBeacon } from '../services/api';
  import { showToast } from '../stores/appStore';
  import StatusBadge from './StatusBadge.svelte';

  export let discoveredPeers: DiscoveryBeacon[] = [];
  export let localDeviceName: string = 'Windows-PC';
  export let onCompleted: (sessionToken?: string) => void;

  let step: 'role' | 'parent_form' | 'parent_recovery' | 'child_form' = 'role';
  let busy = false;

  // Parent Setup fields
  let familyName = 'Our Family';
  let parentPassword = '';
  let parentPasswordConfirm = '';
  let parentContact = 'Ask Mom or Dad in the living room';
  let firstChildName = 'Alex';
  let generatedRecoveryCode = '';
  let issuedSessionToken = '';

  // Child Setup fields (Set up by a Parent on the Child's PC)
  let childDeviceName = localDeviceName || 'Child-PC';
  let childAdminPassword = '';
  let childAdminPasswordConfirm = '';
  let parentEndpoint = '';
  let pairingCode = '';

  async function submitParentSetup() {
    if (parentPassword.length < 8) {
      showToast('Parent password must be at least 8 characters.', 'error');
      return;
    }
    if (parentPassword !== parentPasswordConfirm) {
      showToast('Passwords do not match.', 'error');
      return;
    }
    busy = true;
    try {
      const res = await api.setupParentMode(familyName, parentPassword, parentContact, firstChildName);
      generatedRecoveryCode = res.recoveryCode;
      issuedSessionToken = res.sessionToken;
      step = 'parent_recovery';
    } catch (e: any) {
      showToast(e.message || 'Failed to configure Parent Mode', 'error');
    } finally {
      busy = false;
    }
  }

  async function submitChildSetup() {
    if (childAdminPassword.length < 8) {
      showToast('Parent protection password must be at least 8 characters.', 'error');
      return;
    }
    if (childAdminPassword !== childAdminPasswordConfirm) {
      showToast('Parent passwords do not match.', 'error');
      return;
    }
    busy = true;
    try {
      await api.setupChildMode(childDeviceName, childAdminPassword, parentContact);
      if (parentEndpoint.trim() && pairingCode.trim()) {
        await api.pairWithParent(parentEndpoint.trim(), pairingCode.trim());
        showToast('Child PC configured by Parent and paired over TLS 1.3!', 'success');
      } else {
        showToast('Vylos Child Mode configured by Parent. Enforcing local rules.', 'success');
      }
      onCompleted();
    } catch (e: any) {
      showToast(e.message || 'Setup warning: ' + String(e), 'error');
      onCompleted();
    } finally {
      busy = false;
    }
  }

  function selectDiscoveredParent(peer: DiscoveryBeacon) {
    parentEndpoint = `${peer.ipAddress}:${peer.tlsPort}`;
  }
</script>

<div style="display:flex; align-items:center; justify-content:center; height:100vh; padding:20px; background:var(--bg-primary);">
  <div class="card" style="width:100%; max-width:580px; padding:22px 24px;">
    <div style="display:flex; align-items:center; gap:10px; margin-bottom:16px; padding-bottom:12px; border-bottom:1px solid var(--border-subtle);">
      <div class="brand-mark">V</div>
      <div>
        <div style="font-size:15px; font-weight:600;">Vylos Family Control — Initial Setup</div>
        <div style="font-size:11.5px; color:var(--text-secondary);">
          Unified Installer (Parent & Child Mode) · 100% Local LAN · TLS 1.3
        </div>
      </div>
    </div>

    {#if step === 'role'}
      <div style="margin-bottom:14px; color:var(--text-secondary); font-size:12.5px;">
        <strong>Parent Setup:</strong> Select how this Windows computer will operate in your household:
      </div>

      <div class="grid-2" style="margin-bottom:16px;">
        <button
          class="card"
          style="text-align:left; cursor:pointer; background:var(--bg-elevated);"
          on:click={() => (step = 'parent_form')}
        >
          <div class="row-between" style="margin-bottom:6px;">
            <span style="font-weight:600; color:var(--accent); font-size:13.5px;">Parent Mode</span>
            <StatusBadge variant="success" label="Parent PC" />
          </div>
          <p style="color:var(--text-secondary); font-size:12px;">
            Set up on the parent computer. Provides the Vylos dashboard for managing children, devices, screen time, applications, and websites over LAN.
          </p>
        </button>

        <button
          class="card"
          style="text-align:left; cursor:pointer; background:var(--bg-elevated);"
          on:click={() => (step = 'child_form')}
        >
          <div class="row-between" style="margin-bottom:6px;">
            <span style="font-weight:600; color:var(--text-primary); font-size:13.5px;">Child Mode</span>
            <StatusBadge variant="warning" label="Parent Setup Required" />
          </div>
          <p style="color:var(--text-secondary); font-size:12px;">
            Must be set up by a parent on the child's computer. Locks settings with a Parent Password, pairs with the Parent PC, and enforces rules in the background.
          </p>
        </button>
      </div>

      <div class="notice-box">
        <span>
          <strong>Unified Vylos MSI Package:</strong> Both Parent and Child computers use this same application. No cloud server, no subscriptions, and no hidden surveillance.
        </span>
      </div>

    {:else if step === 'parent_form'}
      <div style="display:flex; flex-direction:column; gap:11px;">
        <div class="field-group">
          <label class="field-label" for="fam-name">Family / Household Name</label>
          <input id="fam-name" class="input" bind:value={familyName} placeholder="e.g. Miller Family" />
        </div>

        <div class="grid-2">
          <div class="field-group">
            <label class="field-label" for="first-child">First Child Profile Name</label>
            <input id="first-child" class="input" bind:value={firstChildName} placeholder="e.g. Leo" />
          </div>
          <div class="field-group">
            <label class="field-label" for="parent-contact">Parent Contact Note (shown on Child PC)</label>
            <input id="parent-contact" class="input" bind:value={parentContact} placeholder="Talk to Mom or Dad" />
          </div>
        </div>

        <div class="grid-2">
          <div class="field-group">
            <label class="field-label" for="p-pass">Parent Master Password (Argon2id)</label>
            <input id="p-pass" type="password" class="input" bind:value={parentPassword} placeholder="Min. 8 characters" />
          </div>
          <div class="field-group">
            <label class="field-label" for="p-pass2">Confirm Password</label>
            <input id="p-pass2" type="password" class="input" bind:value={parentPasswordConfirm} placeholder="Repeat password" />
          </div>
        </div>

        <div class="row-between" style="margin-top:8px;">
          <button class="btn" on:click={() => (step = 'role')}>Back</button>
          <button class="btn btn-primary" disabled={busy} on:click={submitParentSetup}>
            {busy ? 'Initializing Cryptography...' : 'Initialize Vylos Parent Dashboard'}
          </button>
        </div>
      </div>

    {:else if step === 'parent_recovery'}
      <div style="display:flex; flex-direction:column; gap:12px;">
        <StatusBadge variant="warning" label="Save Your Emergency Recovery Key" />
        <p style="color:var(--text-secondary); font-size:12.5px;">
          Because Vylos operates strictly offline without cloud accounts, this one-time recovery key is the only way to reset a forgotten Parent password. Write it down and store it safely:
        </p>
        <div class="card mono" style="text-align:center; font-size:15px; font-weight:600; color:var(--accent); letter-spacing:0.06em; background:var(--bg-input);">
          {generatedRecoveryCode}
        </div>
        <div class="row-between" style="margin-top:6px;">
          <span style="font-size:11.5px; color:var(--text-secondary);">Hashed locally with Argon2id</span>
          <button class="btn btn-primary" on:click={() => onCompleted(issuedSessionToken)}>
            I Have Saved This Key — Open Vylos Dashboard
          </button>
        </div>
      </div>

    {:else if step === 'child_form'}
      <div style="display:flex; flex-direction:column; gap:11px;">
        <div class="notice-box" style="margin-bottom:2px;">
          <span>
            <strong>Parent Configuration Step:</strong> A parent must complete this setup on the child's computer. Once activated, the child cannot change rules or exit Child Mode without the Parent Password.
          </span>
        </div>

        <div class="field-group">
          <label class="field-label" for="c-devname">Child Computer Name</label>
          <input id="c-devname" class="input" bind:value={childDeviceName} />
        </div>

        <div class="grid-2">
          <div class="field-group">
            <label class="field-label" for="c-pass">Parent Protection Password (Argon2id)</label>
            <input id="c-pass" type="password" class="input" bind:value={childAdminPassword} placeholder="Min. 8 characters" />
          </div>
          <div class="field-group">
            <label class="field-label" for="c-pass2">Confirm Parent Password</label>
            <input id="c-pass2" type="password" class="input" bind:value={childAdminPasswordConfirm} placeholder="Repeat password" />
          </div>
        </div>

        <div class="card" style="background:var(--bg-input);">
          <div class="card-title" style="margin-bottom:6px;">Pair with Parent PC over LAN (Generate Code on Parent PC → Devices)</div>
          {#if discoveredPeers.filter(p => p.mode === 'parent').length > 0}
            <div style="margin-bottom:8px; font-size:11.5px; color:var(--text-secondary);">
              Discovered Vylos Parent PC on LAN:
              {#each discoveredPeers.filter(p => p.mode === 'parent') as peer}
                <button class="btn btn-sm" style="margin-left:6px;" on:click={() => selectDiscoveredParent(peer)}>
                  {peer.deviceName} ({peer.ipAddress}:{peer.tlsPort})
                </button>
              {/each}
            </div>
          {/if}
          <div class="grid-2">
            <div class="field-group">
              <label class="field-label" for="p-ep">Parent PC IP:Port</label>
              <input id="p-ep" class="input mono" bind:value={parentEndpoint} placeholder="e.g. 192.168.1.10:41843" />
            </div>
            <div class="field-group">
              <label class="field-label" for="p-code">One-Time Pairing Code</label>
              <input id="p-code" class="input mono" bind:value={pairingCode} placeholder="XXXX-XXXX" />
            </div>
          </div>
        </div>

        <div class="row-between" style="margin-top:6px;">
          <button class="btn" on:click={() => (step = 'role')}>Back</button>
          <button class="btn btn-primary" disabled={busy} on:click={submitChildSetup}>
            {busy ? 'Activating Child Agent...' : 'Lock & Activate Child Mode'}
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>

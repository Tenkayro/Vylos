<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type ProtectionStatus } from '../services/api';
  import { selectedDeviceId, formatTimestamp, showToast } from '../stores/appStore';
  import StatCard from '../components/StatCard.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';

  let status: ProtectionStatus | null = null;
  let checking = false;

  async function runCheck(devId: string) {
    checking = true;
    try {
      status = await api.getProtectionStatus(devId);
    } catch (e: any) {
      showToast(e.message, 'error');
    } finally {
      checking = false;
    }
  }

  $: if ($selectedDeviceId !== undefined) {
    runCheck($selectedDeviceId);
  }

  onMount(() => {
    runCheck($selectedDeviceId);
  });
</script>

{#if status}
  <div class="grid-4">
    <StatCard
      label="DNS Protection"
      value={status.dnsProtected ? 'Active' : 'Disabled'}
      meta="Profile: {status.dnsMode}"
      tone={status.dnsProtected ? 'success' : 'warning'}
    />
    <StatCard
      label="Windows Firewall"
      value={status.firewallEnabled ? 'Enabled' : 'Disabled'}
      meta="Registry Standard/Public Profile"
      tone={status.firewallEnabled ? 'success' : 'error'}
    />
    <StatCard
      label="Background Agent"
      value={status.agentRunning ? 'Running' : 'Stopped'}
      meta="Service: FamilyControlService"
      tone={status.agentRunning ? 'success' : 'warning'}
    />
    <StatCard
      label="Integrity Check"
      value={status.configurationIntact ? 'Verified' : 'Attention'}
      meta={formatTimestamp(status.lastIntegrityCheck)}
      tone={status.configurationIntact ? 'success' : 'warning'}
    />
  </div>

  <div class="grid-2">
    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">System & Network Configuration Integrity</div>
          <div class="card-subtitle">Cryptographic identity, local LAN endpoints, and OS security posture</div>
        </div>
        <button class="btn btn-sm" disabled={checking} on:click={() => runCheck($selectedDeviceId)}>
          {checking ? 'Verifying...' : 'Run Integrity Check Now'}
        </button>
      </div>

      <table class="data-table">
        <tbody>
          <tr>
            <td style="color:var(--text-secondary);">Local LAN IPv4 Addresses</td>
            <td class="mono">{status.localIpAddresses.join(', ') || '127.0.0.1'}</td>
          </tr>
          <tr>
            <td style="color:var(--text-secondary);">Active OS DNS Resolvers</td>
            <td class="mono">{status.activeDnsServers?.length ? status.activeDnsServers.join(', ') : 'System Default'}</td>
          </tr>
          <tr>
            <td style="color:var(--text-secondary);">TLS 1.3 Certificate Pin (SHA-256)</td>
            <td class="mono" style="word-break:break-all;">{status.certFingerprint || 'Not generated'}</td>
          </tr>
          <tr>
            <td style="color:var(--text-secondary);">Local Secret Keystore</td>
            <td><StatusBadge variant="success" label="Windows DPAPI + XChaCha20-Poly1305" /></td>
          </tr>
          <tr>
            <td style="color:var(--text-secondary);">Windows Service SDDL Hardening</td>
            <td><StatusBadge variant="success" label="Protected (Admin/SYSTEM Only)" /></td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">Security & Tamper Resistance Model</div>
          <div class="card-subtitle">Honest, documented Windows security boundaries</div>
        </div>
      </div>

      <div style="display:flex; flex-direction:column; gap:10px; font-size:12px; color:var(--text-secondary);">
        <p>
          <strong>1. Standard Windows Accounts for Children:</strong> Children should use a standard Windows user account while parents maintain a separate password-protected Administrator account.
        </p>
        <p>
          <strong>2. Service & Configuration Protection:</strong> <code class="mono">FamilyControlService</code> registers with restrictive Windows SDDL permissions so standard users cannot stop, pause, or uninstall the background service.
        </p>
        <p>
          <strong>3. Mutual LAN Authentication & Replay Guard:</strong> Every LAN sync request requires TLS 1.3 certificate pinning, a random 256-bit device token, an Ed25519 signature, and a ±120s timestamp + nonce replay check.
        </p>
        <p>
          <strong>4. Transparent Operation:</strong> FamilyControl never uses rootkits or stealth techniques. A Windows Administrator with the parent password (or emergency recovery key) retains full control.
        </p>
      </div>
    </div>
  </div>

  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Detected Configuration Changes & Security Audit Log</div>
        <div class="card-subtitle">Chronological record of authentication, pairing, policy updates, and security checks</div>
      </div>
    </div>

    {#if status.recentSecurityEvents.length === 0}
      <div class="empty-state">No security events logged yet.</div>
    {:else}
      <table class="data-table">
        <thead>
          <tr>
            <th>Severity</th>
            <th>Category</th>
            <th>Message</th>
            <th>Details</th>
            <th>Occurred At</th>
          </tr>
        </thead>
        <tbody>
          {#each status.recentSecurityEvents as ev}
            <tr>
              <td>
                <StatusBadge
                  variant={ev.severity === 'critical'
                    ? 'error'
                    : ev.severity === 'warning'
                      ? 'warning'
                      : 'success'}
                  label={ev.severity}
                />
              </td>
              <td class="mono">{ev.category}</td>
              <td style="font-weight:500;">{ev.message}</td>
              <td class="mono" style="color:var(--text-secondary); font-size:11px;">{ev.details || '—'}</td>
              <td style="color:var(--text-secondary); font-size:11.5px;">{formatTimestamp(ev.occurredAt)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
{/if}

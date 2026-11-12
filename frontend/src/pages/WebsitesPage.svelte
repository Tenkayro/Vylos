<script lang="ts">
  import { onMount } from 'svelte';
  import {
    api,
    type WebsitePolicy,
    type DNSVerificationStatus,
    type BlockedDomainEvent,
  } from '../services/api';
  import { selectedDeviceId, validateDomainInput, formatTimestamp, showToast } from '../stores/appStore';
  import StatCard from '../components/StatCard.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';

  let policy: WebsitePolicy | null = null;
  let verification: DNSVerificationStatus | null = null;
  let blockedEvents: BlockedDomainEvent[] = [];
  let newBlockDomain = '';
  let newAllowDomain = '';
  let saving = false;

  async function loadWebsites(devId: string) {
    try {
      const res = await api.getWebsitesBundle(devId);
      policy = res.policy;
      verification = res.verification;
      blockedEvents = res.blockedEvents || [];
    } catch (e: any) {
      showToast(e.message, 'error');
    }
  }

  $: if ($selectedDeviceId !== undefined) {
    loadWebsites($selectedDeviceId);
  }

  onMount(() => {
    loadWebsites($selectedDeviceId);
  });

  function selectDNSMode(mode: WebsitePolicy['dnsMode']) {
    if (!policy) return;
    policy.dnsMode = mode;
    if (mode === 'cloudflare_family') {
      policy.blockAdult = true;
      policy.blockMalware = true;
      policy.primaryIpv4 = '1.1.1.3';
      policy.secondaryIpv4 = '1.0.0.3';
      policy.primaryIpv6 = '2606:4700:4700::1113';
      policy.secondaryIpv6 = '2606:4700:4700::1003';
    } else if (mode === 'cloudflare_malware') {
      policy.blockAdult = false;
      policy.blockMalware = true;
      policy.primaryIpv4 = '1.1.1.2';
      policy.secondaryIpv4 = '1.0.0.2';
      policy.primaryIpv6 = '2606:4700:4700::1112';
      policy.secondaryIpv6 = '2606:4700:4700::1002';
    } else if (mode === 'off') {
      policy.blockAdult = false;
      policy.blockMalware = false;
    }
  }

  async function saveWebsitesPolicy() {
    if (!policy) return;
    saving = true;
    try {
      policy = await api.saveWebsitePolicy(policy);
      await loadWebsites($selectedDeviceId);
      showToast('Website & Cloudflare DNS policy saved.', 'success');
    } catch (e: any) {
      showToast(e.message, 'error');
    } finally {
      saving = false;
    }
  }

  async function addBlockDomain() {
    if (!policy) return;
    const valid = validateDomainInput(newBlockDomain);
    if (!valid) {
      showToast('Enter a valid domain name (e.g. example.com).', 'error');
      return;
    }
    if (!policy.blocklist.includes(valid)) {
      policy.blocklist = [...policy.blocklist, valid];
    }
    newBlockDomain = '';
    await saveWebsitesPolicy();
  }

  async function removeBlockDomain(domain: string) {
    if (!policy) return;
    policy.blocklist = policy.blocklist.filter((d) => d !== domain);
    await saveWebsitesPolicy();
  }

  async function addAllowDomain() {
    if (!policy) return;
    const valid = validateDomainInput(newAllowDomain);
    if (!valid) {
      showToast('Enter a valid domain name (e.g. khanacademy.org).', 'error');
      return;
    }
    if (!policy.allowlist.includes(valid)) {
      policy.allowlist = [...policy.allowlist, valid];
    }
    newAllowDomain = '';
    await saveWebsitesPolicy();
  }

  async function removeAllowDomain(domain: string) {
    if (!policy) return;
    policy.allowlist = policy.allowlist.filter((d) => d !== domain);
    await saveWebsitesPolicy();
  }

  async function handleEnforceNow() {
    try {
      verification = await api.enforceDnsAndHosts($selectedDeviceId);
      showToast('DNS & Hosts blocklist rules applied.', 'success');
    } catch (e: any) {
      showToast('Saved policy (' + e.message + ')', 'info');
    }
  }
</script>

{#if policy}
  <div class="notice-box">
    <span>
      <strong>Technical Transparency Note:</strong> {policy.transparencyNote}
    </span>
    <button class="btn btn-sm" on:click={handleEnforceNow}>Apply DNS & Hosts Now</button>
  </div>

  <div class="grid-4">
    <StatCard
      label="Cloudflare Family IPv4"
      value="{policy.primaryIpv4} / {policy.secondaryIpv4}"
      meta="Primary & Secondary DNS"
      tone={policy.dnsMode === 'cloudflare_family' ? 'success' : 'default'}
    />
    <StatCard
      label="Cloudflare Family IPv6"
      value="2606:4700:4700::1113"
      meta="Secondary: 2606:4700:4700::1003"
      tone={policy.dnsMode === 'cloudflare_family' ? 'success' : 'default'}
    />
    <StatCard
      label="Malware & Adult Filter"
      value={policy.blockAdult ? 'Malware + Adult' : policy.blockMalware ? 'Malware Only' : 'Custom / Off'}
      meta="Network-wide resolver filter"
      tone={policy.blockMalware ? 'success' : 'warning'}
    />
    <StatCard
      label="Custom Domain Rules"
      value="{policy.blocklist.length} Blocked"
      meta="{policy.allowlist.length} allowlisted domains"
    />
  </div>

  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Cloudflare 1.1.1.1 for Families DNS Configuration</div>
        <div class="card-subtitle">Select DNS resolver filtering profile for the target computer</div>
      </div>
      <button class="btn btn-primary" disabled={saving} on:click={saveWebsitesPolicy}>
        {saving ? 'Saving...' : 'Save DNS Configuration'}
      </button>
    </div>

    <div class="grid-3">
      <button
        class="card"
        style="text-align:left; cursor:pointer; background:{policy.dnsMode === 'cloudflare_family' ? 'var(--bg-elevated)' : 'var(--bg-input)'}; border-color:{policy.dnsMode === 'cloudflare_family' ? 'var(--accent)' : 'var(--border-subtle)'};"
        on:click={() => selectDNSMode('cloudflare_family')}
      >
        <div class="row-between" style="margin-bottom:4px;">
          <span style="font-weight:600;">Malware + Adult Content</span>
          <StatusBadge variant="success" label="Recommended" />
        </div>
        <div class="mono" style="font-size:11px; color:var(--accent); margin-bottom:4px;">
          IPv4: 1.1.1.3, 1.0.0.3<br />
          IPv6: 2606:4700:4700::1113, ::1003
        </div>
        <p style="font-size:11.5px; color:var(--text-secondary);">
          Blocks known malware, phishing domains, and adult content at the DNS layer.
        </p>
      </button>

      <button
        class="card"
        style="text-align:left; cursor:pointer; background:{policy.dnsMode === 'cloudflare_malware' ? 'var(--bg-elevated)' : 'var(--bg-input)'}; border-color:{policy.dnsMode === 'cloudflare_malware' ? 'var(--accent)' : 'var(--border-subtle)'};"
        on:click={() => selectDNSMode('cloudflare_malware')}
      >
        <div class="row-between" style="margin-bottom:4px;">
          <span style="font-weight:600;">Malware Filtering Only</span>
          <StatusBadge variant="neutral" label="Security" />
        </div>
        <div class="mono" style="font-size:11px; color:var(--accent); margin-bottom:4px;">
          IPv4: 1.1.1.2, 1.0.0.2<br />
          IPv6: 2606:4700:4700::1112, ::1002
        </div>
        <p style="font-size:11.5px; color:var(--text-secondary);">
          Blocks malware and phishing domains while leaving general content unrestricted.
        </p>
      </button>

      <button
        class="card"
        style="text-align:left; cursor:pointer; background:{policy.dnsMode === 'off' ? 'var(--bg-elevated)' : 'var(--bg-input)'}; border-color:{policy.dnsMode === 'off' ? 'var(--accent)' : 'var(--border-subtle)'};"
        on:click={() => selectDNSMode('off')}
      >
        <div class="row-between" style="margin-bottom:4px;">
          <span style="font-weight:600;">System Default DNS</span>
          <StatusBadge variant="warning" label="Unfiltered DNS" />
        </div>
        <div class="mono" style="font-size:11px; color:var(--text-secondary); margin-bottom:4px;">
          Router / ISP Resolver
        </div>
        <p style="font-size:11.5px; color:var(--text-secondary);">
          Leaves network adapter DNS unchanged; custom domain blocklist still applies via hosts.
        </p>
      </button>
    </div>
  </div>

  <div class="grid-2">
    <div class="card">
      <div class="card-header">
        <div>
          <div class="card-title">Custom Domain Blocklist ({policy.blocklist.length})</div>
          <div class="card-subtitle">Domains blocked locally via managed hosts sinkhole</div>
        </div>
      </div>

      <div class="row-inline" style="margin-bottom:10px;">
        <input class="input mono" bind:value={newBlockDomain} placeholder="e.g. tiktok.com" />
        <button class="btn btn-danger" on:click={addBlockDomain}>Block Domain</button>
      </div>

      {#if policy.blocklist.length === 0}
        <div class="empty-state">No custom blocked domains added.</div>
      {:else}
        <table class="data-table">
          <thead>
            <tr>
              <th>Blocked Domain</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each policy.blocklist as d}
              <tr>
                <td class="mono">{d}</td>
                <td style="text-align:right;">
                  <button class="btn btn-sm" on:click={() => removeBlockDomain(d)}>Remove</button>
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
          <div class="card-title">Domain Allowlist ({policy.allowlist.length})</div>
          <div class="card-subtitle">Explicitly trusted domains exempt from custom blocklist rules</div>
        </div>
      </div>

      <div class="row-inline" style="margin-bottom:10px;">
        <input class="input mono" bind:value={newAllowDomain} placeholder="e.g. wikipedia.org" />
        <button class="btn btn-primary" on:click={addAllowDomain}>Allow Domain</button>
      </div>

      {#if policy.allowlist.length === 0}
        <div class="empty-state">No custom allowlisted domains configured.</div>
      {:else}
        <table class="data-table">
          <thead>
            <tr>
              <th>Allowed Domain</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each policy.allowlist as d}
              <tr>
                <td class="mono">{d}</td>
                <td style="text-align:right;">
                  <button class="btn btn-sm" on:click={() => removeAllowDomain(d)}>Remove</button>
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
        <div class="card-title">Blocked-Domain History & DNS Monitoring</div>
        <div class="card-subtitle">
          {verification ? verification.statusSummary : 'Monitoring DNS configuration'}
        </div>
      </div>
    </div>

    {#if blockedEvents.length === 0}
      <div class="empty-state">No blocked domain attempts recorded yet.</div>
    {:else}
      <table class="data-table">
        <thead>
          <tr>
            <th>Domain</th>
            <th>Filter Category</th>
            <th>Reason</th>
            <th>Timestamp</th>
          </tr>
        </thead>
        <tbody>
          {#each blockedEvents as ev}
            <tr>
              <td class="mono">{ev.domain}</td>
              <td><StatusBadge variant="warning" label={ev.category} /></td>
              <td style="color:var(--text-secondary);">{ev.reason}</td>
              <td style="color:var(--text-secondary); font-size:11.5px;">{formatTimestamp(ev.occurredAt)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
{/if}

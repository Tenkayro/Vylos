<script lang="ts">
  export let label: string;
  export let value: string | number;
  export let meta: string = '';
  export let tone: 'default' | 'success' | 'warning' | 'error' = 'default';
</script>

<div class="stat-card">
  <div class="stat-label">{label}</div>
  <div
    class="stat-value"
    style={tone === 'success'
      ? 'color: var(--success)'
      : tone === 'warning'
        ? 'color: var(--warning)'
        : tone === 'error'
          ? 'color: var(--error)'
          : ''}
  >
    {value}
  </div>
  {#if meta}
    <div class="stat-meta">{meta}</div>
  {/if}
</div>

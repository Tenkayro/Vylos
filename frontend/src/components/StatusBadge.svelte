<script lang="ts">
  export let variant: 'success' | 'warning' | 'error' | 'neutral' = 'neutral';
  export let label: string = '';
</script>

<span class="badge badge-{variant}">
  <span class="badge-dot"></span>
  <span>{label}</span>
</span>

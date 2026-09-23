<script lang="ts">
  let { label, value, hint = '', tone = '' }: { label: string; value: string; hint?: string; tone?: string } = $props()
</script>

<div class="card stat {tone}">
  <div class="label">{label}</div>
  <div class="value">{value}</div>
  {#if hint}<div class="hint">{hint}</div>{/if}
</div>

<style>
  .stat {
    padding: 14px 16px;
  }
  .label {
    color: var(--muted);
    font-size: 13px;
  }
  .value {
    font-size: 24px;
    font-weight: 700;
    margin-top: 4px;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
    margin-top: 2px;
  }
  .ok .value {
    color: var(--ok);
  }
  .warn .value {
    color: var(--warn);
  }
  .danger .value {
    color: var(--danger);
  }
</style>

<script lang="ts">
  let {
    open,
    title,
    text,
    confirmLabel = 'Да',
    cancelLabel = 'Отмена',
    danger = false,
    onconfirm,
    oncancel,
  }: {
    open: boolean
    title: string
    text: string
    confirmLabel?: string
    cancelLabel?: string
    danger?: boolean
    onconfirm: () => void
    oncancel: () => void
  } = $props()
</script>

{#if open}
  <div class="backdrop" role="presentation">
    <div class="card dialog" role="dialog" aria-modal="true" aria-label={title}>
      <h3>{title}</h3>
      <p class="muted">{text}</p>
      <div class="row actions">
        <button onclick={oncancel}>{cancelLabel}</button>
        <button class={danger ? 'danger' : 'primary'} onclick={onconfirm}>{confirmLabel}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    display: grid;
    place-items: center;
    z-index: 10;
    padding: 16px;
  }
  .dialog {
    max-width: 440px;
    width: 100%;
  }
  .actions {
    justify-content: flex-end;
    margin-top: 16px;
  }
</style>

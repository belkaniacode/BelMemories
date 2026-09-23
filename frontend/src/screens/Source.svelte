<script lang="ts">
  import { onMount } from 'svelte'
  import { Backend, fmtBytes, errorText, reportError } from '../lib/api'
  import { sources, errorMessage, scanning, step, scanStats, scanSummary } from '../lib/stores'

  type Drive = { path: string; label: string; freeBytes: number; totalBytes: number; removable: boolean }

  let drives = $state<Drive[]>([])
  let recent = $state<string[]>([])

  async function loadDrives() {
    try {
      drives = (await Backend.ListDrives()) ?? []
    } catch (e) {
      reportError(e)
    }
  }

  onMount(async () => {
    await loadDrives()
    try {
      const st = await Backend.GetSettings()
      recent = (st.recentSources ?? []).slice(0, 6)
    } catch (e) {
      reportError(e)
    }
  })

  function add(path: string) {
    if (!path) return
    sources.update((s) => (s.includes(path) ? s : [...s, path]))
  }

  function remove(path: string) {
    sources.update((s) => s.filter((p) => p !== path))
  }

  async function pick() {
    try {
      add(await Backend.PickFolder('Выберите папку или диск с фото и видео'))
    } catch (e) {
      errorMessage.set(errorText(e))
    }
  }

  async function scan() {
    errorMessage.set('')
    scanStats.set(null)
    scanSummary.set(null)
    try {
      await Backend.StartScan($sources)
      scanning.set(true)
      step.set('scan')
    } catch (e) {
      errorMessage.set(errorText(e))
    }
  }
</script>

<h2>Откуда брать фото и видео</h2>
<p class="muted">Выберите один или несколько дисков или папок. Программа только читает их — ничего не удаляет и не изменяет.</p>

<div class="card section">
  <div class="row head">
    <h3>Диски</h3>
    <button class="link" onclick={loadDrives}>Обновить</button>
  </div>
  <div class="drives">
    {#each drives as d}
      <button class="drive" class:selected={$sources.includes(d.path)} onclick={() => add(d.path)}>
        <div class="dlabel">{d.removable ? '💽' : '🖴'} {d.label}</div>
        <div class="path muted">{d.path}</div>
        <div class="muted small">свободно {fmtBytes(d.freeBytes)} из {fmtBytes(d.totalBytes)}</div>
      </button>
    {/each}
  </div>
  <div class="row" style="margin-top:12px">
    <button onclick={pick}>📁 Добавить папку…</button>
    {#each recent.filter((r) => !$sources.includes(r)) as r}
      <button class="link path" title="Недавний источник" onclick={() => add(r)}>{r}</button>
    {/each}
  </div>
</div>

<div class="card section">
  <h3>Выбрано</h3>
  {#if $sources.length === 0}
    <p class="muted">Пока ничего не выбрано</p>
  {:else}
    {#each $sources as s}
      <div class="row selrow">
        <span class="path grow">{s}</span>
        <button class="link" onclick={() => remove(s)}>Убрать</button>
      </div>
    {/each}
  {/if}
</div>

<div class="row">
  <button class="primary" disabled={$sources.length === 0} onclick={scan}>🔍 Сканировать</button>
</div>

<style>
  .section {
    margin-bottom: 16px;
  }
  .head {
    justify-content: space-between;
  }
  .drives {
    display: grid;
    gap: 10px;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  }
  .drive {
    text-align: left;
    background: var(--surface-2);
    padding: 10px 12px;
  }
  .drive.selected {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent) inset;
  }
  .dlabel {
    font-weight: 600;
    margin-bottom: 2px;
  }
  .small {
    font-size: 12px;
    margin-top: 4px;
  }
  .selrow {
    padding: 4px 0;
    border-bottom: 1px solid var(--border);
  }
  .grow {
    flex: 1;
  }
</style>

<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from '../components/Icon.svelte'
  import { Backend, fmtBytes, errorText, reportError } from '../lib/api'
  import { tr } from '../lib/i18n'
  import { sources, errorMessage, scanning, step, scanStats, scanSummary, report, lastRequest } from '../lib/stores'

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
      recent = (st.recentSources ?? []).slice(0, 3)
    } catch (e) {
      reportError(e)
    }
  })

  /** Short label for a recent path: its last two segments. */
  function shortPath(p: string): string {
    const parts = p.split(/[\\/]/).filter(Boolean)
    return parts.length > 2 ? '…/' + parts.slice(-2).join('/') : p
  }

  async function clearRecent() {
    console.log('[FIX] recent sources: clearing', recent)
    try {
      const st = await Backend.GetSettings()
      st.recentSources = []
      await Backend.SaveSettings(st)
      recent = []
    } catch (e) {
      console.error('[FIX] recent sources: clear failed', e)
      reportError(e)
    }
  }

  function add(path: string) {
    if (!path) return
    sources.update((s) => (s.includes(path) ? s : [...s, path]))
  }

  function remove(path: string) {
    sources.update((s) => s.filter((p) => p !== path))
  }

  async function pick() {
    try {
      add(await Backend.PickFolder(tr('Выберите папку или диск с фото и видео', 'Choose a folder or disk with photos and videos')))
    } catch (e) {
      errorMessage.set(errorText(e))
    }
  }

  async function scan() {
    errorMessage.set('')
    scanStats.set(null)
    scanSummary.set(null)
    // A new scan replaces the backend's scan: an old dry-run report must not
    // offer "Archive now" for a plan that no longer exists.
    report.set(null)
    lastRequest.set(null)
    try {
      await Backend.StartScan($sources)
      scanning.set(true)
      step.set('scan')
    } catch (e) {
      errorMessage.set(errorText(e))
    }
  }
</script>

<h2>{tr('Откуда брать фото и видео', 'Where to take photos and videos from')}</h2>
<p class="muted">{tr('Выберите один или несколько дисков или папок. Программа только читает их — ничего не удаляет и не изменяет.', 'Choose one or more disks or folders. The app only reads them — nothing is deleted or changed.')}</p>

<div class="card section">
  <div class="row head">
    <h3>{tr('Диски', 'Disks')}</h3>
    <button class="link" onclick={loadDrives}><Icon name="refresh" />{tr('Обновить', 'Refresh')}</button>
  </div>
  <div class="drives">
    {#each drives as d}
      <button class="drive" class:selected={$sources.includes(d.path)} onclick={() => add(d.path)}>
        <div class="dlabel"><Icon name={d.removable ? 'usb-drive' : 'hard-drive'} /><span>{d.label}</span></div>
        <div class="path muted">{d.path}</div>
        <div class="muted small">{tr(`свободно ${fmtBytes(d.freeBytes)} из ${fmtBytes(d.totalBytes)}`, `${fmtBytes(d.freeBytes)} free of ${fmtBytes(d.totalBytes)}`)}</div>
      </button>
    {/each}
  </div>
  <div class="addrow">
    <button onclick={pick}><Icon name="folder" />{tr('Добавить папку…', 'Add folder…')}</button>
    {#if recent.some((r) => !$sources.includes(r))}
      <span class="recent-label muted" title={tr('Недавние источники', 'Recent sources')}><Icon name="history" /></span>
      <!-- One line only: chips that do not fit wrap onto a hidden second line. -->
      <div class="recent">
        {#each recent.filter((r) => !$sources.includes(r)) as r}
          <button class="chip path" title={r} onclick={() => add(r)}>{shortPath(r)}</button>
        {/each}
      </div>
      <button class="icon-only" title={tr('Очистить недавние', 'Clear recent')} aria-label={tr('Очистить недавние', 'Clear recent')} onclick={clearRecent}>
        <Icon name="x" />
      </button>
    {/if}
  </div>
</div>

<div class="card section">
  <h3>{tr('Выбрано', 'Selected')}</h3>
  {#if $sources.length === 0}
    <p class="muted">{tr('Пока ничего не выбрано', 'Nothing selected yet')}</p>
  {:else}
    {#each $sources as s}
      <div class="row selrow">
        <span class="path grow">{s}</span>
        <button class="link" onclick={() => remove(s)}>{tr('Убрать', 'Remove')}</button>
      </div>
    {/each}
  {/if}
</div>

<div class="row">
  <button class="primary" disabled={$sources.length === 0} onclick={scan}><Icon name="search" />{tr('Сканировать', 'Scan')}</button>
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
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }
  .dlabel span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .addrow {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 12px;
    min-width: 0;
  }
  .addrow > button {
    flex: none;
  }
  .recent-label {
    display: inline-flex;
    flex: none;
  }
  .recent {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    height: 30px;
    overflow: hidden;
  }
  .chip {
    height: 30px;
    max-width: 100%;
    padding: 0 10px;
    border-radius: 999px;
    background: none;
    color: var(--accent);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    word-break: normal;
  }
  .icon-only {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    padding: 0;
    background: none;
    border-color: transparent;
    color: var(--muted);
  }
  .icon-only:hover:not(:disabled) {
    color: var(--text);
    background: var(--surface-2);
    filter: none;
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

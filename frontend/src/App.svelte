<script lang="ts">
  import logo from './assets/logo.png'
  import { onMount } from 'svelte'
  import Source from './screens/Source.svelte'
  import ScanResult from './screens/ScanResult.svelte'
  import Destination from './screens/Destination.svelte'
  import Progress from './screens/Progress.svelte'
  import Report from './screens/Report.svelte'
  import ConfirmDialog from './components/ConfirmDialog.svelte'
  import AboutDialog from './components/AboutDialog.svelte'
  import Icon from './components/Icon.svelte'
  import { toggleTheme } from './lib/actions'
  import { tr } from './lib/i18n'
  import { Backend, EventsOn, type Progress as P, type Report as R, type ScanStats, type ScanSummary, type ClassifierStatus } from './lib/api'
  import { sources, step, scanning, scanStats, scanSummary, archiving, progress, report, classifier, errorMessage, theme, type Step } from './lib/stores'

  let confirmClose = $state(false)
  let aboutOpen = $state(false)
  let destinationScreen = $state<ReturnType<typeof Destination> | null>(null)

  const steps: { id: Step; title: string; hint: string }[] = [
    { id: 'source', title: tr('Источник', 'Source'), hint: tr('диски и папки', 'disks and folders') },
    { id: 'scan', title: tr('Сканирование', 'Scan'), hint: tr('что найдено', 'what was found') },
    { id: 'destination', title: tr('Назначение', 'Destination'), hint: tr('куда и как', 'where and how') },
    { id: 'progress', title: tr('Архивация', 'Archiving'), hint: tr('копирование', 'copying') },
    { id: 'report', title: tr('Отчёт', 'Report'), hint: tr('итоги', 'summary') },
  ]

  onMount(() => {

    const offs = [
      EventsOn('scan:progress', (s: ScanStats) => scanStats.set(s)),
      EventsOn('scan:done', (s: ScanSummary) => {
        scanning.set(false)
        scanSummary.set(s)
      }),
      EventsOn('archive:progress', (p: P) => progress.set(p)),
      EventsOn('archive:done', (r: R) => {
        archiving.set(false)
        report.set(r)
        // A finished real run is done with these folders: the next run must not
        // silently include them again (they stay one click away in "recent").
        if (!r.dryRun) {
          console.log('[FIX] archive done: clearing selected sources')
          sources.set([])
        }
        step.set('report')
      }),
      EventsOn('archive:error', (msg: string) => {
        archiving.set(false)
        errorMessage.set(msg)
        step.set('destination')
      }),
      EventsOn('classifier:ready', (s: ClassifierStatus) => classifier.set(s)),
      EventsOn('reindex:done', () => destinationScreen?.onReindexDone()),
      EventsOn('app:confirm-close', () => (confirmClose = true)),
    ]
    return () => offs.forEach((off) => off())
  })

  // Steps the user may jump to right now (no navigation while busy).
  function enabled(id: Step): boolean {
    if ($archiving) return id === 'progress'
    if ($scanning) return id === 'scan'
    switch (id) {
      case 'source':
        return true
      case 'scan':
        return !!$scanSummary
      case 'destination':
        return !!$scanSummary
      case 'progress':
        return false
      case 'report':
        return !!$report
    }
  }
</script>

<div class="layout">
  <aside>
    <div class="brand">
      <img class="logo" src={logo} alt="" aria-hidden="true" />
      <div>
        <div class="name">BelMemories</div>
        <div class="tagline">{tr('архив фото и видео', 'photo & video archive')}</div>
      </div>
    </div>
    <nav>
      {#each steps as s, i}
        <button class="step" class:active={$step === s.id} disabled={!enabled(s.id)} onclick={() => step.set(s.id)}>
          <span class="num">{i + 1}</span>
          <span class="txt"><span>{s.title}</span><span class="hint">{s.hint}</span></span>
        </button>
      {/each}
    </nav>
  </aside>

  <div class="toolbar">
    <button
      class="icon-btn"
      onclick={toggleTheme}
      title={$theme === 'dark' ? tr('Светлая тема', 'Light theme') : tr('Тёмная тема', 'Dark theme')}
      aria-label={$theme === 'dark' ? tr('Включить светлую тему', 'Switch to light theme') : tr('Включить тёмную тему', 'Switch to dark theme')}
    >
      <Icon name={$theme === 'dark' ? 'sun' : 'moon'} size={18} />
    </button>
    <button class="icon-btn" onclick={() => (aboutOpen = true)} title={tr('О программе', 'About')} aria-label={tr('О программе', 'About')}>
      <Icon name="info" size={18} />
    </button>
  </div>

  <main>
    {#if $errorMessage}
      <div class="alert danger err row">
        <span>{$errorMessage}</span>
        <button class="link" onclick={() => errorMessage.set('')} aria-label={tr('Скрыть', 'Dismiss')}><Icon name="x" /></button>
      </div>
    {/if}
    {#if $step === 'source'}
      <Source />
    {:else if $step === 'scan'}
      <ScanResult />
    {:else if $step === 'destination'}
      <Destination bind:this={destinationScreen} />
    {:else if $step === 'progress'}
      <Progress />
    {:else if $step === 'report'}
      <Report />
    {/if}
  </main>
</div>

<ConfirmDialog
  open={confirmClose}
  title={tr('Идёт архивация', 'Archiving in progress')}
  text={tr(
    'Закрыть программу? Архивация будет остановлена, текущий файл корректно отменён. Уже скопированные файлы сохранятся.',
    'Close the app? Archiving will stop and the current file will be safely cancelled. Files already copied are kept.',
  )}
  confirmLabel={tr('Остановить и закрыть', 'Stop and close')}
  danger
  onconfirm={() => {
    confirmClose = false
    Backend.ConfirmClose()
  }}
  oncancel={() => (confirmClose = false)}
/>

<AboutDialog open={aboutOpen} onclose={() => (aboutOpen = false)} />

<style>
  .layout {
    display: grid;
    grid-template-columns: 220px 1fr;
    height: 100%;
  }
  aside {
    background: var(--surface);
    border-right: 1px solid var(--border);
    padding: 18px 12px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .brand {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 0 6px 4px;
  }
  .logo {
    width: 36px;
    height: 36px;
    flex: none;
    display: block;
  }
  .name {
    font-weight: 700;
    font-size: 16px;
  }
  .tagline {
    font-size: 12px;
    color: var(--muted);
  }
  nav {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .step {
    text-align: left;
    background: none;
    border: 1px solid transparent;
    display: flex;
    gap: 10px;
    align-items: center;
  }
  .step.active {
    background: var(--accent-soft);
    border-color: var(--border);
  }
  .step.active .txt > span:first-child {
    font-weight: 600;
  }
  .txt {
    display: flex;
    flex-direction: column;
    line-height: 1.25;
  }
  .hint {
    font-size: 11.5px;
    color: var(--muted);
  }
  .num {
    display: inline-grid;
    place-items: center;
    width: 22px;
    height: 22px;
    border-radius: 50%;
    background: var(--surface-2);
    font-size: 12px;
  }
  .step.active .num {
    background: var(--accent);
    color: var(--accent-text);
  }
  .toolbar {
    position: fixed;
    top: 14px;
    right: 22px;
    z-index: 10;
    display: flex;
    gap: 4px;
  }
  .icon-btn {
    display: grid;
    place-items: center;
    width: 34px;
    height: 34px;
    padding: 0;
    background: none;
    border-color: transparent;
    color: var(--muted);
  }
  .icon-btn:hover:not(:disabled) {
    color: var(--text);
    background: var(--surface-2);
    filter: none;
  }
  main {
    padding: 24px 28px;
    overflow: auto;
  }
  /* Keep the page title and error line clear of the top-right toolbar. */
  main > :global(h2),
  .err {
    margin-right: 76px;
  }
  .err {
    justify-content: space-between;
    margin-bottom: 16px;
  }
  @media (max-width: 760px) {
    .layout {
      grid-template-columns: 1fr;
      grid-template-rows: auto 1fr;
    }
    aside {
      border-right: none;
      border-bottom: 1px solid var(--border);
    }
    nav {
      flex-direction: row;
      flex-wrap: wrap;
    }
    .hint {
      display: none;
    }
  }
</style>

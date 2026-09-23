<script lang="ts">
  import { onMount } from 'svelte'
  import Source from './screens/Source.svelte'
  import ScanResult from './screens/ScanResult.svelte'
  import Destination from './screens/Destination.svelte'
  import Progress from './screens/Progress.svelte'
  import Report from './screens/Report.svelte'
  import ConfirmDialog from './components/ConfirmDialog.svelte'
  import { Backend, EventsOn, type Progress as P, type Report as R, type ScanStats, type ScanSummary, type ClassifierStatus } from './lib/api'
  import { step, scanning, scanStats, scanSummary, archiving, progress, report, classifier, errorMessage, type Step } from './lib/stores'

  let confirmClose = $state(false)
  let destinationScreen = $state<ReturnType<typeof Destination> | null>(null)

  const steps: { id: Step; title: string }[] = [
    { id: 'source', title: 'Источник' },
    { id: 'scan', title: 'Сканирование' },
    { id: 'destination', title: 'Назначение' },
    { id: 'progress', title: 'Архивация' },
    { id: 'report', title: 'Отчёт' },
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
    <div class="brand">🗂 MemoryArchive</div>
    <nav>
      {#each steps as s, i}
        <button class="step" class:active={$step === s.id} disabled={!enabled(s.id)} onclick={() => step.set(s.id)}>
          <span class="num">{i + 1}</span>{s.title}
        </button>
      {/each}
    </nav>
    <div class="foot muted">Архив фото и видео по годам</div>
  </aside>

  <main>
    {#if $errorMessage}
      <div class="alert danger err row">
        <span>{$errorMessage}</span>
        <button class="link" onclick={() => errorMessage.set('')}>✕</button>
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
  title="Идёт архивация"
  text="Закрыть программу? Архивация будет остановлена, текущий файл корректно отменён. Уже скопированные файлы сохранятся."
  confirmLabel="Остановить и закрыть"
  danger
  onconfirm={() => {
    confirmClose = false
    Backend.ConfirmClose()
  }}
  oncancel={() => (confirmClose = false)}
/>

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
    font-weight: 700;
    font-size: 16px;
    padding: 0 8px;
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
    background: var(--surface-2);
    border-color: var(--border);
    font-weight: 600;
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
  .foot {
    margin-top: auto;
    font-size: 12px;
    padding: 0 8px;
  }
  main {
    padding: 24px 28px;
    overflow: auto;
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
    .foot {
      display: none;
    }
  }
</style>

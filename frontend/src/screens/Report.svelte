<script lang="ts">
  import Icon from '../components/Icon.svelte'
  import StatCard from '../components/StatCard.svelte'
  import ResultTable from '../components/ResultTable.svelte'
  import { Backend, fmtBytes, fmtNum, fmtDuration, errorText, type FileResult } from '../lib/api'
  import { report, step, errorMessage, lastRequest, archiving } from '../lib/stores'
  import { startArchive, newArchive } from '../lib/actions'
  import { tr, plural } from '../lib/i18n'

  let r = $derived($report)
  type Tab = 'renamed' | 'errors' | 'noDate' | 'mtime' | 'planned'
  let tab = $state<Tab>('renamed')

  const statusText: Record<string, string> = {
    completed: tr('Готово', 'Done'),
    cancelled: tr('Остановлено пользователем', 'Stopped by user'),
    disk_error: tr('Остановлено: проблема с диском назначения', 'Stopped: problem with the destination disk'),
    error: tr('Ошибка', 'Error'),
  }

  function rowsFor(t: Tab): FileResult[] {
    if (!r) return []
    switch (t) {
      case 'renamed':
        return r.renamed ?? []
      case 'errors':
        return r.errors ?? []
      case 'noDate':
        return r.noDate ?? []
      case 'mtime':
        return r.mtimeDates ?? []
      case 'planned':
        return r.planned ?? []
    }
  }

  // After a successful dry run the same plan can be executed right away.
  let canRunForReal = $derived(!!r && r.dryRun && r.status === 'completed' && !!$lastRequest && !$archiving)

  function runForReal() {
    if (!$lastRequest || !r) return
    startArchive({ ...$lastRequest, root: r.root, dryRun: false })
  }

  async function open(path: string) {
    try {
      await Backend.OpenPath(path)
    } catch (e) {
      errorMessage.set(errorText(e))
    }
  }
</script>

{#if r}
  <h2>{r.dryRun ? tr('План (пробный запуск)', 'Plan (dry run)') : statusText[r.status] ?? r.status}</h2>
  {#if r.message}<div class="alert danger section">{r.message}</div>{/if}

  <div class="grid section">
    <StatCard label={r.dryRun ? tr('Будет скопировано', 'Will be copied') : tr('Скопировано', 'Copied')} value={fmtNum(r.stats.copied)} tone="ok" hint={fmtBytes(r.stats.writtenBytes)} />
    <StatCard label={tr('Фото / Видео / Картинки', 'Photos / Videos / Pictures')} value="{fmtNum(r.stats.photos)} / {fmtNum(r.stats.videos)} / {fmtNum(r.stats.pictures)}" />
    <StatCard label={tr('Дубли пропущены', 'Duplicates skipped')} value={fmtNum(r.stats.duplicates)} />
    <StatCard label={tr('Переименовано', 'Renamed')} value={fmtNum(r.stats.renamed)} />
    <StatCard label={tr('Без даты', 'No date')} value={fmtNum(r.stats.noDate)} />
    <StatCard label={tr('Ошибки', 'Errors')} value={fmtNum(r.stats.errors)} tone={r.stats.errors > 0 ? 'danger' : ''} />
  </div>
  <p class="muted">{tr('Время:', 'Time:')} {fmtDuration(r.stats.elapsedSeconds)}</p>

  {#if canRunForReal}
    <div class="card cta section">
      <div>
        <h3>{tr('Всё устраивает?', 'Looks good?')}</h3>
        {#if r.stats.copied > 0}
          <p class="muted">
            {tr('Запустить архивацию по этому плану:', 'Run archiving with this plan:')} {fmtNum(r.stats.copied)}
            {plural(r.stats.copied, ['файл', 'файла', 'файлов'], ['file', 'files'])} ({fmtBytes(r.stats.writtenBytes)})
            {tr('в', 'to')}
            <span class="path">{r.root}</span>. {tr('Сканировать заново не нужно.', 'No need to scan again.')}
          </p>
        {:else}
          <p class="muted">{tr('Копировать нечего: все файлы уже есть в архиве.', 'Nothing to copy: all files are already in the archive.')}</p>
        {/if}
      </div>
      <button class="primary big" disabled={r.stats.copied === 0} onclick={runForReal}><Icon name="play" />{tr('Архивировать сейчас', 'Archive now')}</button>
    </div>
  {/if}

  <div class="row section">
    {#if !r.dryRun}<button onclick={() => open(r.root)}><Icon name="folder-open" />{tr('Открыть архив', 'Open archive')}</button>{/if}
    {#if r.csvPath}<button onclick={() => open(r.csvPath)}><Icon name="file-text" />{tr('Полный отчёт (CSV)', 'Full report (CSV)')}</button>{/if}
  </div>

  <div class="card section">
    <div class="row tabs">
      {#if r.dryRun}
        <button class:active={tab === 'planned'} onclick={() => (tab = 'planned')}>{tr('План', 'Plan')} ({rowsFor('planned').length})</button>
      {/if}
      <button class:active={tab === 'renamed'} onclick={() => (tab = 'renamed')}>{tr('Переименованные', 'Renamed')} ({rowsFor('renamed').length})</button>
      <button class:active={tab === 'errors'} onclick={() => (tab = 'errors')}>{tr('Ошибки', 'Errors')} ({rowsFor('errors').length})</button>
      <button class:active={tab === 'noDate'} onclick={() => (tab = 'noDate')}>{tr('Без даты', 'No date')} ({rowsFor('noDate').length})</button>
      <button class:active={tab === 'mtime'} onclick={() => (tab = 'mtime')}>{tr('Дата по файлу', 'Date from file')} ({rowsFor('mtime').length})</button>
    </div>
    {#if tab === 'mtime'}
      <p class="muted small">
        {tr(
          'Год определён по дате изменения файла — он может быть неточным.',
          'The year was taken from the file modification date — it may be inaccurate.',
        )}
      </p>
    {/if}
    <ResultTable rows={rowsFor(tab)} mode={tab === 'errors' ? 'err' : tab === 'mtime' || tab === 'noDate' ? 'date' : 'dst'} />
  </div>

  <div class="row">
    <button onclick={() => step.set('destination')}><Icon name="arrow-left" />{tr('К настройкам', 'Back to settings')}</button>
    <button class="primary" onclick={newArchive}>{tr('Новая архивация', 'New archive')}</button>
  </div>
{/if}

<style>
  .section {
    margin-bottom: 16px;
  }
  .tabs {
    margin-bottom: 12px;
    gap: 6px;
  }
  .tabs button.active {
    border-color: var(--accent);
    color: var(--accent);
  }
  .small {
    font-size: 12.5px;
  }
  .cta {
    display: flex;
    gap: 16px;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    border-color: var(--accent);
    background: var(--accent-soft);
  }
  .cta h3 {
    margin-bottom: 4px;
  }
  .cta p {
    margin: 0;
  }
  .big {
    padding: 10px 20px;
    font-size: 15px;
  }
</style>

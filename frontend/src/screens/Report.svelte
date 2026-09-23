<script lang="ts">
  import StatCard from '../components/StatCard.svelte'
  import ResultTable from '../components/ResultTable.svelte'
  import { Backend, fmtBytes, fmtNum, fmtDuration, errorText, type FileResult } from '../lib/api'
  import { report, step, errorMessage } from '../lib/stores'

  let r = $derived($report)
  type Tab = 'renamed' | 'errors' | 'noDate' | 'mtime' | 'planned'
  let tab = $state<Tab>('renamed')

  const statusText: Record<string, string> = {
    completed: 'Готово',
    cancelled: 'Остановлено пользователем',
    disk_error: 'Остановлено: проблема с диском назначения',
    error: 'Ошибка',
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

  async function open(path: string) {
    try {
      await Backend.OpenPath(path)
    } catch (e) {
      errorMessage.set(errorText(e))
    }
  }
</script>

{#if r}
  <h2>{r.dryRun ? 'План (пробный запуск)' : statusText[r.status] ?? r.status}</h2>
  {#if r.message}<div class="alert danger section">{r.message}</div>{/if}
  {#if r.missingArchives?.length}
    <div class="alert warn section">Не подключены другие архивы: {r.missingArchives.join(', ')}</div>
  {/if}

  <div class="grid section">
    <StatCard label={r.dryRun ? 'Будет скопировано' : 'Скопировано'} value={fmtNum(r.stats.copied)} tone="ok" hint={fmtBytes(r.stats.writtenBytes)} />
    <StatCard label="Фото / Видео / Картинки" value="{fmtNum(r.stats.photos)} / {fmtNum(r.stats.videos)} / {fmtNum(r.stats.pictures)}" />
    <StatCard label="Дубли пропущены" value={fmtNum(r.stats.duplicates)} />
    <StatCard label="Переименовано" value={fmtNum(r.stats.renamed)} />
    <StatCard label="Без даты" value={fmtNum(r.stats.noDate)} />
    <StatCard label="Ошибки" value={fmtNum(r.stats.errors)} tone={r.stats.errors > 0 ? 'danger' : ''} />
  </div>
  <p class="muted">Время: {fmtDuration(r.stats.elapsedSeconds)}</p>

  <div class="row section">
    {#if !r.dryRun}<button onclick={() => open(r.root)}>📂 Открыть архив</button>{/if}
    {#if r.csvPath}<button onclick={() => open(r.csvPath)}>📄 Полный отчёт (CSV)</button>{/if}
  </div>

  <div class="card section">
    <div class="row tabs">
      {#if r.dryRun}
        <button class:active={tab === 'planned'} onclick={() => (tab = 'planned')}>План ({rowsFor('planned').length})</button>
      {/if}
      <button class:active={tab === 'renamed'} onclick={() => (tab = 'renamed')}>Переименованные ({rowsFor('renamed').length})</button>
      <button class:active={tab === 'errors'} onclick={() => (tab = 'errors')}>Ошибки ({rowsFor('errors').length})</button>
      <button class:active={tab === 'noDate'} onclick={() => (tab = 'noDate')}>Без даты ({rowsFor('noDate').length})</button>
      <button class:active={tab === 'mtime'} onclick={() => (tab = 'mtime')}>Дата по файлу ({rowsFor('mtime').length})</button>
    </div>
    {#if tab === 'mtime'}
      <p class="muted small">Год определён по дате изменения файла — он может быть неточным.</p>
    {/if}
    <ResultTable rows={rowsFor(tab)} mode={tab === 'errors' ? 'err' : tab === 'mtime' || tab === 'noDate' ? 'date' : 'dst'} />
  </div>

  <div class="row">
    <button onclick={() => step.set('destination')}>← К настройкам</button>
    <button class="primary" onclick={() => step.set('source')}>Новая архивация</button>
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
</style>

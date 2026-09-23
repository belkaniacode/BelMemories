<script lang="ts">
  import Icon from '../components/Icon.svelte'
  import { onMount } from 'svelte'
  import { Backend, fmtBytes, fmtNum, errorText, reportError, type LoadLevel } from '../lib/api'
  import { destination, archiving, step, errorMessage, classifier } from '../lib/stores'
  import { startArchive } from '../lib/actions'
  import { tr, plural } from '../lib/i18n'

  type DestInfo = Awaited<ReturnType<typeof Backend.GetDestinationInfo>>

  let info = $state<DestInfo | null>(null)
  let loading = $state(false)
  let load = $state<LoadLevel>('medium')
  let verify = $state(true)
  let dryRun = $state(false)
  let reindexing = $state(false)

  onMount(async () => {
    try {
      const st = await Backend.GetSettings()
      load = (st.load as LoadLevel) || 'medium'
      verify = st.verifyAfterCopy
      if (!$destination && st.lastDestination) destination.set(st.lastDestination)
      if ($destination) await refresh()
      classifier.set(await Backend.ClassifierStatus())
    } catch (e) {
      reportError(e)
    }
  })

  async function refresh() {
    if (!$destination) return
    loading = true
    try {
      info = await Backend.GetDestinationInfo($destination)
    } catch (e) {
      errorMessage.set(errorText(e))
    } finally {
      loading = false
    }
  }

  async function pick() {
    const dir = await Backend.PickFolder(tr('Куда сохранять архив', 'Where to save the archive'))
    if (dir) {
      destination.set(dir)
      await refresh()
    }
  }

  async function reindex() {
    reindexing = true
    try {
      await Backend.Reindex($destination)
    } catch (e) {
      reindexing = false
      errorMessage.set(errorText(e))
    }
  }

  // Called by App when reindex:done arrives.
  export function onReindexDone() {
    reindexing = false
    refresh()
  }

  function start() {
    return startArchive({ root: $destination, dryRun, verify, load })
  }

  let canStart = $derived(!!info && info.exists && info.enough && !info.insideSource && !reindexing)
</script>

<h2>{tr('Куда сохранять', 'Where to save')}</h2>

<div class="card section">
  <div class="row">
    <button onclick={pick}><Icon name="folder" />{tr('Выбрать диск или папку…', 'Choose a drive or folder…')}</button>
    {#if $destination}<span class="path">{$destination}</span>{/if}
  </div>

  {#if loading}
    <p class="muted" style="margin-top:12px">{tr('Проверяю…', 'Checking…')}</p>
  {:else if info}
    <div class="facts">
      {#if info.error}
        <div class="alert danger">{info.error}</div>
      {:else}
        <div>{tr('Свободно на диске:', 'Free disk space:')} <b>{fmtBytes(info.freeBytes)}</b></div>
        {#if info.neededFiles > 0}
          <div>
            {tr('Будет скопировано: до', 'Will be copied: up to')} <b>{fmtNum(info.neededFiles)}</b>
            {plural(info.neededFiles, ['файла', 'файлов', 'файлов'], ['file', 'files'])}, <b>{fmtBytes(info.neededBytes)}</b>
            <span class="muted">{tr('— то, что уже есть в архиве, пропустится', '— files already in the archive will be skipped')}</span>
          </div>
        {:else}
          <div>
            {tr('Будет скопировано:', 'Will be copied:')} <b>{tr('ничего', 'nothing')}</b>
            <span class="muted">{tr('— все найденные файлы уже есть в архиве', '— all found files are already in the archive')}</span>
          </div>
        {/if}
        {#if info.stats.total.count > 0}
          <div>
            {tr('Уже в папке:', 'Already in the folder:')} <b>{fmtNum(info.stats.total.count)}</b>
            {plural(info.stats.total.count, ['файл', 'файла', 'файлов'], ['file', 'files'])} ({fmtBytes(info.stats.total.bytes)}){#if info.stats.years?.length},
              {tr('годы', 'years')} {info.stats.years[0]}–{info.stats.years[info.stats.years.length - 1]}{/if}
          </div>
        {:else}
          <div class="muted">
            {tr(
              'Папка пуста — «Фото», «Видео» и «Картинки» будут созданы автоматически.',
              'The folder is empty — «Фото» (Photos), «Видео» (Videos) and «Картинки» (Pictures) will be created automatically.',
            )}
          </div>
        {/if}
      {/if}
    </div>
    {#if info.exists && !info.enough}
      <div class="alert danger">
        {tr('Недостаточно места: нужно', 'Not enough space: need')} {fmtBytes(info.neededBytes)}
        {tr('и ещё', 'plus')} {fmtBytes(info.reserveBytes)}
        {tr(
          'запаса, чтобы диск не заполнился до конца. Освободите место или выберите другой диск.',
          'in reserve so the disk does not fill up completely. Free up space or choose another disk.',
        )}
      </div>
    {/if}
    {#if info.insideSource}
      <div class="alert danger">
        {tr(
          'Папка назначения находится внутри источника. Выберите другое место.',
          'The destination folder is inside the source. Choose another location.',
        )}
      </div>
    {/if}
    {#if info.needsReindex}
      <div class="alert warn row">
        <span>
          {tr(
            'В папке есть файлы архива, но нет индекса. Постройте индекс, чтобы не копировать дубли.',
            'The folder has archive files but no index. Build the index to avoid copying duplicates.',
          )}
        </span>
        <button onclick={reindex} disabled={reindexing}>{reindexing ? tr('Индексация…', 'Indexing…') : tr('Построить индекс', 'Build index')}</button>
      </div>
    {/if}
  {/if}
</div>

<div class="card section">
  <h3>{tr('Настройки', 'Settings')}</h3>
  <div class="row opt">
    <span>{tr('Нагрузка на компьютер:', 'Computer load:')}</span>
    <select bind:value={load}>
      <option value="low">{tr('Низкая — почти незаметно, медленнее', 'Low — barely noticeable, slower')}</option>
      <option value="medium">{tr('Средняя (рекомендуется)', 'Medium (recommended)')}</option>
      <option value="high">{tr('Высокая — максимально быстро', 'High — as fast as possible')}</option>
    </select>
  </div>
  <label class="row opt">
    <input type="checkbox" bind:checked={verify} />
    {tr('Проверять каждый файл после записи (надёжнее)', 'Verify each file after writing (safer)')}
  </label>
  <label class="row opt">
    <input type="checkbox" bind:checked={dryRun} />
    {tr('Пробный запуск — ничего не копировать, только показать план', 'Dry run — copy nothing, only show the plan')}
  </label>
  <div class="opt">
    {tr('Распознавание фото/картинок:', 'Photo/picture recognition:')}
    {#if $classifier?.clipAvailable}
      <span class="badge ok">{tr('нейросеть CLIP + правила', 'CLIP neural network + rules')}</span>
    {:else if $classifier}
      <span class="badge warn">{tr('только правила', 'rules only')}</span> <span class="muted">({$classifier.reason})</span>
    {:else}
      <span class="badge">{tr('загрузка…', 'loading…')}</span>
    {/if}
  </div>
</div>

<div class="row">
  <button onclick={() => step.set('scan')}><Icon name="arrow-left" />{tr('Назад', 'Back')}</button>
  <button class="primary" disabled={!canStart || $archiving} onclick={start}>
    {#if dryRun}<Icon name="eye" />{tr('Показать план', 'Show plan')}{:else}<Icon name="play" />{tr('Начать архивацию', 'Start archiving')}{/if}
  </button>
</div>

<style>
  .section {
    margin-bottom: 16px;
  }
  .facts {
    margin: 12px 0;
    display: grid;
    gap: 4px;
  }
  .alert {
    margin-top: 8px;
    justify-content: space-between;
  }
  .opt {
    margin: 8px 0;
  }
</style>

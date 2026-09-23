<script lang="ts">
  import { onMount } from 'svelte'
  import { Backend, fmtBytes, fmtNum, errorText, reportError, type LoadLevel } from '../lib/api'
  import { destination, archiving, step, progress, report, errorMessage, classifier } from '../lib/stores'

  type DestInfo = Awaited<ReturnType<typeof Backend.GetDestinationInfo>>

  let info = $state<DestInfo | null>(null)
  let loading = $state(false)
  let load = $state<LoadLevel>('medium')
  let verify = $state(true)
  let dryRun = $state(false)
  let others = $state<string[]>([])
  let reindexing = $state(false)

  onMount(async () => {
    try {
      const st = await Backend.GetSettings()
      load = (st.load as LoadLevel) || 'medium'
      verify = st.verifyAfterCopy
      others = st.otherArchives ?? []
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
    const dir = await Backend.PickFolder('Куда сохранять архив')
    if (dir) {
      destination.set(dir)
      await refresh()
    }
  }

  async function saveOthers(list: string[]) {
    others = list
    try {
      const st = await Backend.GetSettings()
      st.otherArchives = list
      await Backend.SaveSettings(st)
    } catch (e) {
      reportError(e)
    }
  }

  async function addOther() {
    const dir = await Backend.PickFolder('Другой архивный диск (для поиска дублей)')
    if (dir && !others.includes(dir) && dir !== $destination) await saveOthers([...others, dir])
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

  async function start() {
    errorMessage.set('')
    progress.set(null)
    report.set(null)
    try {
      await Backend.StartArchive({ root: $destination, dryRun, verify, load })
      archiving.set(true)
      step.set('progress')
    } catch (e) {
      errorMessage.set(errorText(e))
    }
  }

  let canStart = $derived(!!info && info.exists && info.enough && !info.insideSource && !reindexing)
</script>

<h2>Куда сохранять</h2>

<div class="card section">
  <div class="row">
    <button onclick={pick}>📁 Выбрать диск или папку…</button>
    {#if $destination}<span class="path">{$destination}</span>{/if}
  </div>

  {#if loading}
    <p class="muted" style="margin-top:12px">Проверяю…</p>
  {:else if info}
    <div class="facts">
      {#if info.error}
        <div class="alert danger">{info.error}</div>
      {:else}
        <div>Свободно: <b>{fmtBytes(info.freeBytes)}</b></div>
        <div>Нужно (оценка, без дублей): <b>{fmtBytes(info.neededBytes)}</b></div>
        {#if info.isArchive}
          <div>
            Существующий архив: <b>{fmtNum(info.stats.total.count)}</b> файлов ({fmtBytes(info.stats.total.bytes)})
            {#if info.stats.years?.length}, годы {info.stats.years[0]}–{info.stats.years[info.stats.years.length - 1]}{/if}
          </div>
        {:else}
          <div class="muted">Новый архив: папки «Фото», «Видео», «Картинки» будут созданы автоматически.</div>
        {/if}
      {/if}
    </div>
    {#if info.exists && !info.enough}
      <div class="alert danger">Недостаточно места на диске. Освободите место или выберите другой диск.</div>
    {/if}
    {#if info.insideSource}
      <div class="alert danger">Папка назначения находится внутри источника. Выберите другое место.</div>
    {/if}
    {#if info.needsReindex}
      <div class="alert warn row">
        <span>В папке есть файлы архива, но нет индекса. Постройте индекс, чтобы не копировать дубли.</span>
        <button onclick={reindex} disabled={reindexing}>{reindexing ? 'Индексация…' : 'Построить индекс'}</button>
      </div>
    {/if}
  {/if}
</div>

<div class="card section">
  <h3>Настройки</h3>
  <div class="row opt">
    <span>Нагрузка на компьютер:</span>
    <select bind:value={load}>
      <option value="low">Низкая — почти незаметно, медленнее</option>
      <option value="medium">Средняя (рекомендуется)</option>
      <option value="high">Высокая — максимально быстро</option>
    </select>
  </div>
  <label class="row opt"><input type="checkbox" bind:checked={verify} /> Проверять каждый файл после записи (надёжнее)</label>
  <label class="row opt"><input type="checkbox" bind:checked={dryRun} /> Пробный запуск — ничего не копировать, только показать план</label>
  <div class="opt">
    Распознавание фото/картинок:
    {#if $classifier?.clipAvailable}
      <span class="badge ok">нейросеть CLIP + правила</span>
    {:else if $classifier}
      <span class="badge warn">только правила</span> <span class="muted">({$classifier.reason})</span>
    {:else}
      <span class="badge">загрузка…</span>
    {/if}
  </div>
  <div class="opt">
    <div class="row">
      <span>Другие архивные диски (дубли с них тоже пропускаются):</span>
      <button class="link" onclick={addOther}>+ Добавить</button>
    </div>
    {#each others as o}
      <div class="row"><span class="path">{o}</span><button class="link" onclick={() => saveOthers(others.filter((x) => x !== o))}>Убрать</button></div>
    {/each}
  </div>
</div>

<div class="row">
  <button onclick={() => step.set('scan')}>← Назад</button>
  <button class="primary" disabled={!canStart || $archiving} onclick={start}>
    {dryRun ? '👁 Показать план' : '▶ Начать архивацию'}
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

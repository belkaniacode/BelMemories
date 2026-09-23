<script lang="ts">
  import Icon from '../components/Icon.svelte'
  import StatCard from '../components/StatCard.svelte'
  import ConfirmDialog from '../components/ConfirmDialog.svelte'
  import { Backend, fmtBytes, fmtNum, fmtDuration } from '../lib/api'
  import { tr, plural } from '../lib/i18n'
  import { progress, archiving } from '../lib/stores'

  let confirmStop = $state(false)
  let p = $derived($progress)
  let percent = $derived(p && p.total > 0 ? Math.min(100, (p.processed / p.total) * 100) : 0)
  let paused = $derived(p?.phase === 'paused')
</script>

<h2>{paused ? tr('Пауза', 'Paused') : $archiving ? tr('Архивация…', 'Archiving…') : tr('Завершение…', 'Finishing…')}</h2>

<div class="card section">
  <div class="row between">
    <b>{fmtNum(p?.processed ?? 0)} {tr('из', 'of')} {fmtNum(p?.total ?? 0)} {plural(p?.total ?? 0, ['файла', 'файлов', 'файлов'], ['file', 'files'])}</b>
    <span>{percent.toFixed(1)}%</span>
  </div>
  <div class="bar"><div style="width:{percent}%"></div></div>
  <div class="row between muted small">
    <span>{fmtBytes(p?.processedBytes ?? 0)} {tr('из', 'of')} {fmtBytes(p?.totalBytes ?? 0)}</span>
    <span>{fmtBytes(p?.bytesPerSec ?? 0)}{tr('/с', '/s')} · {tr(`осталось ≈ ${fmtDuration(p?.etaSeconds ?? 0)}`, `≈ ${fmtDuration(p?.etaSeconds ?? 0)} left`)} · {tr(`прошло ${fmtDuration(p?.elapsedSeconds ?? 0)}`, `${fmtDuration(p?.elapsedSeconds ?? 0)} elapsed`)}</span>
  </div>
  <div class="path muted current" title={p?.current}>{p?.current ?? ''}</div>
</div>

<div class="grid section">
  <StatCard
    label={tr('Скопировано', 'Copied')}
    value={fmtNum(p?.copied ?? 0)}
    tone="ok"
    hint="{fmtNum(p?.photos ?? 0)} {plural(p?.photos ?? 0, ['фото', 'фото', 'фото'], ['photo', 'photos'])} · {fmtNum(p?.videos ?? 0)} {plural(p?.videos ?? 0, ['видео', 'видео', 'видео'], ['video', 'videos'])} · {fmtNum(p?.pictures ?? 0)} {plural(p?.pictures ?? 0, ['картинка', 'картинки', 'картинок'], ['picture', 'pictures'])}"
  />
  <StatCard label={tr('Дубли (пропущены)', 'Duplicates (skipped)')} value={fmtNum(p?.duplicates ?? 0)} />
  <StatCard label={tr('Переименовано', 'Renamed')} value={fmtNum(p?.renamed ?? 0)} hint={tr('одинаковое имя, разное содержимое', 'same name, different content')} />
  <StatCard label={tr('Ошибки', 'Errors')} value={fmtNum(p?.errors ?? 0)} tone={(p?.errors ?? 0) > 0 ? 'danger' : ''} />
</div>

<div class="row">
  {#if paused}
    <button class="primary" onclick={() => Backend.ResumeArchive()}><Icon name="play" />{tr('Продолжить', 'Resume')}</button>
  {:else}
    <button onclick={() => Backend.PauseArchive()} disabled={!$archiving}><Icon name="pause" />{tr('Пауза', 'Pause')}</button>
  {/if}
  <button class="danger" onclick={() => (confirmStop = true)} disabled={!$archiving}><Icon name="stop" />{tr('Стоп', 'Stop')}</button>
</div>

<ConfirmDialog
  open={confirmStop}
  title={tr('Остановить архивацию?', 'Stop archiving?')}
  text={tr(
    'Текущий файл будет корректно отменён. Уже скопированные файлы останутся в архиве, при следующем запуске они не будут скопированы повторно.',
    'The current file will be cancelled safely. Files already copied stay in the archive and will not be copied again next time.'
  )}
  confirmLabel={tr('Остановить', 'Stop')}
  danger
  onconfirm={() => {
    confirmStop = false
    Backend.CancelArchive()
  }}
  oncancel={() => (confirmStop = false)}
/>

<style>
  .section {
    margin-bottom: 16px;
  }
  .between {
    justify-content: space-between;
    margin-bottom: 8px;
  }
  .small {
    font-size: 12.5px;
    margin-top: 8px;
  }
  .current {
    margin-top: 8px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    word-break: normal;
  }
</style>

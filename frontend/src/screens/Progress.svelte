<script lang="ts">
  import StatCard from '../components/StatCard.svelte'
  import ConfirmDialog from '../components/ConfirmDialog.svelte'
  import { Backend, fmtBytes, fmtNum, fmtDuration } from '../lib/api'
  import { progress, archiving } from '../lib/stores'

  let confirmStop = $state(false)
  let p = $derived($progress)
  let percent = $derived(p && p.total > 0 ? Math.min(100, (p.processed / p.total) * 100) : 0)
  let paused = $derived(p?.phase === 'paused')
</script>

<h2>{paused ? 'Пауза' : $archiving ? 'Архивация…' : 'Завершение…'}</h2>

<div class="card section">
  <div class="row between">
    <b>{fmtNum(p?.processed ?? 0)} из {fmtNum(p?.total ?? 0)} файлов</b>
    <span>{percent.toFixed(1)}%</span>
  </div>
  <div class="bar"><div style="width:{percent}%"></div></div>
  <div class="row between muted small">
    <span>{fmtBytes(p?.processedBytes ?? 0)} из {fmtBytes(p?.totalBytes ?? 0)}</span>
    <span>{fmtBytes(p?.bytesPerSec ?? 0)}/с · осталось ≈ {fmtDuration(p?.etaSeconds ?? 0)} · прошло {fmtDuration(p?.elapsedSeconds ?? 0)}</span>
  </div>
  <div class="path muted current" title={p?.current}>{p?.current ?? ''}</div>
</div>

<div class="grid section">
  <StatCard label="Скопировано" value={fmtNum(p?.copied ?? 0)} tone="ok" hint="{fmtNum(p?.photos ?? 0)} фото · {fmtNum(p?.videos ?? 0)} видео · {fmtNum(p?.pictures ?? 0)} картинок" />
  <StatCard label="Дубли (пропущены)" value={fmtNum(p?.duplicates ?? 0)} />
  <StatCard label="Переименовано" value={fmtNum(p?.renamed ?? 0)} hint="одинаковое имя, разное содержимое" />
  <StatCard label="Ошибки" value={fmtNum(p?.errors ?? 0)} tone={(p?.errors ?? 0) > 0 ? 'danger' : ''} />
</div>

<div class="row">
  {#if paused}
    <button class="primary" onclick={() => Backend.ResumeArchive()}>▶ Продолжить</button>
  {:else}
    <button onclick={() => Backend.PauseArchive()} disabled={!$archiving}>⏸ Пауза</button>
  {/if}
  <button class="danger" onclick={() => (confirmStop = true)} disabled={!$archiving}>■ Стоп</button>
</div>

<ConfirmDialog
  open={confirmStop}
  title="Остановить архивацию?"
  text="Текущий файл будет корректно отменён. Уже скопированные файлы останутся в архиве, при следующем запуске они не будут скопированы повторно."
  confirmLabel="Остановить"
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

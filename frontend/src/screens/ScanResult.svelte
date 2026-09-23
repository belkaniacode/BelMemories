<script lang="ts">
  import StatCard from '../components/StatCard.svelte'
  import { Backend, fmtBytes, fmtNum, fmtDuration } from '../lib/api'
  import { scanning, scanStats, scanSummary, step } from '../lib/stores'

  let stats = $derived($scanSummary?.stats ?? $scanStats)
  let total = $derived((stats?.photos ?? 0) + (stats?.videos ?? 0))
  let showErrors = $state(false)
</script>

<h2>{$scanning ? 'Сканирование…' : 'Результат сканирования'}</h2>

{#if $scanning}
  <p class="muted">Идёт поиск фото и видео. Папок просмотрено: {fmtNum(stats?.dirs ?? 0)}</p>
  <div class="bar indeterminate"><div></div></div>
{:else if $scanSummary?.cancelled}
  <div class="alert warn">Сканирование остановлено — показаны частичные результаты.</div>
{:else if $scanSummary}
  <p class="muted">Готово за {fmtDuration($scanSummary.seconds)}.</p>
{/if}

<div class="grid stats">
  <StatCard label="Фото" value={fmtNum(stats?.photos ?? 0)} hint={fmtBytes(stats?.photoBytes ?? 0)} />
  <StatCard label="Видео" value={fmtNum(stats?.videos ?? 0)} hint={fmtBytes(stats?.videoBytes ?? 0)} />
  <StatCard label="Другие файлы (пропущены)" value={fmtNum(stats?.skipped ?? 0)} />
  <StatCard
    label="Ошибки доступа"
    value={fmtNum(stats?.errors ?? 0)}
    tone={(stats?.errors ?? 0) > 0 ? 'warn' : ''}
  />
</div>

{#if !$scanning && ($scanSummary?.errorPaths?.length ?? 0) > 0}
  <button class="link" onclick={() => (showErrors = !showErrors)}>
    {showErrors ? 'Скрыть' : 'Показать'} недоступные пути
  </button>
  {#if showErrors}
    <div class="card errors">
      {#each $scanSummary?.errorPaths ?? [] as p}
        <div class="path">{p}</div>
      {/each}
    </div>
  {/if}
{/if}

<p class="muted note">
  На следующем шаге изображения будут разделены на «Фото» (люди, животные, жизнь) и «Картинки» (логотипы, иконки,
  скриншоты, мемы).
</p>

<div class="row">
  {#if $scanning}
    <button class="danger" onclick={() => Backend.CancelScan()}>Остановить</button>
  {:else}
    <button onclick={() => step.set('source')}>← Назад</button>
    <button class="primary" disabled={total === 0} onclick={() => step.set('destination')}>Далее: куда сохранять →</button>
  {/if}
</div>

<style>
  .stats {
    margin: 16px 0;
  }
  .errors {
    max-height: 200px;
    overflow: auto;
    margin: 8px 0 16px;
  }
  .note {
    margin: 12px 0 16px;
  }
  .indeterminate > div {
    width: 30%;
    animation: slide 1.2s ease-in-out infinite;
  }
  @keyframes slide {
    from {
      transform: translateX(-100%);
    }
    to {
      transform: translateX(350%);
    }
  }
</style>

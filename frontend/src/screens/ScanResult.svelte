<script lang="ts">
  import Icon from '../components/Icon.svelte'
  import StatCard from '../components/StatCard.svelte'
  import { Backend, fmtBytes, fmtNum, fmtDuration } from '../lib/api'
  import { tr } from '../lib/i18n'
  import { scanning, scanStats, scanSummary, step } from '../lib/stores'

  let stats = $derived($scanSummary?.stats ?? $scanStats)
  let total = $derived((stats?.photos ?? 0) + (stats?.videos ?? 0))
  let showErrors = $state(false)
</script>

<h2>{$scanning ? tr('Сканирование…', 'Scanning…') : tr('Результат сканирования', 'Scan results')}</h2>

{#if $scanning}
  <p class="muted">{tr('Идёт поиск фото и видео. Папок просмотрено:', 'Looking for photos and videos. Folders scanned:')} {fmtNum(stats?.dirs ?? 0)}</p>
  <div class="bar indeterminate"><div></div></div>
{:else if $scanSummary?.cancelled}
  <div class="alert warn">{tr('Сканирование остановлено — показаны частичные результаты.', 'Scan stopped — showing partial results.')}</div>
{:else if $scanSummary}
  <p class="muted">{tr(`Готово за ${fmtDuration($scanSummary.seconds)}.`, `Done in ${fmtDuration($scanSummary.seconds)}.`)}</p>
{/if}

<div class="grid stats">
  <StatCard label={tr('Фото', 'Photos')} value={fmtNum(stats?.photos ?? 0)} hint={fmtBytes(stats?.photoBytes ?? 0)} />
  <StatCard label={tr('Видео', 'Videos')} value={fmtNum(stats?.videos ?? 0)} hint={fmtBytes(stats?.videoBytes ?? 0)} />
  <StatCard label={tr('Другие файлы (пропущены)', 'Other files (skipped)')} value={fmtNum(stats?.skipped ?? 0)} />
  <StatCard
    label={tr('Ошибки доступа', 'Access errors')}
    value={fmtNum(stats?.errors ?? 0)}
    tone={(stats?.errors ?? 0) > 0 ? 'warn' : ''}
  />
</div>

{#if !$scanning && ($scanSummary?.errorPaths?.length ?? 0) > 0}
  <button class="link" onclick={() => (showErrors = !showErrors)}>
    {showErrors ? tr('Скрыть недоступные пути', 'Hide inaccessible paths') : tr('Показать недоступные пути', 'Show inaccessible paths')}
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
  {tr(
    'На следующем шаге изображения будут разделены на «Фото» (люди, животные, жизнь) и «Картинки» (логотипы, иконки, скриншоты, мемы).',
    'On the next step, images will be split into «Фото» (Photos: people, animals, life) and «Картинки» (Pictures: logos, icons, screenshots, memes).'
  )}
</p>

<div class="row">
  {#if $scanning}
    <button class="danger" onclick={() => Backend.CancelScan()}>{tr('Остановить', 'Stop')}</button>
  {:else}
    <button onclick={() => step.set('source')}><Icon name="arrow-left" />{tr('Назад', 'Back')}</button>
    <button class="primary" disabled={total === 0} onclick={() => step.set('destination')}>{tr('Далее: куда сохранять', 'Next: where to save')}<Icon name="arrow-right" /></button>
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

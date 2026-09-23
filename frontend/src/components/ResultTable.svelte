<script lang="ts">
  import type { FileResult } from '../lib/api'
  import { tr } from '../lib/i18n'

  let { rows, mode = 'dst' }: { rows: FileResult[]; mode?: 'dst' | 'err' | 'date' } = $props()

  const PAGE = 500
  let shown = $state(PAGE)
  let visible = $derived(rows.slice(0, shown))
</script>

{#if rows.length === 0}
  <p class="muted">{tr('Нет записей', 'No entries')}</p>
{:else}
  <div class="wrap">
    <table>
      <thead>
        <tr>
          <th>{tr('Исходный файл', 'Source file')}</th>
          {#if mode === 'err'}<th>{tr('Ошибка', 'Error')}</th>{:else}<th>{tr('В архиве', 'In archive')}</th>{/if}
          {#if mode === 'date'}<th>{tr('Год', 'Year')}</th>{/if}
        </tr>
      </thead>
      <tbody>
        {#each visible as r}
          <tr>
            <td class="path">{r.src}</td>
            {#if mode === 'err'}
              <td class="err">{r.err}</td>
            {:else}
              <td class="path">{r.dst ?? ''}</td>
            {/if}
            {#if mode === 'date'}<td>{r.year || '—'}</td>{/if}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  {#if rows.length > shown}
    <button class="link" onclick={() => (shown += PAGE)}>{tr('Показать ещё', 'Show more')} ({rows.length - shown})</button>
  {/if}
{/if}

<style>
  .wrap {
    max-height: 340px;
    overflow: auto;
  }
  .err {
    color: var(--danger);
  }
</style>

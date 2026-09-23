<script lang="ts">
  import type { FileResult } from '../lib/api'

  let { rows, mode = 'dst' }: { rows: FileResult[]; mode?: 'dst' | 'err' | 'date' } = $props()

  const PAGE = 500
  let shown = $state(PAGE)
  let visible = $derived(rows.slice(0, shown))
</script>

{#if rows.length === 0}
  <p class="muted">Нет записей</p>
{:else}
  <div class="wrap">
    <table>
      <thead>
        <tr>
          <th>Исходный файл</th>
          {#if mode === 'err'}<th>Ошибка</th>{:else}<th>В архиве</th>{/if}
          {#if mode === 'date'}<th>Год</th>{/if}
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
    <button class="link" onclick={() => (shown += PAGE)}>Показать ещё ({rows.length - shown})</button>
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

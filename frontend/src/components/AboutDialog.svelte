<script lang="ts">
  import logo from '../assets/logo.png'
  import Icon from './Icon.svelte'
  import { Backend, reportError } from '../lib/api'
  import { tr, getLang } from '../lib/i18n'

  type AppInfo = Awaited<ReturnType<typeof Backend.GetAppInfo>>

  let { open, onclose }: { open: boolean; onclose: () => void } = $props()

  let info = $state<AppInfo | null>(null)
  let tab = $state<'about' | 'help'>('about')

  $effect(() => {
    if (open && !info) {
      Backend.GetAppInfo()
        .then((i) => (info = i))
        .catch(reportError)
    }
  })

  const osName: Record<string, string> = { linux: 'Linux', windows: 'Windows', darwin: 'macOS' }

  function onkey(e: KeyboardEvent) {
    if (open && e.key === 'Escape') onclose()
  }
</script>

<svelte:window onkeydown={onkey} />

{#if open}
  <div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && onclose()}>
    <div class="card dialog" role="dialog" aria-modal="true" aria-label={tr('О программе', 'About')}>
      <div class="head">
        <img class="logo" src={logo} alt="" aria-hidden="true" />
        <div>
          <h2>BelMemories</h2>
          <div class="muted">{tr('Архиватор фото и видео', 'Photo and video archiver')} · {tr('версия', 'version')} {info?.version ?? '…'}</div>
        </div>
        <button class="link close" onclick={onclose} aria-label={tr('Закрыть', 'Close')}><Icon name="x" size={18} /></button>
      </div>

      <div class="row tabs">
        <button class:active={tab === 'about'} onclick={() => (tab = 'about')}>{tr('О программе', 'About')}</button>
        <button class:active={tab === 'help'} onclick={() => (tab = 'help')}>{tr('Как пользоваться', 'How to use')}</button>
      </div>

      {#if tab === 'about'}
        {#if getLang() === 'ru'}
        <p>
          BelMemories собирает фото и видео с дисков, папок и телефонных бэкапов в один аккуратный архив: раскладывает по
          папкам <b>Фото</b>, <b>Видео</b> и <b>Картинки</b>, а внутри — по годам съёмки. Одинаковые файлы
          распознаются по содержимому (хэшу) и не копируются повторно, а разные файлы с одним именем сохраняются под новым
          именем.
        </p>
        <p class="muted">
          Исходные файлы никогда не изменяются и не удаляются. Каждый файл записывается во временный, проверяется и только
          потом получает своё имя, поэтому сбой или отключение диска не повредит архив.
        </p>
        {:else}
        <p>
          BelMemories gathers photos and videos from disks, folders and phone backups into one tidy archive: it sorts them
          into the folders <b>Фото</b> (Photos), <b>Видео</b> (Videos) and <b>Картинки</b> (Pictures), and inside each
          one by the year they were taken. Identical files are recognized by their content (hash) and are not copied twice,
          while different files with the same name are saved under a new name.
        </p>
        <p class="muted">
          Source files are never changed or deleted. Each file is written to a temporary file, verified, and only then gets
          its real name, so a crash or a disconnected disk will not damage the archive.
        </p>
        {/if}

        <dl>
          <dt>{tr('Разработчик', 'Developer')}</dt>
          <dd>{info?.author ?? '…'}</dd>
          <dt>{tr('Почта', 'Email')}</dt>
          <dd>
            {#if info}<button class="link inline" onclick={() => Backend.OpenURL('mailto:' + info!.email)}>{info.email}</button>{/if}
          </dd>
          <dt>{tr('Исходный код', 'Source code')}</dt>
          <dd>
            {#if info}<button class="link inline" onclick={() => Backend.OpenURL(info!.repoUrl)}>{info.repoUrl.replace('https://', '')}</button>{/if}
          </dd>
          <dt>{tr('Система', 'System')}</dt>
          <dd>{info ? `${osName[info.os] ?? info.os} · ${info.arch}` : '…'}</dd>
          <dt>{tr('Распознавание', 'Recognition')}</dt>
          <dd>
            {#if info?.classifier.clipAvailable}
              <span class="badge ok">{tr('нейросеть CLIP + правила', 'CLIP neural network + rules')}</span>
            {:else if info}
              <span class="badge warn">{tr('только правила', 'rules only')}</span> <span class="muted">{info.classifier.reason}</span>
            {/if}
          </dd>
          <dt>{tr('Настройки', 'Settings')}</dt>
          <dd class="path">{info?.configDir ?? ''}</dd>
          <dt>{tr('Журнал', 'Log')}</dt>
          <dd>
            {#if info?.logPath}<button class="link inline path" onclick={() => Backend.OpenPath(info!.logPath).catch(reportError)}>{info.logPath}</button>{/if}
          </dd>
        </dl>
      {:else if getLang() === 'ru'}
        <ol class="help">
          <li><b>Источник.</b> Добавьте диск, папку или флешку, откуда брать файлы. Сканируется только то, что вы выбрали.</li>
          <li>
            <b>Сканирование.</b> Программа находит только фото и видео (остальные файлы игнорирует) и показывает, сколько их
            и какой объём.
          </li>
          <li>
            <b>Назначение.</b> Выберите диск или папку архива. Можно включить «Пробный запуск», чтобы увидеть план без
            копирования, — затем на экране плана нажать «Архивировать сейчас».
          </li>
          <li><b>Архивация.</b> Её можно поставить на паузу или остановить. Нагрузку на компьютер выбирают в настройках.</li>
          <li><b>Отчёт.</b> Что скопировано, какие файлы переименованы, какие без даты и где были ошибки.</li>
        </ol>
        <h3>Фото или картинка?</h3>
        <p class="muted">
          В «Фото» попадают снимки людей, животных, мест и событий. В «Картинки» — скриншоты, логотипы, иконки, мемы,
          документы и рисунки. Решают правила (данные камеры EXIF, размер, прозрачность) и нейросеть CLIP. Если на
          изображении есть люди, оно остаётся в «Фото», даже когда похоже на скриншот.
        </p>
        <h3>Структура архива</h3>
        <pre class="path tree">Архив/
  Фото/2023/…
  Видео/2023/…
  Картинки/2023/…
  Без даты/Фото/…</pre>
      {:else}
        <ol class="help">
          <li><b>Source.</b> Add a disk, folder or USB drive to take files from. Only what you choose is scanned.</li>
          <li>
            <b>Scan.</b> The app finds only photos and videos (other files are ignored) and shows how many there are and
            their total size.
          </li>
          <li>
            <b>Destination.</b> Choose the archive disk or folder. You can turn on “Dry run” to see the plan without
            copying, then press “Archive now” on the plan screen.
          </li>
          <li><b>Archiving.</b> It can be paused or stopped. The load on your computer is set in the settings.</li>
          <li><b>Report.</b> What was copied, which files were renamed, which have no date, and where errors occurred.</li>
        </ol>
        <h3>Photo or picture?</h3>
        <p class="muted">
          «Фото» (Photos) gets shots of people, animals, places and events. «Картинки» (Pictures) gets screenshots, logos,
          icons, memes, documents and drawings. The decision is made by rules (EXIF camera data, size, transparency) and
          the CLIP neural network. If an image contains people, it stays in «Фото», even when it looks like a screenshot.
        </p>
        <h3>Archive structure</h3>
        <pre class="path tree">Архив/              (Archive)
  Фото/2023/…       (Photos)
  Видео/2023/…      (Videos)
  Картинки/2023/…   (Pictures)
  Без даты/Фото/…   (No date)</pre>
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: grid;
    place-items: center;
    z-index: 20;
    padding: 16px;
  }
  .dialog {
    max-width: 620px;
    width: 100%;
    max-height: calc(100vh - 32px);
    overflow: auto;
    padding: 22px 24px;
  }
  .head {
    display: flex;
    gap: 14px;
    align-items: center;
    margin-bottom: 16px;
  }
  .head h2 {
    margin: 0;
  }
  .close {
    margin-left: auto;
    align-self: flex-start;
    font-size: 16px;
    color: var(--muted);
  }
  .logo {
    width: 48px;
    height: 48px;
    flex: none;
    display: block;
  }
  .tabs {
    gap: 6px;
    margin-bottom: 14px;
  }
  .tabs button.active {
    border-color: var(--accent);
    color: var(--accent);
  }
  dl {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 6px 16px;
    margin: 16px 0 0;
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
    min-width: 0;
  }
  .inline {
    padding: 0;
    text-align: left;
  }
  .help {
    padding-left: 20px;
    display: grid;
    gap: 8px;
    margin: 0 0 16px;
  }
  h3 {
    margin-top: 12px;
    margin-bottom: 6px;
  }
  .tree {
    background: var(--surface-2);
    border-radius: 8px;
    padding: 10px 12px;
    margin: 0;
  }
  @media (max-width: 520px) {
    dl {
      grid-template-columns: 1fr;
    }
    dt {
      margin-top: 6px;
    }
  }
</style>

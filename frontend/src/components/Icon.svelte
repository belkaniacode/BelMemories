<script lang="ts" module>
  export type IconName =
    | 'sun'
    | 'moon'
    | 'info'
    | 'x'
    | 'folder'
    | 'folder-open'
    | 'file-text'
    | 'search'
    | 'play'
    | 'pause'
    | 'stop'
    | 'eye'
    | 'arrow-left'
    | 'arrow-right'
    | 'hard-drive'
    | 'usb-drive'
    | 'refresh'
    | 'history'
</script>

<script lang="ts">
  // Monochrome line icons (Lucide-style, 24×24 grid) drawn with currentColor.
  let { name, size = 16 }: { name: IconName; size?: number } = $props()
</script>

<svg
  class="icon"
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="2"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  {#if name === 'sun'}
    <circle cx="12" cy="12" r="4" />
    <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
  {:else if name === 'moon'}
    <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
  {:else if name === 'info'}
    <circle cx="12" cy="12" r="10" />
    <path d="M12 16v-4M12 8h.01" />
  {:else if name === 'x'}
    <path d="M18 6 6 18M6 6l12 12" />
  {:else if name === 'folder'}
    <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
  {:else if name === 'folder-open'}
    <path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2" />
  {:else if name === 'file-text'}
    <path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z" />
    <path d="M14 2v4a2 2 0 0 0 2 2h4M16 13H8M16 17H8M10 9H8" />
  {:else if name === 'search'}
    <circle cx="11" cy="11" r="8" />
    <path d="m21 21-4.3-4.3" />
  {:else if name === 'play'}
    <path d="M6 3l14 9-14 9Z" />
  {:else if name === 'pause'}
    <rect x="6" y="4" width="4" height="16" rx="1" />
    <rect x="14" y="4" width="4" height="16" rx="1" />
  {:else if name === 'stop'}
    <rect x="5" y="5" width="14" height="14" rx="2" />
  {:else if name === 'eye'}
    <path d="M2.06 12.35a1 1 0 0 1 0-.7 10.75 10.75 0 0 1 19.88 0 1 1 0 0 1 0 .7 10.75 10.75 0 0 1-19.88 0" />
    <circle cx="12" cy="12" r="3" />
  {:else if name === 'arrow-left'}
    <path d="m12 19-7-7 7-7M19 12H5" />
  {:else if name === 'arrow-right'}
    <path d="M5 12h14M12 5l7 7-7 7" />
  {:else if name === 'hard-drive'}
    <path d="M22 12H2M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11ZM6 16h.01M10 16h.01" />
  {:else if name === 'usb-drive'}
    <rect x="7" y="9" width="10" height="13" rx="2" />
    <path d="M9 9V3h6v6M11 5.5h.01M13 5.5h.01" />
  {:else if name === 'refresh'}
    <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8M21 3v5h-5M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16M8 16H3v5" />
  {:else if name === 'history'}
    <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8M3 3v5h5M12 7v5l4 2" />
  {/if}
</svg>
